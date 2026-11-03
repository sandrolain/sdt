package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/distill"
	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/search"
)

// ── knowledge-pipeline unit enumeration (sdt context wiki units) ──────────────
//
// A read-only, model-free enumeration of a converted artifact's headings: the
// denominator a book-scale coverage record must state. It reuses the fence-aware
// mdstruct splitter the rest of the corpus uses, so a shell comment inside a
// code block is never mistaken for a section.

// wikiUnitKindSection is the only unit kind the fold-in matcher ranks: front and
// back matter and chapter containers group units but carry no concept of their
// own, so they get no candidate row.
const wikiUnitKindSection = "section"

// wikiUnitsReport is the machine view of one artifact.
type wikiUnitsReport struct {
	Artifact string         `json:"artifact" yaml:"artifact"`
	Stats    distill.Stats  `json:"stats" yaml:"stats"`
	Units    []distill.Unit `json:"units" yaml:"units"`
}

// wikiUnitMatch is one candidate page for a unit, with the evidence that put it
// there. It is a proposal for the fold-in, never a decision.
type wikiUnitMatch struct {
	Slug  string   `json:"slug" yaml:"slug"`
	Title string   `json:"title,omitempty" yaml:"title,omitempty"`
	Score float64  `json:"score" yaml:"score"`
	Terms []string `json:"terms,omitempty" yaml:"terms,omitempty"`
}

// wikiUnitRow is a unit plus its candidates, used only when --match is set so
// the unflagged report keeps its shape.
type wikiUnitRow struct {
	Line    int             `json:"line" yaml:"line"`
	End     int             `json:"end" yaml:"end"`
	Level   int             `json:"level" yaml:"level"`
	Kind    string          `json:"kind" yaml:"kind"`
	Title   string          `json:"title" yaml:"title"`
	Matches []wikiUnitMatch `json:"matches,omitempty" yaml:"matches,omitempty"`
	Capped  bool            `json:"capped,omitempty" yaml:"capped,omitempty"`
}

// wikiUnitsMatchReport is the machine view when --match ranks the units against
// the pages of a directory.
type wikiUnitsMatchReport struct {
	Artifact string        `json:"artifact" yaml:"artifact"`
	Match    string        `json:"match" yaml:"match"`
	Stats    distill.Stats `json:"stats" yaml:"stats"`
	Units    []wikiUnitRow `json:"units" yaml:"units"`
}

