package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// glossaryAliasRow is one of a concept's other names and the page that claims
// it. Path is the file the claim lives in, so a reader can open it directly.
type glossaryAliasRow struct {
	Alias string `json:"alias" yaml:"alias"`
	Page  string `json:"page" yaml:"page"`
	Path  string `json:"path" yaml:"path"`
}

// glossaryAliasRowsOf collects one row per alias, sorted by alias and page. A
// blank entry is skipped: it is the lint's finding, not the index's row, which is
// the same rule the term rows follow.
func glossaryAliasRowsOf(pages []*contextwiki.Page) []glossaryAliasRow {
	rows := make([]glossaryAliasRow, 0, len(pages))
	for _, page := range pages {
		for _, raw := range page.Aliases {
			alias := strings.TrimSpace(raw)
			if alias == "" {
				continue
			}
			rows = append(rows, glossaryAliasRow{
				Alias: alias,
				Page:  page.ID,
				Path:  filepath.ToSlash(filepath.Join(sdtWikiDir, page.FileID)) + ".md",
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := strings.ToLower(rows[i].Alias), strings.ToLower(rows[j].Alias)
		if a != b {
			return a < b
		}
		return rows[i].Page < rows[j].Page
	})
	return rows
}

// outputGlossaryAliases writes the alias rows in the requested format.
func outputGlossaryAliases(cmd *cobra.Command, rows []glossaryAliasRow) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, merr := json.MarshalIndent(rows, "", "  ")
		exitWithError(cmd, merr)
		outputBytes(cmd, out)
		return
	case fmtYAML:
		out, merr := yaml.Marshal(rows)
		exitWithError(cmd, merr)
		outputBytes(cmd, out)
		return
	}
	if len(rows) == 0 {
		outputString(cmd, "no page declares an `aliases` entry yet\n")
		return
	}
	for _, row := range rows {
		outputString(cmd, fmt.Sprintf("%s — %s (%s)\n", row.Alias, row.Page, row.Path))
	}
}
