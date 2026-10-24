package contextwiki

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// pageContent builds a valid wiki page body for the given id/title.
func pageContent(id, title string) string {
	return `---
kind: wiki
id: ` + id + `
title: ` + title + `
type: concept
status: active
summary: Summary for ` + title + `
tags:
  - backend
relations:
  refers_to:
    - [[db|Database]]
sources:
  - refs/spec.md
---
## Summary
Shape of ` + title + `.
## Claims
1. One fact. {#claim-1} (refs/spec.md@abc:1)
`
}

// writeFile creates parent dirs and writes the file inside root.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	fm, body := SplitFrontmatter("---\na: 1\n---\nreal\nbody\n")
	if !strings.Contains(fm, "a: 1") {
		t.Errorf("frontmatter = %q, want a: 1", fm)
	}
	if !strings.Contains(body, "body") {
		t.Errorf("body = %q, want body", body)
	}
	if fm+body != "---\na: 1\n---\nreal\nbody\n" {
		t.Errorf("split must preserve full content, got fm=%q body=%q", fm, body)
	}
	// No frontmatter: whole content is body.
	if _, body := SplitFrontmatter("no fm\n"); body != "no fm\n" {
		t.Errorf("body without fm = %q", body)
	}
	// Unclosed delimiter: nothing is body.
	if _, body := SplitFrontmatter("---\na: 1\n"); body != "" {
		t.Errorf("unclosed: body = %q", body)
	}
	// Closing delimiter at EOF without trailing newline must not panic and
	// must round-trip the full content (seen on refs/ corpus files).
	full := "---\na: 1\n---"
	fmEOF, bodyEOF := SplitFrontmatter(full)
	if !strings.Contains(fmEOF, "a: 1") || bodyEOF != "" {
		t.Errorf("eof fm=%q body=%q", fmEOF, bodyEOF)
	}
	if fmEOF+bodyEOF != full {
		t.Errorf("eof split must preserve content: %q+%q", fmEOF, bodyEOF)
	}
}

func TestParseContentFields(t *testing.T) {
	p := ParseContent(pageContent("accounts", "Account Service"), "context/wiki/accounts.md", "accounts")
	if p.ID != "accounts" || p.Title != "Account Service" || p.Kind != "wiki" {
		t.Errorf("basic fields: %+v", p)
	}
	if p.Type != "concept" || p.Status != "active" || p.Summary == "" {
		t.Errorf("enum/summary fields: %+v", p)
	}
	if p.FileID != "accounts" || p.Base != "accounts.md" {
		t.Errorf("file identity: %+v", p)
	}
	if want := 1; p.ClaimCount != want {
		t.Errorf("ClaimCount = %d, want %d", p.ClaimCount, want)
	}
	// 19 lines by `wc -l` (15 frontmatter + 4 body lines, the trailing newline
	// ending the last one rather than opening another).
	if want := 19; p.LineCount != want {
		t.Errorf("LineCount = %d, want %d", p.LineCount, want)
	}
}

func TestParseContentTitleFallback(t *testing.T) {
	content := "---\nkind: wiki\nid: x\n---\nbody\n"
	p := ParseContent(content, "context/wiki/x.md", "x")
	if p.Title != "x" {
		t.Errorf("Title fallback = %q, want fileID", p.Title)
	}
}

func TestParseFrontmatterMapTypedRelations(t *testing.T) {
	content := `---
kind: wiki
id: a
status: active
relations:
  depends_on:
    - [[b|B]]
  contains:
    - [[c|C]]
---
body
`
	p := ParseContent(content, "context/wiki/a.md", "a")
	want := map[string][]string{
		"depends_on": {"[[b|B]]"},
		"contains":   {"[[c|C]]"},
	}
	if !reflect.DeepEqual(p.Relations, want) {
		t.Errorf("Relations = %v, want %v", p.Relations, want)
	}
	// Inline value on the key line reports the unsupported inline form.
	if _, inline := ParseFrontmatterMap("relations: [x]\n", "relations"); !inline {
		t.Error("expected inline map detection")
	}
}