var contextWikiUnitsCmd = &cobra.Command{
	Use:   "units <artifact>",
	Short: "Enumerate a converted artifact's headings (units) and spans",
	Long: `Enumerate a converted artifact's heading spans for a book-scale ingestion.

The unit is a heading outside a fenced code block; a container (chapter, part,
index, front/back matter) groups units and is reported but not counted. Levels
1-3 are read by default, matching the corpus convention that a chapter and its
sections demarcate the units; adjust with --levels.

The row carries the 1-based line span, so a page citation (:n or :n-m) maps onto
it directly. The stats block is what an ingestion receipt quotes: raw heading
lines (including code-fence comments), headings outside fences, section-level
headings (levels 2-3, the routing proxy) and body lines.

Examples:
  sdt context wiki units refs/converted/book-abcdef123456.md
  sdt context wiki units refs/converted/book-abcdef123456.md --format json
  sdt context wiki units refs/converted/book.md --levels 2 --containers '^Chapitre'
  sdt context wiki units refs/converted/book.md --match wiki/architecture --limit 3

With --match <dir> each unit is also ranked against the pages under that
directory with the shared corpus index, and the top --limit candidates follow
the unit line with the words they share. It is a proposal for the fold-in merge,
read-only: it never writes a page, reserves a slug or dispositions a unit.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		artifact, err := resolveCorpusFile(args[0])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		source, err := os.ReadFile(artifact) //#nosec G304 -- corpus path resolved under context/
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		opt := distill.Options{MaxLevel: getIntFlag(cmd, "levels", false)}
		if pattern := getStringFlag(cmd, "containers", false); pattern != "" {
			re, rerr := regexp.Compile(pattern)
			if rerr != nil {
				exitWithError(cmd, fmt.Errorf("--containers is not a regular expression: %w", rerr))
				return
			}
			opt.Containers = re
		}
		units, stats := distill.Parse(string(source), opt)
		rel := corpusRelative(artifact)
		matchDir := getStringFlag(cmd, "match", false)
		limit := getIntFlag(cmd, "limit", false)
		if limit <= 0 {
			limit = 3
		}
		var rows []wikiUnitRow
		var ix *search.Index
		if matchDir != "" {
			dirID, derr := resolveCorpusDir(matchDir)
			if derr != nil {
				exitWithError(cmd, derr)
				return
			}
			built, ierr := wikiMatchIndex()
			if ierr != nil {
				exitWithError(cmd, ierr)
				return
			}
			ix = built
			lines := strings.Split(string(source), "\n")
			prefix := sdtWorkDir + "/" + dirID + "/"
			rows = make([]wikiUnitRow, 0, len(units))
			for _, u := range units {
				row := wikiUnitRow{Line: u.Line, End: u.End, Level: u.Level, Kind: u.Kind, Title: u.Title}
				// Only a section is a fold-in target: front/back matter and
				// chapter containers group units but carry no concept of their own.
				if u.Kind == wikiUnitKindSection {
					row.Matches, row.Capped = wikiUnitMatches(ix, u, unitExcerpt(lines, u, 200), prefix, limit)
				}
				rows = append(rows, row)
			}
		}
		switch getFormat(cmd) {
		case fmtJSON:
			var payload any = wikiUnitsReport{Artifact: rel, Stats: stats, Units: units}
			if matchDir != "" {
				payload = wikiUnitsMatchReport{Artifact: rel, Match: matchDir, Stats: stats, Units: rows}
			}
			out, merr := json.MarshalIndent(payload, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			var payload any = wikiUnitsReport{Artifact: rel, Stats: stats, Units: units}
			if matchDir != "" {
				payload = wikiUnitsMatchReport{Artifact: rel, Match: matchDir, Stats: stats, Units: rows}
			}
			out, merr := yaml.Marshal(payload)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, fmt.Sprintf("artifact: %s\n", rel))
			outputString(cmd, fmt.Sprintf("lines: %d  raw_headings: %d  outside_fences: %d  units: %d  section_headings(2-3): %d\n",
				stats.Lines, stats.RawHeadings, stats.OutsideFences, stats.Units, stats.Headings23))
			levels := make([]int, 0, len(stats.ByLevel))
			for level := range stats.ByLevel {
				levels = append(levels, level)
			}
			sort.Ints(levels)
			for _, level := range levels {
				outputString(cmd, fmt.Sprintf("  h%d: %d\n", level, stats.ByLevel[level]))
			}
			if matchDir != "" {
				outputString(cmd, fmt.Sprintf("match: %s  limit: %d\n", matchDir, limit))
			}
			for i, u := range units {
				line := fmt.Sprintf("  L%5d-%-5d h%d %-10s %s", u.Line, u.End, u.Level, u.Kind, u.Title)
				if matchDir != "" && u.Kind == wikiUnitKindSection {
					line += formatWikiMatches(rows[i].Matches, rows[i].Capped)
				}
				outputString(cmd, line+"\n")
			}
		}
	},
}

// resolveCorpusFile resolves a path as given, as context/-relative, or as a
// refs/ path, and refuses anything outside the context corpus.
func resolveCorpusFile(ref string) (string, error) {
	candidates := []string{ref}
	if !strings.HasPrefix(ref, sdtWorkDir+string(filepath.Separator)) && ref != sdtWorkDir {
		candidates = append(candidates, filepath.Join(sdtWorkDir, ref))
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		abs, aerr := filepath.Abs(candidate)
		if aerr != nil {
			return "", aerr
		}
		root, rerr := filepath.Abs(sdtWorkDir)
		if rerr != nil {
			return "", rerr
		}
		if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return "", fmt.Errorf("%q is outside %s/", ref, sdtWorkDir)
		}
		return candidate, nil
	}
	return "", fmt.Errorf("artifact %q not found (looked at %s)", ref, strings.Join(candidates, ", "))
}

// corpusRelative returns a path relative to context/ for display and citations.
func corpusRelative(path string) string {
	clean := filepath.Clean(path)
	prefix := sdtWorkDir + string(filepath.Separator)
	if strings.HasPrefix(clean, prefix) {
		return filepath.ToSlash(strings.TrimPrefix(clean, prefix))
	}
	return filepath.ToSlash(clean)
}

// resolveCorpusDir normalizes a --match directory to a path relative to
// context/, accepting `wiki/x` and `context/wiki/x`, and refuses anything that
// does not exist under the corpus.
func resolveCorpusDir(ref string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(ref))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, sdtWorkDir+"/")
	clean = strings.TrimSuffix(clean, "/")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("--match must name a directory under %s/ (got %q)", sdtWorkDir, ref)
	}
	info, err := os.Stat(filepath.Join(sdtWorkDir, clean))
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("--match directory %q not found under %s/", ref, sdtWorkDir)
	}
	return clean, nil
}

// wikiMatchIndex builds the shared lexical index over the project corpus. It is
// the same index `sdt context search` serves, so the matcher adds no second
// ranking; the project root is read from .sdt.yaml, never from the working
// directory.
func wikiMatchIndex() (*search.Index, error) {
	root := sdtProjectRoot()
	if root == "" {
		return nil, fmt.Errorf("--match needs a project root: no %s found above the working directory", sdtConfigFile)
	}
	refresh, err := mdindex.EnsureFresh(root)
	if err != nil {
		return nil, err
	}
	return search.LoadOrRebuild(root, refresh.Manifest.EntriesSorted(), refresh.Changed, refresh.Removed)
}

// wikiUnitMatches ranks the pages under prefix for one unit and returns at most
// limit of them, plus whether the cap bit. The shared index proposes candidates
// from the unit title and the first lines of its span; a candidate is kept only
// when it shares a significant word with the unit, and the shared words order
// the list. A page the unit has no word in common with is not a candidate, which
// is what keeps a generic heading from matching the whole corpus.
func wikiUnitMatches(ix *search.Index, u distill.Unit, excerpt, prefix string, limit int) ([]wikiUnitMatch, bool) {
	unitText := strings.TrimSpace(u.Title + " " + excerpt)
	if unitText == "" {
		return nil, false
	}
	res, err := ix.SearchQuery(search.HybridQuery{Q: unitText, Max: 100})
	if err != nil {
		return nil, false
	}
	unitTerms := wikiTermSet(u.Title)
	candidates := make([]wikiUnitMatch, 0, limit)
	for _, r := range res.Results {
		if !strings.HasPrefix(r.Path, prefix) {
			continue
		}
		terms := sharedTermsFrom(unitTerms, r.Title+" "+r.Summary+" "+r.Snippet)
		if len(terms) == 0 {
			continue
		}
		candidates = append(candidates, wikiUnitMatch{
			Slug:  pageSlug(r.Path),
			Title: r.Title,
			Score: math.Round(r.Score*1000) / 1000,
			Terms: terms,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if len(candidates[i].Terms) != len(candidates[j].Terms) {
			return len(candidates[i].Terms) > len(candidates[j].Terms)
		}
		return candidates[i].Score > candidates[j].Score
	})
	capped := len(candidates) > limit
	if capped {
		candidates = candidates[:limit]
	}
	return candidates, capped
}

// pageSlug is the page id the corpus uses in relations: the path under
// context/wiki/ without the extension.
func pageSlug(path string) string {
	p := strings.TrimSuffix(path, ".md")
	p = strings.TrimPrefix(p, sdtWorkDir+"/")
	p = strings.TrimPrefix(p, "wiki/")
	return p
}

// unitExcerpt reads the first prose lines of a unit's span, skipping headings
// and fenced code, and stops at max bytes.
func unitExcerpt(lines []string, u distill.Unit, max int) string {
	var b strings.Builder
	fence := false
	for i := u.Line + 1; i <= u.End && i <= len(lines); i++ {
		t := strings.TrimSpace(lines[i-1])
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if fence || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t)
		if b.Len() >= max {
			break
		}
	}
	s := b.String()
	if len(s) > max {
		s = s[:max]
	}
	return s
}

// wikiMatchStop keeps the evidence line honest: a shared article or a generic
// corpus word is not a reason to propose a page.
var wikiMatchStop = map[string]bool{
	"this": true, "that": true, "with": true, "from": true, "into": true,
	"when": true, "what": true, "which": true, "their": true, "there": true,
	"then": true, "than": true, "they": true, "have": true, "will": true,
	"your": true, "also": true, "more": true, "most": true, "such": true,
	"each": true, "other": true, "some": true, "only": true, "over": true,
	"using": true, "used": true, "system": true, "systems": true,
	"chapter": true, wikiUnitKindSection: true, ctxMapNameSummary: true,
}

// sharedTermsFrom is the explanation on a candidate: the significant words the
// unit and the page title/summary/snippet have in common.
func sharedTermsFrom(unit map[string]bool, page string) []string {
	bs := wikiTermSet(page)
	out := make([]string, 0, 3)
	for t := range unit {
		if bs[t] {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func wikiTermSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
	}) {
		t = strings.Trim(t, "-")
		if len(t) < 4 || wikiMatchStop[t] {
			continue
		}
		set[t] = true
	}
	return set
}

// formatWikiMatches renders the candidates after a unit line.
func formatWikiMatches(matches []wikiUnitMatch, capped bool) string {
	if len(matches) == 0 {
		return "  ->  (no candidate)"
	}
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		part := fmt.Sprintf("%s (%d", m.Slug, len(m.Terms))
		if len(m.Terms) > 0 {
			part += ": " + strings.Join(m.Terms, ",")
		}
		parts = append(parts, part+")")
	}
	out := "  ->  " + strings.Join(parts, ", ")
	if capped {
		out += " +"
	}
	return out
}

func init() {
	contextWikiUnitsCmd.Flags().Int("levels", 3, "Deepest heading level counted as a unit")
	contextWikiUnitsCmd.Flags().String("containers", "", "Regular expression marking structural containers (default: the corpus convention)")
	contextWikiUnitsCmd.Flags().String("match", "", "Rank each unit against the pages of a corpus directory (read-only)")
	contextWikiUnitsCmd.Flags().Int("limit", 3, "Candidate pages per unit with --match")
	contextWikiCmd.AddCommand(contextWikiUnitsCmd)
}
