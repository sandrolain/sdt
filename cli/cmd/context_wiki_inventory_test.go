package cmd

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

const inventoryPage = `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: Alpha concept.
sources:
  - refs/converted/book.md
---

## Summary

Alpha is a concept with enough words to exist in the fixture for the inventory
check, which does not read the prose but does load the page through the wiki
builder. The body carries one anchored claim.

## Claims

1. {#claim-1} Alpha (refs/converted/book.md@abcdef12:1-2)
`

func inventoryNoteFixture(candidates, paged, merged, inline, declined, pending int, rows string) string {
	return `---
kind: notes
note_type: concept-inventory
sources:
  - refs/converted/book.md
summary: inventory fixture
inventory:
  units: 2
  candidates: ` + strconv.Itoa(candidates) + `
  paged: ` + strconv.Itoa(paged) + `
  merged: ` + strconv.Itoa(merged) + `
  inline: ` + strconv.Itoa(inline) + `
  declined: ` + strconv.Itoa(declined) + `
  pending: ` + strconv.Itoa(pending) + `
---

## Paged concepts

| id | concept | disposition | target | claim | source |
|---|---|---|---|---|---|
` + rows
}

func writeInventoryFixture(t *testing.T, note string) {
	t.Helper()
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", inventoryPage)
	writeCtxDoc(t, "context/notes/inv.md", note)
}

func TestWikiInventoryClean(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	out := execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md")
	if len(out) != 0 {
		t.Fatalf("expected a clean inventory, got:\n%s", out)
	}
}

func TestWikiInventoryArithmetic(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(3, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "does not add up") {
		t.Fatalf("arithmetic mismatch was not reported:\n%s", out)
	}
}

func TestWikiInventoryMissingTarget(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Missing | page | architecture/missing | claim-1 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "targets missing page") {
		t.Fatalf("missing target was not reported:\n%s", out)
	}
}

func TestWikiInventoryMissingClaim(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-9 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "names claim") {
		t.Fatalf("missing claim was not reported:\n%s", out)
	}
}

func TestWikiInventoryScope(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	// beta cites the source but has no row: the scope check must report it.
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/wiki/architecture/beta.md", strings.ReplaceAll(inventoryPage, "architecture/alpha", "architecture/beta"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "no inventory row") || !strings.Contains(out, "architecture/beta") {
		t.Fatalf("unlisted sourced page was not reported:\n%s", out)
	}
}

func TestWikiInventoryPending(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(2, 1, 0, 0, 0, 1,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "pending") {
		t.Fatalf("pending was not reported:\n%s", out)
	}
}

func TestWikiInventoryChapterPendingDespiteFrontmatterZero(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	note := inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n")
	note += `
## Chapter 9

| candidate | kind | disposition |
|---|---|---|
| generative testing | emerging | pending (read ch9) |

## Totals

| disposition | count |
|---|---:|
| pending (needs a read) | 1 |
`
	writeInventoryFixture(t, note)
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "generative testing") || !strings.Contains(out, "totals say 1 pending") {
		t.Fatalf("chapter/totals pending was not reported:\n%s", out)
	}
}

func TestWikiInventoryDuplicateID(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(2, 2, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n| C1 | Beta | page | architecture/beta | claim-1 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "context/notes/inv.md"))
	if !strings.Contains(out, "duplicate candidate id") {
		t.Fatalf("duplicate id was not reported:\n%s", out)
	}
}

const inventoryIndexPage = `---
kind: wiki
id: sources/book
title: Book
type: entity
status: draft
summary: Book index.
sources:
  - refs/converted/book.md
---

## Summary
`