func TestCoverageBlock(t *testing.T) {
	valid := `---
kind: wiki
id: book
title: Book
coverage:
  units: 10
  ok: 6
  part: 2
  gap: 1
  no_op: 1
---
## Summary
Shape.
`
	c, present, err := CoverageBlock(valid)
	if !present || err != nil {
		t.Fatalf("valid block: present=%v err=%v", present, err)
	}
	want := Coverage{Units: 10, OK: 6, Part: 2, Gap: 1, NoOp: 1}
	if c != want {
		t.Errorf("Coverage = %+v, want %+v", c, want)
	}

	if _, present, err := CoverageBlock("---\nkind: wiki\n---\nbody\n"); present || err != nil {
		t.Errorf("absent block: present=%v err=%v, want false/nil", present, err)
	}

	bad := []struct{ name, fm string }{
		{"inline value", "---\ncoverage: 10\n---\n"},
		{"unknown key", "---\ncoverage:\n  units: 1\n  ok: 1\n  part: 0\n  gap: 0\n  no_op: 0\n  extra: 0\n---\n"},
		{"repeated key", "---\ncoverage:\n  units: 1\n  units: 1\n  ok: 1\n  part: 0\n  gap: 0\n  no_op: 0\n---\n"},
		{"not an integer", "---\ncoverage:\n  units: many\n  ok: 1\n  part: 0\n  gap: 0\n  no_op: 0\n---\n"},
		{"negative", "---\ncoverage:\n  units: 1\n  ok: 2\n  part: 0\n  gap: 0\n  no_op: -1\n---\n"},
		{"missing key", "---\ncoverage:\n  units: 1\n  ok: 1\n---\n"},
	}
	for _, tc := range bad {
		if _, present, err := CoverageBlock(tc.fm); !present || err == nil {
			t.Errorf("%s: present=%v err=%v, want a reported malformed block", tc.name, present, err)
		}
	}

	// The sum identity and a non-zero gap are the caller's findings, not parse
	// errors: the block parses and the caller reports them.
	_, present, err = CoverageBlock("---\ncoverage:\n  units: 10\n  ok: 6\n  part: 2\n  gap: 1\n  no_op: 0\n---\n")
	if !present || err != nil {
		t.Errorf("sum/gap findings must not be parse errors: present=%v err=%v", present, err)
	}
}

func TestConceptsBlock(t *testing.T) {
	valid := `---
kind: wiki
id: book
title: Book
concepts:
  from: notes/book-inventory.md
  candidates: 10
  paged: 6
  merged: 1
  inline: 2
  declined: 0
  pending: 1
---
## Summary
Shape.
`
	c, present, err := ConceptsBlock(valid)
	if !present || err != nil {
		t.Fatalf("valid block: present=%v err=%v", present, err)
	}
	want := Concepts{From: "notes/book-inventory.md", Candidates: 10, Paged: 6, Merged: 1, Inline: 2, Declined: 0, Pending: 1}
	if c != want {
		t.Errorf("Concepts = %+v, want %+v", c, want)
	}

	if _, present, err := ConceptsBlock("---\nkind: wiki\n---\nbody\n"); present || err != nil {
		t.Errorf("absent block: present=%v err=%v, want false/nil", present, err)
	}

	bad := []struct{ name, fm string }{
		{"inline value", "---\nconcepts: 10\n---\n"},
		{"unknown key", "---\nconcepts:\n  from: n.md\n  candidates: 1\n  paged: 1\n  merged: 0\n  inline: 0\n  declined: 0\n  pending: 0\n  extra: 0\n---\n"},
		{"repeated key", "---\nconcepts:\n  from: n.md\n  candidates: 1\n  candidates: 1\n  paged: 1\n  merged: 0\n  inline: 0\n  declined: 0\n  pending: 0\n---\n"},
		{"not an integer", "---\nconcepts:\n  from: n.md\n  candidates: many\n  paged: 1\n  merged: 0\n  inline: 0\n  declined: 0\n  pending: 0\n---\n"},
		{"negative", "---\nconcepts:\n  from: n.md\n  candidates: 1\n  paged: 1\n  merged: 0\n  inline: 0\n  declined: -1\n  pending: 0\n---\n"},
		{"missing from", "---\nconcepts:\n  candidates: 1\n  paged: 1\n  merged: 0\n  inline: 0\n  declined: 0\n  pending: 0\n---\n"},
		{"missing key", "---\nconcepts:\n  from: n.md\n  candidates: 1\n  paged: 1\n---\n"},
	}
	for _, tc := range bad {
		if _, present, err := ConceptsBlock(tc.fm); !present || err == nil {
			t.Errorf("%s: present=%v err=%v, want a reported malformed block", tc.name, present, err)
		}
	}

	// The sum identity and a pending candidate are the caller's findings, not
	// parse errors.
	_, present, err = ConceptsBlock("---\nconcepts:\n  from: n.md\n  candidates: 10\n  paged: 6\n  merged: 0\n  inline: 3\n  declined: 0\n  pending: 1\n---\n")
	if !present || err != nil {
		t.Errorf("sum/pending findings must not be parse errors: present=%v err=%v", present, err)
	}
}

