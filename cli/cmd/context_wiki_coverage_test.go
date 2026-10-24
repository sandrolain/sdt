package cmd

import (
	"os"
	"strings"
	"testing"
)

const covRunArtifact = `---
source: test
---

# Book

# Chapter 1. One

## Alpha

one
two
three

## Beta

four
five

## Gamma

six

# Index

## Entries
`

const covRunIndexPage = `---
kind: wiki
id: sources/book
title: Book
type: entity
status: draft
summary: s
relations: {}
tags: [sources]
sources:
  - refs/converted/book-abcdef123456.md
---

## Summary

## Structure
`

const covRunConcept = `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: s
relations: {refers_to: "[[sources/book|book]]"}
tags: [test]
sources:
  - refs/converted/book-abcdef123456.md
---

## Summary

Alpha is the first concept.

## Claims

1. {#claim-1} Alpha (refs/converted/book-abcdef123456.md@abcdef12:9-14)
2. {#claim-2} Beta starts here (refs/converted/book-abcdef123456.md@abcdef12:15)
`

func writeCovRunFixture(t *testing.T) {
	t.Helper()
	writeCtxDoc(t, "context/refs/converted/book-abcdef123456.md", covRunArtifact)
	writeCtxDoc(t, "context/wiki/sources/book.md", covRunIndexPage)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", covRunConcept)
}

func TestWikiCoverageClassifiesAndWrites(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)

	out := execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--write")
	if !strings.Contains(string(out), "coverage updated") {
		t.Fatalf("unexpected output:\n%s", out)
	}

	content, err := os.ReadFile("context/wiki/sources/book.md")
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	for _, want := range []string{
		"coverage:",
		"  units: 5",
		"  ok: 1",
		"  part: 1",
		"  gap: 1",
		"  no_op: 2",
		"- L9 · Alpha · OK · —",
		"- L15 · Beta · PART · gap",
		"- L5 · Book · CLOSED · not-conceptual",
		"- L20 · Gamma · GAP · gap",
		"- L26 · Entries · CLOSED · not-conceptual",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q:\n%s", want, page)
		}
	}

	// Idempotence: a second run must be byte-identical.
	execute(t, contextWikiCoverageCmd, nil, "sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--write")
	again, err := os.ReadFile("context/wiki/sources/book.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != page {
		t.Errorf("--write is not idempotent:\nfirst:\n%s\nsecond:\n%s", page, again)
	}
}

func TestWikiCoverageReceipt(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	out := execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--receipt", "--format", "text")
	text := string(out)
	for _, want := range []string{
		"source: refs/converted/book-abcdef123456.md",
		"route: single-pass",
		"route_headings: 4",
		"coverage_units: 5",
		"coverage: ok=1 part=1 gap=1 no_op=2",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("receipt is missing %q:\n%s", want, text)
		}
	}
}

func TestWikiCoverageExcludesTheIndexPageFromEvidence(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	// The index page cites the whole artifact: if it counted, every unit would
	// be OK. The destination is never its own evidence.
	writeCtxDoc(t, "context/wiki/sources/book.md", strings.Replace(covRunIndexPage,
		"## Structure",
		"Frame (refs/converted/book-abcdef123456.md@abcdef12:1-26)\n\n## Structure", 1))
	out := execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--format", "json")
	if !strings.Contains(string(out), `"ok": 1`) {
		t.Errorf("index page citation leaked into the audit:\n%s", out)
	}
}

func TestWikiCoverageReadsDispositions(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeCtxDoc(t, "context/wiki/sources/book.md", strings.Replace(covRunIndexPage,
		"## Structure",
		"## Structure\n\n- L5 · Book · GAP · no-op\n", 1))
	out := execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--format", "json")
	if !strings.Contains(string(out), `"no_op": 2`) {
		t.Errorf("the recorded disposition was not kept:\n%s", out)
	}
}

const covRunInventory = `---
kind: notes
note_type: concept-inventory
sources:
  - refs/converted/book-abcdef123456.md
summary: inventory fixture
inventory:
  units: 5
  candidates: 4
  paged: 2
  merged: 0
  inline: 2
  declined: 0
  pending: 0
---

## Paged concepts

| id | concept | disposition | target | claim | source |
|---|---|---|---|---|---|
| C1 | Alpha | page | architecture/alpha | claim-1 | refs/converted/book-abcdef123456.md |
| C2 | Alpha | page | architecture/alpha | claim-2 | refs/converted/book-abcdef123456.md |
`

func TestWikiCoverageWritesConcepts(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeCtxDoc(t, "context/notes/book-inventory.md", covRunInventory)

	out := execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md",
		"--inventory", "notes/book-inventory.md", "--write")
	if !strings.Contains(string(out), "coverage updated") {
		t.Fatalf("unexpected output:\n%s", out)
	}

	content, err := os.ReadFile("context/wiki/sources/book.md")
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	for _, want := range []string{
		"coverage:",
		"  units: 5",
		"concepts:",
		"  from: notes/book-inventory.md",
		"  candidates: 4",
		"  paged: 2",
		"  merged: 0",
		"  inline: 2",
		"  declined: 0",
		"  pending: 0",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q:\n%s", want, page)
		}
	}
	if strings.Index(page, "coverage:") > strings.Index(page, "concepts:") {
		t.Errorf("concepts must follow coverage:\n%s", page)
	}

	// A second run without --inventory reads the recorded `from` and is
	// byte-identical.
	execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--write")
	again, err := os.ReadFile("context/wiki/sources/book.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != page {
		t.Errorf("a concepts write without --inventory is not idempotent:\nfirst:\n%s\nsecond:\n%s", page, again)
	}
}

func TestWikiCoverageReceiptQuotesConcepts(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeCtxDoc(t, "context/notes/book-inventory.md", covRunInventory)
	execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md",
		"--inventory", "notes/book-inventory.md", "--write")
	out := execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md", "--receipt")
	if !strings.Contains(string(out), "concepts: 2/4") {
		t.Errorf("receipt does not quote the concept counters:\n%s", out)
	}
}

func TestWikiCoverageRefusesAnInventoryForAnotherSource(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeCtxDoc(t, "context/notes/book-inventory.md", strings.Replace(covRunInventory,
		"refs/converted/book-abcdef123456.md", "refs/converted/other-abcdef123456.md", 2))
	stubWikiExit(t)
	execute(t, contextWikiCoverageCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md",
		"--inventory", "notes/book-inventory.md", "--write")
}
