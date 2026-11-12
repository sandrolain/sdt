package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── sdt context wiki glossary (read-only) ────────────────────────────────────
//
// The index of the definitions the corpus carries. A term is defined by a claim in
// its page's `## Terms` section (instructions/wiki.md, *Page contract*), so the
// glossary derives from the same parser the lint uses and can never disagree with it:
// the lint reports what an entry lacks, the glossary lists what it has.

// glossaryRow is one defined term.
type glossaryRow struct {
	Term   string `json:"term" yaml:"term"`
	Page   string `json:"page" yaml:"page"`
	Claim  string `json:"claim" yaml:"claim"`
	Anchor int    `json:"anchor" yaml:"anchor"`
	Digest string `json:"digest,omitempty" yaml:"digest,omitempty"`
	Span   string `json:"span,omitempty" yaml:"span,omitempty"`
}

var contextWikiGlossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "List the claim-backed terms the corpus defines (read-only)",
	Long: `List every term a page defines in its ` + "`## Terms`" + ` section, with the claim that
defines it, the claim's digest and the span the definition restates.

WHEN — you need to know whether the corpus defines a term, or which claim a definition
rests on, before writing material or citing one.
WHEN NOT — you want the page a term belongs to: open the page; the glossary is an
index, not a reading.

RETURNS — one row per entry: the term, its page, the claim id and anchor, the digest
that moves when the wording does, and the source span. An entry the lint would report
as malformed is left out rather than listed half-formed, so the glossary and
` + "`sdt context wiki lint`" + ` never disagree. Derived and read-only: it holds no file
and no status, so it cannot go stale.

With ` + "`--aliases`" + ` it answers the other question — which page a concept is called X
by another name — from the ` + "`aliases`" + ` frontmatter field. Same verb, same rows, same
read-only source; the default output is unchanged.

EXAMPLE — sdt context wiki glossary
          sdt context wiki glossary --aliases
          sdt context wiki glossary --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("load wiki: %w", err))
			return
		}
		if getBoolFlag(cmd, "aliases", false) {
			outputGlossaryAliases(cmd, glossaryAliasRowsOf(bld.Pages))
			return
		}
		outputGlossaryTerms(cmd, glossaryRowsOf(bld.Pages))
	},
}

// outputGlossaryTerms writes the term rows in the requested format.
func outputGlossaryTerms(cmd *cobra.Command, rows []glossaryRow) {
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
		outputString(cmd, "the corpus defines no term in a `## Terms` section yet\n")
		return
	}
	for _, row := range rows {
		line := fmt.Sprintf("%s — %s#%s", row.Term, row.Page, row.Claim)
		if row.Digest != "" {
			line += " (" + row.Digest + ")"
		}
		if row.Span != "" {
			line += " " + row.Span
		}
		outputString(cmd, line+"\n")
	}
}

// glossaryRowsOf collects one row per claim-backed term, sorted by term and page. An
// entry with anything missing is skipped: it is the lint's finding, not the index's
// row.
func glossaryRowsOf(pages []*contextwiki.Page) []glossaryRow {
	rows := make([]glossaryRow, 0, len(pages))
	for _, page := range pages {
		entries := parseTermEntries(page.Body)
		if len(entries) == 0 {
			continue
		}
		digests := map[string]string{}
		for _, claim := range page.Claims() {
			digests[claim.ID] = claim.Digest
		}
		for _, entry := range entries {
			if len(entry.Missing) > 0 || entry.ClaimID == "" || entry.Term == "" {
				continue
			}
			rows = append(rows, glossaryRow{
				Term:   entry.Term,
				Page:   page.ID,
				Claim:  entry.ClaimID,
				Anchor: entry.Anchor,
				Digest: digests[entry.ClaimID],
				Span:   entry.Span,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Term != rows[j].Term {
			return rows[i].Term < rows[j].Term
		}
		return rows[i].Page < rows[j].Page
	})
	return rows
}

func init() {
	contextWikiGlossaryCmd.Flags().Bool("aliases", false, "List the alias → page rows instead of the claim-backed terms")
	contextWikiCmd.AddCommand(contextWikiGlossaryCmd)
}
