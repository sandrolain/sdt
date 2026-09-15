// Package search wraps bleve fulltext search behind a pure-Go, CGO-free
// package boundary for the sdt viewer search index (ADR-0005). It provides an
// in-memory index built at serve startup over the markdown corpus, queried via
// ranked text results with kind/date filters.
package search

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/sandrolain/sdt/internal/contextwiki"
	corpuspkg "github.com/sandrolain/sdt/internal/corpus"
)

// Index is an in-memory bleve fulltext index over the corpus markdown.
type Index struct {
	idx      bleve.Index
	registry map[string]doc
}

// Result is one ranked search hit returned to the caller.
type Result struct {
	Path    string  `json:"path"`
	Kind    string  `json:"kind,omitempty"`
	Title   string  `json:"title,omitempty"`
	Summary string  `json:"summary,omitempty"`
	Created string  `json:"created,omitempty"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
	IsMap   bool    `json:"isMap,omitempty"`
	MapID   string  `json:"mapId,omitempty"`
}

// Results is the response body for /api/search.
type Results struct {
	Results []Result `json:"results"`
	Total   int64    `json:"total"`
}

// doc is the indexing representation of a markdown corpus file. The bleve doc
// id is the corpus-relative path, and the same path keys the registry that
// restores display fields for search hits.
type doc struct {
	Path        string
	Kind        string
	Title       string
	Summary     string
	Body        string
	Frontmatter string
	CreatedDays int64
	RawCreated  string
}

// buildIndexMapping returns the bleve mapping for corpus-md docs.
func buildIndexMapping() (mapping.IndexMapping, error) {
	im := bleve.NewIndexMapping()
	dm := bleve.NewDocumentMapping()
	dm.AddFieldMappingsAt("Kind", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Path", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Title", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("Summary", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("Body", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("Frontmatter", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("CreatedDays", bleve.NewNumericFieldMapping())
	im.AddDocumentMapping("corpus-md", dm)
	im.DefaultMapping = dm
	return im, nil
}

// skipDir reports whether the directory is excluded from the corpus scan (the
// shared corpus exclusion set: tmp/, scripts/, refs/, commands/, instructions/,
// sdtdocs/).
func skipDir(name string) bool { return corpuspkg.ExcludedDirName(name) }

// corpusDirName is the served corpus subdirectory under the project root.
const corpusDirName = "context"

// New builds an in-memory bleve index over the markdown corpus under
// <root>/context, skipping tmp/, scripts/ and refs/. .canvas files are never
// searchable. Doc IDs and result paths are project-root-relative
// (context/<...>). Build time and doc count are logged. A missing root is a
// fatal error; a missing corpus yields an empty index (non-fatal). Unreadable
// docs are skipped with a warning.
func New(root string) (*Index, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("search root: %w", err)
	}
	ix, err := newMemIndex()
	if err != nil {
		return nil, err
	}
	corpusPath := filepath.Join(root, corpusDirName)
	start := time.Now()
	indexed, err := ix.indexCorpus(corpusPath)
	if err != nil {
		ix.closeLog()
		return nil, err
	}
	log.Printf("sdtviewer: search index built in %s — %d docs (corpus %s)",
		time.Since(start).Round(time.Millisecond), indexed, corpusPath)
	return ix, nil
}

// newMemIndex allocates the in-memory bleve index with the corpus mapping.
func newMemIndex() (*Index, error) {
	m, err := buildIndexMapping()
	if err != nil {
		return nil, fmt.Errorf("search mapping: %w", err)
	}
	idx, err := bleve.NewMemOnly(m)
	if err != nil {
		return nil, fmt.Errorf("bleve mem: %w", err)
	}
	return &Index{idx: idx, registry: map[string]doc{}}, nil
}

// indexCorpus walks corpus indexing .md docs (skipping tmp/, scripts/ and
// refs/) and returns the number of indexed docs. A missing corpus yields an
// empty index with nil error.
func (ix *Index) indexCorpus(dir string) (int, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("sdtviewer: search index empty (no %s)", dir)
			return 0, nil
		}
		return 0, fmt.Errorf("search corpus: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("search corpus: %s is not a directory", dir)
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		return ix.addEntry(dir, path, d, err)
	})
	if err != nil {
		return 0, fmt.Errorf("search walk: %w", err)
	}
	return len(ix.registry), nil
}

// addEntry indexes one .md file from the corpus walk, skipping excluded dirs,
// non-.md files and unreadable docs.
func (ix *Index) addEntry(dir, path string, d os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if d.IsDir() {
		if path == dir {
			return nil
		}
		if d.Name() != "." && skipDir(d.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	if filepath.Ext(d.Name()) != ".md" {
		return nil
	}
	rel, rerr := filepath.Rel(dir, path)
	if rerr != nil {
		return nil
	}
	docID := filepath.ToSlash(filepath.Join(corpusDirName, rel))
	if corpuspkg.ExcludedPath(docID) {
		return nil
	}
	cd, perr := parseDoc(docID, path)
	if perr != nil {
		log.Printf("sdtviewer: search skip %s: %v", docID, perr)
		return nil
	}
	ix.registry[docID] = cd
	if ierr := ix.idx.Index(docID, cd); ierr != nil {
		log.Printf("sdtviewer: search index %s: %v", docID, ierr)
	}
	return nil
}

// closeLog releases the mem index; a close failure is logged, not fatal.
func (ix *Index) closeLog() {
	if cerr := ix.idx.Close(); cerr != nil {
		log.Printf("sdtviewer: search index close: %v", cerr)
	}
}

// parseDoc reads and parses one glyph of the corpus into a doc.
func parseDoc(docID, path string) (doc, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus walk target
	if err != nil {
		return doc{}, err
	}
	content := string(data)
	d := doc{Path: docID, Body: content}
	if i := strings.Index(content, fmStart); i == 0 {
		end := strings.Index(content[len(fmStart):], fmEnd)
		if end > 0 {
			fmEndIdx := len(fmStart) + end + len(fmEnd)
			d.Frontmatter = content[:fmEndIdx]
			d.Body = content[fmEndIdx:]
		}
	}
	lines := strings.Split(d.Frontmatter, "\n")
	for i, line := range lines {
		if i == 0 {
			continue // opening "---"
		}
		trim := strings.TrimSpace(line)
		if trim == fmDelim {
			break
		}
		switch {
		case strings.HasPrefix(line, "kind:"):
			d.Kind = strings.Trim(trimmedValue(line, "kind"), `"`)
		case strings.HasPrefix(line, "title:"):
			d.Title = strings.Trim(trimmedValue(line, "title"), `"`)
		case strings.HasPrefix(line, "summary:"):
			d.Summary = strings.Trim(trimmedValue(line, "summary"), `"`)
		case strings.HasPrefix(line, "created:"):
			d.RawCreated = strings.Trim(trimmedValue(line, "created"), `"`)
			d.CreatedDays = parseCreatedDays(d.RawCreated)
		}
	}
	return d, nil
}

