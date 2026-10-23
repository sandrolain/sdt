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
		"- L5 · Book · GAP · not-conceptual",
		"- L20 · Gamma · GAP · gap",
		"- L26 · Entries · GAP · not-conceptual",
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
		"section_headings: 4",
		"coverage: 1/5",
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
