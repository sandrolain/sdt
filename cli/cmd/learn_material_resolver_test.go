//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// writeResolverFixture copies one testdata material into the lane, so the
// resolver reads the same header shape the corpus carries. base is the repo's
// cli/cmd directory, captured before the test changes directory.
func writeResolverFixture(t *testing.T, base, syllabus, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, "testdata", "materials", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	dir := filepath.Join("context", "renders", "materials", syllabus)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.it.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLearnMaterialResolverRefusesAmbiguity proves the lesson side resolves
// with the export resolver's rigor: a concept carried by two syllabi is
// refused with the candidate list — never resolved by sort order — and naming
// the syllabus (with or without `syllabi/`), the language, or both, resolves
// it. Zero matches is no error: the bundle omits the material.
func TestLearnMaterialResolverRefusesAmbiguity(t *testing.T) {
	base, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	writeResolverFixture(t, base, "one", "alpha-syllabus-one.it.md")
	writeResolverFixture(t, base, "two", "alpha-syllabus-two.it.md")

	_, refused := learnMaterialOf("architecture/alpha")
	if refused == nil {
		t.Fatal("a concept in two syllabi must be refused")
	}
	for _, want := range []string{"2 materials match", "one/alpha.it.md", "two/alpha.it.md"} {
		if !strings.Contains(refused.Error(), want) {
			t.Errorf("refusal %q does not mention %q", refused, want)
		}
	}

	// Naming the syllabus — either spelling — resolves to the one match.
	matches, err := matchMaterialFiles("architecture/alpha", "one", "")
	if err != nil || len(matches) != 1 || filepath.Base(matches[0].Path) != "alpha.it.md" ||
		matches[0].Header.Syllabus != "syllabi/one" {
		t.Errorf("syllabus one: matches = %+v err = %v", matches, err)
	}
	matches, err = matchMaterialFiles("wiki/architecture/alpha", "syllabi/two", "it")
	if err != nil || len(matches) != 1 {
		t.Errorf("syllabus two with prefixes: matches = %+v err = %v", matches, err)
	}

	// A language the corpus does not carry matches nothing, and that is no
	// error: absence is the caller's call.
	matches, err = matchMaterialFiles("architecture/alpha", "", "en")
	if err != nil || len(matches) != 0 {
		t.Errorf("absent language: matches = %+v err = %v", matches, err)
	}
}

// TestLearnBundleRefusesAmbiguousMaterial proves the refusal reaches the
// bundle's caller: a concept with two materials makes learnBundleOf fail with
// the candidate list, instead of the bundle quietly naming a sorted first.
func TestLearnBundleRefusesAmbiguousMaterial(t *testing.T) {
	base, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	writeResolverFixture(t, base, "one", "alpha-syllabus-one.it.md")
	writeResolverFixture(t, base, "two", "alpha-syllabus-two.it.md")
	// A page for the concept the fixtures carry, so the bundle's material
	// lookup is the ambiguous one.
	page := &contextwiki.Page{ID: "architecture/alpha", Title: "Alpha", Status: ctxWikiStatusActive}
	bld := &contextwiki.Builder{Pages: []*contextwiki.Page{page}, ByID: map[string]*contextwiki.Page{page.ID: page}}
	if _, err := learnBundleOf(bld, page); err == nil {
		t.Fatal("an ambiguous material must refuse the bundle")
	} else if !strings.Contains(err.Error(), "2 materials match") {
		t.Errorf("refusal %q does not name the count", err)
	}
}
