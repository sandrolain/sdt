package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// corpus creates a temp corpus with known .md docs, excluded dirs and a
// non-md file for boundary coverage.
func corpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("context/wiki/alpha.md", `---
kind: wiki
title: Alpha module
summary: The alpha module handles login tokens
created: 2026-09-10
---

Alpha body with a special unique-term-x9y8z7 mention.
`)
	write("context/wiki/beta.md", `---
kind: wiki
title: Beta module
created: 2026-09-11
---

Beta body talks about tokens too.
`)
	write("context/analysis/analy.md", `---
kind: analysis
title: Analysis one
created: 2026-09-12
---

Analysis body with neutral words only.
`)
	write("context/notes/todo.md", `---
kind: notes
title: A note
created: 2026-09-01
---

A note page mentioning tokens.
`)
	// excluded dirs and non-md
	write("context/tmp/scratch.md", `---
kind: wiki
title: Scratch
created: 2026-08-01
---

scratch tokens
`)
	write("context/scripts/x.md", `---
kind: wiki
title: Script doc
created: 2026-08-01
---

script tokens
`)
	write("context/refs/ref-a.md", `---
kind: wiki
title: Refs clone page
created: 2026-08-01
---

refs tokens
`)
	write("context/wiki/board.canvas", `{"nodes":[]}`)
	// map document: unique term so it does not disturb ranked/kind/date tests
	write("context/wiki/topic.map.md", `---
kind: wiki
title: Topic map
created: 2026-09-14
---

outlineword map content
`)
	// outside the corpus: root-level docs must never be indexed
	write("docs/outside.md", `---
kind: notes
title: Outside docs
created: 2026-09-13
---

outsideonly unique term not in the corpus.
`)
	return root
}

func TestNewAndBulkIndex(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	// wiki(2) + analysis(1) + notes(1) + map(1) = 5; tmp/scripts/refs/canvas/root-docs skipped
	if len(ix.registry) != 5 {
		t.Errorf("indexed %d docs, want 5", len(ix.registry))
	}
	if _, ok := ix.registry["docs/outside.md"]; ok {
		t.Error("outside-corpus doc indexed")
	}
}

func TestSearchMapResult(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	res, err := ix.Search("outlineword", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Results) != 1 {
		t.Fatalf("map search: %+v", res)
	}
	r := res.Results[0]
	if !r.IsMap || r.MapID != "topic.map" {
		t.Errorf("map flags wrong: %+v", r)
	}
}

func TestSearchIgnoresOutsideCorpus(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	res, err := ix.Search("outsideonly", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Errorf("total = %d, want 0 (outside-corpus term hit)", res.Total)
	}
}

