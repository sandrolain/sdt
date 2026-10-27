package cmd

import (
	"strings"
	"testing"
)

const selectorsDoc = "context/notes/selectors.md"

// selectorsFixture is a document with two tables (one inside a section) and two
// fenced blocks, so every selector has a scope to be wrong about.
func selectorsFixture(t *testing.T) string {
	t.Helper()
	setupContextProject(t)
	body := "---\nkind: notes\nuid: " + testUIDv7(21) + "\nsummary: s\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n" +
		"## Data\n\n| Name | Value |\n| --- | --- |\n| alpha | 1 |\n| beta | 2 |\n\n" +
		"## Code\n\n" + "```go\nold code\n```\n\n" + "```mermaid\ngraph LR\n  A-->B\n```\n"
	writeCtxDoc(t, selectorsDoc, body)
	return "notes/selectors.md"
}

func TestContextEditRowInsideASection(t *testing.T) {
	ref := selectorsFixture(t)
	execute(t, contextEditCmd, nil, ref, "--section", "Data", "--row", "alpha", "--input", "| alpha | 9 |")

	got := readDoc(t, ref)
	if !strings.Contains(got, "| alpha | 9 |") || strings.Contains(got, "| alpha | 1 |") {
		t.Errorf("row not replaced:\n%s", got)
	}
	if !strings.Contains(got, "| beta | 2 |") {
		t.Errorf("a sibling row was lost:\n%s", got)
	}
}

func TestContextEditRowScopedToTheSection(t *testing.T) {
	ref := selectorsFixture(t)
	// `alpha` lives under Data, not Code: the selector must refuse.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, ref, "--section", "Code", "--row", "alpha", "--input", "| alpha | 9 |"))
	})
	if !strings.Contains(readDoc(t, ref), "| alpha | 1 |") {
		t.Error("a refused row edit wrote the document")
	}
}

func TestContextEditRowRefusesAmbiguousAndMissing(t *testing.T) {
	ref := selectorsFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, ref, "--row", "nope", "--input", "| nope | 0 |"))
	})

	writeCtxDoc(t, selectorsDoc, strings.Replace(readDoc(t, ref), "| beta | 2 |", "| alpha | 2 |", 1))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, ref, "--row", "alpha", "--input", "| alpha | 9 |"))
	})
}

func TestContextEditFenceByIndexAndInfo(t *testing.T) {
	ref := selectorsFixture(t)
	execute(t, contextEditCmd, nil, ref, "--fence", "1", "--input", "new code")
	got := readDoc(t, ref)
	if !strings.Contains(got, "```go\nnew code\n```") {
		t.Errorf("go fence not replaced:\n%s", got)
	}
	if !strings.Contains(got, "```mermaid\ngraph LR") {
		t.Errorf("the second fence was touched:\n%s", got)
	}

	execute(t, contextEditCmd, nil, ref, "--fence", "mermaid", "--input", "graph TD\n  X-->Y")
	got = readDoc(t, ref)
	if !strings.Contains(got, "```mermaid\ngraph TD\n  X-->Y\n```") {
		t.Errorf("mermaid fence not replaced by info:\n%s", got)
	}
}

func TestContextEditFenceRefusesMissingAndCombinations(t *testing.T) {
	ref := selectorsFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, ref, "--fence", "9", "--input", "x"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, ref, "--fence", "go", "--section", "Code", "--input", "x"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextEditCmd, nil, ref, "--row", "alpha", "--append", "--input", "x"))
	})
}

func TestContextEditRowDryRunWritesNothing(t *testing.T) {
	ref := selectorsFixture(t)
	before := readDoc(t, ref)
	execute(t, contextEditCmd, nil, ref, "--row", "alpha", "--input", "| alpha | 9 |", "--dry-run")
	if got := readDoc(t, ref); got != before {
		t.Error("--dry-run wrote the document")
	}
}