func TestParseContentTypedBodyLinks(t *testing.T) {
	p := ParseContent("---\nid: a\n---\nBody with [[depends_on::B]] and [[plain|Label]].", "context/wiki/a.md", "a")
	if len(p.Links) != 2 {
		t.Fatalf("Links = %+v", p.Links)
	}
	if p.Links[0].Verb != "depends_on" || p.Links[0].Target != "B" {
		t.Errorf("typed link = %+v", p.Links[0])
	}
	if p.Links[1].Verb != "" || p.Links[1].Target != "plain" || !strings.Contains(p.Links[1].Raw, "plain|Label") {
		t.Errorf("plain link = %+v", p.Links[1])
	}
}

func TestParsePageFromDisk(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "wiki/accounts.md", pageContent("accounts", "Account Service"))
	p, err := ParsePage(filepath.Join(root, "wiki"), filepath.Join(root, "wiki/accounts.md"))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "accounts" || p.FileID != "accounts" {
		t.Errorf("ParsePage = %+v", p)
	}
}

func TestLoadWikiSubdirIDs(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "wiki")
	writeFile(t, wiki, "backend/auth.md", pageContent("backend/auth", "Authentication"))
	writeFile(t, wiki, "backend/db.md", pageContent("backend/db", "Database"))
	writeFile(t, wiki, "accounts.md", pageContent("accounts", "Account Service"))

	bld, errs, werr := LoadWiki(wiki)
	if werr != nil {
		t.Fatalf("LoadWiki walk error: %v", werr)
	}
	if len(errs) != 0 {
		t.Fatalf("LoadWiki page errors: %+v", errs)
	}
	if len(bld.Pages) != 3 {
		t.Fatalf("pages = %d, want 3", len(bld.Pages))
	}
	sub := bld.ByID["backend/auth"]
	if sub == nil {
		t.Fatal("nested page not indexed by subpath id")
	}
	if sub.FileID != "backend/auth" {
		t.Errorf("FileID = %q, want backend/auth", sub.FileID)
	}
	if bld.ByID["accounts"] == nil {
		t.Error("flat page not indexed")
	}
	if got := len(bld.ByTitle["Database"]); got != 1 {
		t.Errorf("byTitle[Database] len = %d, want 1", got)
	}
}

func TestLoadWikiErrors(t *testing.T) {
	// Missing directory → walk error.
	if _, _, werr := LoadWiki(filepath.Join(t.TempDir(), "missing")); werr == nil || !os.IsNotExist(werr) {
		t.Errorf("missing dir walkErr = %v, want IsNotExist", werr)
	}
	// Unreadable page → reported as LoadError, others still loaded.
	root := t.TempDir()
	wiki := filepath.Join(root, "wiki")
	writeFile(t, wiki, "a.md", pageContent("a", "A"))
	writeFile(t, wiki, "b.md", pageContent("b", "B"))
	// Replace b.md with an unreadable directory of the same name.
	if err := os.Remove(filepath.Join(wiki, "b.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(wiki, "b.md"), 0o750); err != nil {
		t.Fatal(err)
	}
	bld, errs, werr := LoadWiki(wiki)
	if werr != nil {
		t.Fatalf("walk error on directory-like .md: %v", werr)
	}
	// A directory named b.md is skipped by the walker (it only loads files), so
	// no LoadError and no b page — pages = 1.
	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none for a directory entry", errs)
	}
	if len(bld.Pages) != 1 || bld.Pages[0].ID != "a" {
		t.Errorf("pages = %+v, want only a", bld.Pages)
	}
}

func TestBuilderResolve(t *testing.T) {
	bld := &Builder{
		ByID:    map[string]*Page{"db": {ID: "db", Title: "Database"}},
		ByTitle: map[string][]*Page{"Database": {{ID: "db", Title: "Database"}}},
	}
	if got := bld.Resolve("db"); got == nil || got.ID != "db" {
		t.Errorf("resolve by id = %+v", got)
	}
	if got := bld.Resolve("Database"); got == nil || got.ID != "db" {
		t.Errorf("resolve by title = %+v", got)
	}
	if got := bld.Resolve("  Database  "); got == nil {
		t.Error("resolve should trim whitespace")
	}
	if got := bld.Resolve("nope"); got != nil {
		t.Errorf("resolve missing = %+v, want nil", got)
	}
	// Ambiguous title resolves to nil.
	bld2 := &Builder{
		ByTitle: map[string][]*Page{"X": {{ID: "a", Title: "X"}, {ID: "b", Title: "X"}}},
	}
	if got := bld2.Resolve("X"); got != nil {
		t.Errorf("resolve ambiguous title = %+v, want nil", got)
	}
}

