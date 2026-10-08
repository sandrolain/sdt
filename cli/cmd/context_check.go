package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxCheckPendingFlag is the --pending flag name (the item status constant is
// taskStatusTodo).
const ctxCheckPendingFlag = "pending"

// contextCheckCmd is the generalized checklist writer: it ticks any `- [ ]`
// item of any context/ document (task file, plan `## Phases` /
// `## Completion criteria`, questions checklist) addressed by its id anchor,
// falling back to the positional ordinal.

func contextCheckStatus(cmd *cobra.Command) (string, error) {
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
			status = c.status
			chosen++
		}
	}
	if chosen != 1 {
		return "", errors.New("pass exactly one of --done, --wip, --block or --pending")
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

// selectChecklistIDs returns the anchor ids of the items a batch selector
// matches: inside the `## Phase <phase>` section when phase is non-empty, whose
// text contains grep (case-insensitive) when grep is non-empty, or every item
// when all is true. Ids are read from content verbatim, so callers stamp first.

func selectChecklistIDs(content, phase, grep string, all bool) []string {
	lines := strings.Split(content, "\n")
	lo, hi := 0, len(lines)
	if phase != "" {
		h := phaseHeadingIndex(lines, phase)
		if h < 0 {
			return nil
		}
		lo = h + 1
		for i := lo; i < len(lines); i++ {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
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

// applyChecklistSelection stamps missing ids, then sets the status of every item
// the selector matches (--all / --phase <n> / --grep <text>). It returns the new
// content and the number of items updated; a missing selector or no match is an
// error so a batch tick never silently does nothing (F5).

func applyChecklistSelection(content, phase, grep string, all bool, status, reason string) (string, int, error) {
	if !all && phase == "" && strings.TrimSpace(grep) == "" {
		return content, 0, errors.New("pass an item id or a selector: --all, --phase <n>, --grep <text>")
	}
	stamped, _ := stampChecklistIDs(content)
	ids := selectChecklistIDs(stamped, phase, grep, all)
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
` + "`--phase <n>`" + ` ticks the items in that phase section, ` + "`--all`" + ` every
item, and ` + "`--grep <text>`" + ` the items whose text contains the substring.

Pass exactly one of --done, --wip, --block or --pending; --block takes an
optional --reason. The document's ` + "`updated`" + ` is refreshed when its kind
requires it.

Examples:
  sdt context check plan/<plan-file> --done c4
  sdt context check context/tasks/<task-file>.md --wip 2
  sdt context check plan/<plan-file> --done --phase 3
  sdt context check plan/<plan-file> --done --grep "F5"
  sdt context check questions/20260927-091146-... --block c3 --reason "needs data"`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDocPath(args[0])
		exitWithError(cmd, err)
		//#nosec G304 -- user work file
		data, err := os.ReadFile(doc.Path)
		exitWithError(cmd, err)
		content := string(data)
		if len(parseChecklistItems(content)) == 0 {
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
				updated, batchCount, err = applyChecklistSelection(content,
					getStringFlag(cmd, "phase", false), getStringFlag(cmd, "grep", false),
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
	contextCheckCmd.Flags().Bool(taskStatusDone, false, "Mark the item done")
	contextCheckCmd.Flags().Bool(taskStatusWip, false, "Mark the item in progress")
	contextCheckCmd.Flags().Bool(taskStatusBlock, false, "Mark the item blocked")
	contextCheckCmd.Flags().Bool(ctxCheckPendingFlag, false, "Reset the item to todo")
	contextCheckCmd.Flags().Bool("waive", false, "Record a structured waiver (keeps the item unfinished; requires --reason)")
	contextCheckCmd.Flags().String("reason", "", "Reason for --"+taskStatusBlock+" or --waive")
	contextCheckCmd.Flags().Bool("all", false, "Select every checklist item (batch)")
	contextCheckCmd.Flags().String("phase", "", "Select the items in the `## Phase <n>` section (batch)")
	contextCheckCmd.Flags().String("grep", "", "Select the items whose text contains this substring (batch)")
	addProvenanceFlags(contextCheckCmd)
	addCascadeFlag(contextCheckCmd)
}
