// Package mdindex holds the shared, derived section index over the context/
// corpus: the manifest that drives incremental rebuilds, the section model
// produced by internal/mdstruct, and the facet metadata (kind/status/topics/
// entities/objective/dates) shared by the agent CLI and the viewer search.
//
// The source of truth stays the markdown files: everything here is derived and
// rebuildable. Internal/search owns the bleve engine and consumes this package
// for the document set, facets and change detection, so a single scan feeds
// both consumers.
package mdindex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	corpuspkg "github.com/sandrolain/sdt/internal/corpus"
	"github.com/sandrolain/sdt/internal/mdstruct"
)

// ManifestVersion invalidates a cached manifest when the scan model changes.
const ManifestVersion = 1

// ContextDir is the served knowledge base subdirectory under the project root.
const ContextDir = "context"

// Entry is one indexed document with its derived facets.
type Entry struct {
	// ID is the project-root-relative path (context/...), the stable doc id.
	ID string `json:"id"`
	// AbsPath is the filesystem path of the document. It is not cached; SetRoot
	// re-derives it from the project root when a manifest is loaded.
	AbsPath string `json:"-"`
	// Kind, Status, Objective, Title, Summary are frontmatter facets.
	Kind      string   `json:"kind,omitempty"`
	Status    string   `json:"status,omitempty"`
	Objective string   `json:"objective,omitempty"`
	Title     string   `json:"title,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Topics    []string `json:"topics,omitempty"`
	Entities  []string `json:"entities,omitempty"`
	// Created is the raw frontmatter created value; Updated likewise.
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
	// Hash is the content hash; Size and ModTimeNS drive cheap change checks.
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
	ModTimeNS int64  `json:"modtime_ns"`
	// Sections is the fence-aware section split (ids are stable per document).
	Sections []SectionRef `json:"sections,omitempty"`
	// Body is the markdown body after the frontmatter. It is not cached (the
	// source of truth is the file) and is re-read on demand.
	Body string `json:"-"`
	// Name is the searchable filename stem (derived, not cached).
	Name string `json:"-"`
	// Indexed marks an entry whose Body has been loaded for indexing.
	Indexed bool `json:"-"`
}

// LoadBody reads and caches the document body from disk, so callers that need
// content (search indexing, snippets) work with entries reused from the cache.
// Body-derived fields (Name) are refreshed too. A missing/unreadable file
// leaves the entry without a body and is not fatal.
func (e *Entry) LoadBody() error {
	if e.Indexed {
		return nil
	}
	if e.AbsPath == "" {
		return fmt.Errorf("mdindex: %s has no path", e.ID)
	}
	data, err := os.ReadFile(e.AbsPath) //#nosec G304 -- derived from a corpus scan
	if err != nil {
		return err
	}
	_, body := contextwiki.SplitFrontmatter(string(data))
	e.Body = body
	e.Name = docName(e.ID)
	e.Indexed = true
	return nil
}

// SectionRef is the lightweight section descriptor stored in the manifest.
type SectionRef struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
}

// Manifest is the derived, machine-readable index state for a corpus. It is
// safe to delete: everything is rebuildable from the markdown sources.
type Manifest struct {
	Version   int               `json:"version"`
	ScannedAt time.Time         `json:"scanned_at"`
	Entries   map[string]*Entry `json:"entries"`
	facets    *Facets           `json:"-"`
}

// Facets are the distinct facet values observed in a scan, used to build
// filter dropdowns and to report index coverage.
type Facets struct {
	Kinds      []string `json:"kinds"`
	Statuses   []string `json:"statuses"`
	Objectives []string `json:"objectives"`
	Topics     []string `json:"topics"`
	Entities   []string `json:"entities"`
}

// ScanResult is the output of an incremental scan.
type ScanResult struct {
	Manifest     *Manifest
	Changed      []string // doc ids added or whose content changed
	Removed      []string // doc ids no longer present
	Unchanged    int
	ScannedCount int
}

// corpusPrefix is the project-root-relative prefix of a corpus document.
func corpusPrefix(root string) string { return filepath.Join(root, ContextDir) }

// Scan walks the corpus under root/context, skipping the shared exclusions, and
// returns the manifest plus the set of documents that changed relative to prev
// (a nil prev forces a full rebuild). A missing corpus yields an empty manifest.
func Scan(root string, prev *Manifest) (*ScanResult, error) {
	dir := corpusPrefix(root)
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("mdindex root: %w", err)
	}
	m := &Manifest{Version: ManifestVersion, ScannedAt: time.Now().UTC(), Entries: map[string]*Entry{}}
	res := &ScanResult{Manifest: m}
	var prevEntries map[string]*Entry
	if prev != nil && prev.Version == ManifestVersion {
		prevEntries = prev.Entries
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, werr error) error {
		return scanEntry(root, dir, path, d, werr, prevEntries, m, res)
	})
	if err != nil {
		return nil, fmt.Errorf("mdindex scan: %w", err)
	}

	for id := range prevEntries {
		if _, ok := m.Entries[id]; !ok {
			res.Removed = append(res.Removed, id)
		}
	}
	sort.Strings(res.Changed)
	sort.Strings(res.Removed)
	m.facets = collectFacets(m.Entries)
	return res, nil
}

// scanEntry handles one walk entry: directories are filtered against the shared
// exclusions, markdown files are parsed or reused from prev when unchanged.
// Excluded, non-markdown and unreadable entries are skipped, never fatal.
func scanEntry(root, dir, path string, d fs.DirEntry, werr error, prev map[string]*Entry, m *Manifest, res *ScanResult) error {
	if werr != nil {
		return werr
	}
	if d.IsDir() {
		if path != dir && d.Name() != "." && corpuspkg.ExcludedDirName(d.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	if filepath.Ext(d.Name()) != ".md" {
		return nil
	}
	id := walkID(root, path)
	if id == "" || corpuspkg.ExcludedPath(id) {
		return nil
	}
	info, ierr := d.Info()
	if ierr != nil {
		return nil
	}
	res.ScannedCount++
	if old, ok := prev[id]; ok && !changed(old, info) {
		m.Entries[id] = old
		res.Unchanged++
		return nil
	}
	entry, perr := parseEntry(root, id, path)
	if perr != nil {
		return nil
	}
	m.Entries[id] = entry
	res.Changed = append(res.Changed, id)
	return nil
}

// walkID returns the project-root-relative slash id for a walked path, or ""
// when the path is outside the root.
func walkID(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// changed reports whether a file differs from its cached entry cheaply: size or
// mtime divergence forces a re-parse (content hash is the tie-breaker).
func changed(old *Entry, info fs.FileInfo) bool {
	return old.Size != info.Size() || old.ModTimeNS != info.ModTime().UnixNano()
}

// parseEntry reads and derives one document entry.
func parseEntry(root, id, path string) (*Entry, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus walk target
	if err != nil {
		return nil, err
	}
	content := string(data)
	_, body := contextwiki.SplitFrontmatter(content)
	e := &Entry{
		ID:        id,
		AbsPath:   path,
		Kind:      contextwiki.FrontmatterField(content, "kind"),
		Status:    contextwiki.FrontmatterField(content, "status"),
		Objective: contextwiki.FrontmatterField(content, "objective"),
		Title:     contextwiki.FrontmatterField(content, "title"),
		Summary:   contextwiki.FrontmatterField(content, "summary"),
		Topics:    contextwiki.FrontmatterList(content, "topics"),
		Entities:  contextwiki.FrontmatterList(content, "entities"),
		Created:   contextwiki.FrontmatterField(content, "created"),
		Updated:   contextwiki.FrontmatterField(content, "updated"),
		Body:      body,
		Name:      docName(id),
	}
	e.Hash = shortHash(content)
	e.Indexed = true
	if info, serr := os.Stat(path); serr == nil {
		e.Size = info.Size()
		e.ModTimeNS = info.ModTime().UnixNano()
	}
	for _, s := range mdstruct.SplitSections(body) {
		e.Sections = append(e.Sections, SectionRef{ID: s.ID, Heading: s.Heading, Level: s.Level})
	}
	return e, nil
}

// collectFacets derives the distinct facet values from the entries.
func collectFacets(entries map[string]*Entry) *Facets {
	sets := map[string]map[string]struct{}{}
	add := func(key, v string) {
		if v == "" {
			return
		}
		if sets[key] == nil {
			sets[key] = map[string]struct{}{}
		}
		sets[key][v] = struct{}{}
	}
	for _, e := range entries {
		add("kinds", e.Kind)
		add("statuses", e.Status)
		add("objectives", e.Objective)
		for _, t := range e.Topics {
			add("topics", t)
		}
		for _, x := range e.Entities {
			add("entities", x)
		}
	}
	return &Facets{
		Kinds:      sortedSet(sets["kinds"]),
		Statuses:   sortedSet(sets["statuses"]),
		Objectives: sortedSet(sets["objectives"]),
		Topics:     sortedSet(sets["topics"]),
		Entities:   sortedSet(sets["entities"]),
	}
}

func sortedSet(s map[string]struct{}) []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// Facets returns the facet summary for this manifest.
func (m *Manifest) Facets() *Facets {
	if m == nil {
		return &Facets{}
	}
	if m.facets == nil {
		m.facets = collectFacets(m.Entries)
	}
	return m.facets
}

// EntriesSorted returns the entries ordered by doc id (stable output).
func (m *Manifest) EntriesSorted() []*Entry {
	out := make([]*Entry, 0, len(m.Entries))
	for _, e := range m.Entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Save writes the manifest as JSON, creating parent directories.
func (m *Manifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301 -- derived cache dir
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644) //#nosec G306 -- derived cache file
}

// Load reads a manifest from JSON; a missing or invalid file returns (nil, nil)
// so the caller falls back to a full rebuild.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- derived cache path
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil // corrupt cache: rebuild
	}
	if m.Version != ManifestVersion {
		return nil, nil
	}
	if m.Entries == nil {
		m.Entries = map[string]*Entry{}
	}
	return &m, nil
}

// SetRoot re-derives each entry's absolute path from the project root, so a
// manifest loaded from the cache can re-read document bodies on demand.
func (m *Manifest) SetRoot(root string) {
	if m == nil {
		return
	}
	for id, e := range m.Entries {
		if e == nil {
			continue
		}
		e.AbsPath = filepath.Join(root, filepath.FromSlash(id))
	}
}

// docName derives the searchable filename stem: base name without the `.md`
// extension, leading `YYYYMMDD-HHMMSS-` date prefix stripped, `-_.` normalized.
func docName(id string) string {
	base := strings.TrimSuffix(filepath.Base(id), ".md")
	if len(base) > 16 && base[8] == '-' && base[15] == '-' {
		base = base[16:]
	}
	repl := strings.NewReplacer("-", " ", "_", " ", ".", " ")
	return strings.TrimSpace(repl.Replace(base))
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
