package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/mdstruct"
	"github.com/spf13/cobra"
)

// ctxCheckPendingFlag is the --pending flag name (the item status constant is
// taskStatusTodo).
const ctxCheckPendingFlag = "pending"

// contextCheckCmd is the generalized checklist writer: it ticks any `- [ ]`
// item of any context/ document (task file, plan `## Phases` /
// `## Completion criteria`, questions checklist) addressed by its id anchor,
// falling back to the positional ordinal.

// ctxCheckStates is the closed set `--state` accepts: one flag for the four
// checklist moves, so a state is a value rather than a growing family of
// booleans. The old booleans stay as deprecated spellings.
var ctxCheckStates = []string{taskStatusDone, taskStatusWip, taskStatusBlock, taskStatusTodo, ctxCheckPendingFlag}

// ctxCheckStateAliases maps a documented word to the status it stores. The reset
// was documented as `pending` and stored as `todo`, so both spell the same move
// and neither is refused.
var ctxCheckStateAliases = map[string]string{ctxCheckPendingFlag: taskStatusTodo}

func contextCheckStatus(cmd *cobra.Command) (string, error) {
	state := strings.TrimSpace(getStringFlag(cmd, "state", false))
	if state != "" {
		if !slices.Contains(ctxCheckStates, state) {
			return "", fmt.Errorf("invalid --state %q (use %s)", state, strings.Join(ctxCheckStates, "|"))
		}
		if alias, ok := ctxCheckStateAliases[state]; ok {
			state = alias
		}
		for _, c := range []struct{ flag, status string }{
			{taskStatusDone, taskStatusDone},
			{taskStatusWip, taskStatusWip},
			{taskStatusBlock, taskStatusBlock},
			{ctxCheckPendingFlag, taskStatusTodo},
		} {
			if getBoolFlag(cmd, c.flag, false) {
				return "", errors.New("--state and the per-state flags are alternatives: pass one")
			}
		}
		if state == taskStatusBlock && strings.TrimSpace(getStringFlag(cmd, "reason", false)) == "" {
			// --block's reason requirement applies to --state block too.
			return "", errors.New("--state block requires --reason")
		}
		return state, nil
	}
	candidates := []struct{ flag, status string }{
		{taskStatusDone, taskStatusDone},
		{taskStatusWip, taskStatusWip},
		{taskStatusBlock, taskStatusBlock},
		{ctxCheckPendingFlag, taskStatusTodo},
	}
	status := ""
	chosen := 0
	for _, c := range candidates {
		if getBoolFlag(cmd, c.flag, false) {
			if f := cmd.Flags().Lookup(c.flag); f != nil && f.Changed {
				// The deprecated thing is the *flag*, not the command: a notice
				// naming the whole verb reads as "stop using check".
				deprecatedFlagNotice(cmd, c.flag, "--state "+c.status)
			}
			status = c.status
			chosen++
		}
	}
	if chosen != 1 {
		return "", errors.New("pass --state done|wip|block|pending")
	}
	return status, nil
}

func outputCheckResult(cmd *cobra.Command, item checklistItem) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(item, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(item)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, fmt.Sprintf("%s [%s] %s\n", item.ID, item.Marker, item.Text))
	}
}

