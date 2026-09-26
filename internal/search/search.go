// Package search wraps bleve fulltext search behind a pure-Go, CGO-free
// package boundary for the sdt viewer search index (ADR-0005). It provides an
// in-memory index built at serve startup over the markdown corpus, queried via
// ranked text results with kind/date filters.
package search

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/sandrolain/sdt/internal/contextwiki"
	corpuspkg "github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/mdstruct"
	"github.com/sandrolain/sdt/internal/semantic"
)

// Index is an in-memory bleve fulltext index over the corpus markdown.
type Index struct {
	idx      bleve.Index
	registry map[string]doc
	// sections holds the section metadata per document path, keyed by the
	// stable section id `path#anchor`, for hybrid fusion and `show`-style reads.
	sections map[string]SectionMeta
}

// SectionMeta is the section descriptor retained for hybrid fusion.
type SectionMeta struct {
	ID     string
	Path   string
	Anchor string
	Text   string // embedding recipe text (title + summary + heading path + body)
}

// Result is one ranked search hit returned to the caller.
type Result struct {
	// Section is the matched section id (`<anchor>`), when section-level hits
	// are enabled; empty for whole-document hits.
	Section   string   `json:"section,omitempty"`
	Path      string   `json:"path"`
	Kind      string   `json:"kind,omitempty"`
	Status    string   `json:"status,omitempty"`
	Title     string   `json:"title,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Objective string   `json:"objective,omitempty"`
	Topics    []string `json:"topics,omitempty"`
	Entities  []string `json:"entities,omitempty"`
	Created   string   `json:"created,omitempty"`
	Modified  string   `json:"modified,omitempty"`
	Score     float64  `json:"score"`
	Snippet   string   `json:"snippet"`
	IsMap     bool     `json:"isMap,omitempty"`
	MapID     string   `json:"mapId,omitempty"`
	IsMermaid bool     `json:"isMermaid,omitempty"`
	MermaidID string   `json:"mermaidId,omitempty"`
	IsCanvas  bool     `json:"isCanvas,omitempty"`
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
	Name        string
	Kind        string
	Status      string
	Title       string
	Summary     string
	Objective   string
	Topics      []string
	Entities    []string
	Body        string
	Frontmatter string
	CreatedDays int64
	RawCreated  string
	Modified    string
}

var (
	datePrefixRe = regexp.MustCompile(`^\d{8}-\d{6}-`)
	nameSepRe    = regexp.MustCompile(`[-_.]+`)
)

// docName derives the searchable filename stem: base name without its
// extension, leading `YYYYMMDD-HHMMSS-` date prefix stripped and `-_.`
// separators normalized to single spaces.
func docName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = datePrefixRe.ReplaceAllString(base, "")
	return strings.TrimSpace(nameSepRe.ReplaceAllString(base, " "))
}

// buildIndexMapping returns the bleve mapping for corpus-md docs.
func buildIndexMapping() (mapping.IndexMapping, error) {
	im := bleve.NewIndexMapping()
	dm := bleve.NewDocumentMapping()
	dm.AddFieldMappingsAt("Kind", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Status", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Path", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Name", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("Title", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("Summary", bleve.NewTextFieldMapping())
	dm.AddFieldMappingsAt("Objective", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Topics", bleve.NewKeywordFieldMapping())
	dm.AddFieldMappingsAt("Entities", bleve.NewKeywordFieldMapping())
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

// canvasKind is the synthesized kind of `.canvas` board resources.
const canvasKind = "canvas"

// New builds an in-memory bleve index over the markdown corpus under
// <root>/context, skipping the shared corpus exclusions. .canvas files are
// never searchable. Doc IDs and result paths are project-root-relative
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
	start := time.Now()
	corpusPath := filepath.Join(root, corpusDirName)
	indexed, err := ix.indexCorpus(corpusPath)
	if err != nil {
		ix.closeLog()
		return nil, err
	}
	log.Printf("sdtviewer: search index built in %s — %d docs (corpus %s)",
		time.Since(start).Round(time.Millisecond), indexed, corpusPath)
	return ix, nil
}

// NewFromEntries builds an in-memory bleve index from the shared mdindex entry
// set, so the CLI and the viewer index exactly the same derived documents.
// Entries reused from the manifest cache carry no body/name (they are not
// cached), so LoadBody refreshes them from disk before indexing. Section
// metadata is derived with the same fence-aware splitter, for hybrid fusion.
func NewFromEntries(entries []*mdindex.Entry) (*Index, error) {
	ix, err := newMemIndex()
	if err != nil {
		return nil, err
	}
	return buildFromEntries(ix.idx, entries, true)
}

// buildFromEntries populates a bleve index and the serving registry/sections
// from the shared entry set. When indexDocs is true each document is written
// into idx (used for in-memory and fresh stores); when false only the serving
// structures are derived from entries (used when opening a prebuilt store whose
// documents are already on disk).
// planObjectivesByID maps each plan entry id to its objective, so a task entry
// can inherit the objective of the plan it sources (the task's own legacy
// `objective`, if any, is ignored).
func planObjectivesByID(entries []*mdindex.Entry) map[string]string {
	m := map[string]string{}
	for _, e := range entries {
		if e != nil && e.Kind == "plan" && e.Objective != "" {
			m[e.ID] = e.Objective
		}
	}
	return m
}

// docForEntry derives the search doc for an entry, applying the task-objective
// inheritance rule (tasks read their plan's objective).
func docForEntry(e *mdindex.Entry, planObjective map[string]string) doc {
	d := docFromEntry(e)
	if e.Kind == "tasks" {
		d.Objective = planObjective[e.PlanRef]
	}
	return d
}

func buildFromEntries(idx bleve.Index, entries []*mdindex.Entry, indexDocs bool) (*Index, error) {
	ix := &Index{idx: idx, registry: map[string]doc{}, sections: map[string]SectionMeta{}}
	// A task inherits its plan's objective: the indexer recomputes it from the
	// entry set so both fresh and cached manifests agree (never the task's own
	// legacy `objective`).
	planObjective := planObjectivesByID(entries)
	for _, e := range entries {
		if err := e.LoadBody(); err != nil {
			continue // unreadable doc is skipped, never fatal
		}
		d := docForEntry(e, planObjective)
		if indexDocs {
			if err := ix.addDoc(d); err != nil {
				return nil, err
			}
		} else {
			ix.registry[e.ID] = d
		}
		// Section metadata/embeddings are markdown-only: viewable .canvas/.mmd
		// resources carry raw text, not headings.
		if filepath.Ext(e.ID) == contextwiki.MarkdownExt {
			ix.addSections(e)
		}
	}
	return ix, nil
}

// addSections derives the embeddable sections of a document and stores their
// metadata keyed by the stable id `path#anchor`.
func (ix *Index) addSections(e *mdindex.Entry) {
	for _, s := range mdstruct.SplitSections(e.Body) {
		if s.Level == 0 && strings.TrimSpace(s.Body) == "" {
			continue // empty preamble: nothing to embed
		}
		anchor := s.ID
		id := e.ID + "#" + anchor
		ix.sections[id] = SectionMeta{
			ID:     id,
			Path:   e.ID,
			Anchor: anchor,
			Text:   sectionRecipe(e, s),
		}
	}
}

// sectionRecipe composes the embedding text of a section: document title,
// summary and the section heading plus its body (recipe v1).
func sectionRecipe(e *mdindex.Entry, s mdstruct.Section) string {
	parts := []string{}
	if e.Title != "" {
		parts = append(parts, e.Title)
	}
	if e.Summary != "" {
		parts = append(parts, e.Summary)
	}
	if s.Heading != "" {
		parts = append(parts, s.Heading)
	}
	parts = append(parts, strings.TrimSpace(s.Body))
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// Sections returns the embeddable sections of the index (stable order).
func (ix *Index) Sections() []SectionMeta {
	out := make([]SectionMeta, 0, len(ix.sections))
	for _, s := range ix.sections {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// addDoc indexes one derived document into the in-memory index and registry.
func (ix *Index) addDoc(d doc) error {
	ix.registry[d.Path] = d
	return ix.idx.Index(d.Path, d)
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
	return &Index{idx: idx, registry: map[string]doc{}, sections: map[string]SectionMeta{}}, nil
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
	ix.applyTaskObjectives()
	return len(ix.registry), nil
}

// applyTaskObjectives recomputes each task's inherited objective from the plan
// it sources (never the task's own legacy `objective`), so legacy
// corpus-walking indexes match the entry-based path (NewFromEntries/store).
func (ix *Index) applyTaskObjectives() {
	plans := map[string]string{}
	for id, d := range ix.registry {
		if d.Kind == "plan" && d.Objective != "" {
			plans[id] = d.Objective
		}
	}
	for id, d := range ix.registry {
		if d.Kind != "tasks" {
			continue
		}
		d.Objective = plans[taskPlanRefFromRegistry(d)]
		ix.registry[id] = d
		if ierr := ix.idx.Index(id, d); ierr != nil {
			log.Printf("sdtviewer: search index %s: %v", id, ierr)
		}
	}
}

// taskPlanRefFromRegistry resolves the plan corpus id for a task registry doc
// from the plan/ reference recorded in its frontmatter.
func taskPlanRefFromRegistry(d doc) string {
	for _, ref := range contextwiki.FrontmatterList(d.Frontmatter, "sources") {
		if r := normalizePlanRef(ref); r != "" {
			return r
		}
	}
	for _, ref := range contextwiki.FrontmatterList(d.Frontmatter, "links") {
		if r := normalizePlanRef(ref); r != "" {
			return r
		}
	}
	return ""
}

func normalizePlanRef(ref string) string {
	clean := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(ref), "./"), contextwiki.MarkdownExt)
	if clean == "" {
		return ""
	}
	if !strings.HasPrefix(clean, corpusDirName+"/") {
		clean = corpusDirName + "/" + clean
	}
	clean += contextwiki.MarkdownExt
	if !strings.Contains(clean, "/plan/") {
		return ""
	}
	return clean
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
		if _, ok := auxKind(d.Name()); !ok {
			return nil
		}
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
	d := doc{Path: docID, Name: docName(docID), Body: content}
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
		case strings.HasPrefix(line, "status:"):
			d.Status = strings.Trim(trimmedValue(line, "status"), `"`)
		case strings.HasPrefix(line, "objective:"):
			d.Objective = strings.Trim(trimmedValue(line, "objective"), `"`)
		case strings.HasPrefix(line, "created:"):
			d.RawCreated = strings.Trim(trimmedValue(line, "created"), `"`)
			d.CreatedDays = parseCreatedDays(d.RawCreated)
		case strings.HasPrefix(line, "updated:"):
			d.Modified = strings.Trim(trimmedValue(line, "updated"), `"`)
		}
	}
	// Modified defaults to the file modification time when the frontmatter
	// carries no updated date, mirroring the tree's date semantics.
	if d.Modified == "" {
		if info, serr := os.Stat(path); serr == nil {
			d.Modified = info.ModTime().UTC().Format(time.RFC3339)
		}
	}
	d.Topics = contextwiki.FrontmatterList(content, "topics")
	d.Entities = contextwiki.FrontmatterList(content, "entities")
	// Non-markdown viewable resources (.canvas/.mmd): synthesize kind from the
	// extension and use the raw file text as the searchable body.
	if kind, ok := auxKind(docID); ok {
		d.Kind = kind
		d.Title = strings.TrimSuffix(filepath.Base(docID), filepath.Ext(docID))
		d.Body = content
	}
	return d, nil
}

