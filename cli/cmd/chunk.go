package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/mdstruct"
	"github.com/sandrolain/sdt/internal/semantic"
)

// ── transient chunk search (sdt chunk search) ────────────────────────────────
//
// Rank an artifact's sections against a query without adding them to the corpus
// index. The book is split in memory (distill + mdstruct), embedded into a
// throwaway chromem collection, and discarded; --with-wiki reads the persisted
// wiki snapshot but never saves it. No file under .sdt/cache is written.

const (
	chunkRRFK         = 60
	chunkModeSemantic = "semantic"
	chunkModeLexical  = "lexical"
)

// conversionAnchor matches the <a id="…"></a> bookmarks doc2md leaves in the
// converted artifact; they are navigation residue, not content.
var conversionAnchor = regexp.MustCompile(`<a id="[^"]*"></a>`)

// chunkSection is one embeddable unit with its provenance.
type chunkSection struct {
	ID     string
	Path   string
	Anchor string
	Title  string
	Start  int
	End    int
	SHA    string
	Scope  string
	Text   string
}

// chunkHit is one result row.
type chunkHit struct {
	Path    string  `json:"path" yaml:"path"`
	Anchor  string  `json:"anchor" yaml:"anchor"`
	Title   string  `json:"title" yaml:"title"`
	Scope   string  `json:"scope" yaml:"scope"`
	Start   int     `json:"start" yaml:"start"`
	End     int     `json:"end" yaml:"end"`
	SHA     string  `json:"sha,omitempty" yaml:"sha,omitempty"`
	Locator string  `json:"locator" yaml:"locator"`
	Score   float64 `json:"score" yaml:"score"`
	Snippet string  `json:"snippet" yaml:"snippet"`
}

// chunkSearchReport is the machine view of one transient query.
type chunkSearchReport struct {
	Artifact     string     `json:"artifact" yaml:"artifact"`
	Query        string     `json:"query" yaml:"query"`
	Mode         string     `json:"mode" yaml:"mode"`
	Model        string     `json:"model,omitempty" yaml:"model,omitempty"`
	Units        int        `json:"units" yaml:"units"`
	WithWiki     bool       `json:"with_wiki" yaml:"with_wiki"`
	WikiSections int        `json:"wiki_sections,omitempty" yaml:"wiki_sections,omitempty"`
	Results      []chunkHit `json:"results" yaml:"results"`
}

var chunkCmd = &cobra.Command{
	Use:   "chunk",
	Short: "Transient chunk search over an artifact (no index written)",
	Long:  `Search a converted artifact by section without adding it to the corpus index.`,
}

