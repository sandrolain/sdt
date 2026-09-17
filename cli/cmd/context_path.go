package cmd

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type contextPathResult struct {
	Path string `json:"path" yaml:"path"`
	Type string `json:"type" yaml:"type"`
	Slug string `json:"slug,omitempty" yaml:"slug,omitempty"`
}

func outputContextPath(cmd *cobra.Command, res contextPathResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, res.Path+"\n")
	}
}

var contextPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path for a context/ work file",
	Long: `Print the full path of a context/ work file with the correct date/time
prefix. Does not create anything.

Types: plan/analysis/worklog/notes/archive (<YYYYMMDD-HHMMSS>-<slug>.md),
tasks (<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md with --phase <n> and
--plan), tmp (<slug>), architecture (<slug>.md),
decision (<NNNN>-<slug>.md with --number).

Examples:
  sdt context path --type worklog --slug review-deps
  sdt context path --type tasks --phase 1 --plan 20260911-155545-plan-context-file-formats-cli.md
  sdt context path --type plan --format json
  sdt context path --type decision --number 0001 --slug auth-choice`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		phase := getStringFlag(cmd, "phase", false)
		number := getStringFlag(cmd, "number", false)
		switch typ {
		case ctxTypeDecision:
			if number == "" {
				exitWithError(cmd, errors.New("--number is required for type decision (create with `sdt context new --type decision` to auto-assign)"))
			}
			if err := validateDecisionNumber(number); err != nil {
				exitWithError(cmd, err)
			}
			if slug == "" {
				exitWithError(cmd, errors.New("--slug is required for type decision"))
			}
			p := filepath.Join(sdtDecisionsDir, number+"-"+slug+".md")
			outputContextPath(cmd, contextPathResult{Path: p, Type: ctxTypeDecision, Slug: slug})
			return
		}
		p, err := contextPath(typ, slug, phase, getStringFlag(cmd, "plan", false))
		exitWithError(cmd, err)
		outputContextPath(cmd, contextPathResult{Path: p, Type: typ, Slug: slug})
	},
}

// ── context new (decision helpers) ───────────────────────────────────────────
