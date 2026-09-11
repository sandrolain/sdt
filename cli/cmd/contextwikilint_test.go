package cmd

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// wikiPageFixture returns a minimally valid wiki page body for the given id/title.
func wikiPageFixture(id, title, typ, status string) string {
	return `---
kind: wiki
id: ` + id + `
title: ` + title + `
type: ` + typ + `
status: ` + status + `
summary: Summary for ` + title + `
tags:
  - backend
relations:
  refers_to:
    - [[accounts|Account Service]]
    - [[db|Database]]
sources:
  - refs/spec.md
---
## Summary
Shape of ` + title + `.
## Claims
1. One fact. {#claim-1} (refs/spec.md@abc:1)
`
}

const wikiDBPage = `---
kind: wiki
id: db
title: Database
type: concept
status: active
summary: Shared data store
tags:
  - backend
relations:
  refers_to:
    - [[accounts|Account Service]]
sources:
  - refs/spec.md
---
## Summary
Shape of the data store.
## Claims
1. DB stores rows. {#claim-1} (refs/spec.md@abc:3)
`

// wikiSourceDirs creates the empty immutable source dirs that a wiki lint run
// expects (missing markers are a warning).
func wikiSourceDirs(t *testing.T) {
	t.Helper()
	for _, d := range []string{sdtIngestionDir, sdtRefsDir} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWikiLintClean(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/accounts.md", wikiPageFixture("accounts", "Account Service", "module", "active"))
	writeCtxDoc(t, "context/wiki/db.md", wikiDBPage)
	out := execute(t, contextWikiLintCmd, nil)
	if len(out) != 0 {
		t.Fatalf("expected clean lint, got:\n%s", out)
	}
}

func TestWikiLintCleanJSON(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/accounts.md", wikiPageFixture("accounts", "Account Service", "module", "active"))
	writeCtxDoc(t, "context/wiki/db.md", wikiDBPage)
	out := execute(t, contextWikiLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "[]") {
		t.Fatalf("expected empty issue array in JSON output:\n%s", out)
	}
}

// TestWikiLintSchemaErrors exercises every schema CRITICAL on one broken page.
func TestWikiLintSchemaErrors(t *testing.T) {
	setupContextProject(t)
	broken := `---
kind: nope
id: wrong-slug
summary: has summary but no title/type/status/tags
verified: maybe
tags:
  - Bad Tag!
---
## Summary
x
## Claims
1. Uncited fact. {#claim-9}
`
	writeCtxDoc(t, "context/wiki/broken.md", broken)
	out := wikiLintError(t)
	for _, want := range []string{
		`kind "nope"`,
		`does not match the file id "broken"`,
		"missing mandatory `title`",
		"unknown `type`",
		"missing `status`",
		"`verified` must be true or false",
		"malformed tag",
		"claim {#claim-9} has no refs/ citation",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in wiki lint output:\n%s", want, out)
		}
	}
}