var chunkSearchCmd = &cobra.Command{
	Use:   "search <artifact> <query>",
	Short: "Rank an artifact's sections against a query, transiently",
	Long: `Rank the sections of a converted artifact against a query.

The artifact is split in memory with the same fence-aware splitter the corpus
uses (mdstruct, through internal/distill), embedded into a throwaway in-memory
collection, and discarded at exit. Nothing is written under .sdt/cache: the
corpus index and the vector snapshot are read-only inputs.

Each result carries the section anchor and a refs/<file>@<sha8>:<start>-<end>
locator, so it can be cited without re-reading the book. --with-wiki also ranks
the wiki pages' sections, reusing the persisted snapshot vectors read-only, and
fuses the two lists with RRF.

When no embedding model is cached (sdt models fetch), the command degrades to
lexical ranking and says so; it still succeeds.

Examples:
  sdt chunk search refs/converted/book-abcdef123456.md "how is coupling measured?"
  sdt chunk search refs/converted/book.md "saga compensation" --with-wiki --top 5
  sdt chunk search refs/converted/book.md "granularity" --format json`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		artifact, err := resolveCorpusFile(args[0])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		data, err := os.ReadFile(artifact) //#nosec G304 -- corpus path resolved under context/
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		source := string(data)
		rel := corpusRelative(artifact)
		sha := chunkSHA8(data)
		levels := getIntFlag(cmd, "levels", false)
		top := getIntFlag(cmd, "top", false)
		if top <= 0 {
			top = 8
		}
		budget := getIntFlag(cmd, "budget", false)
		withWiki := getBoolFlag(cmd, "with-wiki", false)
		model := semantic.Model(getStringFlag(cmd, "model", false))
		if model == "" {
			model = semantic.ModelBase8M
		}

		book := buildChunkSections(source, rel, sha, levels, getBoolFlag(cmd, "all-units", false))
		if len(book) == 0 {
			exitWithError(cmd, fmt.Errorf("no searchable sections in %s", rel))
			return
		}

		root, err := os.Getwd()
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		var wiki []chunkSection
		var hashes map[string]string
		if withWiki {
			wiki, hashes, err = buildWikiChunkSections(root)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: wiki sections unavailable: %v\n", err)
			}
		}

		report := chunkSearchReport{
			Artifact:     rel,
			Query:        args[1],
			Model:        string(model),
			Units:        len(book),
			WithWiki:     withWiki,
			WikiSections: len(wiki),
		}
		hits, mode, note := chunkRank(cmd.Context(), args[1], model, book, wiki, hashes, top)
		report.Mode = mode
		report.Results = hits
		if note != "" {
			fmt.Fprintln(os.Stderr, note)
		}
		for i := range report.Results {
			report.Results[i].Snippet = chunkSnippet(report.Results[i].Snippet, budget)
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
			outputString(cmd, fmt.Sprintf("artifact: %s  units: %d  mode: %s", report.Artifact, report.Units, report.Mode))
			if report.WithWiki {
				outputString(cmd, fmt.Sprintf("  wiki sections: %d", report.WikiSections))
			}
			outputString(cmd, "\n")
			for _, h := range report.Results {
				outputString(cmd, fmt.Sprintf("%.3f  %-4s %s#%s  %s\n", h.Score, h.Scope, h.Path, h.Anchor, h.Locator))
				outputString(cmd, fmt.Sprintf("       %s\n", h.Title))
				for _, line := range strings.Split(h.Snippet, "\n") {
					if strings.TrimSpace(line) == "" {
						continue
					}
					outputString(cmd, "       "+strings.TrimSpace(line)+"\n")
				}
				outputString(cmd, "\n")
			}
		}
	},
}

// buildChunkSections turns an artifact into chunks: non-container units with
// the corrected span and the mdstruct anchor.
func buildChunkSections(source, rel, sha string, levels int, all bool) []chunkSection {
	units, _ := distill.Parse(source, distill.Options{MaxLevel: levels})
	byLine := make(map[int]distill.Unit, len(units))
	for _, u := range units {
		byLine[u.Line] = u
	}
	lines := strings.Split(source, "\n")
	var out []chunkSection
	for _, s := range mdstruct.SplitSections(source) {
		if s.Level == 0 || s.Heading == "" {
			continue
		}
		u, ok := byLine[s.StartLine]
		if !ok || u.Kind == distill.UnitContainer {
			continue
		}
		if u.Kind == distill.UnitFrontBack && !all {
			continue
		}
		end := u.End
		if end > len(lines) {
			end = len(lines)
		}
		out = append(out, chunkSection{
			ID:     rel + "#" + s.ID,
			Path:   rel,
			Anchor: s.ID,
			Title:  u.Title,
			Start:  u.Line,
			End:    end,
			SHA:    sha,
			Scope:  "book",
			Text:   cleanChunkText(strings.Join(lines[u.Line-1:end], "\n")),
		})
	}
	return out
}

