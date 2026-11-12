package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// glossaryRowsResult decodes the command's JSON output.
func glossaryRowsResult(t *testing.T, out []byte) []glossaryRow {
	t.Helper()
	var rows []glossaryRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("glossary is not readable: %v\n%s", err, out)
	}
	return rows
}

// TestWikiGlossaryListsClaimBackedTerms proves the index lists what the lint accepts
// and skips what it reports: a malformed entry is a finding, not a half-row.
func TestWikiGlossaryListsClaimBackedTerms(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: a page with two defined terms and one that is not
created: 2026-10-01T00:00:00Z
updated: 2026-10-20T10:00:00Z
project: p
---

## Summary

A page.

## Claims

1. {#claim-1} <!-- c-aaaaaaaa --> a fact (refs/spec.md@abcdef01:1)
2. {#claim-2} <!-- c-bbbbbbbb --> another fact (refs/spec.md@abcdef01:2)

## Terms

- {#claim-1} <!-- c-aaaaaaaa --> **Alpha** — the first term. (refs/spec.md@abcdef01:3-4)
- {#claim-2} <!-- c-bbbbbbbb --> **Beta** — the second term. (refs/spec.md@abcdef01:5-6)
- **Gamma** — reported by the lint, not listed here. (refs/spec.md@abcdef01:7)
`)

	rows := glossaryRowsResult(t, execute(t, contextWikiGlossaryCmd, nil, "--format", "json"))
	if len(rows) != 2 {
		t.Fatalf("want two defined terms, got %+v", rows)
	}
	if rows[0].Term != "Alpha" || rows[0].Claim != "c-aaaaaaaa" || rows[0].Digest == "" {
		t.Errorf("first row = %+v", rows[0])
	}
	if rows[1].Term != "Beta" || rows[1].Page != "architecture/alpha" || rows[1].Anchor != 2 {
		t.Errorf("second row = %+v", rows[1])
	}
	if !strings.Contains(rows[0].Span, "@abcdef01:3-4") {
		t.Errorf("the span is the entry's own, got %+v", rows[0])
	}

	// The same parser feeds the lint: Gamma is a finding there.
	findings := strings.Join(lintTermsOf(termsPage("- **Gamma** — a definition. (refs/spec.md@abcdef01:7)")), "\n")
	if !strings.Contains(findings, "no claim anchor") {
		t.Errorf("the lint must report the entry the index skips, got %q", findings)
	}
}

// TestWikiGlossaryIsEmptyAndHonest proves an empty corpus says so rather than
// printing nothing.
func TestWikiGlossaryIsEmptyAndHonest(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: a page with no terms
created: 2026-10-01T00:00:00Z
updated: 2026-10-20T10:00:00Z
project: p
---

## Summary

A page.

## Claims

1. {#claim-1} <!-- c-aaaaaaaa --> a fact (refs/spec.md@abcdef01:1)
`)
	out := string(execute(t, contextWikiGlossaryCmd, nil))
	if !strings.Contains(out, "defines no term") {
		t.Errorf("an empty glossary must say so, got %q", out)
	}
}

// aliasGlossaryCorpus writes two pages: one claiming two other names and one
// claiming none, so a test can assert both the rows and the untouched default.
func aliasGlossaryCorpus(t *testing.T) {
	t.Helper()
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", `---
kind: wiki
id: architecture/alpha
title: Alpha
aliases:
  - "Event Sourcing"
  - "event store"
type: concept
status: active
summary: a page with two other names
---

## Summary

A page.
`)
	writeCtxDoc(t, "context/wiki/architecture/beta.md", `---
kind: wiki
id: architecture/beta
title: Beta
type: concept
status: active
summary: a page with no alias
---

## Summary

Another page.
`)
}

// TestWikiGlossaryAliasesResolveAPage proves the flag answers "which page is
// called X by another name", with the file to open, and that the case fold is
// only comparison: the written form reaches the output.
func TestWikiGlossaryAliasesResolveAPage(t *testing.T) {
	aliasGlossaryCorpus(t)

	out := string(execute(t, contextWikiGlossaryCmd, nil, "--aliases", "--format", "json"))
	var rows []glossaryAliasRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the two aliases", rows)
	}
	// "Event Sourcing" and "event store" sort case-insensitively, by lower form.
	if rows[0].Alias != "Event Sourcing" || rows[1].Alias != "event store" {
		t.Errorf("order = %q, %q", rows[0].Alias, rows[1].Alias)
	}
	for _, row := range rows {
		if row.Page != "architecture/alpha" || row.Path != "context/wiki/architecture/alpha.md" {
			t.Errorf("row = %+v, want alpha and its file", row)
		}
	}

	text := string(execute(t, contextWikiGlossaryCmd, nil, "--aliases"))
	if !strings.Contains(text, "Event Sourcing — architecture/alpha (context/wiki/architecture/alpha.md)") {
		t.Errorf("text output is missing the alias row:\n%s", text)
	}
}

// TestWikiGlossaryAliasesLeaveTheDefaultUntouched proves the new flag cannot
// change an existing consumer: without it the output is the term list, and a
// corpus with aliases but no `## Terms` says what it always said.
func TestWikiGlossaryAliasesLeaveTheDefaultUntouched(t *testing.T) {
	aliasGlossaryCorpus(t)

	out := string(execute(t, contextWikiGlossaryCmd, nil))
	if strings.Contains(out, "Event Sourcing") {
		t.Errorf("the default output must not carry alias rows:\n%s", out)
	}
	if !strings.Contains(out, "defines no term") {
		t.Errorf("the default on a term-less corpus must keep its wording:\n%s", out)
	}
}

// TestWikiGlossaryAliasesEmptyIsHonest proves a corpus that declares no alias
// says so rather than printing nothing.
func TestWikiGlossaryAliasesEmptyIsHonest(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/beta.md", `---
kind: wiki
id: architecture/beta
title: Beta
type: concept
status: active
summary: a page with no alias
---

## Summary

Another page.
`)

	if out := string(execute(t, contextWikiGlossaryCmd, nil, "--aliases")); !strings.Contains(out, "no page declares an `aliases` entry yet") {
		t.Errorf("an empty alias list must say so, got %q", out)
	}
	if out := string(execute(t, contextWikiGlossaryCmd, nil, "--aliases", "--format", "json")); strings.TrimSpace(out) != "[]" {
		t.Errorf("empty JSON = %q, want []", out)
	}
}
