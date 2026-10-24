package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextIngestListAndManifest(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/books/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	sha, _, err := fileSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/refs/converted/book-"+sha[:12]+".md",
		"---\nsource: "+source+"\nsha256: "+sha+"\n---\n\n# Book\n")

	out := string(execute(t, contextIngestListCmd, nil, "--write"))
	if !strings.Contains(out, "book.epub") || !strings.Contains(out, "converted") {
		t.Fatalf("list did not derive the converted source:\n%s", out)
	}
	manifest, err := os.ReadFile(filepath.Join("context/ingestion", ingestManifestName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"generated:", "sources:", "path: ingestion/books/book.epub", "status: converted", "converted: refs/converted/book-"} {
		if !strings.Contains(string(manifest), want) {
			t.Errorf("manifest is missing %q:\n%s", want, manifest)
		}
	}
	if strings.Contains(out, ingestManifestName) {
		t.Errorf("the manifest listed itself:\n%s", out)
	}
}

func TestContextIngestStatusByBasename(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/books/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	sha, _, err := fileSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/refs/converted/book-"+sha[:12]+".md",
		"---\nsource: "+source+"\nsha256: "+sha+"\n---\n\n# Book\n")

	out := string(execute(t, contextIngestStatusCmd, nil, "book.epub"))
	for _, want := range []string{"source: ingestion/books/book.epub", "status: converted", "converted: refs/converted/book-"} {
		if !strings.Contains(out, want) {
			t.Errorf("status is missing %q:\n%s", want, out)
		}
	}
}

func TestContextIngestArchiveMovesAndRecords(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/books/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	sha, _, err := fileSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/refs/converted/book-"+sha[:12]+".md",
		"---\nsource: "+source+"\nsha256: "+sha+"\n---\n\n# Book\n")

	out := string(execute(t, contextIngestArchiveCmd, nil, "book.epub"))
	if !strings.Contains(out, "archived: ingestion/books/book.epub -> refs/book.epub") {
		t.Fatalf("unexpected archive output:\n%s", out)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Errorf("source still exists after archive: %v", err)
	}
	dest := filepath.Join("context/refs", "book.epub")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("archive destination missing: %v", err)
	}
	sidecar := dest + ingestSidecarSuffix
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source: ingestion/books/book.epub", "sha256: " + sha, "converted: refs/converted/book-"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("sidecar is missing %q:\n%s", want, data)
		}
	}
}

func TestContextIngestArchiveRefusesCollisionAndUnconverted(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	writeCtxDoc(t, "context/refs/book.epub", "already here")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextIngestArchiveCmd, nil, "book.epub"))
	})

	// A different basename has no collision, but it is unconverted.
	writeCtxDoc(t, "context/ingestion/other.epub", "other-bytes")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextIngestArchiveCmd, nil, "other.epub"))
	})
	out := string(execute(t, contextIngestArchiveCmd, nil, "other.epub", "--force"))
	if !strings.Contains(out, "archived:") {
		t.Fatalf("--force did not archive the unconverted source:\n%s", out)
	}
}

func TestContextIngestPlanPrintsRoutingAndScope(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/book.epub", "epub-bytes")
	writeCtxDoc(t, "context/refs/converted/book-abcdef123456.md", `---
source: context/ingestion/book.epub
sha256: abcdef123456
---

# Book

## Alpha

one

## Beta

two
`)

	out := string(execute(t, contextIngestPlanCmd, nil,
		"book.epub", "--artifact", "refs/converted/book-abcdef123456.md",
		"--scope", "ch1", "--mode", "merge"))
	for _, want := range []string{
		"source: ingestion/book.epub",
		"status: converted",
		"artifact: refs/converted/book-abcdef123456.md",
		"route: single-pass",
		"route_headings: 2",
		"coverage_units: 3",
		"coverage: not audited",
		"scope: ch1",
		"mode: merge",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan is missing %q:\n%s", want, out)
		}
	}
}

func TestContextIngestPlanRejectsBadMode(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/book.epub", "epub-bytes")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextIngestPlanCmd, nil, "book.epub", "--mode", "replace"))
	})
}

func TestContextIngestListSkipsManifestAndSidecars(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/book.epub", "epub-bytes")
	writeCtxDoc(t, "context/ingestion/book.epub"+ingestSidecarSuffix, "sidecar: true\n")
	writeCtxDoc(t, "context/ingestion/"+ingestManifestName, "sources: []\n")

	out := string(execute(t, contextIngestListCmd, nil))
	if !strings.Contains(out, "book.epub") {
		t.Fatalf("the source is missing:\n%s", out)
	}
	if strings.Contains(out, ingestManifestName) || strings.Contains(out, ingestSidecarSuffix) {
		t.Fatalf("manifest or sidecar was listed:\n%s", out)
	}
}

func TestContextIngestPlanWritesRunManifest(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	writeCtxDoc(t, "context/refs/converted/book-abcdef123456.md", `---
source: context/ingestion/book.epub
sha256: abcdef123456
---

# Book

## Alpha

one
`)

	out := string(execute(t, contextIngestPlanCmd, nil,
		"book.epub", "--artifact", "refs/converted/book-abcdef123456.md",
		"--scope", "ch1", "--mode", "merge", "--write"))
	if !strings.Contains(out, "run manifest: ingestion/runs/") {
		t.Fatalf("plan did not print the run manifest path:\n%s", out)
	}
	files, err := filepath.Glob("context/ingestion/runs/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("run manifests = %d, want 1", len(files))
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source: ingestion/book.epub", "status: converted", "route: single-pass", "coverage: not audited", "scope: ch1", "mode: merge", "created:"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("run manifest is missing %q:\n%s", want, data)
		}
	}
	if _, err := os.Stat(source); err != nil {
		t.Errorf("plan mutated the source: %v", err)
	}
}

func TestContextIngestListIgnoresRuns(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/book.epub", "epub-bytes")
	writeCtxDoc(t, "context/ingestion/runs/20261024T000000Z.yaml", "source: ingestion/book.epub\n")
	out := string(execute(t, contextIngestListCmd, nil))
	if strings.Contains(out, "runs/") {
		t.Fatalf("list included a run manifest:\n%s", out)
	}
}
