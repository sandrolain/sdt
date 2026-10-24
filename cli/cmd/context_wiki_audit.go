package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── receipt audit (sdt context wiki audit / receipt lint) ────────────────────
//
// The receipt is the run's public record. audit generates it from the same
// report coverage builds; receipt lint checks a hand-written note for the lines
// that record would need. Neither measures comprehension.

type wikiAuditReport struct {
	wikiCoverageReport
	RouteHeadings     int      `json:"route_headings" yaml:"route_headings"`
	CoverageUnits     int      `json:"coverage_units" yaml:"coverage_units"`
	CitingPages       int      `json:"citing_pages" yaml:"citing_pages"`
	Claims            int      `json:"claims" yaml:"claims"`
	Inventory         string   `json:"inventory" yaml:"inventory"`
	InventoryFindings []string `json:"inventory_findings,omitempty" yaml:"inventory_findings,omitempty"`
	Orphans           int      `json:"orphans" yaml:"orphans"`
	OrphanPages       []string `json:"orphan_pages,omitempty" yaml:"orphan_pages,omitempty"`
}

type receiptFinding struct {
	Path    string `json:"path" yaml:"path"`
	Block   int    `json:"block" yaml:"block"`
	Message string `json:"message" yaml:"message"`
}

var contextWikiAuditCmd = &cobra.Command{
	Use:   "audit <index-page> --artifact <artifact>",
	Short: "Print the receipt block, page/claim totals and inventory status",
	Long: `Generate the auditable receipt for one book-scale source.

The command is read-only: it builds the same coverage report as
"sdt context wiki coverage", then prints the receipt block, the number of pages
that cite the artifact, their claim anchors, and (with --inventory) the
inventory check. It never writes the page.

Examples:
  sdt context wiki audit sources/the-hard-parts --artifact refs/converted/x.md
  sdt context wiki audit sources/the-hard-parts --artifact refs/converted/x.md --inventory notes/x.md --format json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		report, _, _, err := buildCoverageReport(cmd, args[0], getStringFlag(cmd, "artifact", true))
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		audit := wikiAuditReport{
			wikiCoverageReport: report,
			RouteHeadings:      report.Stats.Headings23,
			CoverageUnits:      report.Counts.Units,
			CitingPages:        len(report.Pages),
			Claims:             countClaims(report.Pages),
			Inventory:          "not checked",
		}
		bld, _, berr := contextwiki.LoadWiki(sdtWikiDir)
		if berr != nil {
			exitWithError(cmd, fmt.Errorf("load wiki for orphans: %w", berr))
			return
		}
		orphans := orphanFindings(bld)
		audit.Orphans = len(orphans)
		for _, f := range orphans {
			audit.OrphanPages = append(audit.OrphanPages, f.Page)
		}
		if ref := getStringFlag(cmd, "inventory", false); ref != "" {
			notePath, nerr := resolveInventoryNote(ref)
			if nerr != nil {
				exitWithError(cmd, nerr)
				return
			}
			bld, _, berr := contextwiki.LoadWiki(sdtWikiDir)
			if berr != nil {
				exitWithError(cmd, fmt.Errorf("load wiki for inventory check: %w", berr))
				return
			}
			findings := checkInventory(notePath, bld)
			audit.Inventory = "clean"
			if len(findings) > 0 {
				audit.Inventory = fmt.Sprintf("%d finding(s)", len(findings))
				for _, f := range findings {
					audit.InventoryFindings = append(audit.InventoryFindings, fmt.Sprintf("%s: %s", corpusRelative(f.Path), f.Message))
				}
			}
		}

		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(audit, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(audit)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			var b strings.Builder
			b.WriteString(receiptBlock(report))
			fmt.Fprintf(&b, "pages: %d\n", audit.CitingPages)
			fmt.Fprintf(&b, "claims: %d\n", audit.Claims)
			fmt.Fprintf(&b, "inventory: %s\n", audit.Inventory)
			fmt.Fprintf(&b, "orphans: %d\n", audit.Orphans)
			for _, f := range audit.InventoryFindings {
				fmt.Fprintf(&b, "  - %s\n", f)
			}
			outputString(cmd, b.String())
		}
	},
}

// countClaims sums the claim anchors in the pages that cite the artifact.
func countClaims(pages []string) int {
	total := 0
	for _, rel := range pages {
		path, err := resolveCorpusFile(rel)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path) //#nosec G304 -- corpus path resolved under context/
		if err != nil {
			continue
		}
		total += len(contextwiki.ClaimRegexp.FindAll(data, -1))
	}
	return total
}

var contextWikiReceiptCmd = &cobra.Command{
	Use:   "receipt",
	Short: "Receipt operations (lint a note's receipt block)",
}

var contextWikiReceiptLintCmd = &cobra.Command{
	Use:   "lint <note>",
	Short: "Check a note's fenced receipt block for the required counters",
	Long: `Check every fenced receipt block in a note.

A receipt block starts with "source:" and "route:" and must carry the routing
count, the coverage denominator, the four coverage counters and the six concept
counters:

  source: <refs/...>
  route: single-pass|book-scale
  route_headings: <n>
  lines: <n>
  coverage_units: <n>
  coverage: ok=<n> part=<n> gap=<n> no_op=<n>
  concepts: candidates=<n> paged=<n> merged=<n> inline=<n> declined=<n> pending=<n>

A missing line is a finding and exits 1. The command reads the note; it never
writes it.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path, err := resolveCorpusFile(args[0])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		data, err := os.ReadFile(path) //#nosec G304 -- corpus path resolved under context/
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		findings := lintReceiptBlocks(corpusRelative(path), string(data))
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
			if len(findings) == 0 {
				outputString(cmd, "receipt: clean\n")
			}
			for _, f := range findings {
				outputString(cmd, fmt.Sprintf("%s: receipt block %d: %s\n", f.Path, f.Block, f.Message))
			}
		}
		if len(findings) > 0 {
			exitWithError(cmd, fmt.Errorf("receipt lint: %d finding(s)", len(findings)))
		}
	},
}

