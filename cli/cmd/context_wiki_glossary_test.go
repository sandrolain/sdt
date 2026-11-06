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
