//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func writeMaterialStatusFixture(t *testing.T, classification, ledger string) {
	t.Helper()
	setupContextProject(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	header := materialHeaderLine("architecture/alpha", [][2]string{
		{"c-aaaaaaaa", digests["c-aaaaaaaa"]},
	})
	header = strings.Replace(header, "classification=undeclared", "classification="+classification, 1)
	writeMaterial(t, "alpha.it.md", header, materialBody())
	if ledger != "" {
		writeCtxDoc(t, "context/renders/materials/software-architect/alpha.it.ledger.json", ledger)
	}
}

func runMaterialStatus(t *testing.T, args ...string) []materialStatusRow {
	t.Helper()
	out := execute(t, materialStatusCmd, nil, append([]string{"--format", "json"}, args...)...)
	var rows []materialStatusRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("parse status json: %v\n%s", err, out)
	}
	return rows
}

func TestMaterialStatusNoLedger(t *testing.T) {
	writeMaterialStatusFixture(t, "undeclared", "")
	rows := runMaterialStatus(t)
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %+v", rows)
	}
	row := rows[0]
	if row.Freshness != renderFresh || row.Review != "not run" || row.Publishable {
		t.Errorf("a fresh material with no ledger is unreviewed and not publishable, got %+v", row)
	}
	if !strings.Contains(row.Detail, "no ledger") {
		t.Errorf("the missing ledger must be named, got %q", row.Detail)
	}
}

func TestMaterialStatusNotRunLedger(t *testing.T) {
	writeMaterialStatusFixture(t, "undeclared", `{"cycle":{"critic":"not run","reader":"not run"}}`)
	row := runMaterialStatus(t)[0]
	if row.Review != "not run" || row.Publishable {
		t.Errorf("an explicit not-run ledger is still not run, got %+v", row)
	}
}

func TestMaterialStatusPendingLedger(t *testing.T) {
	writeMaterialStatusFixture(t, "undeclared", `{"cycle":{"critic":"cold read","reader":"to run"}}`)
	row := runMaterialStatus(t)[0]
	if row.Review != "pending" || row.Publishable {
		t.Errorf("a reader to run is pending, not passed, got %+v", row)
	}
}

func TestMaterialStatusRecordedButUndeclared(t *testing.T) {
	writeMaterialStatusFixture(t, "undeclared", `{"cycle":{"critic":"cold read","reader":"cold read, passes"}}`)
	row := runMaterialStatus(t)[0]
	if row.Review != "recorded" {
		t.Fatalf("both cycle fields recorded is a recorded review, got %+v", row)
	}
	if row.Publishable {
		t.Errorf("an undeclared classification blocks publishable, got %+v", row)
	}
	if reason := publishableReason(row); !strings.Contains(reason, "classification undeclared") {
		t.Errorf("the reason must name the classification, got %q", reason)
	}
}

func TestMaterialStatusPublishable(t *testing.T) {
	writeMaterialStatusFixture(t, "public", `{"cycle":{"critic":"cold read","reader":"cold read, passes"}}`)
	row := runMaterialStatus(t)[0]
	if !row.Publishable || row.Review != "recorded" || row.Freshness != renderFresh {
		t.Errorf("fresh + recorded + declared is publishable, got %+v", row)
	}
	if reason := publishableReason(row); reason != "" {
		t.Errorf("a publishable row carries no reason, got %q", reason)
	}
}

func TestMaterialStatusFiltersByLanguage(t *testing.T) {
	writeMaterialStatusFixture(t, "undeclared", "")
	guide := "context/renders/materials/software-architect/alpha.it.md"
	data, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	enHeader := strings.Replace(first, "lang=it", "lang=en", 1)
	writeMaterial(t, "alpha.en.md", enHeader, materialBody())

	if rows := runMaterialStatus(t, "architecture/alpha", "--lang", "it"); len(rows) != 1 || rows[0].Lang != "it" {
		t.Errorf("the language filter must narrow to one row, got %+v", rows)
	}
	if rows := runMaterialStatus(t, "architecture/alpha"); len(rows) != 2 {
		t.Errorf("without a filter the status lists both languages, got %+v", rows)
	}
}

// writeStatusDeck puts a deck beside the guide, with the same claim at the given
// digest: the live one for a fresh deck, a wrong one for a stale deck.
func writeStatusDeck(t *testing.T, digest string) {
	t.Helper()
	header := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digest}})
	writeCtxDoc(t, "context/renders/materials/software-architect/alpha.it.slide.md",
		header+"\n\n---\ntheme: default\npaginate: true\n---\n\n# Deck\n")
}

func TestMaterialStatusDeckFresh(t *testing.T) {
	writeMaterialStatusFixture(t, "public", `{"cycle":{"critic":"cold read","reader":"cold read, passes"}}`)
	digests := liveDigests(t, "architecture/alpha")
	writeStatusDeck(t, digests["c-aaaaaaaa"])
	row := runMaterialStatus(t)[0]
	if row.Deck != renderFresh || !row.Publishable {
		t.Errorf("a fresh guide and a fresh deck with a recorded review are publishable, got %+v", row)
	}
}

func TestMaterialStatusDeckStale(t *testing.T) {
	writeMaterialStatusFixture(t, "public", `{"cycle":{"critic":"cold read","reader":"cold read, passes"}}`)
	writeStatusDeck(t, "000000000000")
	row := runMaterialStatus(t)[0]
	if row.Deck != renderStale || row.Publishable {
		t.Errorf("a stale deck blocks publishable, got %+v", row)
	}
	if reason := publishableReason(row); !strings.Contains(reason, "deck stale") {
		t.Errorf("the reason must name the deck, got %q", reason)
	}
}
