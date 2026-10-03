package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

var contextCheckCmd = &cobra.Command{
	Use:   "check <doc-ref> <id>",
	Short: "Tick a checklist item of a context/ document by its id",
	Long: `Set the status of one checklist item of an existing context/ document
(plan, task file or questions), addressed by its id anchor ("c3" or "3") with
the positional ordinal as fallback. Un-anchored items are stamped on write.

Pass exactly one of --done, --wip, --block or --pending; --block takes an
optional --reason. The document's ` + "`updated`" + ` is refreshed when its kind
requires it.

Examples:
  sdt context check plan/<plan-file> --done c4
  sdt context check context/tasks/<task-file>.md --wip 2
  sdt context check questions/20260927-091146-... --block c3 --reason "needs data"`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		status, err := contextCheckStatus(cmd)
		exitWithError(cmd, err)
		doc, err := resolveContextDocPath(args[0])
		exitWithError(cmd, err)
		//#nosec G304 -- user work file
		data, err := os.ReadFile(doc.Path)
		exitWithError(cmd, err)
		content := string(data)
		items := parseChecklistItems(content)
		if len(items) == 0 {
			exitWithError(cmd, fmt.Errorf("%s has no checklist items", doc.Path))
		}
		reason := ""
		if status == taskStatusBlock {
			reason = getStringFlag(cmd, "reason", false)
		}
		updated, err := updateChecklistItem(content, args[1], status, reason)
		exitWithError(cmd, err)
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
		item, _ := resolveChecklistItem(updated, args[1])
		outputCheckResult(cmd, item)
		cascadeAfterWrite(cmd, doc.Path)
	},
}

func init() {
	contextCheckCmd.Flags().Bool(taskStatusDone, false, "Mark the item done")
	contextCheckCmd.Flags().Bool(taskStatusWip, false, "Mark the item in progress")
	contextCheckCmd.Flags().Bool(taskStatusBlock, false, "Mark the item blocked")
	contextCheckCmd.Flags().Bool(ctxCheckPendingFlag, false, "Reset the item to todo")
	contextCheckCmd.Flags().String("reason", "", "Reason for --"+taskStatusBlock)
	addCascadeFlag(contextCheckCmd)
}
