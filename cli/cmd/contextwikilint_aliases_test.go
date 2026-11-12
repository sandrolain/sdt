package cmd

import (
	"strings"
	"testing"
)

// aliasPage is a valid dense page that declares the given aliases. An empty
// string is written as a `- ""` entry, so the empty-alias rule can be exercised
// through real frontmatter rather than a hand-built struct.
func aliasPage(id, title string, aliases ...string) string {
	var b strings.Builder
	b.WriteString("---\nkind: wiki\nid: " + id + "\ntitle: " + title +
		"\ntype: concept\nstatus: active\nsummary: Summary for " + title + "\ntags:\n  - backend\n")
	if len(aliases) > 0 {
		b.WriteString("aliases:\n")
		for _, a := range aliases {
			b.WriteString("  - \"" + a + "\"\n")
		}
	}
	b.WriteString("relations:\n  refers_to:\n    - [[db|Database]]\nsources:\n  - refs/spec.md\n---")
	b.WriteString(denseWikiBody("Shape of " + title + "."))
	return b.String()
}

// aliasLint runs the wiki lint and returns its output and the exit code it
// asked for (-1 when it asked for none), so a test can assert a rule's severity
// as well as its message.
func aliasLint(t *testing.T) (string, int) {
	t.Helper()
	code := -1
	orig := exit
	exit = func(c int) { code = c }
	out := string(execute(t, contextWikiLintCmd, nil))
	exit = orig
	return out, code
}

// TestWikiLintAliasesClean proves the field is optional: a page with no alias
// and a page with one both stay silent.
func TestWikiLintAliasesClean(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/accounts.md", aliasPage("accounts", "Account Service"))
	writeCtxDoc(t, "context/wiki/db.md", aliasPage("db", "Database", "Datastore"))
	writeCtxDoc(t, "context/wiki/ledger.md", aliasPage("ledger", "Ledger", "Ledger"))

	out, code := aliasLint(t)
	if strings.Contains(out, "alias") {
		t.Fatalf("a valid alias set produced a finding:\n%s", out)
	}
	if code != -1 {
		t.Errorf("a clean alias set asked for exit code %d, want none", code)
	}
}

// TestWikiLintAliasesDuplicateOnOnePage flags the same alias twice: the page
// cannot mean two things by one name.
func TestWikiLintAliasesDuplicateOnOnePage(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/db.md", aliasPage("db", "Database", "Datastore", "datastore"))

	out, code := aliasLint(t)
	want := `[CRITICAL] context/wiki/db.md: duplicate alias "datastore" on this page`
	if !strings.Contains(out, want) {
		t.Fatalf("lint did not report %q:\n%s", want, out)
	}
	if code != exitWikiError {
		t.Errorf("exit code = %d, want %d for a critical finding", code, exitWikiError)
	}
}

// TestWikiLintAliasesClaimedByTwoPages is the collision the field exists for:
// two pages claiming one name, found case-insensitively, naming both pages.
func TestWikiLintAliasesClaimedByTwoPages(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/db.md", aliasPage("db", "Database", "Event Store"))
	writeCtxDoc(t, "context/wiki/stream.md", aliasPage("stream", "Event Stream", "event store"))

	out, code := aliasLint(t)
	want := `[CRITICAL] context/wiki/stream.md: alias "event store" is already claimed by context/wiki/db.md`
	if !strings.Contains(out, want) {
		t.Fatalf("lint did not report %q:\n%s", want, out)
	}
	if code != exitWikiError {
		t.Errorf("exit code = %d, want %d for a critical finding", code, exitWikiError)
	}
}

// TestWikiLintAliasesCollideWithAnotherPageName warns when an alias is another
// page's title or id: the corpus reads fine, but two names point at one concept.
func TestWikiLintAliasesCollideWithAnotherPageName(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/db.md", aliasPage("db", "Database"))
	writeCtxDoc(t, "context/wiki/ledger.md", aliasPage("ledger", "Ledger", "Database"))

	out, code := aliasLint(t)
	want := `[WARNING] context/wiki/ledger.md: alias "Database" is another page's title or id (context/wiki/db.md)`
	if !strings.Contains(out, want) {
		t.Fatalf("lint did not report %q:\n%s", want, out)
	}
	if code != exitWikiWarn {
		t.Errorf("exit code = %d, want %d for a warning", code, exitWikiWarn)
	}

	// A page naming itself is not a collision: that is what the title is for.
	writeCtxDoc(t, "context/wiki/ledger.md", aliasPage("ledger", "Ledger", "Ledger"))
	if out, _ := aliasLint(t); strings.Contains(out, "another page's title") {
		t.Fatalf("a page's own title was reported as a collision:\n%s", out)
	}
}

// TestWikiLintAliasesEmptyEntry warns on a blank name, which claims nothing and
// would silently create no index row.
func TestWikiLintAliasesEmptyEntry(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "verified")
	wikiSourceDirs(t)
	writeCtxDoc(t, "context/wiki/db.md", aliasPage("db", "Database", "   "))

	out, code := aliasLint(t)
	want := "[WARNING] context/wiki/db.md: empty `aliases` entry"
	if !strings.Contains(out, want) {
		t.Fatalf("lint did not report %q:\n%s", want, out)
	}
	if code != exitWikiWarn {
		t.Errorf("exit code = %d, want %d for a warning", code, exitWikiWarn)
	}
}
