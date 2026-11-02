package sourceclass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSidecar puts one sidecar in the current temp project and returns the
// corpus-relative source path. The project is created once per test: two calls
// must land in the same tree, or the second one silently replaces the first.
func writeSidecar(t *testing.T, source, body string) string {
	t.Helper()
	rel := filepath.Join("refs", source)
	if err := os.MkdirAll(filepath.Dir(filepath.Join("context", rel)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("context", rel+SidecarSuffix), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return rel
}

// TestStrictestWins: two declared classifications resolve to the stricter one,
// and the mix is reported so the page has to say it out loud.
func TestStrictestWins(t *testing.T) {
	t.Chdir(t.TempDir())
	internal := writeSidecar(t, "a.md", "classification: internal\n")
	public := writeSidecar(t, "b.md", "classification: public\n")
	page := ForSources([]string{public, internal})
	if page.Classification != TierInternal {
		t.Fatalf("classification = %q, want %q", page.Classification, TierInternal)
	}
	if len(page.Findings) != 1 || !strings.Contains(page.Findings[0], "different classifications") {
		t.Errorf("a mixed set must be reported: %v", page.Findings)
	}
	if got := CheckPage("", "", page); len(got) == 0 || !strings.Contains(got[0], "state the strictest") {
		t.Errorf("an undeclared page inheriting a mix must be asked to state it: %v", got)
	}
}

// TestUniformInheritanceIsSilent: one classification, no finding, and a page that
// declares it passes.
func TestUniformInheritanceIsSilent(t *testing.T) {
	t.Chdir(t.TempDir())
	a := writeSidecar(t, "a.md", "classification: internal\nlicense: CC-BY-4.0\n")
	b := writeSidecar(t, "b.md", "classification: internal\nlicense: CC-BY-4.0\n")
	page := ForSources([]string{a, b})
	if page.Classification != TierInternal || page.License != "CC-BY-4.0" {
		t.Fatalf("inherited = %+v", page)
	}
	if findings := CheckPage("internal", "CC-BY-4.0", page); len(findings) != 0 {
		t.Errorf("a uniform page must inherit cleanly, got %v", findings)
	}
}

// TestWeakerDeclarationAndMixedLicencesAreReported: a page may be stricter than
// its sources, never weaker, and two licences cannot be averaged.
func TestWeakerDeclarationAndMixedLicencesAreReported(t *testing.T) {
	t.Chdir(t.TempDir())
	a := writeSidecar(t, "a.md", "classification: confidential\nlicense: all-rights-reserved\n")
	b := writeSidecar(t, "b.md", "classification: confidential\nlicense: MIT\n")
	page := ForSources([]string{a, b})
	if !strings.Contains(strings.Join(page.Findings, " "), "different licences") {
		t.Fatalf("mixed licences must be reported: %v", page.Findings)
	}
	findings := CheckPage(TierPublic, "", page)
	if !strings.Contains(strings.Join(findings, " "), "weaker") {
		t.Errorf("a page weaker than its sources must be reported: %v", findings)
	}
	if got := CheckPage(TierConfidential, "", page); len(got) != 1 {
		t.Errorf("a page as strict as its sources may only inherit the licence finding, got %v", got)
	}
}

// TestUndeclaredIsNotADefect: most sources predate the field, and silence at the
// page level is not a lint matter — it is a release gate.
func TestUndeclaredIsNotADefect(t *testing.T) {
	t.Chdir(t.TempDir())
	a := writeSidecar(t, "a.md", "source: refs/a.md\n")
	page := ForSources([]string{a, "refs/never-archived.md"})
	if page.Classification != "" || len(page.Findings) != 0 {
		t.Errorf("an undeclared source must stay silent, got %+v", page)
	}
	if findings := CheckPage("", "", page); len(findings) != 0 {
		t.Errorf("silence is not a page finding, got %v", findings)
	}
}

// TestUnknownVocabularyIsReported: the tier list is closed.
func TestUnknownVocabularyIsReported(t *testing.T) {
	if Known("secret") {
		t.Error("`secret` is not in the vocabulary")
	}
	got := CheckPage("secret", "", Page{})
	if len(got) != 1 || !strings.Contains(got[0], "not one of") {
		t.Errorf("an unknown tier must be reported, got %v", got)
	}
}
