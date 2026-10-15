package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/mdstruct"
	"github.com/sandrolain/sdt/internal/search"
	"github.com/sandrolain/sdt/internal/semantic"
)

// ctxBuildSemanticSnapshot loads the model and (re)builds the vector snapshot
// over the shared search index sections, through the single build path in
// internal/semantic. The model is loaded from the local cache; --semantic-model
// overrides the default. A load/encode failure is returned so the caller can
// degrade explicitly; the snapshot itself is a derived cache.
func ctxBuildSemanticSnapshot(cmd *cobra.Command, ix *search.Index) (*semantic.Index, semantic.BuildResult, error) {
	model := semantic.Model(getStringFlag(cmd, "semantic-model", false))
	if model == "" {
		model = semantic.ModelBase8M
	}
	root, err := os.Getwd()
	if err != nil {
		return nil, semantic.BuildResult{}, fmt.Errorf("determine project root: %w", err)
	}
	hashes := make(map[string]string)
	if ctxSearchRefresh != nil {
		for _, e := range ctxSearchRefresh.Manifest.EntriesSorted() {
			hashes[e.ID] = e.Hash
		}
	}
	return semantic.BuildSnapshot(cmd.Context(), root, model, ix.SemanticSections(), hashes, mdindex.ManifestVersion)
}

// loadSemanticIndex builds the semantic index for the hybrid search path.
func loadSemanticIndex(cmd *cobra.Command, ix *search.Index) (*semantic.Index, error) {
	sem, _, err := ctxBuildSemanticSnapshot(cmd, ix)
	return sem, err
}

// ctxBuildSearchIndex builds the shared search index lazily per invocation.
// The persistent store (if usable) backs it; the manifest scan is the change
// signal and the in-memory build the fallback.
var (
	ctxSearchIndex   *search.Index
	ctxSearchRefresh *mdindex.Refresh
)

func ctxBuildSearchIndex(cmd *cobra.Command) *search.Index {
	if ctxSearchIndex != nil {
		return ctxSearchIndex
	}
	root, err := os.Getwd()
	exitWithError(cmd, err)
	refresh, err := mdindex.EnsureFresh(root)
	exitWithError(cmd, err)
	ix, err := search.LoadOrRebuild(root, refresh.Manifest.EntriesSorted(), refresh.Changed, refresh.Removed)
	exitWithError(cmd, err)
	ctxSearchIndex = ix
	ctxSearchRefresh = refresh
	return ix
}