// TestWikiLintClosedVerbs flags verbs outside the closed vocabulary in both the
// relations block and body links.
func TestWikiLintClosedVerbs(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/accounts.md", `---
kind: wiki
id: accounts
title: Account Service
type: module
status: active
summary: s
tags:
  - backend
relations:
  relates_to:
    - [[db|Database]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
This [[extends::db]] link uses a bad verb.
`)
	writeCtxDoc(t, "context/wiki/db.md", wikiDBPage)
	out := wikiLintError(t)
	if !strings.Contains(out, `relation verb "relates_to" is outside the closed vocabulary`) {
		t.Errorf("missing relation verb error:\n%s", out)
	}
	if !strings.Contains(out, `body link verb "extends" is outside the closed vocabulary`) {
		t.Errorf("missing body verb error:\n%s", out)
	}
}

func TestWikiLintLinkResolution(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/accounts.md", `---
kind: wiki
id: accounts
title: Duplicate Title
type: module
status: active
summary: s
tags:
  - backend
relations:
  depends_on:
    - [[missing-db|Database]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
Broken [[missing-db]] link and [[Duplicate Title]].
`)
	writeCtxDoc(t, "context/wiki/a.md", `---
kind: wiki
id: a
title: Duplicate Title
type: concept
status: active
summary: s
tags:
  - backend
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	out := wikiLintError(t)
	for _, want := range []string{
		`relation depends_on targets missing page "missing-db"`,
		`broken link "[[missing-db]]"`,
		`link target "Duplicate Title" is ambiguous`,
		`duplicate title "Duplicate Title"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in wiki lint output:\n%s", want, out)
		}
	}
}

func TestWikiLintDependsOnCycle(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/a.md", `---
kind: wiki
id: a
title: A
type: concept
status: active
summary: s
tags:
  - x
relations:
  depends_on:
    - [[b|B]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	writeCtxDoc(t, "context/wiki/b.md", `---
kind: wiki
id: b
title: B
type: concept
status: active
summary: s
tags:
  - x
relations:
  depends_on:
    - [[a|A]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	out := wikiLintError(t)
	if !strings.Contains(out, "depends_on cycle detected") {
		t.Errorf("expected cycle detection:\n%s", out)
	}
}

func TestWikiLintSupersedeWarning(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/old.md", `---
kind: wiki
id: old
title: Old Way
type: pattern
status: active
summary: s
tags:
  - x
relations:
  refers_to:
    - [[new|New Way]]
sources:
  - refs/spec.md
---
`+blob(140))
	writeCtxDoc(t, "context/wiki/new.md", `---
kind: wiki
id: new
title: New Way
type: pattern
status: active
summary: s
tags:
  - x
relations:
  supersedes:
    - [[old|Old Way]]
sources:
  - refs/spec.md
---
`+blob(140))
	out := wikiLintWarning(t)
	if !strings.Contains(out, `supersedes "old" but target is still active`) {
		t.Errorf("expected supersede warning:\n%s", out)
	}
	if !strings.Contains(out, "over budget") {
		t.Errorf("expected budget warning:\n%s", out)
	}
}

func TestWikiLintBudgetWarnings(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	claims := []int{}
	for i := 1; i <= 25; i++ {
		claims = append(claims, i)
	}
	writeCtxDoc(t, "context/wiki/long.md", firstPageFixture("long", "Long Page")+blob(1245)+fileBody(claims))
	out := wikiLintWarning(t)
	if !strings.Contains(out, "over budget") {
		t.Errorf("expected budget warning:\n%s", out)
	}
}

// firstPageFixture is the frontmatter part of a page with the given id/title.
func firstPageFixture(id, title string) string {
	return `---
kind: wiki
id: ` + id + `
title: ` + title + `
type: concept
status: active
summary: s
tags:
  - x
sources:
  - refs/spec.md
---
`
}

func TestWikiLintMarkdownLD(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/ld.md", `---
kind: wiki
id: ld
title: LD Page
type: concept
status: active
summary: s
tags:
  - x
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
<!-- markdown-ld -->
{"@type": "Thing", "name": "Other Name", "@context": "https://schema.org"}
`)
	out := wikiLintWarning(t)
	if !strings.Contains(out, `markdown-ld name "Other Name" does not match title "LD Page"`) {
		t.Errorf("expected markdown-ld name mismatch:\n%s", out)
	}

	setupContextProject(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/badld.md", `---
kind: wiki
id: badld
title: Bad LD
type: concept
status: active
summary: s
tags:
  - x
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
<!-- markdown-ld -->
{not json}
`)
	out = wikiLintWarning(t)
	if !strings.Contains(out, "markdown-ld JSON is not well-formed") {
		t.Errorf("expected markdown-ld malformed JSON:\n%s", out)
	}
}

func TestWikiLintMarkers(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/ingestion/raw.md", "---\nkind: wiki\nstatus: archived\nsummary: s\n---\nbody\n")
	writeCtxDoc(t, "context/refs/archived.md", "---\nkind: reference\nstatus: pending\nsummary: s\n---\nbody\n")
	out := wikiLintError(t)
	if !strings.Contains(out, `marker `+"`kind`"+` "wiki" must be "reference" in ingestion/`) {
		t.Errorf("expected ingestion kind marker error:\n%s", out)
	}
	if !strings.Contains(out, `marker `+"`status`"+` "pending" must be "archived" in refs/`) {
		t.Errorf("expected refs status marker error:\n%s", out)
	}
}

func TestWikiLintMissingDir(t *testing.T) {
	dir := setupContextProject(t)
	_ = os.RemoveAll(dir + "/context/wiki")
	out := wikiLintWarning(t)
	if !strings.Contains(out, "directory missing: context/wiki") {
		t.Errorf("expected missing wiki dir warning:\n%s", out)
	}
}

func TestWikiLintOrphanSuggestion(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	// page without inbound links/relations → orphan SUGGESTION, exit stays 0
	writeCtxDoc(t, "context/wiki/a.md", `---
kind: wiki
id: a
title: A
type: concept
status: active
summary: s
tags:
  - x
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	out := execute(t, contextWikiLintCmd, nil)
	if !strings.Contains(string(out), "[SUGGESTION]") || !strings.Contains(string(out), "orphan page") {
		t.Errorf("expected orphan suggestion:\n%s", out)
	}
}

// TestWikiLintSubdirs covers nested wiki pages: valid subpath id, id mismatch
// on a subpath page, and duplicate titles across subdirectories.
func TestWikiLintSubdirs(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	// Valid nested pages: context/wiki/<ctx>/<slug>.md, id = relative subpath.
	writeCtxDoc(t, "context/wiki/backend/auth.md", `---
kind: wiki
id: backend/auth
title: Authentication
type: concept
status: active
summary: s
tags:
  - backend
relations:
  refers_to:
    - [[backend/db|Database]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	writeCtxDoc(t, "context/wiki/backend/db.md", `---
kind: wiki
id: backend/db
title: Database
type: concept
status: active
summary: s
tags:
  - backend
relations:
  refers_to:
    - [[accounts|Account Service]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	// Flat pages may reference nested ones by subpath id.
	writeCtxDoc(t, "context/wiki/accounts.md", `---
kind: wiki
id: accounts
title: Account Service
type: module
status: active
summary: s
tags:
  - backend
relations:
  refers_to:
    - [[backend/auth|Authentication]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	out := execute(t, contextWikiLintCmd, nil)
	if len(out) != 0 {
		t.Fatalf("expected clean recursive lint, got:\n%s", out)
	}

	// Subpath page with a flat slug id → CRITICAL id mismatch.
	writeCtxDoc(t, "context/wiki/backend/badid.md", `---
kind: wiki
id: auth
title: Bad ID
type: concept
status: active
summary: s
tags:
  - backend
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	bad := wikiLintError(t)
	if !strings.Contains(bad, "`id` \"auth\" does not match the file id \"backend/badid\"") {
		t.Errorf("expected subpath id mismatch:\n%s", bad)
	}

	// Duplicate titles across subdirectories still flagged.
	writeCtxDoc(t, "context/wiki/frontend/db.md", `---
kind: wiki
id: frontend/db
title: Database
type: concept
status: active
summary: s
tags:
  - frontend
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	dup := wikiLintError(t)
	if !strings.Contains(dup, `duplicate title "Database"`) {
		t.Errorf("expected duplicate title across subdirs:\n%s", dup)
	}
}

// TestWikiLintBrokenNestedLink verifies that a flat page linking to a nested
// page by wrong (flat) id is flagged, and that nested pages resolve flat ids.
func TestWikiLintBrokenNestedLink(t *testing.T) {
	setupContextProject(t)
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/backend/db.md", `---
kind: wiki
id: backend/db
title: Database
type: concept
status: active
summary: s
tags:
  - backend
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	writeCtxDoc(t, "context/wiki/accounts.md", `---
kind: wiki
id: accounts
title: Account Service
type: module
status: active
summary: s
tags:
  - backend
relations:
  refers_to:
    - [[db|Database]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. f. {#claim-1} (refs/spec.md@a:1)
`)
	out := wikiLintWarning(t)
	if !strings.Contains(out, `relation refers_to targets missing page "db"`) {
		t.Errorf("expected flat-to-nested id mismatch warning:\n%s", out)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

func blob(n int) string {
	return strings.Repeat("filler line content\n", n)
}

func fileBody(claims []int) string {
	var b strings.Builder
	b.WriteString("## Summary\ns\n## Claims\n")
	for _, c := range claims {
		b.WriteString("1. c. {#claim-" + strconv.Itoa(c) + "} (refs/spec.md@a:1)\n")
	}
	return b.String()
}

// wikiLintError runs lint expecting exit code 2 and returns its text output.
func wikiLintError(t *testing.T) string {
	t.Helper()
	var out string
	shouldExitWithCode(t, exitWikiError, func() string {
		out = string(execute(t, contextWikiLintCmd, nil))
		return out
	})
	return out
}

// wikiLintWarning runs lint expecting exit code 1 and returns its text output.
func wikiLintWarning(t *testing.T) string {
	t.Helper()
	var out string
	shouldExitWithCode(t, exitWikiWarn, func() string {
		out = string(execute(t, contextWikiLintCmd, nil))
		return out
	})
	return out
}
