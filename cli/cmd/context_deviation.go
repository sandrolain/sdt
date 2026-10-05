package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// Deliberate deviations: when an agent diverges from an approved plan it records
// the divergence in a closed vocabulary instead of silently reinterpreting the
// plan. `sdt context task deviation add` writes the trace; `sdt context lint`
// reports one that was never closed, or a kind outside the vocabulary.
//
// The default kind is `stop-and-ask`: "when in doubt, take the heavier path, and
// never weaken an agreed plan mid-task without saying so" (AGENTS.md, No silent
// downgrades). A kind is only recorded when the caller states it.

// ctxDeviationsHeader is the task-file section that records deviations. It is
// created before `## Review`, so the review stays the last word on the file.
const ctxDeviationsHeader = "## Deviations"

// The deviation kinds, named once. They are deliberately *not* shared with the
// same-valued literals elsewhere (the `add` subcommand verb, the JSON Patch
// `add` op): equal spelling is not equal meaning, so each domain declares its
// own constant rather than coupling them.
const (
	ctxDeviationKindFix     = "fix"
	ctxDeviationKindAdd     = "add"
	ctxDeviationKindUnblock = "unblock"
	ctxDeviationKindStop    = "stop-and-ask"
)

// ctxDeviationDefaultKind is the kind used when the caller does not state one:
// the instruction is to ask the user rather than assume.
const ctxDeviationDefaultKind = ctxDeviationKindStop

// ctxDeviationSep separates the kind from the deviation text on an item line.
const ctxDeviationSep = " — "

// ctxDeviationKinds is the closed vocabulary of a deliberate deviation:
// `fix` the plan was wrong and the correction is made, `add` something had to be
// added, `unblock` a blocker was resolved, `stop-and-ask` the work halted for the
// user's decision. An unknown kind is refused at write time and reported by lint,
// so the vocabulary cannot drift by hand-editing.
var ctxDeviationKinds = []string{ctxDeviationKindFix, ctxDeviationKindAdd, ctxDeviationKindUnblock, ctxDeviationKindStop}

// ctxDeviationReasonRegexp extracts the optional `(reason: ...)` suffix.
var ctxDeviationReasonRegexp = regexp.MustCompile(`\s*\(reason:\s*(.*)\)\s*$`)

// validDeviationKind reports whether kind is in the closed vocabulary.
func validDeviationKind(kind string) bool {
	for _, k := range ctxDeviationKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// ctxDeviationKindHelp renders the closed vocabulary for help text and errors.
func ctxDeviationKindHelp() string {
	return strings.Join(ctxDeviationKinds, " | ")
}

// deviationItem is one recorded deviation.
type deviationItem struct {
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Kind   string `json:"kind" yaml:"kind"`
	Text   string `json:"text" yaml:"text"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Open   bool   `json:"open" yaml:"open"`
}

// parseDeviationBody splits an item body into its kind, text and optional reason.
// A body with no separator yields an empty kind, which lint reports as unknown.
func parseDeviationBody(body string) (kind, text, reason string) {
	body = strings.TrimSpace(body)
	if m := ctxDeviationReasonRegexp.FindStringSubmatch(body); m != nil {
		reason = strings.TrimSpace(m[1])
		body = strings.TrimSpace(ctxDeviationReasonRegexp.ReplaceAllString(body, ""))
	}
	k, rest, found := strings.Cut(body, ctxDeviationSep)
	if !found {
		return "", body, reason
	}
	return strings.TrimSpace(k), strings.TrimSpace(rest), reason
}

// parseDeviations reads the `## Deviations` section of a task file. A file
// without the section declares no deviations.
func parseDeviations(content string) []deviationItem {
	lines := strings.Split(content, "\n")
	at := indexOfTrimmedLine(lines, ctxDeviationsHeader, 0, len(lines))
	if at < 0 {
		return nil
	}
	end := sectionEnd(lines, at, 2)
	var items []deviationItem
	for _, e := range parseChecklistEntries(lines[at+1 : end]) {
		body, _ := splitChecklistAnchor(e.Body)
		kind, text, reason := parseDeviationBody(body)
		items = append(items, deviationItem{
			ID:     e.ID,
			Kind:   kind,
			Text:   text,
			Reason: reason,
			Open:   e.Marker != "x",
		})
	}
	return items
}

// appendDeviationItem records one deviation under the `## Deviations` section,
// creating the section immediately before `## Review` when absent. The item is
// open (`- [ ]`) until a phase closes it with `sdt context task done <id>`.
func appendDeviationItem(content, kind, text, reason string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	line := "- [ ] " + kind + ctxDeviationSep + text
	if r := strings.TrimSpace(reason); r != "" {
		line += " (reason: " + r + ")"
	}
	if at := indexOfTrimmedLine(lines, ctxDeviationsHeader, 0, len(lines)); at >= 0 {
		end := sectionAppendIndex(lines, sectionEnd(lines, at, 2))
		lines = insertAt(lines, end, line)
	} else {
		at := len(lines)
		if review := indexOfTrimmedLine(lines, ctxReviewBlockHeader, 0, len(lines)); review >= 0 {
			at = review
			for at > 0 && strings.TrimSpace(lines[at-1]) == "" {
				at--
			}
		}
		lines = insertAt(lines, at, "", ctxDeviationsHeader, "", line)
	}
	stamped, _ := stampChecklistIDs(strings.Join(lines, "\n") + "\n")
	return stamped
}

// lintDeviation reports two things about a task file's `## Deviations` section: a
// deviation whose kind is outside the closed vocabulary (WARNING, in any file —
// the vocabulary must not drift), and a deviation still open in a `completed`
// file (WARNING — the file claims to be finished while a divergence it recorded
// was never resolved). SUGGESTION-level noise is deliberately avoided: a deviation
// is a trace to read, not a defect to fix.
func lintDeviation(path, content string) []ctxLintIssue {
	var issues []ctxLintIssue
	completed := parseFrontmatterField(content, "status") == taskFileStatusCompleted
	for _, d := range parseDeviations(content) {
		if !validDeviationKind(d.Kind) {
			shown := d.Kind
			if shown == "" {
				shown = "(none)"
			}
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning,
				Message: fmt.Sprintf("deviation declares unknown kind %s; want %s", shown, ctxDeviationKindHelp())})
			continue
		}
		if completed && d.Open {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning,
				Message: fmt.Sprintf("completed task file carries an open deviation (%s): %s", d.Kind, truncateLintText(d.Text))})
		}
	}
	return issues
}