const (
	fmStart = "---\n"
	fmEnd   = "\n---"
	fmDelim = "---"
)

// trimmedValue returns the YAML inline value for key on line "key: value".
func trimmedValue(line, key string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
}

// parseCreatedDays converts a created date to days since 2000-01-01 UTC for
// numeric range filtering; returns 0 on parse failure.
func parseCreatedDays(raw string) int64 {
	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return int64(t.Sub(epoch).Hours() / 24)
		}
	}
	return 0
}

// Search runs a fulltext query with optional kind and from/to date filters,
// returning up to max ranked hits. Query terms drive a match query; a non-empty
// issue in the query is treated as an empty result set (never an error).
func (ix *Index) Search(q, kind, from, to string, max int) (Results, error) {
	if max <= 0 || max > 100 {
		max = 20
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return Results{Results: []Result{}}, nil
	}
	base := bleve.NewMatchQuery(q)
	must := []query.Query{base}

	if kind != "" {
		kindQ := bleve.NewTermQuery(kind)
		kindQ.SetField("Kind")
		must = append(must, kindQ)
	}
	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			lo := float64(int64(t.Sub(epoch).Hours() / 24))
			rangeQ := bleve.NewNumericRangeQuery(&lo, nil)
			rangeQ.SetField("CreatedDays")
			must = append(must, rangeQ)
		}
	}
	if to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			hi := float64(int64(t.AddDate(0, 0, 1).Sub(epoch).Hours() / 24))
			rangeQ := bleve.NewNumericRangeQuery(nil, &hi)
			rangeQ.SetField("CreatedDays")
			must = append(must, rangeQ)
		}
	}

	qry := query.NewBooleanQuery(must, nil, nil)
	req := bleve.NewSearchRequest(qry)
	req.Size = max
	req.SortBy([]string{"-_score"})
	sr, err := ix.idx.Search(req)
	if err != nil {
		return Results{}, fmt.Errorf("bleve search: %w", err)
	}
	out := make([]Result, 0, len(sr.Hits))
	for _, hit := range sr.Hits {
		doc, ok := ix.registry[hit.ID]
		if !ok {
			continue
		}
		isMap := contextwiki.IsMapDoc(doc.Path)
		out = append(out, Result{
			Path:    doc.Path,
			Kind:    doc.Kind,
			Title:   doc.Title,
			Summary: doc.Summary,
			Created: doc.RawCreated,
			Score:   hit.Score,
			Snippet: Snippet(doc, q, 160),
			IsMap:   isMap,
			MapID:   mapID(doc.Path, isMap),
		})
	}
	total := sr.Total
	if total > math.MaxInt64 {
		total = math.MaxInt64
	}
	return Results{Results: out, Total: int64(total)}, nil
}