func TestWikiInventoryWriteRepairsCounters(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(9, 9, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write"))
	if !strings.Contains(out, "inventory updated") {
		t.Fatalf("write did not report an update:\n%s", out)
	}
	note := string(mustReadFile(t, "context/notes/inv.md"))
	for _, want := range []string{"  candidates: 1", "  paged: 1", "  merged: 0", "  inline: 0", "  pending: 0"} {
		if !strings.Contains(note, want) {
			t.Errorf("note is missing %q:\n%s", want, note)
		}
	}
}

func TestWikiInventoryWriteSyncsIndex(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(9, 9, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryIndexPage)
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write", "--sync-index", "--page", "sources/book"))
	if !strings.Contains(out, "concepts updated") {
		t.Fatalf("sync-index did not report the page:\n%s", out)
	}
	page := string(mustReadFile(t, "context/wiki/sources/book.md"))
	for _, want := range []string{
		"concepts:",
		"  from: notes/inv.md",
		"  candidates: 1",
		"  paged: 1",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q:\n%s", want, page)
		}
	}
}

func TestWikiInventoryWriteDryRunDoesNotWrite(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(9, 9, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	before := string(mustReadFile(t, "context/notes/inv.md"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write", "--dry-run"))
	if !strings.Contains(out, "candidates: 1") {
		t.Fatalf("dry run did not print the diff:\n%s", out)
	}
	after := string(mustReadFile(t, "context/notes/inv.md"))
	if before != after {
		t.Errorf("dry run wrote the note")
	}
}

func TestWikiInventoryWriteRefusesEmptyTopTable(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(0, 0, 0, 0, 0, 0, ""))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write"))
	})
}

func TestWikiInventoryBareInline(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 0, 0, 1, 0, 0,
		"| C1 | Alpha | inline | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md"))
	if !strings.Contains(out, "bare inline") {
		t.Fatalf("bare inline was not reported:\n%s", out)
	}
}

func TestWikiInventoryWriteIsIdempotent(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(9, 9, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write")
	first := string(mustReadFile(t, "context/notes/inv.md"))
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write")
	second := string(mustReadFile(t, "context/notes/inv.md"))
	if first != second {
		t.Errorf("inventory --write is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestWikiInventoryWriteRefusesUnknownDisposition(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 0, 0, 0, 0, 0,
		"| C1 | Alpha | mystery | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write"))
	})
}

func TestWikiInventoryWriteRefusesMissingBlock(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	note := strings.Replace(inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"),
		"inventory:\n  units: 2\n  candidates: 1\n  paged: 1\n  merged: 0\n  inline: 0\n  declined: 0\n  pending: 0\n", "", 1)
	writeInventoryFixture(t, note)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write"))
	})
}

func TestWikiInventoryWritePreservesDeclaredInline(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(101, 37, 2, 62, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write")
	note := string(mustReadFile(t, "context/notes/inv.md"))
	for _, want := range []string{"  candidates: 63", "  paged: 1", "  inline: 62"} {
		if !strings.Contains(note, want) {
			t.Errorf("note is missing %q:\n%s", want, note)
		}
	}
}

func TestWikiInventoryWriteUpdatesAggregateTotals(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	note := inventoryNoteFixture(9, 9, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n")
	note += `
## Totals

| disposition | count |
|---|---:|
| pages implied for this book | **9** |
| inline mechanisms/claims | ~0 |
| pending (needs a read) | 2 |
`
	writeInventoryFixture(t, note)
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write")
	updated := string(mustReadFile(t, "context/notes/inv.md"))
	for _, want := range []string{"| pages implied for this book | **1** |", "| inline mechanisms/claims | ~0 |", "| pending (needs a read) | 0 |"} {
		if !strings.Contains(updated, want) {
			t.Errorf("totals are missing %q:\n%s", want, updated)
		}
	}
}

func TestWikiInventoryMultiNotePageRefused(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/notes/inv2.md", inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryIndexPage)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiInventoryCmd, nil,
			"notes/inv.md", "notes/inv2.md", "--write", "--sync-index", "--page", "sources/book"))
	})
}

func TestWikiInventoryPageWriteFailureRestoresNote(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(9, 9, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryIndexPage)
	if err := os.Chmod("context/wiki/sources/book.md", 0o444); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	before := string(mustReadFile(t, "context/notes/inv.md"))
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiInventoryCmd, nil,
			"notes/inv.md", "--write", "--sync-index", "--page", "sources/book"))
	})
	after := string(mustReadFile(t, "context/notes/inv.md"))
	if before != after {
		t.Errorf("note was not restored after the page write failed")
	}
}

