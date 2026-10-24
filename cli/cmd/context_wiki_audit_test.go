package cmd

import (
	"strings"
	"testing"
)

func TestWikiAuditPrintsReceiptPagesAndClaims(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)

	out := string(execute(t, contextWikiAuditCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md"))
	for _, want := range []string{
		"source: refs/converted/book-abcdef123456.md",
		"route: single-pass",
		"route_headings: 4",
		"coverage_units: 5",
		"coverage: ok=1 part=1 gap=1 no_op=2",
		"pages: 1",
		"claims: 2",
		"inventory: not checked",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("audit is missing %q:\n%s", want, out)
		}
	}
}

func TestWikiAuditJSONAndInventory(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeCtxDoc(t, "context/notes/book-inventory.md", covRunInventory)

	out := string(execute(t, contextWikiAuditCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md",
		"--inventory", "notes/book-inventory.md", "--format", "json"))
	for _, want := range []string{
		`"citing_pages": 1`,
		`"claims": 2`,
		`"inventory": "clean"`,
		`"route_headings": 4`,
		`"coverage_units": 5`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("audit JSON is missing %q:\n%s", want, out)
		}
	}
}

const goodReceiptNote = `---
kind: notes
summary: receipt fixture
---

## Receipt

` + "```text" + `
source: refs/converted/book-abcdef123456.md
route: single-pass
route_headings: 4
lines: 26
coverage_units: 5
coverage: ok=1 part=1 gap=1 no_op=2
concepts: candidates=4 paged=2 merged=0 inline=2 declined=0 pending=0
` + "```" + `
`

func TestWikiReceiptLintAcceptsCompleteBlock(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/receipt.md", goodReceiptNote)
	out := string(execute(t, contextWikiReceiptLintCmd, nil, "notes/receipt.md"))
	if !strings.Contains(out, "receipt: clean") {
		t.Fatalf("complete receipt was not accepted:\n%s", out)
	}
}

func TestWikiReceiptLintAcceptsGeneratedAudit(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeCtxDoc(t, "context/notes/book-inventory.md", covRunInventory)
	receipt := string(execute(t, contextWikiAuditCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md",
		"--inventory", "notes/book-inventory.md"))
	writeCtxDoc(t, "context/notes/generated-receipt.md",
		"---\nkind: notes\nsummary: generated receipt\n---\n\n## Receipt\n\n```text\n"+receipt+"```\n")
	out := string(execute(t, contextWikiReceiptLintCmd, nil, "notes/generated-receipt.md"))
	if !strings.Contains(out, "receipt: clean") {
		t.Fatalf("generated receipt did not pass lint:\n%s", out)
	}
}

func TestWikiReceiptLintRejectsOldBlock(t *testing.T) {
	setupContextProject(t)
	old := strings.Replace(goodReceiptNote, "route_headings: 4\n", "section_headings: 4\n", 1)
	old = strings.Replace(old, "coverage_units: 5\n", "", 1)
	old = strings.Replace(old, "coverage: ok=1 part=1 gap=1 no_op=2", "coverage: 1/5", 1)
	old = strings.Replace(old, "concepts: candidates=4 paged=2 merged=0 inline=2 declined=0 pending=0\n", "", 1)
	writeCtxDoc(t, "context/notes/receipt.md", old)

	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, contextWikiReceiptLintCmd, nil, "notes/receipt.md"))
		return out
	})
	for _, want := range []string{"missing route_headings", "coverage is missing ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("receipt lint is missing %q:\n%s", want, out)
		}
	}
}

func TestWikiReceiptLintAcceptsSinglePassWithoutCoverage(t *testing.T) {
	setupContextProject(t)
	note := `---
kind: notes
summary: single-pass receipt
---

## Receipt

` + "```text" + `
source: refs/converted/book-abcdef123456.md
route: single-pass
route_headings: 4
lines: 26
` + "```" + `
`
	writeCtxDoc(t, "context/notes/receipt.md", note)
	out := string(execute(t, contextWikiReceiptLintCmd, nil, "notes/receipt.md"))
	if !strings.Contains(out, "receipt: clean") {
		t.Fatalf("a minimal single-pass receipt was rejected:\n%s", out)
	}
}

func TestWikiReceiptLintRejectsBookScaleWithoutCoverage(t *testing.T) {
	setupContextProject(t)
	note := `---
kind: notes
summary: book-scale receipt
---

## Receipt

` + "```text" + `
source: refs/converted/book-abcdef123456.md
route: book-scale
route_headings: 159
lines: 5383
` + "```" + `
`
	writeCtxDoc(t, "context/notes/receipt.md", note)
	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, contextWikiReceiptLintCmd, nil, "notes/receipt.md"))
		return out
	})
	for _, want := range []string{"missing coverage_units", "missing coverage", "missing concepts"} {
		if !strings.Contains(out, want) {
			t.Errorf("book-scale lint is missing %q:\n%s", want, out)
		}
	}
}

func TestWikiReceiptLintRejectsNoBlock(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/receipt.md", "---\nkind: notes\nsummary: no receipt\n---\n\n## Receipt\n\nnone\n")
	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, contextWikiReceiptLintCmd, nil, "notes/receipt.md"))
		return out
	})
	if !strings.Contains(out, "no fenced receipt block found") {
		t.Errorf("missing no-block finding:\n%s", out)
	}
}
