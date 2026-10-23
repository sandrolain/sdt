package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/distill"
)

// ── knowledge-pipeline unit enumeration (sdt context wiki units) ──────────────
//
// A read-only, model-free enumeration of a converted artifact's headings: the
// denominator a book-scale coverage record must state. It reuses the fence-aware
// mdstruct splitter the rest of the corpus uses, so a shell comment inside a
// code block is never mistaken for a section.

// wikiUnitsReport is the machine view of one artifact.
type wikiUnitsReport struct {
	Artifact string         `json:"artifact" yaml:"artifact"`
	Stats    distill.Stats  `json:"stats" yaml:"stats"`
	Units    []distill.Unit `json:"units" yaml:"units"`
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
  sdt context wiki units refs/converted/book.md --levels 2 --containers '^Chapitre'`,
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
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(wikiUnitsReport{Artifact: rel, Stats: stats, Units: units}, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(wikiUnitsReport{Artifact: rel, Stats: stats, Units: units})
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
			for _, u := range units {
				outputString(cmd, fmt.Sprintf("  L%5d-%-5d h%d %-10s %s\n", u.Line, u.End, u.Level, u.Kind, u.Title))
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

func init() {
	contextWikiUnitsCmd.Flags().Int("levels", 3, "Deepest heading level counted as a unit")
	contextWikiUnitsCmd.Flags().String("containers", "", "Regular expression marking structural containers (default: the corpus convention)")
	contextWikiCmd.AddCommand(contextWikiUnitsCmd)
}
