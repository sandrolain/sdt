package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// contextTouchCmd is the CLI-only path for a body-only edit: it refreshes the
// `updated` timestamp of a context/ document without touching anything else, so
// agents never hand-edit the field.

var contextTouchCmd = &cobra.Command{
	Use:   "touch <ref>",
	Short: "Refresh the `updated` timestamp of a context/ document",
	Long: `Refresh the ` + "`updated`" + ` frontmatter field of an existing context/
document, resolved by path or identity like ` + "`status get`" + `. It is the
CLI-only way to record a body edit once ` + "`updated`" + ` is no longer
hand-stamped; no other field, the body or the filename is changed.

Types without an ` + "`updated`" + ` field (notes, decision) are rejected.

Examples:
  sdt context touch context/plan/<plan-file>.md
  sdt context touch plan/<plan-file>
  sdt context touch --type analysis --slug backend --format json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		if !doc.Type.hasUpdated {
			exitWithError(cmd, fmt.Errorf("type %s has no `updated` field (touch unsupported)", ctxKindLabel(doc.Type)))
		}
		//#nosec G304 -- user work file
		data, err := os.ReadFile(doc.Path)
		exitWithError(cmd, err)
		now := contextNow().UTC().Format(time.RFC3339)
		content, changed := setFrontmatterFields(string(data), []frontmatterPatch{{key: statusUpdated, value: now}})
		if !changed {
			exitWithError(cmd, fmt.Errorf("no `updated` field written to %s (missing frontmatter)", doc.Path))
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(doc.Path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(ctxStatusResult{Path: doc.Path, Type: doc.Type.kind, Status: frontmatterField(content, ctxMapStatus), Updated: now}, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(ctxStatusResult{Path: doc.Path, Type: doc.Type.kind, Status: frontmatterField(content, ctxMapStatus), Updated: now})
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, now+"\n")
		}
	},
}

func init() {
	addContextStatusRefFlags(contextTouchCmd)
}