func lintReceiptBlocks(path, content string) []receiptFinding {
	var findings []receiptFinding
	receiptNo := 0
	for _, block := range receiptBlocks(content) {
		if !strings.Contains(block, "source:") || !strings.Contains(block, "route:") {
			continue
		}
		receiptNo++
		findings = append(findings, lintReceiptBlock(path, receiptNo, block)...)
	}
	if receiptNo == 0 {
		findings = append(findings, receiptFinding{Path: path, Message: "no fenced receipt block found"})
	}
	return findings
}

func lintReceiptBlock(path string, receiptNo int, block string) []receiptFinding {
	var findings []receiptFinding
	add := func(msg string) {
		findings = append(findings, receiptFinding{Path: path, Block: receiptNo, Message: msg})
	}
	for _, key := range []string{"source:", "route:", "route_headings:", "lines:"} {
		if !hasReceiptLine(block, key) {
			add("missing " + strings.TrimSuffix(key, ":"))
		}
	}
	// A single-pass source owes the routing counts but not a coverage record;
	// a book-scale source owes all three lines. A line that is present is
	// checked whatever the route.
	bookScale := !strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(receiptLine(block, "route:"), "route:")), "single-pass")
	if bookScale && !hasReceiptLine(block, "coverage_units:") {
		add("missing coverage_units")
	}
	checkReceiptLine(block, "coverage:", "coverage", []string{"ok=", "part=", "gap=", "no_op="}, bookScale, add)
	checkReceiptLine(block, "concepts:", "concepts", []string{"candidates=", "paged=", "merged=", "inline=", "declined=", "pending="}, bookScale, add)
	return findings
}

func checkReceiptLine(block, lineKey, label string, keys []string, required bool, add func(string)) {
	line := receiptLine(block, lineKey)
	if line == "" {
		if required {
			add("missing " + label)
		}
		return
	}
	for _, key := range keys {
		if !strings.Contains(line, key) {
			add(label + " is missing " + strings.TrimSuffix(key, "="))
		}
	}
}

func receiptBlocks(content string) []string {
	var blocks []string
	var current []string
	inFence := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inFence {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inFence = false
			} else {
				inFence = true
			}
			continue
		}
		if inFence {
			current = append(current, line)
		}
	}
	return blocks
}

func hasReceiptLine(block, key string) bool {
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), key) {
			return true
		}
	}
	return false
}

func receiptLine(block, key string) string {
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), key) {
			return line
		}
	}
	return ""
}

func init() {
	contextWikiAuditCmd.Flags().String("artifact", "", "Converted artifact the page represents (required)")
	contextWikiAuditCmd.Flags().String("inventory", "", "Concept-inventory note to check (optional)")
	contextWikiAuditCmd.Flags().StringArray("from", nil, "Page carrying the citations (repeatable; default: every wiki page citing the artifact)")
	contextWikiReceiptCmd.AddCommand(contextWikiReceiptLintCmd)
	contextWikiCmd.AddCommand(contextWikiAuditCmd)
	contextWikiCmd.AddCommand(contextWikiReceiptCmd)
}