// auxKind maps a non-markdown corpus extension to its synthesized kind.
func auxKind(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".canvas":
		return canvasKind, true
	case contextwiki.MermaidSuffix:
		return "mermaid", true
	default:
		return "", false
	}
}

// DocFromEntry builds a search doc from a shared mdindex entry, so the CLI and
// the viewer index the same derived document model.
func docFromEntry(e *mdindex.Entry) doc {
	d := doc{
		Path:      e.ID,
		Name:      e.Name,
		Kind:      e.Kind,
		Status:    e.Status,
		Title:     e.Title,
		Summary:   e.Summary,
		Objective: e.Objective,
		Topics:    e.Topics,
		Entities:  e.Entities,
		Body:      e.Body,
	}
	d.RawCreated = e.Created
	d.CreatedDays = parseCreatedDays(e.Created)
	if e.Updated != "" {
		d.Modified = e.Updated
	} else if e.ModTimeNS > 0 {
		d.Modified = time.Unix(0, e.ModTimeNS).UTC().Format(time.RFC3339)
	}
	return d
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

// Search runs a fulltext query with optional kind, objective and from/to date
// filters, returning up to max ranked hits. Query terms drive a match query; a
// non-empty issue in the query is treated as an empty result set (never an
// error).
func (ix *Index) Search(q, kind, objective, status, topic, from, to string, max int) (Results, error) {
	if max <= 0 || max > 100 {
		max = 20
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return Results{Results: []Result{}}, nil
	}
	// Filename-aware scoring: a document whose filename matches the query
	// outranks body-only matches. A disjunction of boosted sub-queries drives
	// relevance while the filters below stay conjunctive (must).
	namePhrase := bleve.NewMatchPhraseQuery(q)
	namePhrase.SetField("Name")
	namePhrase.SetBoost(8)
	nameMatch := bleve.NewMatchQuery(q)
	nameMatch.SetField("Name")
	nameMatch.SetBoost(4)
	titleMatch := bleve.NewMatchQuery(q)
	titleMatch.SetField("Title")
	titleMatch.SetBoost(2)
	bodyMatch := bleve.NewMatchQuery(q)
	bodyMatch.SetBoost(1)

	scored := bleve.NewDisjunctionQuery(namePhrase, nameMatch, titleMatch, bodyMatch)
	must := []query.Query{scored}

	if kind != "" {
		kindQ := bleve.NewTermQuery(kind)
		kindQ.SetField("Kind")
		must = append(must, kindQ)
	}
	if objective != "" {
		objQ := bleve.NewTermQuery(objective)
		objQ.SetField("Objective")
		must = append(must, objQ)
	}
	if status != "" {
		stQ := bleve.NewTermQuery(status)
		stQ.SetField("Status")
		must = append(must, stQ)
	}
	if topic != "" {
		tpQ := bleve.NewTermQuery(topic)
		tpQ.SetField("Topics")
		must = append(must, tpQ)
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
		isMermaid := contextwiki.IsMermaidDoc(doc.Path)
		out = append(out, Result{
			Path:      doc.Path,
			Kind:      doc.Kind,
			Status:    doc.Status,
			Title:     doc.Title,
			Summary:   doc.Summary,
			Objective: doc.Objective,
			Topics:    doc.Topics,
			Entities:  doc.Entities,
			Created:   doc.RawCreated,
			Modified:  doc.Modified,
			Score:     hit.Score,
			Snippet:   Snippet(doc, q, 160),
			IsMap:     isMap,
			MapID:     mapID(doc.Path, isMap),
			IsMermaid: isMermaid,
			MermaidID: mermaidID(doc.Path, isMermaid),
			IsCanvas:  doc.Kind == canvasKind,
		})
	}
	total := sr.Total
	if total > math.MaxInt64 {
		total = math.MaxInt64
	}
	return Results{Results: out, Total: int64(total)}, nil
}

// HybridOptions carries the optional semantic branch for SearchHybrid. When
// Semantic is nil the result equals the lexical Search (graceful degradation).
type HybridOptions struct {
	Semantic *semantic.Index
	// SemanticK is the number of semantic sections fetched before fusion.
	SemanticK int
}

// SemanticSections returns the embeddable sections of the index, for building
// the semantic collection.
func (ix *Index) SemanticSections() []semantic.Section {
	out := make([]semantic.Section, 0, len(ix.sections))
	for _, s := range ix.sections {
		out = append(out, semantic.Section{
			ID:     s.ID,
			Path:   s.Path,
			Anchor: s.Anchor,
			Text:   s.Text,
			Meta:   map[string]string{"path": s.Path, "section": s.Anchor},
		})
	}
	return out
}

// SearchHybrid runs the lexical search and, when a semantic index is supplied,
// fuses the two rankings with RRF. Filters apply to the lexical branch; the
// semantic branch is ranked and then fused, so its hits outside the filter set
// are dropped when a filter is active.
func (ix *Index) SearchHybrid(ctx context.Context, q HybridQuery, opts HybridOptions) (Results, error) {
	lexical, err := ix.Search(q.Q, q.Kind, q.Objective, q.Status, q.Topic, q.From, q.To, q.Max)
	if err != nil || opts.Semantic == nil || !opts.Semantic.Available() {
		if err != nil {
			return lexical, err
		}
		return lexical, nil
	}
	k := opts.SemanticK
	if k <= 0 {
		k = 50
	}
	hits, err := opts.Semantic.Query(ctx, q.Q, k)
	if err != nil {
		return lexical, nil // semantic failure degrades to lexical-only
	}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	fused := fuseHybrid(lexical.Results, ids)
	// Enrich semantic-only hits (no lexical counterpart) with their registry
	// display fields so every result is presented consistently.
	for i := range fused {
		if fused[i].Kind != "" || fused[i].Summary != "" {
			continue
		}
		if d, ok := ix.registry[fused[i].Path]; ok {
			fused[i].Kind = d.Kind
			fused[i].Status = d.Status
			fused[i].Title = d.Title
			fused[i].Summary = d.Summary
			fused[i].Objective = d.Objective
			fused[i].Topics = d.Topics
			fused[i].Entities = d.Entities
			fused[i].Created = d.RawCreated
			fused[i].Modified = d.Modified
			fused[i].Snippet = Snippet(d, q.Q, 160)
			fused[i].IsMap = contextwiki.IsMapDoc(d.Path)
			fused[i].MapID = mapID(d.Path, fused[i].IsMap)
			fused[i].IsMermaid = contextwiki.IsMermaidDoc(d.Path)
			fused[i].MermaidID = mermaidID(d.Path, fused[i].IsMermaid)
			fused[i].IsCanvas = d.Kind == canvasKind
		}
	}
	// Filters apply to the lexical branch; drop semantic-only hits that fall
	// outside the same filter set so the fused ranking honors the contract of
	// the lexical branch.
	if f := hybridFilter(q); f != nil {
		kept := fused[:0]
		for i := range fused {
			if d, ok := ix.registry[fused[i].Path]; !ok || f(d.Path, &d) {
				kept = append(kept, fused[i])
			}
		}
		fused = kept
	}
	if len(fused) > q.Max && q.Max > 0 {
		fused = fused[:q.Max]
	}
	return Results{Results: fused, Total: lexical.Total}, nil
}

// hybridFilter mirrors the conjunctive filters of Search against a doc's
// registry facet values, so semantic-only fused hits are held to the same
// filter contract as the lexical branch. Returns nil when no filter is active.
func hybridFilter(q HybridQuery) func(path string, d *doc) bool {
	if q.Kind == "" && q.Objective == "" && q.Status == "" && q.Topic == "" && q.From == "" && q.To == "" {
		return nil
	}
	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var loDays, hiDays *int64
	if t, err := time.Parse("2006-01-02", q.From); err == nil {
		v := int64(t.Sub(epoch).Hours() / 24)
		loDays = &v
	}
	if t, err := time.Parse("2006-01-02", q.To); err == nil {
		v := int64(t.AddDate(0, 0, 1).Sub(epoch).Hours() / 24)
		hiDays = &v
	}
	return func(path string, d *doc) bool {
		if q.Kind != "" && d.Kind != q.Kind {
			return false
		}
		if q.Objective != "" && d.Objective != q.Objective {
			return false
		}
		if q.Status != "" && d.Status != q.Status {
			return false
		}
		if q.Topic != "" && !slices.Contains(d.Topics, q.Topic) {
			return false
		}
		if loDays != nil && d.CreatedDays < *loDays {
			return false
		}
		if hiDays != nil && d.CreatedDays >= *hiDays {
			return false
		}
		return true
	}
}

// HybridQuery is the filter/size set shared by lexical and hybrid search.
type HybridQuery struct {
	Q         string
	Kind      string
	Objective string
	Status    string
	Topic     string
	From      string
	To        string
	Max       int
}

// mapID returns the canonical map id for map documents, "" otherwise.
func mapID(path string, isMap bool) string {
	if !isMap {
		return ""
	}
	return contextwiki.DocID(path)
}

// mermaidID returns the canonical id for mermaid documents, "" otherwise.
func mermaidID(path string, isMermaid bool) string {
	if !isMermaid {
		return ""
	}
	return contextwiki.MermaidID(path)
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