func TestSearchRankedResults(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	res, err := ix.Search("tokens", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 {
		t.Errorf("total = %d, want 3", res.Total)
	}
	paths := map[string]string{}
	for _, r := range res.Results {
		paths[r.Path] = r.Kind
	}
	for _, want := range []string{"context/wiki/alpha.md", "context/wiki/beta.md", "context/notes/todo.md"} {
		if _, ok := paths[want]; !ok {
			t.Errorf("missing %s in %v", want, res.Results)
		}
	}
	alpha := res.Results[0]
	if alpha.Path != "context/wiki/alpha.md" {
		if !hasPath(res.Results, "context/wiki/alpha.md") {
			t.Errorf("alpha not ranked first: %+v", res.Results)
		}
	}
	// snippets are never empty and reference the summary when the term is there
	for _, r := range res.Results {
		if r.Snippet == "" {
			t.Errorf("empty snippet for %s", r.Path)
		}
	}
	// ranking: alpha (title+summary+body) should out-rank beta (title+body)
	scoreOf := func(p string) float64 {
		for _, r := range res.Results {
			if r.Path == p {
				return r.Score
			}
		}
		return -1
	}
	if scoreOf("context/wiki/alpha.md") <= scoreOf("context/wiki/beta.md") {
		t.Errorf("alpha score %v not above beta %v", scoreOf("context/wiki/alpha.md"), scoreOf("context/wiki/beta.md"))
	}
}

func hasPath(rs []Result, path string) bool {
	for _, r := range rs {
		if r.Path == path {
			return true
		}
	}
	return false
}

func TestSearchKindFilter(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	res, _ := ix.Search("tokens", "wiki", "", "", 0)
	if res.Total != 2 {
		t.Errorf("kind=wiki total = %d, want 2", res.Total)
	}
	for _, r := range res.Results {
		if r.Kind != "wiki" {
			t.Errorf("non-wiki result: %+v", r)
		}
	}
}

func TestSearchDateFilter(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	res, _ := ix.Search("tokens", "", "2026-09-10", "2026-09-10", 0)
	if res.Total != 1 {
		t.Errorf("from==to total = %d, want 1 (alpha only)", res.Total)
	}
	res, _ = ix.Search("tokens", "", "", "2026-09-01", 0)
	if res.Total != 1 {
		t.Errorf("to=2026-09-01 total = %d, want 1 (notes only)", res.Total)
	}
	res, _ = ix.Search("tokens", "", "2026-09-12", "", 0)
	if res.Total != 0 {
		t.Errorf("from=2026-09-12 total = %d, want 0", res.Total)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	res, err := ix.Search("", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 || len(res.Results) != 0 {
		t.Errorf("empty query: %+v", res)
	}
}

func TestSearchLimitAndMax(t *testing.T) {
	root := corpus(t)
	ix, _ := New(root)
	defer ix.Close()
	res, _ := ix.Search("tokens", "", "", "", 2)
	if len(res.Results) > 2 {
		t.Errorf("limit 2 returned %d", len(res.Results))
	}
	res, _ = ix.Search("tokens", "", "", "", 999)
	if len(res.Results) > 20 {
		t.Errorf("cap at 20 returned %d", len(res.Results))
	}
}

func TestSearchMissingRoot(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected error for missing root")
	}
}

func TestSearchMissingCorpus(t *testing.T) {
	ix, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	if len(ix.registry) != 0 {
		t.Errorf("indexed %d docs for corpus-less root, want 0", len(ix.registry))
	}
	if res, _ := ix.Search("anything", "", "", "", 0); res.Total != 0 {
		t.Errorf("search on empty index returned hits: %+v", res)
	}
}

func TestSearchCorpusNotDir(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "context")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root); err == nil {
		t.Error("expected error when corpus path is a file")
	}
}

func TestParseDocFields(t *testing.T) {
	dir := t.TempDir()
	raw := `---
kind: wiki
title: "Quoted Title"
summary: "A summary"
created: 2026-09-10
---

# Body
`
	p := filepath.Join(dir, "x.md")
	if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	d, _ := parseDoc("context/wiki/x.md", p)
	if d.Kind != "wiki" || d.Title != "Quoted Title" || d.Summary != "A summary" {
		t.Errorf("parsed: %+v", d)
	}
	if d.CreatedDays == 0 {
		t.Error("created days not parsed")
	}
	if !strings.Contains(d.Body, "# Body") {
		t.Errorf("body not separated: %q", d.Body)
	}
}

func TestParseCreatedDays(t *testing.T) {
	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	want := func(layout, raw string) int64 {
		tm, _ := time.Parse(layout, raw)
		return int64(tm.Sub(epoch).Hours() / 24)
	}
	cases := []struct {
		raw  string
		want int64
	}{
		{"2026-09-10", want("2006-01-02", "2026-09-10")},
		{"2026-09-10T00:00:00Z", want(time.RFC3339, "2026-09-10T00:00:00Z")},
		{"2026-09-10T12:30:00Z", want(time.RFC3339, "2026-09-10T12:30:00Z")},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseCreatedDays(c.raw); got != c.want {
			t.Errorf("%q: got %d want %d", c.raw, got, c.want)
		}
	}
}

func TestSnippet(t *testing.T) {
	d := doc{
		Summary: "handles login tokens securely",
		Body:    "A long body that mentions tokens elsewhere.",
	}
	if s := Snippet(d, "tokens", 20); s == "" || !strings.Contains(s, "tokens") {
		t.Errorf("snippet missing term: %q", s)
	}
	// body-only matching
	d2 := doc{Body: "Page about passkeys and vaults."}
	if s := Snippet(d2, "vaults", 40); !strings.Contains(s, "vaults") {
		t.Errorf("body snippet missing term: %q", s)
	}
	// no match at all → truncated prefix fallback
	d3 := doc{Summary: "unrelated summary", Body: "unrelated body text"}
	if s := Snippet(d3, "zzz", 20); s == "" {
		t.Errorf("fallback snippet empty")
	}
}

func TestSearchIndexClose(t *testing.T) {
	root := corpus(t)
	ix, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}
}