// contextDeviationCmd groups the deviation verbs under `context task`.
var contextDeviationCmd = &cobra.Command{
	Use:   "deviation",
	Short: "Record and list deliberate deviations from the plan",
}

var contextDeviationAddCmd = &cobra.Command{
	Use:   `add "<text>"`,
	Short: "Record a deliberate deviation from the plan",
	Long: `Record a deliberate deviation in the task file's ` + "`## Deviations`" + ` section,
in the closed vocabulary ` + ctxDeviationKindHelp() + `.

The default kind is ` + ctxDeviationDefaultKind + `: when the caller does not say why the
plan is being departed from, the recorded instruction is to ask the user. The
kind is never inferred.

With --id <checklist-id> the referenced phase item is marked rather than deleted:
` + "`[~]`" + ` for a deviation being worked through, ` + "`[!]`" + ` for one that stopped to ask.
The item keeps its text and its id.

Examples:
  sdt context task deviation add "phase 3 check moved to phase 5" --kind fix
  sdt context task deviation add "blocked on the user's answer" --id c14 --reason "waiting on the answer"
  sdt context task deviation list`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		text := strings.TrimSpace(args[0])
		if text == "" {
			exitWithError(cmd, fmt.Errorf("deviation text is empty; state what departed from the plan"))
			return
		}
		kind := strings.TrimSpace(getStringFlag(cmd, "kind", false))
		if kind == "" {
			kind = ctxDeviationDefaultKind
		}
		if !validDeviationKind(kind) {
			exitWithError(cmd, fmt.Errorf("unknown deviation kind %q; want %s", kind, ctxDeviationKindHelp()))
			return
		}
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		stream := taskStreamFlag(cmd)
		content, err := readTaskFile(phase, stream, plan)
		exitWithError(cmd, err)
		reason := getStringFlag(cmd, "reason", false)
		if id := strings.TrimSpace(getStringFlag(cmd, "id", false)); id != "" {
			// The linked item keeps its text; only its marker changes, so a
			// reviewer still sees what was planned next to why it moved.
			marker := taskStatusWip
			if kind == ctxDeviationDefaultKind {
				marker = taskStatusBlock
			}
			updated, err := updateChecklistItem(content, id, marker, reason)
			if err != nil {
				exitWithError(cmd, err)
				return
			}
			content = setTaskFileStatus(updated, taskFileNextStatus(marker, updated))
		}
		content = appendDeviationItem(content, kind, text, reason)
		path := taskFileForRef(phase, stream, plan)
		//#nosec G306 -- user work file
		if err := writeTaskFile(path, content); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, path+"\n")
		cascadeAfterWrite(cmd, path)
	},
}

var contextDeviationListCmd = &cobra.Command{
	Use:   useList,
	Short: "List the deviations recorded in the task file",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		content, err := readTaskFile(phase, taskStreamFlag(cmd), plan)
		exitWithError(cmd, err)
		items := parseDeviations(content)
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(items, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(items)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			if len(items) == 0 {
				outputString(cmd, "no deviations recorded\n")
				return
			}
			for _, d := range items {
				state := "open"
				if !d.Open {
					state = "closed"
				}
				line := fmt.Sprintf("%s. [%s] %s: %s", d.ID, state, d.Kind, d.Text)
				if d.Reason != "" {
					line += " — " + d.Reason
				}
				outputString(cmd, line+"\n")
			}
		}
	},
}

// writeTaskFile writes a task file after a mutation, normalizing the trailing
// newline so a write never churns the file's last line.
func writeTaskFile(path, content string) error {
	//#nosec G304 G306 -- path resolved from the corpus, not from user input
	return os.WriteFile(path, []byte(strings.TrimRight(content, "\n")+"\n"), 0o644)
}