func TestBuilderInbound(t *testing.T) {
	bld := &Builder{
		Pages: []*Page{
			{
				ID:   "a",
				Path: "context/wiki/a.md",
				Relations: map[string][]string{
					"refers_to": {"[[b|B]]"},
				},
			},
			{
				ID:    "b",
				Path:  "context/wiki/b.md",
				Links: []Link{{Raw: "[[c|C]]", Verb: "depends_on", Target: "c"}},
			},
			{ID: "c", Path: "context/wiki/c.md"},
		},
	}
	bld.ByID = map[string]*Page{}
	for _, p := range bld.Pages {
		bld.ByID[p.ID] = p
	}
	inbound := bld.Inbound()
	wantB := []string{"a"}
	wantC := []string{"b"}
	sort.Strings(inbound["b"])
	if !reflect.DeepEqual(inbound["b"], wantB) {
		t.Errorf("inbound[b] = %v, want %v", inbound["b"], wantB)
	}
	sort.Strings(inbound["c"])
	if !reflect.DeepEqual(inbound["c"], wantC) {
		t.Errorf("inbound[c] = %v, want %v", inbound["c"], wantC)
	}
}

func TestBuilderCycles(t *testing.T) {
	bld := &Builder{
		Pages: []*Page{
			{ID: "a", Path: "context/wiki/a.md", Relations: map[string][]string{"depends_on": {"[[b|B]]"}}},
			{ID: "b", Path: "context/wiki/b.md", Relations: map[string][]string{"depends_on": {"[[a|A]]"}}},
			{ID: "c", Path: "context/wiki/c.md"},
		},
		ByID: map[string]*Page{
			"a": {ID: "a"}, "b": {ID: "b"}, "c": {ID: "c"},
		},
	}
	cycles := bld.Cycles()
	// The DFS flags the page that rediscovers a path on the DFS stack; in the
	// a↔b loop that is whichever page is reached first (a here), mirroring the
	// original lint behavior.
	if !cycles["context/wiki/a.md"] {
		t.Errorf("cycles = %v, want a in cycle", cycles)
	}
	if cycles["context/wiki/c.md"] {
		t.Error("c should not be in a cycle")
	}
}

func TestPageHelpers(t *testing.T) {
	p := ParseContent(pageContent("accounts", "Account Service"), "context/wiki/accounts.md", "accounts")
	if !p.Active() {
		t.Error("expected active page")
	}
	if !p.ExplicitTitle() {
		t.Error("expected explicit title")
	}
	noTitle := ParseContent("---\nid: x\n---\nbody\n", "x.md", "x")
	if noTitle.ExplicitTitle() {
		t.Error("expected no explicit title when absent")
	}
	if !CleanTag("backend/auth") || CleanTag("Bad Tag!") {
		t.Error("CleanTag misclassification")
	}
}

