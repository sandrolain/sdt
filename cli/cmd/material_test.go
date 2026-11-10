//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// materialExportFixture writes a page and a guide whose body carries every
// authoring residue the export must remove, and returns the guide's path.
func materialExportFixture(t *testing.T) (string, string) {
	t.Helper()
	setupContextProject(t)
	writeMaterialPage(t, "architecture/alpha")
	writeMaterialPage(t, "architecture/beta")
	digests := liveDigests(t, "architecture/alpha")
	header := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]},
	})
	body := "# Guida\n\nVedi `architecture/alpha` e `bounded-context`.\n" +
		"<!-- claim=architecture/alpha#c-aaaaaaaa digest=" + digests["c-aaaaaaaa"] + " -->\n\n" +
		"**Verifica.** Le domande sono nella verifica del corso.\n" +
		"<!-- assess=architecture/alpha items=a-1 -->\n\nFine.\n"
	path := writeMaterial(t, "alpha.it.md", header, body)
	return path, digests["c-aaaaaaaa"]
}

func TestMaterialExportCleansTheGuide(t *testing.T) {
	guide, _ := materialExportFixture(t)
	out := t.TempDir()
	execute(t, materialExportCmd, nil, "architecture/alpha", "--out", out, "--lang", "it")

	exported, err := os.ReadFile(filepath.Join(out, "alpha.it.md"))
	if err != nil {
		t.Fatalf("read the export: %v", err)
	}
	text := string(exported)
	for _, residue := range []string{"<!--", "claims=", "verifica del corso", "architecture/alpha"} {
		if strings.Contains(text, residue) {
			t.Errorf("the export still carries %q:\n%s", residue, text)
		}
	}
	if !strings.Contains(text, "Alpha") {
		t.Errorf("the page slug must become the page title, got:\n%s", text)
	}
	if !strings.Contains(text, "Fine.") {
		t.Errorf("the prose must survive the cleaning:\n%s", text)
	}
	if _, err := os.Stat(guide); err != nil {
		t.Errorf("the authoring source must be untouched: %v", err)
	}
}

func TestMaterialExportRefusesAmbiguity(t *testing.T) {
	guide, _ := materialExportFixture(t)
	header, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(header), "\n")
	enHeader := strings.Replace(first, "lang=it", "lang=en", 1)
	writeMaterial(t, "alpha.en.md", enHeader, "# Guida\n\nTesto.\n")
	otherHeader := strings.Replace(first, "syllabus=syllabi/architect", "syllabus=syllabi/other", 1)
	writeCtxDoc(t, "context/renders/materials/other/alpha.it.md", otherHeader+"\n\n# Guida\n\nTesto.\n")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, materialExportCmd, nil, "architecture/alpha", "--out", t.TempDir()))
	})
	out := t.TempDir()
	execute(t, materialExportCmd, nil, "architecture/alpha", "--out", out, "--lang", "it", "--syllabus", "architect")
	if _, err := os.Stat(filepath.Join(out, "alpha.it.md")); err != nil {
		t.Errorf("the syllabus and language must disambiguate: %v", err)
	}
}

func TestMaterialCheckRefusesTheSourceAndPassesTheExport(t *testing.T) {
	guide, _ := materialExportFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, materialCheckCmd, nil, guide))
	})
	out := t.TempDir()
	execute(t, materialExportCmd, nil, "architecture/alpha", "--out", out, "--lang", "it")
	clean := string(execute(t, materialCheckCmd, nil, filepath.Join(out, "alpha.it.md")))
	if strings.TrimSpace(clean) != "clean" {
		t.Errorf("the export must pass the check, got %q", clean)
	}
}

func TestMaterialResolveSkipsDecks(t *testing.T) {
	guide, _ := materialExportFixture(t)
	header, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(header), "\n")
	writeMaterial(t, "alpha.it.slide.md", first, "# Deck\n\nTesto.\n")
	out := t.TempDir()
	execute(t, materialExportCmd, nil, "architecture/alpha", "--out", out, "--lang", "it")
	if _, err := os.Stat(filepath.Join(out, "alpha.it.md")); err != nil {
		t.Errorf("the resolver must ignore the deck: %v", err)
	}
}
