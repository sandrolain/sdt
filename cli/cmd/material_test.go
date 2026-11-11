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
	deck := header + "\n\n---\ntheme: default\npaginate: true\n---\n\n# Deck\n\n" +
		"Vedi `architecture/alpha`.\n\n" +
		"<!-- claim=architecture/alpha#c-aaaaaaaa digest=" + digests["c-aaaaaaaa"] + " -->\n\n" +
		"<!-- Note: claim 1. Una nota del relatore. -->\n\n---\n\n# Seconda\n\n" +
		"---\n\n# Verifica: quattro domande chiuse\n\n- sono nella verifica del corso\n\n" +
		"---\n\n# Terza\n"
	writeCtxDoc(t, "context/renders/materials/software-architect/alpha.it.slide.md", deck)
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

	deck, err := os.ReadFile(filepath.Join(out, "alpha.it.slide.md"))
	if err != nil {
		t.Fatalf("read the exported deck: %v", err)
	}
	deckText := string(deck)
	if !strings.HasPrefix(deckText, "---\n") {
		t.Errorf("the deck frontmatter must be first, got:\n%s", deckText)
	}
	for _, residue := range []string{"<!--", "claims=", "claim 1", "architecture/alpha"} {
		if strings.Contains(deckText, residue) {
			t.Errorf("the exported deck still carries %q:\n%s", residue, deckText)
		}
	}
	if !strings.Contains(deckText, "# Seconda") || !strings.Contains(deckText, "# Terza") || !strings.Contains(deckText, "Alpha") {
		t.Errorf("the deck must keep its slides and resolve the slug:\n%s", deckText)
	}
	if strings.Contains(deckText, "Verifica") || strings.Contains(strings.ToLower(deckText), "verifica del corso") {
		t.Errorf("the bank slide must not reach the deliverable:\n%s", deckText)
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
	deckPath := "context/renders/materials/software-architect/alpha.it.slide.md"
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, materialCheckCmd, nil, deckPath))
	})
	out := t.TempDir()
	execute(t, materialExportCmd, nil, "architecture/alpha", "--out", out, "--lang", "it")
	clean := string(execute(t, materialCheckCmd, nil, filepath.Join(out, "alpha.it.md")))
	if strings.TrimSpace(clean) != "clean" {
		t.Errorf("the export must pass the check, got %q", clean)
	}
	cleanDeck := string(execute(t, materialCheckCmd, nil, filepath.Join(out, "alpha.it.slide.md")))
	if strings.TrimSpace(cleanDeck) != "clean" {
		t.Errorf("the exported deck must pass the check, got %q", cleanDeck)
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
