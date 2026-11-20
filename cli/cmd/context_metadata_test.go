//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/metacomp"
)

func TestMaterialLedgerMigrationPreservesFieldsAndPerArtifactClaims(t *testing.T) {
	setupContextProject(t)
	guideHeader := materialHeaderLine("architecture/alpha", [][2]string{{"c-guide", "aaaaaaaaaaaa"}})
	deckHeader := materialHeaderLine("architecture/alpha", [][2]string{{"c-slide", "bbbbbbbbbbbb"}})
	guide := writeMaterial(t, "alpha.it.md", guideHeader, "# Guide\n")
	deck := "context/renders/materials/software-architect/alpha.it.slide.md"
	writeCtxDoc(t, deck, deckHeader+"\n---\ntheme: default\n---\n\n# Slide\n")
	legacy := map[string]any{
		"concept": "architecture/alpha", "syllabus": "syllabi/architect", "lang": "it",
		"files": []string{"alpha.it.md", "alpha.it.slide.md"}, "lesson": "architecture/alpha",
		"claims": []map[string]string{{"id": "c-guide", "digest": "aaaaaaaaaaaa"}},
		"binder": map[string]any{"bound_claims": 1}, "cycle": map[string]any{"critic": "read"},
		"notes": "historical notes", "future_field": map[string]any{"keep": true},
	}
	legacyBytes, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	ledger := strings.TrimSuffix(guide, ".md") + ".ledger.json"
	writeCtxDoc(t, ledger, string(legacyBytes))
	plan, err := materialLedgerMigration(sdtProjectRoot(), ledger)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := metacomp.Decode(plan.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	material := metadataMap(doc["material"])
	artifacts := metadataMap(material["artifacts"])
	if !metadataClaimsMatch(metadataMap(artifacts["guide"])["claims"], map[string]string{"c-guide": "aaaaaaaaaaaa"}) {
		t.Fatal("guide claims were not preserved")
	}
	if !metadataClaimsMatch(metadataMap(artifacts["slide_deck"])["claims"], map[string]string{"c-slide": "bbbbbbbbbbbb"}) {
		t.Fatal("slide claims were not preserved separately")
	}
	review := metadataMap(doc["review"])
	if metadataString(review["notes"]) != "historical notes" || metadataMap(review["future_field"])["keep"] != true {
		t.Fatalf("review history or unknown field was lost: %#v", review)
	}
	if metadataString(metadataMap(doc["target"])["path"]) != filepath.ToSlash(strings.TrimPrefix(guide, "./")) {
		t.Fatalf("target does not bind guide %s: %#v", guide, doc["target"])
	}
}

func TestMaterialMetadataReviewUpdatePreservesUnknownFields(t *testing.T) {
	setupContextProject(t)
	headerLine := materialHeaderLine("architecture/alpha", [][2]string{{"c-guide", "aaaaaaaaaaaa"}})
	guide := writeMaterial(t, "alpha.it.md", headerLine, "# Guide\n")
	header, err := parseMaterialHeader(headerLine)
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.ToSlash(strings.TrimPrefix(guide, "./"))
	guideBytes, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	doc := metacomp.New(rel, metacomp.Digest(guideBytes))
	doc["material"] = map[string]any{
		"concept": header.Concept, "syllabus": header.Syllabus, "lang": header.Lang,
		"classification": header.Classification,
		"artifacts":      map[string]any{"guide": map[string]any{"claims": headerClaims(header)}},
	}
	doc["review"] = map[string]any{"cycle": map[string]any{"critic": "critic pass"}, "future": "preserve"}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := materialMetadataPathOf(guide)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := readMaterialMetadata(guide, header); err != nil || !found {
		t.Fatalf("read canonical metadata = found %t, err %v", found, err)
	}
	if err := updateMaterialMetadataReview(path, materialRoleReader, "cold read", "session", "model", "2026-11-20T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	updated, _, err := metacomp.ReadTarget(sdtProjectRoot(), rel)
	if err != nil {
		t.Fatal(err)
	}
	review := metadataMap(updated["review"])
	cycle := metadataMap(review["cycle"])
	if metadataString(cycle["reader"]) != "cold read" || metadataString(cycle["reader_by"]) != "session" || metadataString(review["future"]) != "preserve" {
		t.Fatalf("review update lost values: %#v", review)
	}
}

func TestMaterialMigrationResumesAfterPartialWrite(t *testing.T) {
	setupContextProject(t)
	headerLine := materialHeaderLine("architecture/alpha", [][2]string{{"c-guide", "aaaaaaaaaaaa"}})
	guide := writeMaterial(t, "alpha.it.md", headerLine, "# Guide\n")
	deckHeader := materialHeaderLine("architecture/alpha", [][2]string{{"c-slide", "bbbbbbbbbbbb"}})
	deck := "context/renders/materials/software-architect/alpha.it.slide.md"
	writeCtxDoc(t, deck, deckHeader+"\n---\ntheme: default\n---\n\n# Slide\n")
	legacy := map[string]any{
		"concept": "architecture/alpha", "syllabus": "syllabi/architect", "lang": "it",
		"files":  []string{"alpha.it.md", "alpha.it.slide.md"},
		"claims": []map[string]string{{"id": "c-guide", "digest": "aaaaaaaaaaaa"}},
		"cycle":  map[string]any{"critic": "reviewed", "reader": "reviewed"},
	}
	legacyBytes, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	ledger := strings.TrimSuffix(guide, ".md") + ".ledger.json"
	writeCtxDoc(t, ledger, string(legacyBytes))
	legacyDigest := metacomp.Digest(legacyBytes)
	plan, err := materialLedgerMigration(sdtProjectRoot(), ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Path, plan.Bytes, 0o600); err != nil {
		t.Fatal(err)
	}
	resumed, err := materialLedgerMigration(sdtProjectRoot(), ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Existing {
		t.Fatal("rerun did not recognize the already-written companion")
	}
	currentLegacy, err := os.ReadFile(ledger)
	if err != nil || metacomp.Digest(currentLegacy) != legacyDigest {
		t.Fatalf("legacy record changed during migration: %v", err)
	}
}