// buildWikiChunkSections reads the concept pages under context/wiki and builds
// their embeddable sections. The page title and summary are part of the recipe,
// matching the persisted snapshot so AddIncremental can reuse its vectors; the
// source index pages (those with a coverage block) are skipped because their
// unit lists are navigation, not knowledge.
func buildWikiChunkSections(root string) ([]chunkSection, map[string]string, error) {
	dir := filepath.Join(root, sdtWikiDir)
	hashes := map[string]string{}
	var out []chunkSection
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() || filepath.Ext(d.Name()) != ".md" {
			return nil
		}
		data, rerr := os.ReadFile(path) //#nosec G304,G122 -- path from the trusted wiki walk
		if rerr != nil {
			return nil
		}
		content := string(data)
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		id := "wiki/" + strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		page := contextwiki.ParseContent(content, path, id)
		if _, isIndex, covErr := contextwiki.CoverageBlock(page.Frontmatter); isIndex && covErr == nil {
			return nil // a source frame, not a concept page
		}
		if page.Status == contextwiki.StatusArchived {
			return nil
		}
		hashes[id] = chunkSHA8([]byte(content))
		for _, sec := range mdstruct.SplitSections(page.Body) {
			if sec.Level == 0 || sec.Heading == "" {
				continue
			}
			text := strings.TrimSpace(strings.Join([]string{page.Title, page.Summary, sec.Heading, strings.TrimSpace(sec.Body)}, "\n"))
			out = append(out, chunkSection{
				ID:     id + "#" + sec.ID,
				Path:   id,
				Anchor: sec.ID,
				Title:  sec.Heading,
				Scope:  "wiki",
				Text:   text,
			})
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return out, hashes, nil
}

// chunkRank runs the semantic path and degrades to lexical. It returns the
// rows, the mode and an optional stderr note.
func chunkRank(ctx context.Context, query string, model semantic.Model, book, wiki []chunkSection, hashes map[string]string, top int) ([]chunkHit, string, string) {
	sections := append(append([]chunkSection{}, book...), wiki...)
	sem, err := semantic.New(ctx, model)
	if err != nil {
		return lexicalChunkHits(query, sections, top), chunkModeLexical, fmt.Sprintf("warning: semantic unavailable, lexical ranking: %v", err)
	}
	if err := sem.Add(ctx, toSemanticSections(book)); err != nil {
		return lexicalChunkHits(query, sections, top), chunkModeLexical, fmt.Sprintf("warning: semantic unavailable, lexical ranking: %v", err)
	}
	note := ""
	if len(wiki) > 0 {
		snap := semantic.LoadSnapshot(rootDir())
		if _, _, _, aerr := sem.AddIncremental(ctx, toSemanticSections(wiki), hashes, mdindex.ManifestVersion, snap); aerr != nil {
			note = fmt.Sprintf("warning: wiki branch dropped: %v", aerr)
			wiki = nil
		}
	}
	k := top
	if len(wiki) > 0 {
		k = top * 4
	}
	raw, err := sem.Query(ctx, query, k)
	if err != nil {
		return lexicalChunkHits(query, sections, top), chunkModeLexical, fmt.Sprintf("warning: semantic unavailable, lexical ranking: %v", err)
	}
	if len(wiki) > 0 {
		return rrfChunkHits(raw, book, wiki, top), chunkModeSemantic, note
	}
	return directChunkHits(raw, book, top), chunkModeSemantic, note
}

func rootDir() string {
	root, err := os.Getwd()
	if err != nil {
		return "."
	}
	return root
}

func toSemanticSections(in []chunkSection) []semantic.Section {
	out := make([]semantic.Section, 0, len(in))
	for _, s := range in {
		out = append(out, semantic.Section{
			ID:     s.ID,
			Path:   s.Path,
			Anchor: s.Anchor,
			Text:   s.Text,
			Meta:   map[string]string{"path": s.Path, "section": s.Anchor, "scope": s.Scope},
		})
	}
	return out
}

// directChunkHits maps the raw semantic order, without wiki fusion.
func directChunkHits(raw []semantic.Hit, book []chunkSection, top int) []chunkHit {
	byID := indexChunkSections(book)
	out := make([]chunkHit, 0, top)
	for _, h := range raw {
		s, ok := byID[h.ID]
		if !ok {
			continue
		}
		out = append(out, hitFromSection(s, float64(h.Similarity)))
		if len(out) == top {
			break
		}
	}
	return out
}

// rrfChunkHits fuses the two scoped sublists with reciprocal rank fusion.
func rrfChunkHits(raw []semantic.Hit, book, wiki []chunkSection, top int) []chunkHit {
	bookIDs := indexChunkSections(book)
	wikiIDs := indexChunkSections(wiki)
	scores := map[string]float64{}
	var order []string
	add := func(id string, rank int) {
		if _, seen := scores[id]; !seen {
			order = append(order, id)
		}
		scores[id] += 1.0 / float64(chunkRRFK+rank+1)
	}
	bookRank, wikiRank := 0, 0
	for _, h := range raw {
		if _, ok := bookIDs[h.ID]; ok {
			add(h.ID, bookRank)
			bookRank++
			continue
		}
		if _, ok := wikiIDs[h.ID]; ok {
			add(h.ID, wikiRank)
			wikiRank++
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	out := make([]chunkHit, 0, top)
	for _, id := range order {
		s, ok := bookIDs[id]
		if !ok {
			s, ok = wikiIDs[id]
		}
		if !ok {
			continue
		}
		out = append(out, hitFromSection(s, scores[id]))
		if len(out) == top {
			break
		}
	}
	return out
}

// lexicalChunkHits is the degradation path: term overlap, deterministic order.
func lexicalChunkHits(query string, sections []chunkSection, top int) []chunkHit {
	terms := strings.Fields(strings.ToLower(query))
	type scored struct {
		s     chunkSection
		score int
	}
	var rows []scored
	for _, s := range sections {
		low := strings.ToLower(s.Text)
		score := 0
		for _, term := range terms {
			if len(term) < 2 {
				continue
			}
			score += strings.Count(low, term)
		}
		if score > 0 {
			rows = append(rows, scored{s: s, score: score})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if rows[i].s.Path != rows[j].s.Path {
			return rows[i].s.Path < rows[j].s.Path
		}
		return rows[i].s.Start < rows[j].s.Start
	})
	if len(rows) > top {
		rows = rows[:top]
	}
	out := make([]chunkHit, 0, len(rows))
	for _, r := range rows {
		out = append(out, hitFromSection(r.s, float64(r.score)))
	}
	return out
}

func indexChunkSections(in []chunkSection) map[string]chunkSection {
	out := make(map[string]chunkSection, len(in))
	for _, s := range in {
		out[s.ID] = s
	}
	return out
}

func hitFromSection(s chunkSection, score float64) chunkHit {
	locator := s.Path + "#" + s.Anchor
	if s.Scope == "book" {
		locator = fmt.Sprintf("%s@%s:%d-%d", s.Path, s.SHA, s.Start, s.End)
	}
	return chunkHit{
		Path:    s.Path,
		Anchor:  s.Anchor,
		Title:   s.Title,
		Scope:   s.Scope,
		Start:   s.Start,
		End:     s.End,
		SHA:     s.SHA,
		Locator: locator,
		Score:   score,
		Snippet: strings.TrimSpace(s.Text),
	}
}

// chunkSnippet caps the returned text at a token budget (0 = whole section).
func chunkSnippet(text string, budget int) string {
	text = strings.TrimSpace(text)
	if budget <= 0 || text == "" {
		return text
	}
	if CountTokens(text, familyCL100K) <= budget {
		return text
	}
	words := strings.Fields(text)
	lo, hi, best := 0, len(words), 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if CountTokens(strings.Join(words[:mid], " "), familyCL100K) <= budget {
			best = mid
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	if best == 0 {
		return "…"
	}
	return strings.Join(words[:best], " ") + " …"
}

// cleanChunkText removes conversion bookmarks and collapses runs of blank
// lines, so the embedded text and the snippet are prose rather than markup.
func cleanChunkText(text string) string {
	text = conversionAnchor.ReplaceAllString(text, "")
	blank := regexp.MustCompile(`\n{3,}`)
	return strings.TrimSpace(blank.ReplaceAllString(text, "\n\n"))
}

func chunkSHA8(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}

func init() {
	chunkSearchCmd.Flags().Int("top", 8, "Number of results")
	chunkSearchCmd.Flags().Int("levels", 3, "Deepest heading level treated as a unit")
	chunkSearchCmd.Flags().Bool("all-units", false, "Include front/back matter (excluded by default)")
	chunkSearchCmd.Flags().Bool("with-wiki", false, "Also rank wiki sections from the persisted snapshot, read-only")
	chunkSearchCmd.Flags().Int("budget", 120, "Snippet token budget (0 = whole section)")
	chunkSearchCmd.Flags().String("model", "", "Embedding model: BASE2M|BASE8M|MULTILINGUAL128M (default BASE8M)")
	chunkCmd.AddCommand(chunkSearchCmd)
	rootCmd.AddCommand(chunkCmd)
}
