package doc2md

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasureCountsLevelsTwoAndThree(t *testing.T) {
	markdown := "# Title\n\n## First section\n\nbody\n\n### A subsection\n\nmore\n## Second\n"
	headings, lines := Measure(markdown)
	if headings != 3 {
		t.Errorf("headings = %d, want 3 (levels 2 and 3 only)", headings)
	}
	wantLines := len(strings.Split(markdown, "\n")) - 1
	if lines != wantLines {
		t.Errorf("lines = %d, want %d", lines, wantLines)
	}
}

func TestMeasureTrailingNewlineEndsTheLastLine(t *testing.T) {
	if _, lines := Measure("a\nb\nc\n"); lines != 3 {
		t.Errorf("lines = %d, want 3", lines)
	}
	if _, lines := Measure("a\nb\nc"); lines != 3 {
		t.Errorf("lines = %d, want 3", lines)
	}
	if headings, lines := Measure(""); headings != 0 || lines != 0 {
		t.Errorf("empty markdown = %d headings, %d lines, want 0, 0", headings, lines)
	}
}

func TestMeasureIsAPlainTextCount(t *testing.T) {
	// The doctrine's routing test is a plain text measurement — the calibration
	// (159 and 165) came from `grep -cE '^#{2,3} '`, so a `## ` line inside a
	// fenced block counts too. Stating the measure is what makes the number
	// re-checkable; being cleverer than the doctrine would drift the route.
	markdown := "## Real\n\n```md\n## not a section\n```\n"
	if headings, _ := Measure(markdown); headings != 2 {
		t.Errorf("headings = %d, want 2 (a plain text count)", headings)
	}
}

func TestMeasureArtifactReadsTheBodyAfterFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book-0123456789ab.md")
	content := "---\nsource: book.epub\nsha256: 0123456789ab\nconverter: anydoc\n---\n\n# Book\n\n## Chapter 1\n\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	headings, lines, err := MeasureArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if headings != 1 || lines != 5 {
		t.Errorf("artifact = %d headings, %d lines, want 1, 5 (frontmatter excluded)", headings, lines)
	}
}
