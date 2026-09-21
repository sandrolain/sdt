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

// loadSemanticIndex builds the semantic index over the search index sections.
// The model is loaded from the local cache; --semantic-model overrides the
// default. A load failure is returned so the caller can degrade explicitly.
func loadSemanticIndex(cmd *cobra.Command, ix *search.Index) (*semantic.Index, error) {
	model := semantic.Model(getStringFlag(cmd, "semantic-model", false))
	if model == "" {
		model = semantic.ModelBase8M
	}
	sem, err := semantic.New(cmd.Context(), model)
	if err != nil {
		return nil, err
	}
	if err := sem.Add(cmd.Context(), ix.SemanticSections()); err != nil {
		return nil, err
	}
	return sem, nil
}

// ctxSearchIndex is built lazily per invocation from the shared mdindex scan:
// short-lived CLI processes rebuild the (in-memory bleve) index on demand, while
// the manifest makes the corpus scan incremental.
var ctxSearchIndex *search.Index

func ctxBuildSearchIndex(cmd *cobra.Command) *search.Index {
	if ctxSearchIndex != nil {
		return ctxSearchIndex
	}
	root, err := os.Getwd()
	exitWithError(cmd, err)
	refresh, err := mdindex.EnsureFresh(root)
	exitWithError(cmd, err)
	ix, err := search.NewFromEntries(refresh.Manifest.EntriesSorted())
	exitWithError(cmd, err)
	ctxSearchIndex = ix
	return ix
}

type ctxSearchHit struct {
	Path      string   `json:"path" yaml:"path"`
	Section   string   `json:"section,omitempty" yaml:"section,omitempty"`
	Kind      string   `json:"kind,omitempty" yaml:"kind,omitempty"`
	Status    string   `json:"status,omitempty" yaml:"status,omitempty"`
	Title     string   `json:"title,omitempty" yaml:"title,omitempty"`
	Objective string   `json:"objective,omitempty" yaml:"objective,omitempty"`
	Topics    []string `json:"topics,omitempty" yaml:"topics,omitempty"`
	Score     float64  `json:"score" yaml:"score"`
	Snippet   string   `json:"snippet,omitempty" yaml:"snippet,omitempty"`
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
  sdt context search "bleve" --topic context-search --limit 5 --format json`,
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
		hq := search.HybridQuery{
			Q:         q,
			Kind:      getStringFlag(cmd, "type", false),
			Objective: getStringFlag(cmd, "objective", false),
			Status:    status,
			Topic:     getStringFlag(cmd, "topic", false),
			From:      getStringFlag(cmd, "since", false),
			To:        getStringFlag(cmd, "until", false),
			Max:       limit,
		}
		var res search.Results
		var err error
		if getBoolFlag(cmd, "semantic", false) {
			sem, serr := loadSemanticIndex(cmd, ix)
			if serr != nil {
				exitWithError(cmd, serr)
			}
			res, err = ix.SearchHybrid(cmd.Context(), hq, search.HybridOptions{Semantic: sem})
		} else {
			res, err = ix.Search(hq.Q, hq.Kind, hq.Objective, hq.Status, hq.Topic, hq.From, hq.To, hq.Max)
		}
		exitWithError(cmd, err)
		hits := make([]ctxSearchHit, 0, len(res.Results))
		for _, r := range res.Results {
			hits = append(hits, ctxSearchHit{
				Path: r.Path, Section: r.Section, Kind: r.Kind, Status: r.Status,
				Title: r.Title, Objective: r.Objective, Topics: r.Topics,
				Score: r.Score, Snippet: r.Snippet,
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
  sdt context show context/analysis/20260920-133906-analysis-harness-improvements-integrations.md
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
			parts := strings.SplitN(lines, ":", 2)
			if len(parts) != 2 {
				exitWithError(cmd, fmt.Errorf("--lines expects <from>:<to>, got %q", lines))
			}
			from, err1 := strconv.Atoi(parts[0])
			to, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || from < 1 {
				exitWithError(cmd, fmt.Errorf("invalid --lines range %q", lines))
			}
			outputBytes(cmd, []byte(lineRange(content, from, to)))
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

// lineRange returns lines [from,to] (1-based, inclusive) of content.
func lineRange(content string, from, to int) string {
	lines := strings.Split(content, "\n")
	if from > len(lines) {
		return ""
	}
	if to > len(lines) {
		to = len(lines)
	}
	return strings.Join(lines[from-1:to], "\n") + "\n"
}
