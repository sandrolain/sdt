//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

func TestCanonicalMaterialCompanionReportsDivergentLegacyLedger(t *testing.T) {
	setupContextProject(t)
	headerLine := materialHeaderLine("architecture/alpha", [][2]string{{"c-guide", "aaaaaaaaaaaa"}})
	guide := writeMaterial(t, "alpha.it.md", headerLine, "# Guide\n")
	header, err := parseMaterialHeader(headerLine)
	if err != nil {
		t.Fatal(err)
	}
	guideBytes, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.ToSlash(strings.TrimPrefix(guide, "./"))
	doc := metacomp.New(rel, metacomp.Digest(guideBytes))
	doc["material"] = map[string]any{
		"concept": header.Concept, "syllabus": header.Syllabus, "lang": header.Lang,
		"classification": header.Classification,
		"artifacts":      map[string]any{"guide": map[string]any{"claims": headerClaims(header)}},
		"files":          []any{"alpha.it.md"},
	}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(materialMetadataPathOf(guide), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := json.Marshal(map[string]any{
		"concept": header.Concept, "syllabus": header.Syllabus, "lang": header.Lang,
		"claims": []map[string]string{{"id": "c-guide", "digest": "aaaaaaaaaaaa"}},
		"files":  []string{"alpha.it.md", "unbound.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, strings.TrimSuffix(guide, ".md")+".ledger.json", string(legacy))
	if _, found, err := readMaterialMetadata(guide, header); err == nil || !found || !strings.Contains(err.Error(), "legacy material ledger conflicts with canonical companion") {
		t.Fatalf("divergent legacy record = found %t, error %v", found, err)
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
	if err := os.Remove(plan.Path); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := materialLedgerMigration(sdtProjectRoot(), ledger)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.Existing || len(rolledBack.Bytes) == 0 {
		t.Fatal("rollback did not return an additive migration plan")
	}
	currentLegacy, err := os.ReadFile(ledger)
	if err != nil || metacomp.Digest(currentLegacy) != legacyDigest {
		t.Fatalf("legacy record changed during migration: %v", err)
	}
}

func TestExistingCompanionMatchesMappedFieldsAndPreservesReviewData(t *testing.T) {
	setupContextProject(t)
	root := sdtProjectRoot()
	target := "context/refs/sample.epub"
	targetBytes := []byte("sample")
	writeCtxDoc(t, target, string(targetBytes))
	proposed := metacomp.New(target, metacomp.Digest(targetBytes))
	proposed["source_archive"] = map[string]any{"sha256": "source-hash", "classification": "public"}
	proposedBytes, err := metacomp.Encode(proposed)
	if err != nil {
		t.Fatal(err)
	}
	existing := metacomp.Document{}
	for key, value := range proposed {
		existing[key] = value
	}
	existing["review"] = map[string]any{"reader": "approved", "future": "keep"}
	encoded, err := metacomp.Encode(existing)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, filepath.FromSlash(target+metacomp.CompanionSuffix))
	if err := os.WriteFile(destination, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := metadataMigrationPlanFor(root, destination, "legacy", proposedBytes)
	if err != nil || !plan.Existing {
		t.Fatalf("matching companion = %#v, err %v", plan, err)
	}
	decoded, _, err := metacomp.ReadTarget(root, target)
	if err != nil || metadataString(metadataMap(decoded["review"])["reader"]) != "approved" {
		t.Fatalf("companion review data changed: %#v, err %v", decoded, err)
	}
}

func TestLegacyMaterialProjectionIgnoresNewCompanionFields(t *testing.T) {
	legacy := map[string]any{
		"concept": "architecture/sample", "syllabus": "syllabi/software-architect", "lang": "it",
		"classification": "public", "claims": []any{map[string]any{"id": "c1", "digest": "d1"}},
	}
	companion := map[string]any{
		"concept": "architecture/sample", "syllabus": "syllabi/software-architect", "lang": "it",
		"classification": "public", "claims": []any{map[string]any{"id": "c1", "digest": "d1"}},
		"privacy": "real", "privacy_clearance": map[string]any{"status": "cleared", "basis": "public example"},
	}
	if !reflect.DeepEqual(projectLegacyMaterialFields(legacy), projectLegacyMaterialFields(companion)) {
		t.Fatal("new companion fields should not conflict with the legacy ledger projection")
	}
	companion["concept"] = "architecture/changed"
	if reflect.DeepEqual(projectLegacyMaterialFields(legacy), projectLegacyMaterialFields(companion)) {
		t.Fatal("mapped legacy field mismatch was ignored")
	}
}

func TestExistingCompanionRejectsSemanticMismatch(t *testing.T) {
	setupContextProject(t)
	root := sdtProjectRoot()
	target := "context/renders/materials/software-architect/sample.it.md"
	targetBytes := []byte("sample")
	writeCtxDoc(t, target, string(targetBytes))
	proposed := metacomp.New(target, metacomp.Digest(targetBytes))
	proposed["material"] = map[string]any{"concept": "architecture/new"}
	proposedBytes, err := metacomp.Encode(proposed)
	if err != nil {
		t.Fatal(err)
	}
	existing := metacomp.New(target, metacomp.Digest(targetBytes))
	existing["material"] = map[string]any{"concept": "architecture/old"}
	encoded, err := metacomp.Encode(existing)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, filepath.FromSlash(target+metacomp.CompanionSuffix))
	if err := os.WriteFile(destination, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = metadataMigrationPlanFor(root, destination, "legacy", proposedBytes)
	if err == nil || !strings.Contains(err.Error(), "semantic companion conflict") {
		t.Fatalf("semantic mismatch error = %v", err)
	}
	current, err := os.ReadFile(destination)
	if err != nil || string(current) != string(encoded) {
		t.Fatalf("conflict changed existing companion: %v", err)
	}
}

func TestExistingCompanionWithInvalidOrMissingTargetFailsClosed(t *testing.T) {
	setupContextProject(t)
	root := sdtProjectRoot()
	for _, target := range []string{
		"context/refs/missing.epub",
		"context/refs/tampered.epub",
	} {
		t.Run(target, func(t *testing.T) {
			if strings.HasSuffix(target, "tampered.epub") {
				writeCtxDoc(t, target, "actual bytes")
			}
			doc := metacomp.New(target, strings.Repeat("0", 64))
			encoded, err := metacomp.Encode(doc)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(root, filepath.FromSlash(target+metacomp.CompanionSuffix))
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, encoded, 0o600); err != nil {
				t.Fatal(err)
			}
			proposed, err := metacomp.Encode(doc)
			if err != nil {
				t.Fatal(err)
			}
			_, err = metadataMigrationPlanFor(root, destination, "legacy", proposed)
			if err == nil {
				t.Fatal("invalid or missing target was accepted")
			}
		})
	}
}
