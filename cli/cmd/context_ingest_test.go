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

func TestContextIngestResolverRefusesLegacySidecars(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/refs/book.pdf.source.yaml", "source: ingestion/book.pdf\n")
	if _, err := resolveIngestSource("book.pdf.source.yaml", true); err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("legacy sidecar must not resolve as an archived source, got %v", err)
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

func TestContextIngestArchiveRenamesInstructionNames(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/repo/AGENTS.md"
	writeCtxDoc(t, source, "third-party rules")
	sha, _, err := fileSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/refs/converted/agents-"+sha[:12]+".md",
		"---\nsource: "+source+"\nsha256: "+sha+"\n---\n\n# Repo\n")

	out := string(execute(t, contextIngestArchiveCmd, nil, "repo/AGENTS.md"))
	if !strings.Contains(out, "archived: ingestion/repo/AGENTS.md -> refs/AGENTS.md"+sourceInstructionArchiveSuffix) {
		t.Fatalf("archive did not rename the instruction file:\n%s", out)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Errorf("source still exists after archive: %v", err)
	}
	dest := filepath.Join("context/refs", "AGENTS.md"+sourceInstructionArchiveSuffix)
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("renamed destination missing: %v", err)
	}
	sidecar := dest + ingestSidecarSuffix
	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("sidecar missing at %s: %v", sidecar, err)
	}
	for _, want := range []string{"original_name: AGENTS.md", "sha256: " + sha, "source: ingestion/repo/AGENTS.md"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("sidecar is missing %q:\n%s", want, data)
		}
	}
}

func TestContextIngestArchiveRefusesRenamedCollision(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/CLAUDE.md"
	writeCtxDoc(t, source, "third-party rules")
	writeCtxDoc(t, "context/refs/CLAUDE.md"+sourceInstructionArchiveSuffix, "already archived")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextIngestArchiveCmd, nil, "context/ingestion/CLAUDE.md", "--force"))
	})
	if _, err := os.Stat(source); err != nil {
		t.Errorf("a refused archive must leave the source in place: %v", err)
	}
}

func TestSourceInstructionNamesCoverTheCorpusRename(t *testing.T) {
	// The rename of 2026-11-01 used this pattern; the closed list must resolve
	// every name it touched, in both cases.
	pattern := []string{
		"AGENTS.md", "AGENT.md", "AGENTS.override.md", "CLAUDE.md", "CLAUDE.local.md",
		"GEMINI.md", "QWEN.md", "SKILL.md", "copilot-instructions.md",
		"NAME.instructions.md", ".cursorrules", ".windsurfrules", ".clinerules", "rule.mdc",
	}
	for _, name := range pattern {
		if !isSourceInstructionName(name) {
			t.Errorf("%q is not in the closed list", name)
		}
		if !isSourceInstructionName(strings.ToLower(name)) {
			t.Errorf("%q is not matched case-insensitively", name)
		}
	}
	for _, name := range []string{"README.md", "AGENTS.md.source", "notes.txt", "skill.md.source"} {
		if isSourceInstructionName(name) {
			t.Errorf("%q must not be renamed: it is not a loadable instruction name", name)
		}
	}
}

// TestContextIngestArchiveRecordsClassification: what a source declares about its
// own use is recorded on the sidecar, and the tier list is closed.
func TestContextIngestArchiveRecordsClassification(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/paper.md", "# A paper\n")
	out := string(execute(t, contextIngestArchiveCmd, nil, "paper.md",
		"--classification", "internal", "--license", "CC-BY-4.0"))
	if !strings.Contains(out, "archived:") {
		t.Fatalf("archive failed:\n%s", out)
	}
	data, err := os.ReadFile("context/refs/paper.md" + ingestSidecarSuffix)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"classification: internal", "license: CC-BY-4.0"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("sidecar is missing %q:\n%s", want, data)
		}
	}

	writeCtxDoc(t, "context/ingestion/other.md", "# Another\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextIngestArchiveCmd, nil, "other.md", "--classification", "secret"))
	})
	if _, err := os.Stat("context/ingestion/other.md"); err != nil {
		t.Errorf("a refused classification must leave the source in place: %v", err)
	}
}

