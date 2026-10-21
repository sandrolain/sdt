package doc2md

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureNow pins the rendered `converted_at`, so two renders of the same
// document are byte-identical and a change means a content change.
var fixtureNow = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

// tempCorpusRoot builds a minimal project root around a corpus directory.
func tempCorpusRoot(t *testing.T) (root string, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "context", "refs", "converted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

// fixtureDoc builds one kept conversion from one shared source: the source's
// bytes go into the kept name and its path into the provenance, so a test
// creates the source once (tempSource's temp dir is not stable across calls)
// and reuses it, or a re-keep would differ by nothing but the `source:` line.
func fixtureDoc(t *testing.T, src string, markdown string, force bool) Document {
	return Document{
		Source:    src,
		Converter: "anydoc",
		Markdown:  markdown,
		Now:       fixtureNow,
		Force:     force,
	}
}

// mustKeep keeps the fixture once and returns the kept name and the source the
// test must reuse to touch the same artifact again.
func mustKeep(t *testing.T, dir, src string) string {
	t.Helper()
	path, created, err := Keep(fixtureDoc(t, src, "# Report\n\nbody\n", false), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("the fixture artifact should have been created")
	}
	return filepath.Base(path)
}

// writeFixture writes a test file into the temp corpus, creating its folder.
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindCitationsReturnsOnlyCorpusCitations(t *testing.T) {
	root, dir := tempCorpusRoot(t)
	src := tempSource(t, "report.csv", "a\n")
	name := mustKeep(t, dir, src)
	writeFixture(t, filepath.Join(root, "context", "notes", "page.md"),
		"claim (refs/converted/"+name+"@d30449b9)\n")
	// Scratch and capture trees must never count: a tmp copy of a citing page is
	// not a claim.
	writeFixture(t, filepath.Join(root, "context", "tmp", "sketch.md"),
		"refs/converted/"+name+"@d30449b9\n")
	writeFixture(t, filepath.Join(root, "context", "refs", "captures", "old.md"),
		"refs/converted/"+name+"@d30449b9\n")

	citations, err := CitationsFor(root, name)
	if err != nil {
		t.Fatal(err)
	}
	if len(citations) != 1 || !strings.HasSuffix(citations[0], "context/notes/page.md") {
		t.Errorf("citations = %v, want only the citing page", citations)
	}

	none, err := CitationsFor(root, "uncited-deadbeefcafe.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("uncited artifact = %v, want none", none)
	}
}

func TestKeepRefusesADivergingRewriteAndNamesCitations(t *testing.T) {
	root, dir := tempCorpusRoot(t)
	src := tempSource(t, "report.csv", "a\n")
	name := mustKeep(t, dir, src)
	writeFixture(t, filepath.Join(root, "context", "analysis", "note.md"),
		"citation refs/converted/"+name+"@12345678\n")

	_, _, err := Keep(fixtureDoc(t, src, "# Report\n\nchanged\n", false), dir)
	if err == nil || !strings.Contains(err.Error(), "refusing to rewrite") {
		t.Fatalf("re-keep = %v, want a refusal", err)
	}
	if !strings.Contains(err.Error(), "context/analysis/note.md") {
		t.Errorf("refusal must name the citation: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("refusal must say how to go through: %v", err)
	}

	// Naming the cost does not lower the guard: force does.
	forced, created, err := Keep(fixtureDoc(t, src, "# Report\n\nchanged\n", true), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created || filepath.Base(forced) != name {
		t.Errorf("forced re-keep = %s created=%v, want a rewrite of %s", forced, created, name)
	}
}

func TestKeepRefusesADivergingRewriteWithoutCitationsToo(t *testing.T) {
	_, dir := tempCorpusRoot(t)
	src := tempSource(t, "report.csv", "a\n")
	mustKeep(t, dir, src)

	_, _, err := Keep(fixtureDoc(t, src, "# Report\n\nchanged\n", false), dir)
	if err == nil {
		t.Fatal("an uncited rewrite must also be refused")
	}
	if !strings.Contains(err.Error(), "no citation found") {
		t.Errorf("refusal = %v", err)
	}
}

func TestKeepIsStillIdempotentForIdenticalBytes(t *testing.T) {
	_, dir := tempCorpusRoot(t)
	src := tempSource(t, "report.csv", "a\n")
	name := mustKeep(t, dir, src)

	// The same source bytes, the same conversion, the pinned clock: no change.
	path, created, err := Keep(fixtureDoc(t, src, "# Report\n\nbody\n", false), dir)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("an identical re-keep is not a creation")
	}
	if filepath.Base(path) != name {
		t.Errorf("path = %s, want %s", filepath.Base(path), name)
	}
}

func TestSetMetadataRefusesACitedChangeUntilForced(t *testing.T) {
	root, dir := tempCorpusRoot(t)
	src := tempSource(t, "report.csv", "a\n")
	name := mustKeep(t, dir, src)
	writeFixture(t, filepath.Join(root, "context", "notes", "page.md"),
		"an assertion (refs/converted/"+name+"@12345678)\n")

	if err := SetMetadata(dir, name, "", nil, false); err != nil {
		t.Fatalf("unchanged set = %v, want a no-op", err)
	}

	if err := SetMetadata(dir, name, "new summary", nil, false); err == nil {
		t.Fatal("changing a cited artifact succeeded without --force")
	} else if !strings.Contains(err.Error(), "context/notes/page.md") {
		t.Errorf("refusal must name the citation: %v", err)
	}

	// An uncited artifact stays cheap to enrich: no force, no citation theatre.
	uncited := "uncited-87428fc52280.md"
	writeFixture(t, filepath.Join(dir, uncited),
		"---\nsource: u.csv\nsha256: 87428fc522803b\nconverter: anydoc\nconverted_at: \"2026-10-21T12:00:00Z\"\n---\n\nbody\n")
	if err := SetMetadata(dir, uncited, "cheap", nil, false); err != nil {
		t.Fatalf("uncited set = %v", err)
	}
}
