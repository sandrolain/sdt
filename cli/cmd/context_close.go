package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// contextCloseCmd closes a document in one step: it sets the terminal
// `completed` status, refuses when the derived state disagrees (unfinished
// children or an unfinished task) unless --allow-unfinished records an explicit
// override, and cascades the completion to the parents. It collapses the
// multi-step closeout friction F6 into one command.

var contextCloseCmd = &cobra.Command{
	Use:   "close <doc-ref>",
	Short: "Close a document in one step (task → plan → analysis)",
	Long: `Set a document to its terminal ` + "`completed`" + ` status in one step and cascade the
completion to its parents. It refuses when the derived state disagrees
(unfinished children, or an unfinished task checklist) unless
` + "`--allow-unfinished`" + ` is given together with ` + "`--reason`" + `, which records the
override in the document body.

Supported kinds: tasks, plan, analysis.

Examples:
  sdt context close context/tasks/<task-file>.md
  sdt context close plan/<plan-file>
  sdt context close plan/<plan-file> --allow-unfinished --reason "shipping the remainder separately"`,
	Args: cobra.MaximumNArgs(1),
	// `close` is the old name for what `status set` now does with a terminal
	// value: one verb per job. It keeps working (an alias, not a removal) and
	// says which verb to use.
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		deprecatedNotice(cmd, "`sdt context status set <ref> --status completed --reason \"<why>\"`")
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		allow := getBoolFlag(cmd, "allow-unfinished", false)
		reason := strings.TrimSpace(getStringFlag(cmd, "reason", false))
		exitWithError(cmd, closeDocument(doc, allow, reason))
		outputString(cmd, "ok\n")
		cascadeAfterWrite(cmd, doc.Path)
	},
}

// closeDocument writes a document's terminal status, recording an explicit
// override note when the derived state disagrees. It is the one implementation
// behind `context close` (deprecated) and `status set` with a terminal value and
// a `--reason`, so the two cannot drift.
func closeDocument(doc ctxResolvedDoc, allow bool, reason string) error {
	terminal := closeTerminalStatus(doc.Type)
	if terminal == "" {
		return fmt.Errorf("type %s cannot be closed", ctxKindLabel(doc.Type))
	}
	if !allow {
		if _, why := derivedCompletionBlock(doc.Path); why != "" {
			return fmt.Errorf("cannot close %s: %s (finish the work, or pass --reason \"<why>\")", ctxKindLabel(doc.Type), why)
		}
	} else if reason == "" {
		return errors.New("--reason is required when the derived state disagrees")
	}
	//#nosec G304 -- user work file
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		return err
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
	//#nosec G306 G703 -- path resolved from the corpus, not from user input
	return os.WriteFile(doc.Path, []byte(content), 0o644)
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
