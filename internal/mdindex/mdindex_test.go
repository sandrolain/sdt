package mdindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDerivesFacetsAndSections(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/analysis/a.md", `---
kind: analysis
status: active
objective: obj-x
title: Hybrid search
summary: design
topics:
  - context-search
  - viewer
entities:
  - bleve
created: 2026-09-01
---

## One

body one

## Two

`+"```\n## not a heading\n```\n")

	res, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := res.Manifest.Entries["context/analysis/a.md"]
	if !ok {
		t.Fatalf("entry missing: %+v", res.Manifest.Entries)
	}
	if e.Kind != "analysis" || e.Status != "active" || e.Objective != "obj-x" {
		t.Errorf("facets wrong: %+v", e)
	}
	if len(e.Topics) != 2 || e.Topics[0] != "context-search" || len(e.Entities) != 1 {
		t.Errorf("topics/entities wrong: %v / %v", e.Topics, e.Entities)
	}
	if len(e.Sections) != 3 || e.Sections[0].Heading != "" || e.Sections[1].ID != "one" || e.Sections[2].ID != "two" {
		t.Errorf("sections wrong: %+v", e.Sections)
	}
	f := res.Manifest.Facets()
	if len(f.Kinds) != 1 || f.Kinds[0] != "analysis" || len(f.Topics) != 2 || len(f.Entities) != 1 {
		t.Errorf("facets summary wrong: %+v", f)
	}
	if f.Objectives[0] != "obj-x" || f.Statuses[0] != "active" {
		t.Errorf("facets summary wrong: %+v", f)
	}
}

func TestScanViewsAuxResources(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/wiki/flow.mmd", "flowchart TD\n  A --> B\n")
	writeDoc(t, root, "context/wiki/board.canvas", `{"nodes":[],"edges":[]}`)

	res, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	mmd, ok := res.Manifest.Entries["context/wiki/flow.mmd"]
	if !ok {
		t.Fatalf("mermaid entry missing: %+v", res.Manifest.Entries)
	}
	if mmd.Kind != "mermaid" || mmd.Title != "flow" || mmd.Name != "flow" {
		t.Errorf("mermaid entry wrong: %+v", mmd)
	}
	if mmd.Body != "flowchart TD\n  A --> B\n" || len(mmd.Sections) != 0 {
		t.Errorf("mermaid body/sections wrong: %q / %+v", mmd.Body, mmd.Sections)
	}
	canvas, ok := res.Manifest.Entries["context/wiki/board.canvas"]
	if !ok {
		t.Fatalf("canvas entry missing: %+v", res.Manifest.Entries)
	}
	if canvas.Kind != "canvas" || canvas.Title != "board" || canvas.Name != "board" {
		t.Errorf("canvas entry wrong: %+v", canvas)
	}
	// Synthesized kinds surface in the facet summary for filter dropdowns.
	kinds := res.Manifest.Facets().Kinds
	if len(kinds) != 2 || kinds[0] != "canvas" || kinds[1] != "mermaid" {
		t.Errorf("facet kinds wrong: %v", kinds)
	}
}

func TestScanSkipsExcludedDirs(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/analysis/keep.md", "---\nkind: analysis\n---\nbody\n")
	writeDoc(t, root, "context/refs/big/ignore.md", "---\nkind: analysis\n---\nbody\n")
	writeDoc(t, root, "context/instructions/ignore.md", "---\nkind: analysis\n---\nbody\n")
	writeDoc(t, root, "context/tmp/ignore.md", "---\nkind: analysis\n---\nbody\n")

	res, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Manifest.Entries) != 1 {
		t.Errorf("expected only the kept doc, got %d: %v", len(res.Manifest.Entries), res.Changed)
	}
	if _, ok := res.Manifest.Entries["context/analysis/keep.md"]; !ok {
		t.Error("kept doc missing")
	}
}

