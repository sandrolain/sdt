package cmd

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// wikiPageFixture returns a valid dense wiki page for the given id/title.
// denseWikiBody keeps a fixture at the density floor: eight claims and a
// summary long enough that the depth check does not fire on unrelated tests.
func denseWikiBody(prefix string) string {
	summary := prefix + " The page states the concept, the mechanism that makes it work, the consequence for the architecture, and the case where it does not apply. It names the trade-off it carries and the related concept that owns the neighbouring decision, so a reader can act without the source, and it names the source it rests on."
	var b strings.Builder
	b.WriteString("\n## Summary\n")
	b.WriteString(summary)
	b.WriteString("\n## Claims\n")
	for i := 1; i <= 8; i++ {
		b.WriteString("1. {#claim-" + strconv.Itoa(i) + "} fact " + strconv.Itoa(i) + " (refs/spec.md@abc:1)\n")
	}
	return b.String()
}

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
---` + denseWikiBody("Shape of "+title+".")
}

var wikiDBPage = `---
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
---` + denseWikiBody("Shape of the data store.")

// wikiSourceDirs creates the empty immutable source dir a wiki lint run
// expects (a missing ingestion/ marker is a warning). refs/ is excluded.
func wikiSourceDirs(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(sdtIngestionDir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func TestWikiLintClean(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
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
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/accounts.md", wikiPageFixture("accounts", "Account Service", "module", "active"))
	writeCtxDoc(t, "context/wiki/db.md", wikiDBPage)
	out := execute(t, contextWikiLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "[]") {
		t.Fatalf("expected empty issue array in JSON output:\n%s", out)
	}
}

func TestWikiLintReachableKnowledgeAxes(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/axes.md", `---
kind: wiki
id: axes
title: Axes
type: concept
status: active
summary: Axis fixture
tags:
  - backend
maturity: stable
verified_at: 2026-10-11T00:00:00Z
---
## Summary
Axis test fixture.
`)

	out := wikiLintWarning(t)
	for _, phrase := range []string{
		"[WARNING] context/wiki/axes.md: `maturity` \"stable\" is outside the vocabulary",
		"[WARNING] context/wiki/axes.md: `verified_at` without `verified_against`",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("sdt context wiki lint did not report %q:\n%s", phrase, out)
		}
	}

	// These kinds are not sent through corpus-wide lint: the same fixture is
	// rejected by the positional corpus-lint gate, while the dedicated loader
	// above owns the wiki-only axis checks.
	if contextLintCorpusPath("context/wiki/axes.md") {
		t.Error("wiki unexpectedly entered corpus-wide lint")
	}
}

func TestGeneratedWikiInstructionRoutesKnowledgeAxesToDedicatedLint(t *testing.T) {
	var wikiInstruction string
	for _, file := range instructionFiles("p", "g") {
		if file.name == "wiki.md" {
			wikiInstruction = file.body
			break
		}
	}
	if wikiInstruction == "" {
		t.Fatal("generated wiki.md instruction not found")
	}
	flat := strings.Join(strings.Fields(wikiInstruction), " ")
	for _, phrase := range []string{
		"`sdt context wiki lint` reports an unknown stage",
		"`sdt context wiki lint` reports half a stamp",
	} {
		if !strings.Contains(flat, phrase) {
			t.Errorf("wiki instruction missing dedicated-linter promise %q", phrase)
		}
	}
}

// TestWikiLintSchemaErrors exercises every schema CRITICAL on one broken page.
// TestWikiLintDuplicateFrontmatterKey flags a page whose frontmatter declares the
// same top-level key twice: the readers keep the first and drop the second, so the
// page cannot be read the way it was written.
func TestWikiLintDuplicateFrontmatterKey(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	page := `---
kind: wiki
id: twice
title: Twice
type: concept
status: active
summary: s
tags:
  - x
relations:
  refers_to:
    - [[accounts|Account Service]]
relations:
  part_of:
    - [[db|Database]]
sources:
  - refs/spec.md
