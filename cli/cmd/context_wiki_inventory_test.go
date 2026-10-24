package cmd

import (
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
