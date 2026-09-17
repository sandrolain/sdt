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
type (analysis, plan, tasks, decision, architecture, worklog, notes, questions,
proposal, prompt, research). Read-only: the CLI never writes documents.

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
			exitWithError(cmd, fmt.Errorf("no instruction file at %s (types: analysis|plan|tasks|decision|architecture|worklog|notes|questions|proposal|prompt|research)", path))
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
	var name string
	switch typ {
	case ctxTypeAnalysis:
		name = "analysis.md"
	case ctxTypePlan:
		name = "plan.md"
	case ctxTypeTasks:
		name = "tasks.md"
	case ctxTypeDecision:
		name = "decision.md"
	case "architecture":
		name = "architecture.md"
	case ctxTypeWorklog:
		name = "worklog.md"
	case ctxTypeNotes:
		name = "notes.md"
	case ctxTypeQuestions:
		name = "questions.md"
	case ctxTypeProposal:
		name = "proposal.md"
	case ctxTypePrompt:
		name = "prompts.md"
	case ctxTypeResearch:
		name = "research.md"
	default:
		return "", fmt.Errorf("unknown type %q (use analysis|plan|tasks|decision|architecture|worklog|notes|questions|proposal|prompt|research)", typ)
	}
	return filepath.Join(sdtInstrDir, name), nil
}