func TestIsMapDoc(t *testing.T) {
	cases := map[string]bool{
		"context/wiki/topic.map.md":  true,
		"context/notes/plan.map.md":  true,
		"context/wiki/topic.md":      false,
		"context/wiki/map.md":        false,
		"context/wiki/topic.map.txt": false,
	}
	for path, want := range cases {
		if got := IsMapDoc(path); got != want {
			t.Errorf("IsMapDoc(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDocID(t *testing.T) {
	cases := map[string]string{
		"context/wiki/topic.md":       "topic",
		"context/wiki/topic.map.md":   "topic.map",
		"context/wiki/sub/page.md":    "sub/page",
		"context/notes/plan.map.md":   "context/notes/plan.map",
		"context/analysis/analy-x.md": "context/analysis/analy-x",
	}
	for path, want := range cases {
		if got := DocID(path); got != want {
			t.Errorf("DocID(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestIsMermaidDoc(t *testing.T) {
	cases := map[string]bool{
		"context/wiki/flow.mmd":     true,
		"context/notes/seq.mmd":     true,
		"context/wiki/flow.md":      false,
		"context/wiki/flow.map.md":  false,
		"context/wiki/flow.mmd.bak": false,
		"context/wiki/mmd":          false,
	}
	for path, want := range cases {
		if got := IsMermaidDoc(path); got != want {
			t.Errorf("IsMermaidDoc(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsSlideDoc(t *testing.T) {
	cases := map[string]bool{
		"context/analysis/analy-x.slide.md": true,
		"context/notes/plan.slide.md":       true,
		"context/wiki/deck.slide.md":        true,
		"context/notes/plan.md":             false,
		"context/notes/plan.map.md":         false,
		"context/notes/slide.md":            false,
		"context/notes/plan.slide":          false,
	}
	for path, want := range cases {
		if got := IsSlideDoc(path); got != want {
			t.Errorf("IsSlideDoc(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestSlideDocIDAndKind: a deck reuses DocID (strip ".md" only), so it keeps its
// own id and, being a plain ".md" to the walk, inherits the base document's kind
// rather than gaining a "slide" kind of its own.
func TestSlideDocIDAndKind(t *testing.T) {
	path := "context/analysis/analy-x.slide.md"
	if got, want := DocID(path), "context/analysis/analy-x.slide"; got != want {
		t.Errorf("DocID(%q) = %q, want %q", path, got, want)
	}
	if got, want := DocID("context/wiki/topic.slide.md"), "topic.slide"; got != want {
		t.Errorf("DocID(wiki deck) = %q, want %q", got, want)
	}

	content := "---\nkind: analysis\nstatus: active\ntitle: Deck\n---\n\n# Body\n"
	page := ParseContent(content, path, DocID(path))
	if page.Kind != "analysis" {
		t.Errorf("deck kind = %q, want the inherited kind analysis", page.Kind)
	}
	if page.FileID != "context/analysis/analy-x.slide" {
		t.Errorf("deck FileID = %q, want its own id", page.FileID)
	}
}

func TestMermaidID(t *testing.T) {
	cases := map[string]string{
		"context/wiki/flow.mmd":  "context/wiki/flow",
		"context/notes/seq.mmd":  "context/notes/seq",
		"context/analysis/x.mmd": "context/analysis/x",
	}
	for path, want := range cases {
		if got := MermaidID(path); got != want {
			t.Errorf("MermaidID(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestFrontmatterFieldStripsQuotes(t *testing.T) {
	fm := "---\ntitle: \"Quoted title\"\ncreated: '2026-09-15'\nplain: value\n---\n"
	cases := map[string]string{
		"title":   "Quoted title",
		"created": "2026-09-15",
		"plain":   "value",
		"missing": "",
	}
	for key, want := range cases {
		if got := FrontmatterField(fm, key); got != want {
			t.Errorf("FrontmatterField(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestFrontmatterTitleDecodesYAMLEscapes(t *testing.T) {
	fm := "---\ntitle: \"UI/UX \\u2014 tokens by intent\"\n---\n"
	if got, want := FrontmatterTitle(fm), "UI/UX — tokens by intent"; got != want {
		t.Errorf("FrontmatterTitle() = %q, want %q", got, want)
	}
}

func TestParseFrontmatterMapReadsInlineFlowMap(t *testing.T) {
	fm := `---
kind: wiki
relations: {refers_to: "[[architecture/fitness-function|fitness function]]", part_of: ["[[architecture/a|a]]", "[[architecture/b|b]]"]}
tags: [architecture/evolution]
---
`
	rels, inline := ParseFrontmatterMap(fm, "relations")
	if !inline {
		t.Fatalf("inline map must be reported as inline")
	}
	if got := rels["refers_to"]; len(got) != 1 || got[0] != "[[architecture/fitness-function|fitness function]]" {
		t.Errorf("refers_to = %v", got)
	}
	if got := rels["part_of"]; len(got) != 2 || got[0] != "[[architecture/a|a]]" || got[1] != "[[architecture/b|b]]" {
		t.Errorf("part_of = %v", got)
	}
}

func TestParseFrontmatterMapKeepsBlockMap(t *testing.T) {
	fm := `---
relations:
  refers_to:
    - "[[a|a]]"
---
`
	rels, inline := ParseFrontmatterMap(fm, "relations")
	if inline {
		t.Fatalf("block map must not be reported as inline")
	}
	if got := rels["refers_to"]; len(got) != 1 || got[0] != `"[[a|a]]"` {
		t.Errorf("refers_to = %v", got)
	}
}

func TestParseContentSeesInlineRelations(t *testing.T) {
	content := `---
kind: wiki
id: architecture/fitness-function
title: Fitness Function
relations: {refers_to: "[[architecture/evolutionary-architecture|evolutionary architecture]]"}
---
body
`
	p := ParseContent(content, "context/wiki/architecture/fitness-function.md", "architecture/fitness-function")
	if len(p.Relations["refers_to"]) != 1 {
		t.Fatalf("inline relations were not read: %v", p.Relations)
	}
}
