package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/semantic"
)

const chunkArtifact = `---
source: test
---

# Book Title

# Chapter 1. One

## Alpha

one two three

## Beta

four five six

# Index

## Entries
`

func TestBuildChunkSectionsSkipsContainersAndFrontBack(t *testing.T) {
	sha := chunkSHA8([]byte(chunkArtifact))
	sections := buildChunkSections(chunkArtifact, "refs/converted/book.md", sha, 3, false)
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2 (Alpha, Beta)", len(sections))
	}
	for _, s := range sections {
		if s.Scope != "book" || s.Path != "refs/converted/book.md" || s.SHA != sha {
			t.Errorf("provenance = %+v", s)
		}
		if s.Start == 0 || s.End < s.Start {
			t.Errorf("span = %d-%d", s.Start, s.End)
		}
		if !strings.Contains(s.Text, s.Title) {
			t.Errorf("text does not carry the heading: %q", s.Text)
		}
	}
	if sections[0].ID != "refs/converted/book.md#alpha" {
		t.Errorf("first id = %q", sections[0].ID)
	}
	if all := buildChunkSections(chunkArtifact, "refs/converted/book.md", sha, 3, true); len(all) != 4 {
		t.Errorf("with --all-units sections = %d, want 4", len(all))
	}
}

func TestBuildChunkSectionsSpanOwnsSubsections(t *testing.T) {
	doc := "---\nsource: t\n---\n\n# Chapter 1. One\n\n## Alpha\n\ntext\n\n### Sub\n\nmore\n\n## Beta\n\nother\n"
	sha := chunkSHA8([]byte(doc))
	sections := buildChunkSections(doc, "refs/converted/x.md", sha, 3, false)
	var alpha chunkSection
	for _, s := range sections {
		if s.Anchor == "alpha" {
			alpha = s
		}
	}
	if alpha.End < alpha.Start+4 {
		t.Errorf("alpha span does not own its subsection: %d-%d", alpha.Start, alpha.End)
	}
	if !strings.Contains(alpha.Text, "more") {
		t.Errorf("alpha text misses the subsection body: %q", alpha.Text)
	}
}

func TestChunkSnippetBudget(t *testing.T) {
	text := strings.Repeat("word ", 200)
	got := chunkSnippet(text, 20)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("long text was not truncated: %q", got)
	}
	if n := CountTokens(got, familyCL100K); n > 22 {
		t.Errorf("snippet = %d tokens, budget 20", n)
	}
	if chunkSnippet("short text", 0) != "short text" {
		t.Errorf("budget 0 must return the whole section")
	}
}

func TestLexicalChunkHitsRank(t *testing.T) {
	sections := []chunkSection{
		{ID: "a", Path: "p", Anchor: "a", Start: 1, Text: "coupling coupling coupling"},
		{ID: "b", Path: "p", Anchor: "b", Start: 2, Text: "coupling once"},
		{ID: "c", Path: "p", Anchor: "c", Start: 3, Text: "nothing here"},
	}
	hits := lexicalChunkHits("coupling", sections, 2)
	if len(hits) != 2 || hits[0].Anchor != "a" || hits[1].Anchor != "b" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestRRFChunkHitsKeepsBothScopes(t *testing.T) {
	book := []chunkSection{{ID: "b#one", Path: "b", Anchor: "one", Scope: "book", Start: 1}}
	wiki := []chunkSection{{ID: "wiki/w#s", Path: "wiki/w", Anchor: "s", Scope: "wiki"}}
	raw := []semantic.Hit{{ID: "b#one", Similarity: 1}, {ID: "wiki/w#s", Similarity: 1}}
	hits := rrfChunkHits(raw, book, wiki, 2)
	if len(hits) != 2 || hits[0].Scope == hits[1].Scope {
		t.Fatalf("fused hits = %+v", hits)
	}
}

func TestChunkSearchNoCacheWrites(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/book-abcdef123456.md", chunkArtifact)
	before := cacheDigest(t, dir)

	out := execute(t, chunkSearchCmd, nil,
		"refs/converted/book-abcdef123456.md", "one", "--model", "NOPE", "--format", "json")
	var report chunkSearchReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("chunk json: %v\n%s", err, out)
	}
	if report.Mode != "lexical" {
		t.Errorf("mode = %q, want lexical with an unknown model", report.Mode)
	}
	if len(report.Results) == 0 {
		t.Errorf("no results: %s", out)
	}
	if after := cacheDigest(t, dir); after != before {
		t.Errorf("cache changed: %q -> %q", before, after)
	}
}

func TestChunkSearchWithWikiReadsWithoutWriting(t *testing.T) {
	dir := setupContextProject(t)
	writeCtxDoc(t, "context/refs/converted/book-abcdef123456.md", chunkArtifact)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: Alpha is a concept.
tags: [test]
sources:
  - refs/converted/book-abcdef123456.md
---

## Summary

Alpha is a concept about coupling.

## Claims

1. {#claim-1} Alpha (refs/converted/book-abcdef123456.md@abcdef12:9-14)
`)
	before := cacheDigest(t, dir)
	out := execute(t, chunkSearchCmd, nil,
		"refs/converted/book-abcdef123456.md", "coupling", "--model", "NOPE", "--with-wiki", "--format", "json")
	var report chunkSearchReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("chunk json: %v\n%s", err, out)
	}
	if !report.WithWiki || report.WikiSections == 0 {
		t.Fatalf("wiki sections were not read: %+v", report)
	}
	found := false
	for _, h := range report.Results {
		if h.Scope == "wiki" {
			found = true
		}
	}
	if !found {
		t.Errorf("lexical fusion returned no wiki row: %+v", report.Results)
	}
	if after := cacheDigest(t, dir); after != before {
		t.Errorf("cache changed: %q -> %q", before, after)
	}
}

// cacheDigest fingerprints the two derived cache files that a transient run
// must never touch.
func cacheDigest(t *testing.T, root string) string {
	t.Helper()
	var parts []string
	for _, rel := range []string{".sdt/cache/manifest.json", ".sdt/cache/vectors.gob.gz"} {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			parts = append(parts, rel+":absent")
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		parts = append(parts, rel+":"+hex.EncodeToString(sum[:8]))
	}
	return strings.Join(parts, "|")
}