func TestWikiInventoryWriteCountsFourColumnPending(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	note := inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n")
	note += `
## Chapter X

| candidate | kind | disposition | note |
|---|---|---|---|
| Later | concept | pending | read next |
`
	writeInventoryFixture(t, note)
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write")
	updated := string(mustReadFile(t, "context/notes/inv.md"))
	if !strings.Contains(updated, "  pending: 1") {
		t.Errorf("four-column pending row was not counted:\n%s", updated)
	}
}

const inventoryCoveredPage = `---
kind: wiki
id: sources/book
title: Book
type: entity
status: draft
summary: Book index.
coverage:
  units: 2
  ok: 2
  part: 0
  gap: 0
  no_op: 0
sources:
  - refs/converted/book.md
---

## Summary
`

func TestWikiInventoryPromotesCleanIndex(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryCoveredPage)
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write", "--sync-index", "--page", "sources/book")
	page := string(mustReadFile(t, "context/wiki/sources/book.md"))
	if !strings.Contains(page, "status: active") {
		t.Fatalf("clean index was not promoted:\n%s", page)
	}
}

func TestWikiInventoryDoesNotPromoteGapOrPending(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	page := strings.Replace(inventoryCoveredPage, "  gap: 0", "  gap: 1", 1)
	writeCtxDoc(t, "context/wiki/sources/book.md", page)
	execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write", "--sync-index", "--page", "sources/book")
	if got := string(mustReadFile(t, "context/wiki/sources/book.md")); !strings.Contains(got, "status: draft") {
		t.Fatalf("a gap page was promoted:\n%s", got)
	}
}

func TestWikiInventoryPromotionDryRunDoesNotWrite(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 1, 0, 0, 0, 0,
		"| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book.md |\n"))
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryCoveredPage)
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md", "--write", "--sync-index", "--page", "sources/book", "--dry-run"))
	if !strings.Contains(out, "status: active") {
		t.Fatalf("dry run did not show the promotion:\n%s", out)
	}
	if got := string(mustReadFile(t, "context/wiki/sources/book.md")); !strings.Contains(got, "status: draft") {
		t.Fatal("dry run wrote the promotion")
	}
}

func TestWikiInventoryInlineClaimFormat(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	note := strings.Replace(inventoryNoteFixture(1, 0, 0, 1, 0, 0,
		"| C1 | Alpha | inline (architecture/alpha) | — | — | refs/converted/book.md |\n"),
		"note_type: concept-inventory\n", "note_type: concept-inventory\ninline_format: claim\n", 1)
	writeInventoryFixture(t, note)
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md"))
	if !strings.Contains(out, "inline (<page>#claim-N; reason)") {
		t.Fatalf("claim-format violation was not reported:\n%s", out)
	}
}

func TestWikiInventoryInlineClaimFormatPasses(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	note := strings.Replace(inventoryNoteFixture(1, 0, 0, 1, 0, 0,
		"| C1 | Alpha | inline (architecture/alpha#claim-1; inline there) | — | claim-1 | refs/converted/book.md |\n"),
		"note_type: concept-inventory\n", "note_type: concept-inventory\ninline_format: claim\n", 1)
	writeInventoryFixture(t, note)
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md"))
	if out != "" {
		t.Fatalf("valid inline claim format was reported:\n%s", out)
	}
}

func TestWikiInventoryLegacyInlineAccepted(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeInventoryFixture(t, inventoryNoteFixture(1, 0, 0, 1, 0, 0,
		"| C1 | Alpha | inline (architecture/alpha) | — | — | refs/converted/book.md |\n"))
	out := string(execute(t, contextWikiInventoryCmd, nil, "notes/inv.md"))
	if out != "" {
		t.Fatalf("legacy inline was reported:\n%s", out)
	}
}
