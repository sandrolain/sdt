package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var contextTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Print the per-type instruction file for a context type",
	Long: `Print the content of context/instructions/<type>.md for one document
type from the registered set (` + ctxTypeHelpText(ctxTemplateTypes()) + `). Read-only:
the CLI never writes documents.

Examples:
  sdt context template --type decision
  sdt context template --type plan --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		path, err := contextInstrPath(typ)
		exitWithError(cmd, err)
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if os.IsNotExist(err) {
			exitWithError(cmd, fmt.Errorf("no instruction file at %s (types: %s)", path, ctxTypeHelpText(ctxTemplateTypes())))
		}
		exitWithError(cmd, err)
		switch getFormat(cmd) {
		case fmtJSON, fmtYAML:
			m := map[string]string{"type": typ, ctxMapPath: path, "content": string(data)}
			var out []byte
			var merr error
			if getFormat(cmd) == fmtJSON {
				out, merr = json.MarshalIndent(m, "", "  ")
			} else {
				out, merr = yaml.Marshal(m)
			}
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, string(data))
		}
	},
}

func contextInstrPath(typ string) (string, error) {
	t, ok := ctxTypeLookup(typ)
	if !ok || !t.templateAllowed || t.templateFile == "" {
		return "", fmt.Errorf("unknown type %q (use %s)", typ, ctxTypeHelpText(ctxTemplateTypes()))
	}
	return filepath.Join(sdtInstrDir, t.templateFile), nil
}
