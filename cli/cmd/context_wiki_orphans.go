package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── orphan pages (sdt context wiki orphans) ──────────────────────────────────
//
// A page with no typed relation and no link to an index page is an orphan in
// the navigable graph. It is a SUGGESTION: the page may still be correct, but a
// reader cannot reach it from the source's index.

type orphanFinding struct {
	Page   string `json:"page" yaml:"page"`
	Title  string `json:"title" yaml:"title"`
	Reason string `json:"reason" yaml:"reason"`
}

var contextWikiOrphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "List concept pages with no typed relation and no index link",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("load wiki: %w", err))
			return
		}
		findings := orphanFindings(bld)
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(findings, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(findings)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			for _, f := range findings {
				outputString(cmd, fmt.Sprintf("[SUGGESTION] %s (%s): %s\n", f.Page, f.Title, f.Reason))
			}
		}
	},
}

func orphanFindings(bld *contextwiki.Builder) []orphanFinding {
	indexes := map[string]bool{}
	for _, page := range bld.Pages {
		if page.Type == wikiTypeEntity {
			indexes[page.ID] = true
		}
	}
	var out []orphanFinding
	for _, page := range bld.Pages {
		if page.Type != "concept" || len(page.Relations) > 0 {
			continue
		}
		linked := false
		for _, link := range page.Links {
			if indexes[link.Target] || link.Target == page.ID {
				linked = true
				break
			}
		}
		if !linked {
			out = append(out, orphanFinding{
				Page:   corpusRelative(page.Path),
				Title:  page.Title,
				Reason: "no typed relation and no index link",
			})
		}
	}
	return out
}

func init() {
	contextWikiCmd.AddCommand(contextWikiOrphansCmd)
}
