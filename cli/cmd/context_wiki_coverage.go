package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
	"github.com/sandrolain/sdt/internal/frontmatter"
)

// ── coverage record audit (sdt context wiki coverage) ─────────────────────────
//
// Classifies an artifact's units from the citations the destination pages
// actually carry, prints the routing block an ingestion receipt quotes, and
// (only with --write) regenerates the index page's coverage block and
// ## Structure list. It measures coverage, never comprehension.

// wikiCoverageReport is the machine view of one coverage run.
type wikiCoverageReport struct {
	Page      string          `json:"page" yaml:"page"`
	Artifact  string          `json:"artifact" yaml:"artifact"`
	Pages     []string        `json:"pages" yaml:"pages"`
	Route     string          `json:"route" yaml:"route"`
	Stats     distill.Stats   `json:"stats" yaml:"stats"`
	Counts    distill.Counts  `json:"coverage" yaml:"coverage"`
	Units     []distill.Unit  `json:"units" yaml:"units"`
	OutOfSpan []distill.Range `json:"out_of_span,omitempty" yaml:"out_of_span,omitempty"`
}

var contextWikiCoverageCmd = &cobra.Command{
	Use:   "coverage <index-page> --artifact <artifact>",
	Short: "Classify an artifact's units from the pages' citations (OK/PART/GAP)",
	Long: `Audit a book-scale source's coverage from the destination's own citations.

Units are enumerated from the artifact (sdt context wiki units). Citations of the
form refs/<file>@<sha>:<lines> are read from the wiki pages that cite it (all
pages by default, or --from), and each unit is classified:

  OK    every substantive line of the span is cited (non-blank, outside fences)
  PART  some lines are cited
  GAP   none are cited

The disposition already written in the page's ## Structure list is kept; a unit
without one gets "gap" when GAP. --receipt prints the routing block and the
coverage line an ingestion receipt quotes. --write regenerates the frontmatter
coverage block and the ## Structure section; it is the only mutating mode and it
preserves every byte outside those two regions.

Examples:
  sdt context wiki coverage sources/the-hard-parts --artifact refs/converted/x.md
  sdt context wiki coverage sources/the-hard-parts --artifact refs/converted/x.md --receipt
  sdt context wiki coverage sources/the-hard-parts --artifact refs/converted/x.md --write --dry-run`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		artifactRef := getStringFlag(cmd, "artifact", true)
		artifact, err := resolveCorpusFile(artifactRef)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		source, err := os.ReadFile(artifact) //#nosec G304 -- corpus path resolved under context/
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		pagePath, err := resolveWikiPage(args[0])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		pageContent, err := os.ReadFile(pagePath) //#nosec G304 -- corpus path resolved under context/wiki/
		if err != nil {
			exitWithError(cmd, err)
			return
		}

		relArtifact := corpusRelative(artifact)
		pageIDs, cites := coverageCitations(cmd, relArtifact, pagePath)
		ranges, whole := distill.RangesFor(cites, relArtifact)
		if whole > 0 {
			fmt.Fprintf(os.Stderr, "warning: %d citation(s) name no line range and cannot be classified\n", whole)
		}

		units, stats := distill.Parse(string(source), distill.Options{})
		dispositions := readStructureDispositions(pageContent)
		for i := range units {
			if d, ok := dispositions[units[i].Line]; ok {
				units[i].Disposition = d
			}
		}
		units = distill.Classify(units, ranges, string(source))
		units = applyDefaultDispositions(units)
		counts := distill.Count(units)
		outOfSpan := citationsOutsideUnits(ranges, units)
		for _, c := range outOfSpan {
			fmt.Fprintf(os.Stderr, "warning: citation %s:%d-%d falls outside every unit span\n", c.Ref, c.Start, c.End)
		}

		route := routeFor(stats)
		report := wikiCoverageReport{
			Page:      corpusRelative(pagePath),
			Artifact:  relArtifact,
			Pages:     pageIDs,
			Route:     route,
			Stats:     stats,
			Counts:    counts,
			Units:     units,
			OutOfSpan: outOfSpan,
		}

		if write := getBoolFlag(cmd, "write", false); write {
			dryRun := getBoolFlag(cmd, "dry-run", false)
			if err := writeCoverage(pagePath, pageContent, report, dryRun); err != nil {
				exitWithError(cmd, err)
				return
			}
			if !dryRun {
				outputString(cmd, fmt.Sprintf("coverage updated: %s\n", report.Page))
			}
		}

		if getBoolFlag(cmd, "receipt", false) {
			outputString(cmd, receiptBlock(report))
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(report, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(report)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, fmt.Sprintf("%s  units=%d ok=%d part=%d gap=%d no_op=%d\n",
				report.Artifact, counts.Units, counts.OK, counts.Part, counts.Gap, counts.NoOp))
			for _, u := range units {
				if u.Kind == distill.UnitContainer {
					continue
				}
				outputString(cmd, fmt.Sprintf("  L%5d-%-5d %-8s %-14s %s\n", u.Line, u.End, u.State, u.Disposition, u.Title))
			}
		}
	},
}