---
## Summary
s
## Claims
1. one. {#claim-1} (refs/spec.md@abc:1)
`
	writeCtxDoc(t, "context/wiki/twice.md", page)
	out := wikiLintError(t)
	if !strings.Contains(out, "duplicate frontmatter key `relations` at lines 10 and 13") {
		t.Errorf("expected the duplicate key and both lines, got:\n%s", out)
	}
}

// TestWikiLintDuplicateClaimAnchor flags the same anchor twice in one page; the
// same number on two different pages is the doctrine and stays clean.
func TestWikiLintDuplicateClaimAnchor(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	dup := `---
kind: wiki
id: dup
title: Dup
type: concept
status: active
summary: s
tags:
  - x
sources:
  - refs/spec.md
---
## Summary
A claim {#claim-1} (refs/spec.md@a:1).
## Claims
1. one {#claim-1} (refs/spec.md@a:1).
2. two {#claim-2} (refs/spec.md@a:2).
`
	writeCtxDoc(t, "context/wiki/dup.md", dup)
	writeCtxDoc(t, "context/wiki/peer.md", wikiDBPage)
	out := wikiLintError(t)
	for _, want := range []string{
		"duplicate claim anchor {#claim-1} at lines 14 and 16",
		// The gap the duplicate implies is not a second finding.
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in wiki lint output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not contiguous") {
		t.Errorf("a duplicate must not also report a gap:\n%s", out)
	}
}

// TestWikiLintClaimAnchorGap is the WARNING side: a missing number is residue, not
// a lie, and the finding names what is missing.
func TestWikiLintClaimAnchorGap(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/gap.md", firstPageFixture("gap", "Gap Page")+fileBody([]int{1, 2, 4}))
	out := wikiLintWarning(t)
	if !strings.Contains(out, "claim anchors are not contiguous: missing claim anchor {#claim-3}") {
		t.Errorf("expected the missing anchor, got:\n%s", out)
	}
}

// TestWikiLintSchemaErrors flags a page whose frontmatter fields are broken.
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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

	out := wikiLintError(t)
	if !strings.Contains(out, "depends_on cycle detected") {
		t.Errorf("expected cycle detection:\n%s", out)
	}
}

func TestWikiLintSupersedeWarning(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
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
	enableFrontmatterFamilies(t, "maturity", "verified")
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

// TestWikiLintBudgetBoundary pins the measure: a page at the budget is at the
// budget (120 `wc -l` lines pass), one more warns, and the finding names `wc -l` so
// a trim has one instrument to re-run.
func TestWikiLintBudgetBoundary(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	atBudget := firstPageFixture("atbudget", "At Budget") + blob(108) // 12 + 108 = 120
	writeCtxDoc(t, "context/wiki/atbudget.md", atBudget)
	if out := string(execute(t, contextWikiLintCmd, nil)); strings.Contains(out, "over budget") {
		t.Fatalf("a page exactly at the budget must pass:\n%s", out)
	}

	writeCtxDoc(t, "context/wiki/over.md", firstPageFixture("over", "Over")+blob(109))
	out := wikiLintWarning(t)
	if !strings.Contains(out, "page is 121 lines (budget ~120; counted as `sdt read --count`)") {
		t.Errorf("expected the labelled count, got:\n%s", out)
	}
}

// TestWikiLintBudgetSkipsIndexPage pins that the concept budget does not apply to
// a source's index page — its `## Structure` list is one line per unit, so its
// length is the source's — while the same length without a coverage block is
// still a split signal.
func TestWikiLintBudgetSkipsIndexPage(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/covered.md",
		coveragePage("covered", "coverage:\n  units: 10\n  ok: 7\n  part: 2\n  gap: 0\n  no_op: 1\n")+blob(130))
	writeCtxDoc(t, "context/wiki/peer.md", coveragePeer)
	if out := string(execute(t, contextWikiLintCmd, nil)); strings.Contains(out, "over budget") {
		t.Fatalf("an index page's unit list must not trip the concept budget:\n%s", out)
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
	enableFrontmatterFamilies(t, "maturity", "verified")
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
	enableFrontmatterFamilies(t, "maturity", "verified")
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
	// refs/ is excluded from lint: its markers are not enforced.
	if strings.Contains(out, "refs/") {
		t.Errorf("expected refs/ to be excluded from lint:\n%s", out)
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
	enableFrontmatterFamilies(t, "maturity", "verified")
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
---`+denseWikiBody("s"))

	out := execute(t, contextWikiLintCmd, nil)
	if !strings.Contains(string(out), "[SUGGESTION]") || !strings.Contains(string(out), "orphan page") {
		t.Errorf("expected orphan suggestion:\n%s", out)
	}
}

// TestWikiLintSubdirs covers nested wiki pages: valid subpath id, id mismatch
// on a subpath page, and duplicate titles across subdirectories.
func TestWikiLintSubdirs(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

	dup := wikiLintError(t)
	if !strings.Contains(dup, `duplicate title "Database"`) {
		t.Errorf("expected duplicate title across subdirs:\n%s", dup)
	}
}

// TestWikiLintBrokenNestedLink verifies that a flat page linking to a nested
// page by wrong (flat) id is flagged, and that nested pages resolve flat ids.
func TestWikiLintBrokenNestedLink(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
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
---`+denseWikiBody("s"))

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
---`+denseWikiBody("s"))

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

// coveragePage is a valid wiki page carrying the given coverage block (a
// `coverage:` key plus its indented lines), or none when block is empty.
func coveragePage(id, block string) string {
	return `---
kind: wiki
id: ` + id + `
title: ` + id + `
type: entity
status: active
summary: s
tags:
  - x
relations:
  refers_to:
    - [[peer|Peer]]
` + block + `sources:
  - refs/spec.md
---
## Summary
Shape of ` + id + `.
`
}

const coveragePeer = `---
kind: wiki
id: peer
title: Peer
type: concept
status: active
summary: s
tags:
  - x
relations:
  refers_to:
    - [[covered|Covered]]
sources:
  - refs/spec.md
---
## Summary
Shape of peer.
`

// TestWikiLintCoverage pins the countable half of the coverage record: a
// consistent block is silent, a sum that does not match `units`, a non-zero gap
// and a malformed block are reported, and an absent block is not a finding.
func TestWikiLintCoverage(t *testing.T) {
	t.Run("consistent block is silent", func(t *testing.T) {
		setupContextProject(t)
		wikiSourceDirs(t)
		writeCtxDoc(t, "context/wiki/covered.md", coveragePage("covered", "coverage:\n  units: 10\n  ok: 7\n  part: 2\n  gap: 0\n  no_op: 1\n"))
		writeCtxDoc(t, "context/wiki/peer.md", coveragePeer)
		if out := string(execute(t, contextWikiLintCmd, nil)); strings.Contains(out, "coverage") {
			t.Fatalf("a consistent coverage block must not be a finding:\n%s", out)
		}
	})

	t.Run("sum mismatch", func(t *testing.T) {
		setupContextProject(t)
		wikiSourceDirs(t)
		writeCtxDoc(t, "context/wiki/covered.md", coveragePage("covered", "coverage:\n  units: 10\n  ok: 6\n  part: 2\n  gap: 0\n  no_op: 1\n"))
		writeCtxDoc(t, "context/wiki/peer.md", coveragePeer)
		if out := wikiLintWarning(t); !strings.Contains(out, "does not add up") {
			t.Fatalf("expected a sum-mismatch finding:\n%s", out)
		}
	})

	t.Run("non-zero gap", func(t *testing.T) {
		setupContextProject(t)
		wikiSourceDirs(t)
		writeCtxDoc(t, "context/wiki/covered.md", coveragePage("covered", "coverage:\n  units: 10\n  ok: 6\n  part: 1\n  gap: 3\n  no_op: 0\n"))
		writeCtxDoc(t, "context/wiki/peer.md", coveragePeer)
		if out := wikiLintWarning(t); !strings.Contains(out, "as gap") {
			t.Fatalf("expected a gap finding:\n%s", out)
		}
	})

	t.Run("malformed block", func(t *testing.T) {
		setupContextProject(t)
		wikiSourceDirs(t)
		writeCtxDoc(t, "context/wiki/covered.md", coveragePage("covered", "coverage: 10\n"))
		writeCtxDoc(t, "context/wiki/peer.md", coveragePeer)
		if out := wikiLintWarning(t); !strings.Contains(out, "coverage block is malformed") {
			t.Fatalf("expected a malformed-block finding:\n%s", out)
		}
	})

	t.Run("absent block is not a finding", func(t *testing.T) {
		setupContextProject(t)
		wikiSourceDirs(t)
		writeCtxDoc(t, "context/wiki/covered.md", coveragePage("covered", ""))
		writeCtxDoc(t, "context/wiki/peer.md", coveragePeer)
		if out := string(execute(t, contextWikiLintCmd, nil)); strings.Contains(out, "coverage") {
			t.Fatalf("an absent coverage block must not be a finding:\n%s", out)
		}
	})
}