// sectionHeadingIndex returns the line index of the heading a section selector
// names — the stable anchor (`phase-3`, `completion-criteria`) or the heading
// text (`Phase 3`, `Completion criteria`), case-insensitively — ignoring
// heading-like lines inside a fence. An exact heading wins over a heading that
// merely starts with the selector, so `Phase 2` addresses `### Phase 2 — Two`.
// It is the same rule `read --section` and `edit --section` resolve with, so the
// three cannot disagree.
func sectionHeadingIndex(lines []string, selector string) int {
	want := strings.ToLower(strings.TrimSpace(selector))
	if want == "" {
		return -1
	}
	wantID := strings.ToLower(mdstruct.HeadingAnchor(strings.TrimSpace(selector)))
	inside, _ := mdstruct.ScanFences(lines)
	prefix := -1
	for i, line := range lines {
		if inside[i] || headingLevel(line) == 0 {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		lower := strings.ToLower(text)
		if lower == want {
			return i
		}
		if wantID != "" && strings.EqualFold(mdstruct.HeadingAnchor(text), wantID) {
			return i
		}
		if prefix < 0 && (strings.HasPrefix(lower, want+" ") || strings.HasPrefix(lower, want+"\t")) {
			prefix = i
		}
	}
	return prefix
}

// selectChecklistIDs returns the anchor ids of the items a batch selector
// matches: inside the section named by section (which --phase composes as
// `Phase <n>`) when non-empty, whose text contains grep (case-insensitive) when
// grep is non-empty, or every item when all is true. Ids are read from content
// verbatim, so callers stamp first.
func selectChecklistIDs(content, section, grep string, all bool) []string {
	lines := strings.Split(content, "\n")
	lo, hi := 0, len(lines)
	if sel := strings.TrimSpace(section); sel != "" {
		h := sectionHeadingIndex(lines, sel)
		if h < 0 {
			return nil
		}
		lo = h + 1
		// A section is flat: it ends at the next heading of any level, the
		// boundary `read --section` already prints.
		inside, _ := mdstruct.ScanFences(lines)
		for i := lo; i < len(lines); i++ {
			if inside[i] {
				continue
			}
			if headingLevel(lines[i]) > 0 {
				hi = i
				break
			}
		}
	}
	want := strings.ToLower(strings.TrimSpace(grep))
	var ids []string
	for _, e := range parseChecklistEntries(lines) {
		if e.First < lo || e.First >= hi {
			continue
		}
		if want != "" && !strings.Contains(strings.ToLower(e.Body), want) {
			continue
		}
		if e.ID != "" {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// checkSectionSelector composes the section a batch addresses: --section as
// given, and --phase <n> as its shortcut for `Phase <n>`.
func checkSectionSelector(cmd *cobra.Command) string {
	if s := strings.TrimSpace(getStringFlag(cmd, "section", false)); s != "" {
		return s
	}
	if p := strings.TrimSpace(getStringFlag(cmd, "phase", false)); p != "" {
		return "Phase " + p
	}
	return ""
}

// checkSelectorConflict refuses two batch selectors at once: --section and
// --phase address the same lines, and --grep or --all beside either is a
// different selection, never a refinement.
func checkSelectorConflict(cmd *cobra.Command) error {
	var chosen []string
	if getBoolFlag(cmd, "all", false) {
		chosen = append(chosen, "--all")
	}
	if strings.TrimSpace(getStringFlag(cmd, "section", false)) != "" {
		chosen = append(chosen, "--section")
	}
	if strings.TrimSpace(getStringFlag(cmd, "phase", false)) != "" {
		chosen = append(chosen, "--phase")
	}
	if strings.TrimSpace(getStringFlag(cmd, "grep", false)) != "" {
		chosen = append(chosen, "--grep")
	}
	if len(chosen) > 1 {
		return fmt.Errorf("%s and %s are alternative selectors: pass one", chosen[0], chosen[1])
	}
	return nil
}

// applyChecklistSelection stamps missing ids, then sets the status of every item
// the selector matches (--all / --section <heading|id> / --grep <text>). It
// returns the new content and the number of items updated; a missing selector or
// no match is an error so a batch tick never silently does nothing (F5).

func applyChecklistSelection(content, section, grep string, all bool, status, reason string) (string, int, error) {
	if !all && strings.TrimSpace(section) == "" && strings.TrimSpace(grep) == "" {
		return content, 0, errors.New("pass an item id or a selector: --all, --section <heading|id>, --phase <n>, --grep <text>")
	}
	stamped, _ := stampChecklistIDs(content)
	ids := selectChecklistIDs(stamped, section, grep, all)
	if len(ids) == 0 {
		return stamped, 0, errors.New("no checklist item matches the selector")
	}
	out := stamped
	for _, id := range ids {
		next, err := updateChecklistItem(out, id, status, reason)
		if err != nil {
			return out, 0, err
		}
		out = next
	}
	return out, len(ids), nil
}

var contextCheckCmd = &cobra.Command{
	Use:   "check <doc-ref> [id]",
	Short: "Tick a checklist item of a context/ document by its id",
	Long: `Set the status of one checklist item of an existing context/ document
(plan, task file or questions), addressed by its id anchor ("c3" or "3") with
the positional ordinal as fallback. Un-anchored items are stamped on write.

Pass a batch selector instead of an id to update several items at once:
` + "`--section <heading|id>`" + ` ticks the items in that section, ` + "`--phase <n>`" + ` the
items in phase <n> (its shortcut for ` + "`--section \"Phase <n>\"`" + `), ` + "`--all`" + ` every
item, and ` + "`--grep <text>`" + ` the items whose text contains the substring. Pass one
selector: they are alternatives, not refinements.

Pass exactly one ` + "`--state`" + `: done, wip, block or pending. ` + "`--state block`" + ` takes an
` + "`--reason`" + `. The document's ` + "`updated`" + ` is refreshed when its kind requires it.
The per-state booleans (` + "`--done`" + `, ` + "`--wip`" + `, ` + "`--block`" + `, ` + "`--pending`" + `) still work but are
deprecated: they name a value where a flag should name a field.

Examples:
  sdt context check plan/<plan-file> c4 --state done
  sdt context check context/tasks/<task-file>.md 2 --state wip
  sdt context check plan/<plan-file> --state done --phase 3
  sdt context check plan/<plan-file> --state done --section "Validation"
  sdt context check plan/<plan-file> --state done --section completion-criteria
  sdt context check plan/<plan-file> --state done --grep "F5"
  sdt context check questions/20260927-091146-... c3 --state block --reason "needs data"`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDocPath(args[0])
		exitWithError(cmd, err)
		//#nosec G304 -- user work file
		data, err := os.ReadFile(doc.Path)
		exitWithError(cmd, err)
		content := string(data)
		if len(parseChecklistItems(content)) == 0 {
			// An open fence swallows every following line, so "no items" can be
			// the fence's fault rather than the document's: name the line.
			if open := mdstruct.UnclosedFence(content); open != nil {
				exitWithError(cmd, fmt.Errorf("%s: line %d opens a code fence (%s) that never closes, so every line after it reads as fence content — close the fence, then check the item", doc.Path, open.Line, open.Marker))
				return
			}
			exitWithError(cmd, fmt.Errorf("%s has no checklist items", doc.Path))
			return
		}
		reason := getStringFlag(cmd, "reason", false)
		updated := ""
		batchCount := 0
		if getBoolFlag(cmd, "waive", false) {
			if len(args) != 2 {
				exitWithError(cmd, errors.New("--waive needs a single <id>"))
				return
			}
			if strings.TrimSpace(reason) == "" {
				exitWithError(cmd, errors.New("--waive requires --reason"))
				return
			}
			updated, err = updateChecklistWaiver(content, args[1], reason)
		} else {
			status, serr := contextCheckStatus(cmd)
			exitWithError(cmd, serr)
			if serr != nil {
				return
			}
			if len(args) == 2 {
				updated, err = updateChecklistItem(content, args[1], status, reason)
			} else {
				if cerr := checkSelectorConflict(cmd); cerr != nil {
					exitWithError(cmd, cerr)
					return
				}
				updated, batchCount, err = applyChecklistSelection(content,
					checkSectionSelector(cmd), getStringFlag(cmd, "grep", false),
					getBoolFlag(cmd, "all", false), status, reason)
			}
		}
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		updated = stampProvenance(updated, cmd)
		if doc.Type.hasUpdated {
			if patched, changed := setFrontmatterFields(updated, []frontmatterPatch{{key: statusUpdated, value: contextNow().UTC().Format(time.RFC3339)}}); changed {
				updated = patched
			}
		}
		if ctxUIDEligibleKind(doc.Type.kind) {
			if stamped, ok := stampUIDMissing(updated); ok {
				updated = stamped
			}
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(doc.Path, []byte(updated), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		if len(args) == 2 {
			item, _ := resolveChecklistItem(updated, args[1])
			outputCheckResult(cmd, item)
		} else {
			outputString(cmd, fmt.Sprintf("%d item(s) updated\n", batchCount))
		}
		cascadeAfterWrite(cmd, doc.Path)
	},
}

// stampProvenance records agent/model/session in the frontmatter when the
// matching flags carry a value, so a state mutation says who performed it
// (F20). It is a no-op when none are given.

func stampProvenance(content string, cmd *cobra.Command) string {
	patches := make([]frontmatterPatch, 0, 3)
	for _, key := range []string{ctxFieldAgent, ctxFieldModel, ctxFieldSession} {
		if v := strings.TrimSpace(getStringFlag(cmd, key, false)); v != "" {
			patches = append(patches, frontmatterPatch{key: key, value: v})
		}
	}
	if len(patches) == 0 {
		return content
	}
	out, _ := setFrontmatterFields(content, patches)
	return out
}

// addProvenanceFlags registers the optional --agent/--model/--session flags a
// state-mutating command accepts to record who performed the mutation (F20).

func addProvenanceFlags(c *cobra.Command) {
	c.Flags().String(ctxFieldAgent, "", "Provenance: agent/tool that performed the mutation")
	c.Flags().String(ctxFieldModel, "", "Provenance: model id that performed the mutation")
	c.Flags().String(ctxFieldSession, "", "Provenance: session id for traceability")
}

func init() {
	contextCheckCmd.Flags().String("state", "", "New state of the selected item(s): done|wip|block|pending")
	contextCheckCmd.Flags().Bool(taskStatusDone, false, "Mark the item done (deprecated: --state done)")
	contextCheckCmd.Flags().Bool(taskStatusWip, false, "Mark the item in progress (deprecated: --state wip)")
	contextCheckCmd.Flags().Bool(taskStatusBlock, false, "Mark the item blocked (deprecated: --state block)")
	contextCheckCmd.Flags().Bool(ctxCheckPendingFlag, false, "Reset the item to todo (deprecated: --state pending)")
	contextCheckCmd.Flags().Bool("waive", false, "Record a structured waiver (keeps the item unfinished; requires --reason)")
	contextCheckCmd.Flags().String("reason", "", "Reason for --"+taskStatusBlock+" or --waive")
	contextCheckCmd.Flags().Bool("all", false, "Select every checklist item (batch)")
	contextCheckCmd.Flags().String("section", "", "Select the items in the section with this heading or stable id (batch)")
	contextCheckCmd.Flags().String("phase", "", "Select the items in the `## Phase <n>` section (batch; shortcut for --section \"Phase <n>\")")
	contextCheckCmd.Flags().String("grep", "", "Select the items whose text contains this substring (batch)")
	addProvenanceFlags(contextCheckCmd)
	addCascadeFlag(contextCheckCmd)
	// The per-state flags keep working but no longer appear in help: `--state`
	// is the one spelling, and a script using an old flag is told so on stderr
	// by contextCheckStatus.
	for _, flag := range []string{taskStatusDone, taskStatusWip, taskStatusBlock, ctxCheckPendingFlag} {
		if err := contextCheckCmd.Flags().MarkHidden(flag); err != nil {
			panic(err)
		}
	}
}