type ctxSearchHit struct {
	Path       string   `json:"path" yaml:"path"`
	Section    string   `json:"section,omitempty" yaml:"section,omitempty"`
	Kind       string   `json:"kind,omitempty" yaml:"kind,omitempty"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Title      string   `json:"title,omitempty" yaml:"title,omitempty"`
	Objective  string   `json:"objective,omitempty" yaml:"objective,omitempty"`
	Topics     []string `json:"topics,omitempty" yaml:"topics,omitempty"`
	Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Score      float64  `json:"score" yaml:"score"`
	Snippet    string   `json:"snippet,omitempty" yaml:"snippet,omitempty"`
}

var contextSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the context/ corpus (section-level, ranked)",
	Long: `Search the context/ knowledge base and print compact ranked results.

The index is section-level and derived from context/*.md (never a source of
truth); the scan is incremental via .sdt/cache/manifest.json. Filters narrow the
result set; by default superseded/archived documents are excluded unless --all.

Examples:
  sdt context search "hybrid search"
  sdt context search "dead-end" --type analysis --status active
  sdt context search "bleve" --topic context-search --limit 5 --format json
  sdt context search "filter" --date updated --last 7d --where objective=my-group` + ctxQueryHelp,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		q := args[0]
		limit := getIntFlag(cmd, "limit", false)
		if limit <= 0 {
			limit = 10
		}
		status := getStringFlag(cmd, "status", false)
		if !getBoolFlag(cmd, "all", false) && status == "" {
			status = "active"
		}
		ix := ctxBuildSearchIndex(cmd)
		filter, err := buildQueryFilter(cmd,
			stringList(getStringFlag(cmd, "type", false)),
			stringList(status),
			stringList(getStringFlag(cmd, "category", false)),
			searchAliasTerms(cmd),
		)
		exitWithError(cmd, err)
		hq := search.HybridQuery{Q: q, Max: limit, Filter: &filter}
		var res search.Results
		if getBoolFlag(cmd, "semantic", false) {
			sem, serr := loadSemanticIndex(cmd, ix)
			if serr != nil {
				exitWithError(cmd, serr)
			}
			res, err = ix.SearchHybrid(cmd.Context(), hq, search.HybridOptions{Semantic: sem})
		} else {
			res, err = ix.SearchQuery(hq)
		}
		exitWithError(cmd, err)
		hits := make([]ctxSearchHit, 0, len(res.Results))
		for _, r := range res.Results {
			hits = append(hits, ctxSearchHit{
				Path: r.Path, Section: r.Section, Kind: r.Kind, Status: r.Status,
				Title: r.Title, Objective: r.Objective, Topics: r.Topics,
				Categories: r.Categories,
				Score:      r.Score, Snippet: r.Snippet,
			})
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(map[string]any{ctxFrontmatterResults: hits, "total": res.Total}, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(map[string]any{ctxFrontmatterResults: hits, "total": res.Total})
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, h := range hits {
				ref := h.Path
				if h.Section != "" {
					ref += "#" + h.Section
				}
				meta := h.Kind
				if h.Status != "" {
					meta += "/" + h.Status
				}
				outputString(cmd, fmt.Sprintf("%-8.2f %-14s %s\n         %s\n", h.Score, meta, ref, strings.TrimSpace(h.Snippet)))
			}
			if res.Total > int64(len(hits)) {
				outputString(cmd, fmt.Sprintf("(showing %d of %d; refine the query or raise --limit)\n", len(hits), res.Total))
			}
		}
	},
}

var contextShowCmd = &cobra.Command{
	Use:   "show <path>",
	Short: "Print a document or one of its sections",
	Long: `Print a context/ document from disk (the source of truth), optionally
restricted to one section or a line range.

Examples:
  sdt context show context/analysis/<analysis-file>.md
  sdt context show context/analysis/x.md --section wave-2
  sdt context show context/analysis/x.md --lines 10:40`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := strings.TrimPrefix(args[0], "#")
		path = strings.SplitN(path, "#", 2)[0]
		if !strings.HasSuffix(path, sdtMarkdownExt) {
			path += sdtMarkdownExt
		}
		data, err := os.ReadFile(filepath.FromSlash(path)) //#nosec G304 -- explicit user path
		exitWithError(cmd, err)
		content := string(data)

		if lines := getStringFlag(cmd, "lines", false); lines != "" {
			outputBytes(cmd, sliceLinesOrExit(cmd, content, lines))
			return
		}
		if section := getStringFlag(cmd, "section", false); section != "" {
			body := ctxDocBody(content)
			for _, s := range mdstruct.SplitSections(body) {
				if strings.EqualFold(s.ID, section) || strings.EqualFold(s.Heading, section) {
					outputString(cmd, strings.TrimRight(s.Body, "\n")+"\n")
					return
				}
			}
			exitWithError(cmd, fmt.Errorf("section %q not found", section))
		}
		outputString(cmd, content)
	},
}

// ctxDocBody returns the markdown body after the frontmatter.
func ctxDocBody(content string) string {
	_, body := contextwiki.SplitFrontmatter(content)
	return body
}

// parseLineRange parses a `from:to` range spec into 1-based inclusive bounds. A
// malformed spec or a non-positive bound is an error rather than a silent
// whole-file read.
func parseLineRange(spec string) (int, int, error) {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("--lines expects <from>:<to>, got %q", spec)
	}
	from, err1 := strconv.Atoi(parts[0])
	to, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || from < 1 || to < 1 {
		return 0, 0, fmt.Errorf("invalid --lines range %q", spec)
	}
	if from > to {
		return 0, 0, fmt.Errorf("--lines range %q is inverted: %d > %d", spec, from, to)
	}
	return from, to, nil
}

// sliceLines returns lines [from,to] of content, 1-based and inclusive.
//
// A range starting past the last line is an error, never an empty answer: an
// empty read and a miscounted one are indistinguishable to the caller, and the
// silent one is always the wrong one. An upper bound past the last line clamps
// to the end, which is what a reader asking for "through line 999" means.
// selectLines returns the lines a `--lines <from>:<to>` spec selects, or every
// line when the spec is empty. It is the one owner of the range rule: `sdt read`
// slices and counts through it and `sdt context show --lines` slices through it,
// so a count and a slice cannot disagree about what the range contains.
func selectLines(content, spec string) ([]string, error) {
	lines := search.SplitLines(content)
	if spec == "" {
		return lines, nil
	}
	from, to, err := parseLineRange(spec)
	if err != nil {
		return nil, err
	}
	if from > len(lines) {
		return nil, fmt.Errorf("--lines range %q starts past the last line (%d)", spec, len(lines))
	}
	if to > len(lines) {
		to = len(lines)
	}
	return lines[from-1 : to], nil
}

func sliceLines(content, spec string) ([]byte, error) {
	lines, err := selectLines(content, spec)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// outputLineCount reports a line count in the caller's format, so `sdt read
// --count` and `sdt grep --count` answer alike whichever format is asked for.
func outputLineCount(cmd *cobra.Command, n int) {
	type lineCount struct {
		Count int `json:"count" yaml:"count"`
	}
	payload := lineCount{Count: n}
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(payload, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(payload)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, fmt.Sprintf("%d\n", n))
	}
}

// sliceLinesOrExit prints the requested range, or reports why it cannot. It is
// the one call shape both readers use, so `sdt read --lines` and
// `sdt context show --lines` cannot drift apart. The nil result after an error
// keeps the command from slicing with bounds that were never validated.
func sliceLinesOrExit(cmd *cobra.Command, content, spec string) []byte {
	out, err := sliceLines(content, spec)
	exitWithError(cmd, err)
	return out
}