// coverageCitations resolves the pages to read and returns their ids and the
// citations they carry. No artifact-wide scan is guessed: every page is a
// document with a path in the report.
func coverageCitations(cmd *cobra.Command, artifact, indexPage string) ([]string, []distill.Citation) {
	var paths []string
	if from := getStringArrayFlag(cmd, "from", false); len(from) > 0 {
		for _, ref := range from {
			if path, err := resolveWikiPage(ref); err == nil {
				paths = append(paths, path)
			}
		}
	} else {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err == nil {
			for _, p := range bld.Pages {
				if strings.Contains(p.Body, artifact) {
					paths = append(paths, p.Path)
				}
			}
		}
	}
	var ids []string
	var cites []distill.Citation
	for _, path := range paths {
		if sameFile(path, indexPage) {
			continue // the index page is the destination, never its own evidence
		}
		data, err := os.ReadFile(path) //#nosec G304 -- corpus path from the wiki walker
		if err != nil {
			continue
		}
		ids = append(ids, corpusRelative(path))
		_, body := contextwiki.SplitFrontmatter(string(data))
		cites = append(cites, distill.ParseCitations(body)...)
	}
	return ids, cites
}

// sameFile compares two paths after resolving them, so a relative and an
// absolute spelling of the index page cannot slip back in as evidence.
func sameFile(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absA == absB
}

// resolveWikiPage resolves a wiki page by path, context-relative path, id or
// bare file name.
func resolveWikiPage(ref string) (string, error) {
	clean := filepath.Clean(ref)
	candidates := []string{
		ref,
		filepath.Join(sdtWorkDir, ref),
		filepath.Join(sdtWikiDir, ref+".md"),
		filepath.Join(sdtWikiDir, clean+".md"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if bld, _, err := contextwiki.LoadWiki(sdtWikiDir); err == nil {
		if p := bld.Resolve(ref); p != nil {
			return p.Path, nil
		}
	}
	return "", fmt.Errorf("wiki page %q not found under %s/", ref, sdtWikiDir)
}

// readStructureDispositions reads `- L<n> · title · STATE · disposition` lines.
func readStructureDispositions(content []byte) map[int]string {
	out := map[int]string{}
	_, body := contextwiki.SplitFrontmatter(string(content))
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "- L") {
			continue
		}
		lineNo, _, disposition, ok := parseStructureLine(line)
		if ok {
			out[lineNo] = disposition
		}
	}
	return out
}

// parseStructureLine parses one `## Structure` row from the right, so a title
// containing the separator survives.
func parseStructureLine(line string) (lineNo int, state, disposition string, ok bool) {
	rest := strings.TrimPrefix(line, "- L")
	parts := strings.Split(rest, " · ")
	if len(parts) < 4 {
		return 0, "", "", false
	}
	if _, err := fmt.Sscanf(parts[0], "%d", &lineNo); err != nil {
		return 0, "", "", false
	}
	disposition = parts[len(parts)-1]
	state = parts[len(parts)-2]
	if disposition == "—" {
		disposition = ""
	}
	return lineNo, state, disposition, true
}

// applyDefaultDispositions fills the disposition a page did not write: GAP/PART
// units are owed, front/back matter is not-conceptual, OK units carry none.
func applyDefaultDispositions(units []distill.Unit) []distill.Unit {
	for i := range units {
		u := &units[i]
		if u.Kind == distill.UnitContainer {
			continue
		}
		switch u.Disposition {
		case "", "—":
			switch {
			case u.Kind == distill.UnitFrontBack:
				u.Disposition = distill.DispositionNotConceptual
			case u.State == distill.StateOK:
				u.Disposition = "—"
			default:
				u.Disposition = distill.DispositionGap
			}
		}
	}
	return units
}

