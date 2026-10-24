package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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
	contextWikiCmd.AddCommand(contextWikiClaimsCmd)
}