func TestScanIncremental(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/analysis/a.md", "---\nkind: analysis\n---\none\n")
	writeDoc(t, root, "context/analysis/b.md", "---\nkind: analysis\n---\ntwo\n")

	first, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Changed) != 2 {
		t.Errorf("first scan should change 2 docs, got %v", first.Changed)
	}

	// No change: manifest reused, no changed docs.
	second, err := Scan(root, first.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changed) != 0 || second.Unchanged != 2 {
		t.Errorf("expected no changes and 2 unchanged, got changed=%v unchanged=%d", second.Changed, second.Unchanged)
	}

	// Modify one file with a different size so mtime/size detects it.
	writeDoc(t, root, "context/analysis/a.md", "---\nkind: analysis\n---\none modified with more text\n")
	third, err := Scan(root, first.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Changed) != 1 || third.Changed[0] != "context/analysis/a.md" {
		t.Errorf("expected only a.md changed, got %v", third.Changed)
	}

	// Remove a file.
	if err := os.Remove(filepath.Join(root, "context/analysis/b.md")); err != nil {
		t.Fatal(err)
	}
	fourth, err := Scan(root, third.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(fourth.Removed) != 1 || fourth.Removed[0] != "context/analysis/b.md" {
		t.Errorf("expected b.md removed, got %v", fourth.Removed)
	}
}

func TestEnsureFreshPersistsManifest(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/analysis/a.md", "---\nkind: analysis\n---\nbody\n")

	first, err := EnsureFresh(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.FromCache {
		t.Error("first refresh must not be from cache")
	}
	if _, err := os.Stat(filepath.Join(root, ManifestFile)); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}

	second, err := EnsureFresh(root)
	if err != nil {
		t.Fatal(err)
	}
	if !second.FromCache || len(second.Changed) != 0 {
		t.Errorf("second refresh should reuse cache with no changes: %+v", second)
	}
}

func TestManifestSaveLoadRoundTripAndVersion(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{Version: ManifestVersion, Entries: map[string]*Entry{
		"context/analysis/a.md": {ID: "context/analysis/a.md", Kind: "analysis", Hash: "x"},
	}}
	path := filepath.Join(root, ManifestFile)
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got == nil {
		t.Fatalf("load failed: %v %v", got, err)
	}
	if got.Entries["context/analysis/a.md"].Kind != "analysis" {
		t.Errorf("round trip lost data: %+v", got.Entries)
	}

	// Stale version is ignored.
	if err := os.WriteFile(path, []byte(`{"version":0,"entries":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(path); got != nil {
		t.Errorf("stale-version manifest must be ignored, got %+v", got)
	}
}

func TestRefreshLockSingleWriter(t *testing.T) {
	root := t.TempDir()
	release, ok := RefreshLock(root)
	if !ok {
		t.Fatal("first lock must succeed")
	}
	if _, ok2 := RefreshLock(root); ok2 {
		t.Error("second concurrent lock must fail")
	}
	release()
	release2, ok3 := RefreshLock(root)
	if !ok3 {
		t.Error("lock must be acquirable after release")
	}
	release2()
}

func TestEntriesSortedAndFacetsNilSafe(t *testing.T) {
	m := &Manifest{Entries: map[string]*Entry{
		"context/b.md": {ID: "context/b.md"},
		"context/a.md": {ID: "context/a.md"},
	}}
	got := m.EntriesSorted()
	if len(got) != 2 || got[0].ID != "context/a.md" || got[1].ID != "context/b.md" {
		t.Errorf("EntriesSorted not ordered: %v", got)
	}
	var nilM *Manifest
	if f := nilM.Facets(); f == nil || len(f.Kinds) != 0 {
		t.Errorf("nil manifest facets must be an empty summary, got %+v", f)
	}
}

func TestManifestSetRootAndLoadBody(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "context/analysis/a.md", "---\nkind: analysis\n---\n\n## H\n\nthe body text\n")
	res, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Manifest.Save(filepath.Join(root, ManifestFile)); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(filepath.Join(root, ManifestFile))
	if err != nil || loaded == nil {
		t.Fatalf("load: %v %v", loaded, err)
	}
	loaded.SetRoot(root)
	e := loaded.Entries["context/analysis/a.md"]
	if err := e.LoadBody(); err != nil {
		t.Fatalf("LoadBody: %v", err)
	}
	if !strings.Contains(e.Body, "the body text") || e.Name == "" {
		t.Errorf("LoadBody did not refresh body/name: %+v", e)
	}
	// Second call is a no-op.
	if err := e.LoadBody(); err != nil {
		t.Errorf("second LoadBody: %v", err)
	}
}
