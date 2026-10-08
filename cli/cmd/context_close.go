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
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		terminal := closeTerminalStatus(doc.Type)
		if terminal == "" {
			exitWithError(cmd, fmt.Errorf("type %s cannot be closed", ctxKindLabel(doc.Type)))
			return
		}
		allow := getBoolFlag(cmd, "allow-unfinished", false)
		reason := strings.TrimSpace(getStringFlag(cmd, "reason", false))
		if !allow {
			if _, why := derivedCompletionBlock(doc.Path); why != "" {
				exitWithError(cmd, fmt.Errorf("cannot close %s: %s (finish the work, or pass --allow-unfinished --reason \"<why>\")", ctxKindLabel(doc.Type), why))
				return
			}
		} else if reason == "" {
			exitWithError(cmd, errors.New("--allow-unfinished requires --reason"))
			return
		}
		//#nosec G304 -- user work file
		data, err := os.ReadFile(doc.Path)
		exitWithError(cmd, err)
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
		//#nosec G306 -- user work file
		if err := os.WriteFile(doc.Path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, "ok\n")
		cascadeAfterWrite(cmd, doc.Path)
	},
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
