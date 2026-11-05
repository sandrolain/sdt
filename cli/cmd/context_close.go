package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// contextCloseCmd closes a document in one step: it sets the terminal
// `completed` status, refuses when the derived state disagrees (unfinished
// children or an unfinished task) unless --allow-unfinished records an explicit
// override, cascades the completion to the parents, and records the crystallized
// closeout of a plan (or of one stream of a split plan) in the worklog. It is the
// close verb; `status set` stays the generic setter for every other value.

var contextCloseCmd = &cobra.Command{
	Use:   "close <doc-ref>",
	Short: "Close a document in one step (task → plan → analysis)",
	Long: `Set a document to its terminal ` + "`completed`" + ` status in one step and cascade the
completion to its parents. It refuses when the derived state disagrees
(unfinished children, or an unfinished task checklist) unless
` + "`--allow-unfinished`" + ` is given together with ` + "`--reason`" + `, which records the
override in the document body.

Closing a plan also writes its crystallized closeout in the worklog directory: the
checklist states, the ` + "`## Review`" + ` verdicts (findings included) and the gate result
are generated from the ledger, while the fields only the author knows can be
recorded in the same invocation with ` + "`--files`" + `, ` + "`--decisions`" + `, ` + "`--lessons`" + `,
` + "`--signal`" + ` and ` + "`--deviations`" + `, so the worklog needs no hand edit. A field left
out keeps its marker; a rerun regenerates the derived part and keeps what the
author wrote.

Supported kinds: tasks, plan, analysis.

Examples:
  sdt context close context/tasks/<task-file>.md
  sdt context close plan/<plan-file>
  sdt context close plan/<plan-file> --allow-unfinished --reason "shipping the remainder separately"`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		allow := getBoolFlag(cmd, "allow-unfinished", false)
		reason := strings.TrimSpace(getStringFlag(cmd, "reason", false))
		entry, err := closeDocument(cmd, doc, allow, reason, closeoutFieldFlags(cmd))
		exitWithError(cmd, err)
		outputString(cmd, "ok\n")
		if entry != "" {
			outputString(cmd, "closeout: "+entry+"\n")
			if slots := closeoutRemainingSlots(entry, closeoutSlug(doc.Path)); len(slots) > 0 {
				outputString(cmd, "closeout slots to fill: "+strings.Join(slots, ", ")+"\n")
			} else {
				outputString(cmd, "closeout slots to fill: none\n")
			}
		}
		if doc.Type.kind == ctxTypePlan {
			//#nosec G304 -- the document was resolved by the caller
			if data, readErr := os.ReadFile(doc.Path); readErr == nil {
				outputString(cmd, "self-assessment: "+planSelfAssessmentInvocation(doc.Path, string(data))+"\n")
			}
		}
		// The workflow ends here for the register too: the ids this document and the
		// chain it completed carry are marked done, and the ids that changed are named
		// so a reader sees the obligation settle.
		paths := []string{doc.Path}
		for _, change := range cascadeAfterWrite(cmd, doc.Path) {
			paths = append(paths, change.Path)
		}
		if closed := closeTodosFor(cmd, paths); len(closed) > 0 {
			outputString(cmd, "todos closed: "+strings.Join(closed, ", ")+"\n")
		}
	},
}

// planSelfAssessmentInvocation renders the `sdt context new` command that
// creates the mandatory per-plan self-assessment note (AGENTS.md, *Workflow
// self-assessment*). It is printed and never run: the note's judgement is
// authored, and the close only knows the plan's title, slug, objective and
// agent (answered 2026-11-05).
func planSelfAssessmentInvocation(path, content string) string {
	slug := planSlugFromPath(path)
	title := frontmatterScalar(content, "title")
	if title == "" {
		title = slug
	}
	args := []string{
		"sdt context new",
		"--type notes",
		"--title " + strconv.Quote(title+" — workflow self-assessment"),
		"--slug " + slug + "-workflow-self-assessment",
		"--note-type self-assessment",
	}
	if agent := frontmatterScalar(content, "agent"); agent != "" {
		args = append(args, "--agent "+agent)
	}
	if objective := frontmatterScalar(content, "objective"); objective != "" {
		args = append(args, "--objective "+objective)
	}
	args = append(args, "--source plan/"+strings.TrimSuffix(filepath.Base(path), ".md"), "--summary "+strconv.Quote("<one line>"))
	return strings.Join(args, " ")
}

// frontmatterScalar is frontmatterField without the YAML quoting: the frontmatter
// reader returns the raw token, so a quoted title would otherwise be quoted
// twice in the printed command.
func frontmatterScalar(content, key string) string {
	v := frontmatterField(content, key)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		if unq, err := strconv.Unquote(v); err == nil {
			return unq
		}
	}
	return v
}

// planSlugFromPath drops the dated prefix (`YYYYMMDD-HHMMSS-`) and the `.md`
// suffix, which is the slug a plan's note is named after.
func planSlugFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	if len(base) > 16 && base[8] == '-' && base[15] == '-' {
		return base[16:]
	}
	return base
}

// closeDocument writes a document's terminal status, recording an explicit
// override note when the derived state disagrees, then records the closeout when
// the document is a plan or a stream of one. It is the one implementation behind
// `context close` and `status set` with a terminal value and a `--reason`, so the
// two cannot drift.
func closeDocument(cmd *cobra.Command, doc ctxResolvedDoc, allow bool, reason string, closeout map[string]string) (string, error) {
	terminal := closeTerminalStatus(doc.Type)
	if terminal == "" {
		return "", fmt.Errorf("type %s cannot be closed", ctxKindLabel(doc.Type))
	}
	if !allow {
		if _, why := derivedCompletionBlock(doc.Path); why != "" {
			return "", fmt.Errorf("cannot close %s: %s (finish the work, or pass --reason \"<why>\")", ctxKindLabel(doc.Type), why)
		}
	} else if reason == "" {
		return "", errors.New("--reason is required when the derived state disagrees")
	}
	//#nosec G304 -- user work file
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		return "", err
	}
	content := string(data)
	if allow {
		content = appendCloseNote(content, reason)
	}
	content, _ = setFrontmatterFields(content, ctxStatusSetPatches(doc.Type, terminal))
	if ctxUIDEligibleKind(doc.Type.kind) {
		if stamped, ok := stampUIDMissing(content); ok {
			content = stamped
		}
	}
	content = stampProvenance(content, cmd)
	//#nosec G306 G703 -- path resolved from the corpus, not from user input
	if err := os.WriteFile(doc.Path, []byte(content), 0o644); err != nil {
		return "", err
	}
	entry, err := closeoutWrite(doc, closeout)
	if err != nil {
		// The status is already written: say the record failed rather than
		// pretending the close produced one.
		return "", fmt.Errorf("%s is closed, but the closeout could not be recorded: %w", ctxKindLabel(doc.Type), err)
	}
	return entry, nil
}

// closeTerminalStatus returns the terminal status `close` writes for a kind, or
// "" when the kind has no one-step close.

func closeTerminalStatus(t ctxDocType) string {
	switch t.kind {
	case ctxTypeTasks, ctxTypePlan, ctxTypeAnalysis:
		return taskFileStatusCompleted
	}
	return ""
}

// appendCloseNote records a `--allow-unfinished` override in the body so an
// early close is never silent.

func appendCloseNote(content, reason string) string {
	note := "> Closed with `--allow-unfinished` (" + contextNow().UTC().Format("2006-01-02") + "): " + reason
	return strings.TrimRight(content, "\n") + "\n\n" + note + "\n"
}