// TestContextIngestArchiveCreatesSourceSheetAndRewritesProvenance: the archive
// move carries the custody join with it — the kept artifact's `source:` header
// follows the archived copy (the `context/tmp/` intermediate shape is rewritten
// too, matched by digest) and the source sheet is created with the pin, the
// extract, checked_at and the restore recipe.
func TestContextIngestArchiveCreatesSourceSheetAndRewritesProvenance(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/books/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	sha, _, err := fileSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	artifact := "context/refs/converted/book-" + sha[:12] + ".md"
	writeCtxDoc(t, artifact, "---\nsource: context/tmp/ingestion-run/book.txt\nsha256: "+sha+"\nconverter: anydoc\n---\n\n# Book\n")
	oldArtifactSHA, _, err := fileSHA256(artifact)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/wiki/citing.md", "---\nkind: wiki\nsummary: citation fixture\n---\n\n## Claims\n\n{#claim-1} Claim. (refs/converted/"+filepath.Base(artifact)+"@"+oldArtifactSHA[:8]+":1)\n")

	out := string(execute(t, contextIngestArchiveCmd, nil, "book.epub"))
	if !strings.Contains(out, "provenance: refs/converted/book-") || !strings.Contains(out, "source -> refs/book.epub") {
		t.Errorf("archive did not report the provenance rewrite:\n%s", out)
	}
	kept, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"source: refs/book.epub", "sha256: " + sha, "converter: anydoc", "# Book"} {
		if !strings.Contains(string(kept), want) {
			t.Errorf("kept artifact is missing %q:\n%s", want, kept)
		}
	}
	if !strings.Contains(out, "sheet: created sources/book.md") {
		t.Errorf("archive did not report the sheet:\n%s", out)
	}
	sheet, err := os.ReadFile("context/sources/book.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"kind: source", "status: curated", "sha256: " + sha,
		"capture_path: refs/book.epub", "extract: refs/converted/book-",
		"checked_at: ", "## Custody notes", "Restore: `sdt doc2md refs/book.epub",
	} {
		if !strings.Contains(string(sheet), want) {
			t.Errorf("sheet is missing %q:\n%s", want, sheet)
		}
	}
	newArtifactSHA, _, err := fileSHA256(artifact)
	if err != nil {
		t.Fatal(err)
	}
	citing, err := os.ReadFile("context/wiki/citing.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(citing), "@"+newArtifactSHA[:8]+":1") || strings.Contains(string(citing), "@"+oldArtifactSHA[:8]+":1") {
		t.Errorf("provenance rewrite did not re-pin the unchanged claim span: old=%s new=%s citation=%s", oldArtifactSHA[:8], newArtifactSHA[:8], citing)
	}
}

// TestContextIngestArchiveLinksExistingSheet: a sheet that already pins the
// source is linked, not duplicated — only the keys it lacks are filled and its
// own frontmatter and body are untouched.
func TestContextIngestArchiveLinksExistingSheet(t *testing.T) {
	setupContextProject(t)
	source := "context/ingestion/books/book.epub"
	writeCtxDoc(t, source, "epub-bytes")
	sha, _, err := fileSHA256(source)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, "context/refs/converted/book-"+sha[:12]+".md",
		"---\nsource: "+source+"\nsha256: "+sha+"\n---\n\n# Book\n")
	writeCtxDoc(t, "context/sources/book.md",
		"---\nkind: source\nuid: u-book\nsummary: A book I already recorded\nstatus: candidate\nurl: https://example.org/book\nsha256: "+sha+"\n---\n\n## Why it matters\n\nRecorded before the archive.\n")

	out := string(execute(t, contextIngestArchiveCmd, nil, "book.epub"))
	if !strings.Contains(out, "sheet: linked sources/book.md") {
		t.Fatalf("archive did not report the link:\n%s", out)
	}
	sheet, err := os.ReadFile("context/sources/book.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"kind: source", "sha256: " + sha, "capture_path: refs/book.epub",
		"extract: refs/converted/book-", "summary: A book I already recorded",
		"status: candidate", "Recorded before the archive.",
	} {
		if !strings.Contains(string(sheet), want) {
			t.Errorf("linked sheet is missing %q:\n%s", want, sheet)
		}
	}
	if strings.Count(string(sheet), "kind: source") != 1 {
		t.Errorf("the link duplicated the sheet:\n%s", sheet)
	}
	if strings.Contains(string(sheet), "status: curated") {
		t.Errorf("the link rewrote the status the sheet already carried:\n%s", sheet)
	}
}

// TestContextIngestArchiveNeverClobbersAForeignSheet: a sheet at the derived
// slug recording other bytes stays untouched; the new record lands on a
// digest-suffixed slug instead.
func TestContextIngestArchiveNeverClobbersAForeignSheet(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/other.epub", "other-bytes")
	foreign := "---\nkind: source\nuid: u-x\nsummary: Someone else's book\nsha256: " + strings.Repeat("a", 64) + "\n---\n\n## Why it matters\n"
	writeCtxDoc(t, "context/sources/other.md", foreign)

	execute(t, contextIngestArchiveCmd, nil, "other.epub", "--force")

	if got, err := os.ReadFile("context/sources/other.md"); err != nil || string(got) != foreign {
		t.Errorf("a foreign sheet was modified: %v\n%s", err, got)
	}
	sha, _, err := fileSHA256("context/refs/other.epub")
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := os.ReadFile("context/sources/other-" + sha[:8] + ".md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sha256: " + sha, "capture_path: refs/other.epub", "status: candidate"} {
		if !strings.Contains(string(sheet), want) {
			t.Errorf("suffixed sheet is missing %q:\n%s", want, sheet)
		}
	}
}

// TestContextIngestArchiveSheetWithoutConvertedArtifact: an archived source
// with no kept rendering still gets its custody record, at the kind's default
// status and without an extract or a re-conversion restore.
func TestContextIngestArchiveSheetWithoutConvertedArtifact(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/note.md", "# A note\n")

	execute(t, contextIngestArchiveCmd, nil, "note.md")

	sheet, err := os.ReadFile("context/sources/note.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kind: source", "status: candidate", "capture_path: refs/note.md", "Restore: none recorded"} {
		if !strings.Contains(string(sheet), want) {
			t.Errorf("sheet is missing %q:\n%s", want, sheet)
		}
	}
	if strings.Contains(string(sheet), "extract:") {
		t.Errorf("a sheet without a kept rendering carries no extract:\n%s", sheet)
	}
}
