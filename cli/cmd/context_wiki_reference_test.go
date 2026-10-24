package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

const referenceIndexPage = `---
kind: wiki
id: sources/book
title: Book
type: entity
status: draft
summary: Book index.
sources:
  - refs/reference/book/a.md
---

## References

- ` + "`refs/reference/book/a.md`" + `
- ` + "`refs/reference/book/b.md`" + `
`

func writeReferenceFixture(t *testing.T, page string) {
	t.Helper()
	writeCtxDoc(t, "context/refs/reference/book/a.md", "a\n")
	writeCtxDoc(t, "context/refs/reference/book/b.md", "b\n")
	if page != "" {
		writeCtxDoc(t, "context/wiki/sources/book.md", page)
	}
}

func TestWikiReferencePassesWhenEveryFileIsCited(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeReferenceFixture(t, referenceIndexPage)
	out := string(execute(t, contextWikiReferenceCmd, nil))
	if !strings.Contains(out, "[OK] refs/reference/book") {
		t.Fatalf("complete bridge did not pass:\n%s", out)
	}
}

func TestWikiReferenceFailsWhenAFileIsMissing(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	page := strings.Replace(referenceIndexPage, "- `refs/reference/book/b.md`\n", "", 1)
	writeReferenceFixture(t, page)
	var out string
	shouldExitWithCode(t, exitWikiError, func() string {
		out = string(execute(t, contextWikiReferenceCmd, nil))
		return out
	})
	if !strings.Contains(out, "missing refs/reference/book/b.md") {
		t.Fatalf("missing file was not reported:\n%s", out)
	}
}

func TestWikiReferenceOptionalWithoutCitingPage(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeReferenceFixture(t, "")
	out := string(execute(t, contextWikiReferenceCmd, nil))
	if !strings.Contains(out, "[OPTIONAL] refs/reference/book") {
		t.Fatalf("unlinked bundle was not optional:\n%s", out)
	}
	shouldExitWithCode(t, exitWikiError, func() string {
		return string(execute(t, contextWikiReferenceCmd, nil, "--strict"))
	})
}

func TestWikiReferenceJSON(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeReferenceFixture(t, referenceIndexPage)
	out := execute(t, contextWikiReferenceCmd, nil, "--format", "json")
	var reports []referenceReport
	if err := json.Unmarshal(out, &reports); err != nil {
		t.Fatalf("reference JSON is not readable: %v\n%s", err, out)
	}
	if len(reports) != 1 || reports[0].Status != "pass" || reports[0].Files != 2 {
		t.Errorf("unexpected report: %+v", reports)
	}
}

func TestWikiReferenceConceptPageDoesNotOwnBundle(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeReferenceFixture(t, "")
	concept := strings.Replace(referenceIndexPage, "type: entity", "type: concept", 1)
	concept = strings.Replace(concept, "id: sources/book", "id: architecture/book", 1)
	writeCtxDoc(t, "context/wiki/architecture/book.md", concept)
	out := string(execute(t, contextWikiReferenceCmd, nil))
	if !strings.Contains(out, "[OPTIONAL] refs/reference/book") {
		t.Fatalf("a concept page made the bundle mandatory:\n%s", out)
	}
	shouldExitWithCode(t, exitWikiError, func() string {
		return string(execute(t, contextWikiReferenceCmd, nil, "--strict"))
	})
}
