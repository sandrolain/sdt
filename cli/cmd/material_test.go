//go:build !wasm

package cmd

import (
	"fmt"
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
		"---\n\n# Terza\n\n---\n\n# Dove sei: prima la lingua, ultima scena del tema\n"
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
	for _, residue := range []string{"claim=", "assess=", "claims=", "claim 1", "architecture/alpha"} {
		if strings.Contains(deckText, residue) {
			t.Errorf("the exported deck still carries %q:\n%s", residue, deckText)
		}
	}
	if !strings.Contains(deckText, "# Seconda") || !strings.Contains(deckText, "# Terza") || !strings.Contains(deckText, "# Dove sei") || !strings.Contains(deckText, "Alpha") {
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

// TestMaterialCheckRefusesCuratorRegister proves the check refuses the
// curator register: every pattern in a plausible sentence, reported with the
// file, the line and the phrase matched; a case-insensitive variant mid-sentence;
// and a clean text still passes.
func TestMaterialCheckRefusesCuratorRegister(t *testing.T) {
	setupContextProject(t)
	sentences := []struct{ phrase, text string }{
		{"come nel corpus", "Il contesto si delimita, come nel corpus, con una riga per linguaggio."},
		{"come nella guida precedente", "Come nella guida precedente, la mappa precede la lista."},
		{"come vedrai", "Come vedrai, il caso si chiude da solo."},
		{"come vedremo", "Come vedremo, il conteggio guida la decisione."},
		{"come nel resto del corso", "Il confine resta disegnato, come nel resto del corso, prima della scelta."},
	}
	var body strings.Builder
	body.WriteString("# Guida\n\n")
	for _, s := range sentences {
		body.WriteString(s.text + "\n")
	}
	registered := "context/renders/materials/software-architect/register.it.md"
	writeCtxDoc(t, registered, body.String())

	var findings string
	shouldExitWithCode(t, 1, func() string {
		findings = string(execute(t, materialCheckCmd, nil, registered))
		return findings
	})
	for i, s := range sentences {
		want := fmt.Sprintf("%s:%d: curator register: %q", registered, 3+i, s.phrase)
		if !strings.Contains(findings, want) {
			t.Errorf("the check must report the pattern at its line, want %q in:\n%s", want, findings)
		}
	}

	variant := "context/renders/materials/software-architect/variant.it.md"
	writeCtxDoc(t, variant, "# Guida\n\nIl confine, Come nel corpus, si disegna prima della scelta.\n")
	var variantFindings string
	shouldExitWithCode(t, 1, func() string {
		variantFindings = string(execute(t, materialCheckCmd, nil, variant))
		return variantFindings
	})
	want := fmt.Sprintf("%s:3: curator register: %q", variant, "come nel corpus")
	if !strings.Contains(variantFindings, want) {
		t.Errorf("the check must match the phrase case-insensitively, want %q in:\n%s", want, variantFindings)
	}

	clean := "context/renders/materials/software-architect/pulita.it.md"
	writeCtxDoc(t, clean, "# Guida\n\nIl confine si disegna prima di sceglierlo.\n\nFine.\n")
	out := string(execute(t, materialCheckCmd, nil, clean))
	if strings.TrimSpace(out) != "clean" {
		t.Errorf("a text without the register must pass the check, got %q", out)
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