// mapID returns the canonical map id for map documents, "" otherwise.
func mapID(path string, isMap bool) string {
	if !isMap {
		return ""
	}
	return contextwiki.DocID(path)
}

// Snippet extracts a context window around the first match of the query terms
// in a doc, preferring summary then body then frontmatter. A trimmed prefix of
// no-match fields is returned as a fallback so hits always carry a snippet.
func Snippet(d doc, q string, n int) string {
	if n <= 0 {
		return ""
	}
	terms := strings.Fields(strings.ToLower(q))
	if snip := matchWindow(d.Summary, terms, n); snip != "" {
		return snip
	}
	if snip := matchWindow(d.Body, terms, n); snip != "" {
		return snip
	}
	if snip := matchWindow(d.Frontmatter, terms, n); snip != "" {
		return snip
	}
	return clampText(strings.TrimSpace(d.Summary)+" "+strings.TrimSpace(strings.ReplaceAll(d.Body, "\n", " ")), n)
}

// matchWindow returns n runes around the first occurrence of any term, trimmed.
func matchWindow(text string, terms []string, n int) string {
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	pos := -1
	for _, t := range terms {
		if p := strings.Index(lower, t); p >= 0 && (pos < 0 || p < pos) {
			pos = p
		}
	}
	if pos < 0 {
		return ""
	}
	runes := []rune(text)
	begin := len([]rune(text[:pos])) - n/2
	if begin < 0 {
		begin = 0
	}
	end := begin + n
	if end > len(runes) {
		end = len(runes)
	}
	return strings.TrimSpace(string(runes[begin:end]))
}

// clampText returns the first n runes of text, ellipsized when truncated.
func clampText(text string, n int) string {
	if n <= 0 || text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= n {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(runes[:n])) + "…"
}

// Close releases the underlying index.
func (ix *Index) Close() error {
	if ix == nil || ix.idx == nil {
		return nil
	}
	return ix.idx.Close()
}
