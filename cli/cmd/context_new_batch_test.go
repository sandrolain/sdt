//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBatch(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "batch.yaml")
	writeTestFile(t, path, body)
	return path
}

func TestContextNewBatchCreatesEveryEntry(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "context/tmp/body.md"), "## Summary\n\nAlpha body.\n")
	path := writeBatch(t, dir, `- kind: wiki
  type: entity
  title: Alpha
  slug: batch/alpha
  summary: First page.
  tags: [batch/test, architecture]
  relations:
    refers_to:
      - "[[architecture/scalability|scalability]]"
  maturity: provisional
  confidence: medium
  source: architecture/scalability.md
  body: tmp/body.md
- kind: wiki
  type: concept
  title: Beta
  slug: batch/beta
  summary: Second page.
  tags: [batch/test]
`)

	out := string(execute(t, contextNewCmd, nil, "--batch", path, "--agent", "opencode"))
	for _, want := range []string{"context/wiki/batch/alpha.md", "context/wiki/batch/beta.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
	alpha := mustReadFile(t, filepath.Join(dir, "context/wiki/batch/alpha.md"))
	for _, want := range []string{
		"kind: wiki", "id: batch/alpha", "title: Alpha", "type: entity",
		"maturity: provisional", "confidence: medium", "refers_to:",
		"[[architecture/scalability|scalability]]", "Alpha body.",
		"sources:", "architecture/scalability.md", "links:",
	} {
		if !strings.Contains(alpha, want) {
			t.Errorf("alpha missing %q:\n%s", want, alpha)
		}
	}
	beta := mustReadFile(t, filepath.Join(dir, "context/wiki/batch/beta.md"))
	if !strings.Contains(beta, "type: concept") {
		t.Errorf("beta missing its wiki type:\n%s", beta)
	}
}

func TestContextNewBatchUnknownKeyLeavesNothing(t *testing.T) {
	dir := runInTempDir(t)
	path := writeBatch(t, dir, `- kind: wiki
  title: Good
  slug: batch/good
  summary: fine
- kind: wiki
  title: Bad
  slug: batch/bad
  summary: fine
  kind_type: typo
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--batch", path))
	})
	if _, err := os.Stat(filepath.Join(dir, "context/wiki/batch/good.md")); !os.IsNotExist(err) {
		t.Error("a failed batch must not write the entries before the bad one")
	}
}

func TestContextNewBatchRefusesExistingSlug(t *testing.T) {
	dir := runInTempDir(t)
	existing := filepath.Join(dir, "context/wiki/batch/alpha.md")
	writeTestFile(t, existing, "---\nkind: wiki\n---\noriginal\n")
	path := writeBatch(t, dir, `- kind: wiki
  title: Alpha
  slug: batch/alpha
  summary: replacement
- kind: wiki
  title: Beta
  slug: batch/beta
  summary: second
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--batch", path))
	})
	if got := mustReadFile(t, existing); !strings.Contains(got, "original") {
		t.Error("a batch must never overwrite an existing page")
	}
	if _, err := os.Stat(filepath.Join(dir, "context/wiki/batch/beta.md")); !os.IsNotExist(err) {
		t.Error("the second entry must not be written when the first is refused")
	}
}

func TestContextNewBatchRefusesBodyOutsideCorpus(t *testing.T) {
	dir := runInTempDir(t)
	path := writeBatch(t, dir, `- kind: wiki
  title: Escape
  slug: batch/escape
  summary: escapes
  body: ../outside.md
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--batch", path))
	})
}

func TestContextNewBatchRefusesBadRelation(t *testing.T) {
	dir := runInTempDir(t)
	path := writeBatch(t, dir, `- kind: wiki
  title: Bad relation
  slug: batch/bad-relation
  summary: bad
  relations:
    refers_to:
      - not-a-wikilink
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--batch", path))
	})
}

func TestContextNewBatchRefusesDecision(t *testing.T) {
	dir := runInTempDir(t)
	path := writeBatch(t, dir, `- kind: decision
  title: A decision
  slug: a-decision
  summary: numbered on write
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--batch", path))
	})
}

func TestContextNewBatchDryRunWritesNothing(t *testing.T) {
	dir := runInTempDir(t)
	path := writeBatch(t, dir, `- kind: wiki
  title: Dry
  slug: batch/dry
  summary: dry
`)
	out := string(execute(t, contextNewCmd, nil, "--batch", path, "--dry-run"))
	if !strings.Contains(out, "context/wiki/batch/dry.md") {
		t.Errorf("dry run should print the planned path: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "context/wiki/batch/dry.md")); !os.IsNotExist(err) {
		t.Error("dry run must not write")
	}
}

func TestContextNewBatchTypeOnlyForWiki(t *testing.T) {
	dir := runInTempDir(t)
	path := writeBatch(t, dir, `- kind: analysis
  type: entity
  title: Not a wiki
  slug: not-a-wiki
  summary: wrong field
`)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextNewCmd, nil, "--batch", path))
	})
}
