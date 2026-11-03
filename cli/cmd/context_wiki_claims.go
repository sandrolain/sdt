package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
)

// ── claim span/atomicity signal (sdt context wiki claims) ────────────────────
//
// A claim that cites forty lines or three units is a paraphrase, not an atomic
// assertion: the reader cannot tell which sentence rests on which passage. The
// check is a SUGGESTION so a legitimate multi-step claim stays visible without
// blocking the corpus.

type claimFinding struct {
	Page       string `json:"page" yaml:"page"`
	Line       int    `json:"line" yaml:"line"`
	Claim      string `json:"claim" yaml:"claim"`
	Ref        string `json:"ref" yaml:"ref"`
	Start      int    `json:"start" yaml:"start"`
	End        int    `json:"end" yaml:"end"`
	Lines      int    `json:"lines" yaml:"lines"`
	Units      int    `json:"units" yaml:"units"`
	Unresolved bool   `json:"unresolved,omitempty" yaml:"unresolved,omitempty"`
}

type claimBlock struct {
	ID   string
	Line int
	Text string
}

var contextWikiClaimsCmd = &cobra.Command{
	Use:   "claims [page...]",
	Short: "Flag broad claims that cite too many lines or units",
	Long: `Flag claims whose citation is too broad to be atomic.

A claim block starts at its {#claim-N} anchor and runs to the next anchor. For
each citation in the block the command counts the cited artifact lines and the
artifact units the span overlaps. A citation over --max-lines (default 40) or
over --max-units (default 2) is a SUGGESTION: the command names the page, claim
id, citation, line count and unit count, exits 0 and never edits a page.

Examples:
  sdt context wiki claims
  sdt context wiki claims architecture/deployment-pipeline --max-lines 80
  sdt context wiki claims --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if getBoolFlag(cmd, "budget", false) {
			runClaimBudget(cmd, args)
			return
		}
		if cmd.Flags().Changed("at") {
			exitWithError(cmd, fmt.Errorf("--at lowers the budget threshold and is only valid with --budget"))
			return
		}
		paths, err := claimPages(args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		maxLines := getIntFlag(cmd, "max-lines", false)
		maxUnits := getIntFlag(cmd, "max-units", false)
		if maxLines < 1 || maxUnits < 1 {
			exitWithError(cmd, fmt.Errorf("--max-lines and --max-units must be positive"))
			return
		}
		var findings []claimFinding
		cache := map[string]artifactSpan{}
		for _, path := range paths {
			pageFindings, ferr := claimFindings(path, maxLines, maxUnits, cache)
			if ferr != nil {
				exitWithError(cmd, ferr)
				return
			}
			findings = append(findings, pageFindings...)
		}
		outputClaimFindings(cmd, findings)
	},
}

func claimPages(args []string) ([]string, error) {
	if len(args) > 0 {
		out := make([]string, 0, len(args))
		for _, ref := range args {
			path, err := resolveWikiPage(ref)
			if err != nil {
				return nil, err
			}
			out = append(out, path)
		}
		return out, nil
	}
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		return nil, fmt.Errorf("load wiki: %w", err)
	}
	out := make([]string, 0, len(bld.Pages))
	for _, page := range bld.Pages {
		out = append(out, page.Path)
	}
	return out, nil
}

type artifactSpan struct {
	units []distill.Unit
	lines int
}

func claimFindings(path string, maxLines, maxUnits int, cache map[string]artifactSpan) ([]claimFinding, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path resolved under context/wiki/
	if err != nil {
		return nil, err
	}
	content := string(data)
	fm, body := contextwiki.SplitFrontmatter(content)
	offset := strings.Count(fm, "\n")
	var findings []claimFinding
	for _, block := range claimBlocks(body) {
		for _, cite := range distill.ParseCitations(block.Text) {
			artifact, rerr := resolveCorpusFile(cite.Ref)
			if rerr != nil {
				findings = append(findings, claimFinding{
					Page: corpusRelative(path), Line: offset + block.Line, Claim: block.ID,
					Ref: cite.Ref, Start: cite.Line, End: cite.End, Unresolved: true,
				})
				continue
			}
			span, ok := cache[artifact]
			if !ok {
				artData, aerr := os.ReadFile(artifact) //#nosec G304 -- corpus path resolved under context/
				if aerr != nil {
					return nil, aerr
				}
				artUnits, _ := distill.Parse(string(artData), distill.Options{})
				span = artifactSpan{units: artUnits, lines: distill.BodyLines(string(artData))}
				cache[artifact] = span
			}
			lines, covered := claimSpan(cite, span)
			if lines > maxLines || covered > maxUnits {
				findings = append(findings, claimFinding{
					Page:  corpusRelative(path),
					Line:  offset + block.Line,
					Claim: block.ID,
					Ref:   cite.Ref,
					Start: cite.Line,
					End:   cite.End,
					Lines: lines,
					Units: covered,
				})
			}
		}
	}
	return findings, nil
}

// claimBlocks splits a body at {#claim-N} anchors. Prose before the first
// anchor belongs to no claim and is ignored.
func claimBlocks(body string) []claimBlock {
	lines := strings.Split(body, "\n")
	var blocks []claimBlock
	current := -1
	for i, line := range lines {
		anchor := contextwiki.ClaimRegexp.FindString(line)
		if anchor == "" {
			if current >= 0 {
				blocks[current].Text += "\n" + line
			}
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(anchor, "{#"), "}")
		blocks = append(blocks, claimBlock{ID: id, Line: i + 1, Text: line})
		current = len(blocks) - 1
	}
	return blocks
}

func claimSpan(cite distill.Citation, span artifactSpan) (lines, covered int) {
	if cite.Whole {
		for _, u := range span.units {
			if u.Kind != distill.UnitContainer {
				covered++
			}
		}
		return span.lines, covered
	}
	if cite.End < cite.Line {
		return 0, 0
	}
	lines = cite.End - cite.Line + 1
	for _, u := range span.units {
		if u.Kind == distill.UnitContainer {
			continue
		}
		if cite.Line <= u.End && cite.End >= u.Line {
			covered++
		}
	}
	return lines, covered
}

// claimBudgetFinding is one page over the claim budget.
type claimBudgetFinding struct {
	Page   string `json:"page" yaml:"page"`
	Claims int    `json:"claims" yaml:"claims"`
	Budget int    `json:"budget" yaml:"budget"`
	Over   int    `json:"over" yaml:"over"`
}

// runClaimBudget reports pages over the claim budget. The default threshold is
// the lint's own constant, so the report and `wiki lint` cannot disagree.
func runClaimBudget(cmd *cobra.Command, args []string) {
	threshold := contextwiki.BudgetClaims
	if cmd.Flags().Changed("at") {
		threshold = getIntFlag(cmd, "at", false)
		if threshold < 1 {
			exitWithError(cmd, fmt.Errorf("--at must be positive"))
			return
		}
	}
	pages, err := claimBudgetPages(args)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	outputClaimBudget(cmd, budgetFindings(pages, threshold))
}

// claimBudgetPages resolves the pages the budget report reads.
func claimBudgetPages(args []string) ([]*contextwiki.Page, error) {
	if len(args) == 0 {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err != nil {
			return nil, fmt.Errorf("load wiki: %w", err)
		}
		return bld.Pages, nil
	}
	out := make([]*contextwiki.Page, 0, len(args))
	for _, ref := range args {
		path, err := resolveWikiPage(ref)
		if err != nil {
			return nil, err
		}
		page, err := contextwiki.ParsePage(sdtWikiDir, path)
		if err != nil {
			return nil, err
		}
		out = append(out, page)
	}
	return out, nil
}

// budgetFindings keeps the pages over the threshold, worst first.
func budgetFindings(pages []*contextwiki.Page, threshold int) []claimBudgetFinding {
	var out []claimBudgetFinding
	for _, p := range pages {
		if p.ClaimCount <= threshold {
			continue
		}
		out = append(out, claimBudgetFinding{
			Page:   p.Path,
			Claims: p.ClaimCount,
			Budget: threshold,
			Over:   p.ClaimCount - threshold,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Over != out[j].Over {
			return out[i].Over > out[j].Over
		}
		return out[i].Page < out[j].Page
	})
	return out
}

func outputClaimBudget(cmd *cobra.Command, findings []claimBudgetFinding) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(findings, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(findings)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, f := range findings {
			outputString(cmd, fmt.Sprintf("[WARNING] %s: page has %d claims (budget ~%d): over budget by %d\n", f.Page, f.Claims, f.Budget, f.Over))
		}
	}
}

func outputClaimFindings(cmd *cobra.Command, findings []claimFinding) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(findings, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(findings)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, f := range findings {
			if f.Unresolved {
				outputString(cmd, fmt.Sprintf("[SUGGESTION] %s:%d %s: %s unresolved\n", f.Page, f.Line, f.Claim, f.Ref))
				continue
			}
			outputString(cmd, fmt.Sprintf("[SUGGESTION] %s:%d %s: %s:%d-%d lines=%d units=%d\n",
				f.Page, f.Line, f.Claim, f.Ref, f.Start, f.End, f.Lines, f.Units))
		}
	}
}

func init() {
	contextWikiClaimsCmd.Flags().Int("max-lines", 40, "Flag a citation longer than this many artifact lines")
	contextWikiClaimsCmd.Flags().Int("max-units", 2, "Flag a citation overlapping more than this many artifact units")
	contextWikiClaimsCmd.Flags().Bool("budget", false, "Report pages over the claim budget, worst first")
	contextWikiClaimsCmd.Flags().Int("at", 0, "With --budget, lower the threshold (default: the lint's constant)")
	contextWikiCmd.AddCommand(contextWikiClaimsCmd)
}
