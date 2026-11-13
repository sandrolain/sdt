//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// materialDeckFixture reads one deck fixture and makes its header live: the
// @CLAIMS@ placeholder becomes the given claim pairs, the value a fresh deck's
// header pins. base is the repo's cli/cmd directory, captured before the test
// changes directory into its temp corpus.
func materialDeckFixture(t *testing.T, base, name, claims string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, "testdata", "materials", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return strings.Replace(string(data), "@CLAIMS@", claims, 1)
}

// TestMaterialStatusDeckRenderBroken is the c7 fixture proof. The broken deck
// carries the header comment the way the pre-fix material decks displaced
// their frontmatter: a leading `<!-- material: … -->`, then a blank line, then
// a `---` the render cannot read as a frontmatter opener, so the theme and
// paginate keys are slide text and no frontmatter opens the document. It is
// reported render-broken instead of fresh, the row says why, and publishable
// says no with it. The correctly-authored deck — the header comment, then the
// `---` frontmatter, the shape every material deck in the corpus carries —
// passes the check and stays fresh.
func TestMaterialStatusDeckRenderBroken(t *testing.T) {
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	setupContextProject(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	claims := "c-aaaaaaaa@" + digests["c-aaaaaaaa"] + ",c-bbbbbbbb@" + digests["c-bbbbbbbb"]
	header := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]}, {"c-bbbbbbbb", digests["c-bbbbbbbb"]},
	})
	writeMaterial(t, "alpha.it.md", header, materialBody())

	deckPath := "context/renders/materials/software-architect/alpha.it.slide.md"
	writeCtxDoc(t, deckPath, materialDeckFixture(t, repo, "deck-displaced-frontmatter.it.slide.md", claims))

	rows, err := materialStatusRows("", "", "")
	if err != nil {
		t.Fatalf("materialStatusRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Deck != deckRenderBroken {
		t.Fatalf("a displaced-frontmatter deck is %q, want %q", row.Deck, deckRenderBroken)
	}
	if !strings.Contains(row.DeckDetail, "frontmatter") {
		t.Errorf("the row does not say why: %+v", row)
	}
	if row.Publishable {
		t.Errorf("a render-broken deck is not publishable: %+v", row)
	}
	if reason := publishableReason(row); !strings.Contains(reason, "deck render-broken") {
		t.Errorf("publishableReason = %q, want the deck named", reason)
	}

	// The same deck, correctly authored: the header comment, then the
	// `---` frontmatter — the shape the viewer normalizes and the render lane
	// reads — stays fresh.
	writeCtxDoc(t, deckPath, materialDeckFixture(t, repo, "deck-correct.it.slide.md", claims))
	rows, err = materialStatusRows("", "", "")
	if err != nil {
		t.Fatalf("materialStatusRows: %v", err)
	}
	if got := rows[0].Deck; got != renderFresh {
		t.Errorf("a correctly-authored deck is %q, want fresh: %+v", got, rows[0])
	}
	if got := rows[0].DeckDetail; got != "" {
		t.Errorf("a fresh deck carries no deck detail, got %q", got)
	}
}

// TestMaterialStatusDeckKeepsFreshnessSemantics proves the structural check
// only replaces `fresh`: a deck the render lane calls stale keeps the lane's
// verdict even when its structure is broken, and the guide's own freshness is
// never touched.
func TestMaterialStatusDeckKeepsFreshnessSemantics(t *testing.T) {
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	setupContextProject(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	stale := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]}, {"c-bbbbbbbb", "000000000000"},
	})
	writeMaterial(t, "alpha.it.md", stale, materialBody())
	writeCtxDoc(t, "context/renders/materials/software-architect/alpha.it.slide.md",
		materialDeckFixture(t, repo, "deck-displaced-frontmatter.it.slide.md", "c-aaaaaaaa@000000000000"))

	rows, err := materialStatusRows("", "", "")
	if err != nil {
		t.Fatalf("materialStatusRows: %v", err)
	}
	row := rows[0]
	if row.Deck == "" {
		t.Fatal("the deck beside the guide vanished from the report")
	}
	if row.Deck == deckRenderBroken {
		t.Errorf("the check replaced a non-fresh lane verdict: %+v", row)
	}
	if row.Deck != renderStale {
		t.Errorf("deck = %q, want the lane's stale", row.Deck)
	}
	if !strings.Contains(publishableReason(row), "deck "+row.Deck) {
		t.Errorf("publishableReason = %q, want the lane verdict named", publishableReason(row))
	}
}