// citationsOutsideUnits reports citations whose span touches no unit, the sign
// that a page cites the wrong artifact or a stale line range.
func citationsOutsideUnits(ranges []distill.Range, units []distill.Unit) []distill.Range {
	var out []distill.Range
	for _, r := range ranges {
		found := false
		for _, u := range units {
			if r.Start <= u.End && r.End >= u.Line {
				found = true
				break
			}
		}
		if !found {
			out = append(out, r)
		}
	}
	return out
}

// routeFor applies the doctrine's routing test: more than 100 section-level
// headings or more than 2500 body lines is book-scale.
func routeFor(stats distill.Stats) string {
	if stats.Headings23 > 100 || stats.Lines > 2500 {
		return "book-scale"
	}
	return "single-pass"
}

// receiptBlock renders the lines an ingestion receipt quotes for one source.
func receiptBlock(r wikiCoverageReport) string {
	return fmt.Sprintf("source: %s\nroute: %s\nsection_headings: %d\nlines: %d\ncoverage: %d/%d\n",
		r.Artifact, r.Route, r.Stats.Headings23, r.Stats.Lines, r.Counts.OK, r.Counts.Units)
}

// writeCoverage regenerates the coverage block and the ## Structure section.
// Every byte outside those two regions is preserved: the frontmatter splice is
// internal/frontmatter's, the body splice only replaces the section.
func writeCoverage(path string, content []byte, report wikiCoverageReport, dryRun bool) error {
	block, err := frontmatter.Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse frontmatter of %s: %w", path, err)
	}
	value := fmt.Sprintf("  units: %d\n  ok: %d\n  part: %d\n  gap: %d\n  no_op: %d",
		report.Counts.Units, report.Counts.OK, report.Counts.Part, report.Counts.Gap, report.Counts.NoOp)
	newYAML, err := block.Set("coverage", value, []string{ctxFrontmatterSources, "tags", "relations"})
	if err != nil {
		return err
	}
	updated := block.ReplaceBody(string(content), newYAML)
	fm, body := contextwiki.SplitFrontmatter(updated)
	body = replaceStructure(body, report.Units)
	if dryRun {
		if _, werr := fmt.Fprintf(os.Stdout, "--- coverage ---\ncoverage:\n%s\n--- structure ---\n%s", strings.TrimPrefix(value, "  "), structureSection(report.Units)); werr != nil {
			return werr
		}
		return nil
	}
	info, statErr := os.Stat(path)
	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, []byte(fm+body), mode) //#nosec G306,G703 -- path resolved under context/wiki, mode preserved
}

// replaceStructure replaces the `## Structure` section (to the next level-2
// heading) with the generated unit list, appending the section when absent.
func replaceStructure(body string, units []distill.Unit) string {
	generated := structureSection(units)
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Structure" {
			start = i
			break
		}
	}
	if start < 0 {
		return strings.TrimRight(body, "\n") + "\n\n" + generated
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, strings.Split(strings.TrimRight(generated, "\n"), "\n")...)
	out = append(out, "")
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// structureSection renders one line per unit, grouped by its container.
func structureSection(units []distill.Unit) string {
	var b strings.Builder
	b.WriteString("## Structure\n\n")
	b.WriteString("One line per unit: artifact span · title · classification · disposition.\n")
	chapter := ""
	for _, u := range units {
		if u.Kind == distill.UnitContainer {
			chapter = u.Title
			b.WriteString("\n### " + chapter + "\n\n")
			continue
		}
		disposition := u.Disposition
		if disposition == "" {
			disposition = "—"
		}
		fmt.Fprintf(&b, "- L%d · %s · %s · %s\n", u.Line, u.Title, u.State, disposition)
	}
	return b.String()
}

func init() {
	contextWikiCoverageCmd.Flags().String("artifact", "", "Converted artifact the page represents (required)")
	contextWikiCoverageCmd.Flags().StringArray("from", nil, "Page carrying the citations (repeatable; default: every wiki page citing the artifact)")
	contextWikiCoverageCmd.Flags().Bool("receipt", false, "Print the ingestion receipt's routing block")
	contextWikiCoverageCmd.Flags().Bool("write", false, "Regenerate the coverage block and ## Structure")
	contextWikiCoverageCmd.Flags().Bool("dry-run", false, "With --write, print the result without writing")
	contextWikiCmd.AddCommand(contextWikiCoverageCmd)
}
