//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sandrolain/sdt/internal/metacomp"
)

func TestMaterialPrivacyCommandUpdatesArtifactWithoutTouchingReview(t *testing.T) {
	root := setupContextProject(t)
	id, guideRel := writePrivacyMaterialFixture(t)
	companion := filepath.Join(root, filepath.FromSlash(guideRel+metacomp.CompanionSuffix))
	if out := string(execute(t, materialPrivacySetCmd, nil, id, "guide", "real", "--basis", "public corporate examples; no personal data")); out == "" {
		t.Fatal("privacy command returned no confirmation")
	}
	doc, found, err := metacomp.ReadTarget(root, guideRel)
	if err != nil || !found {
		t.Fatalf("ReadTarget() = (%v, %v, %v)", doc, found, err)
	}
	material := metadataMap(doc["material"])
	artifacts := metadataMap(material["artifacts"])
	guide := metadataMap(artifacts["guide"])
	clearance := metadataMap(guide["privacy_clearance"])
	if metadataString(guide["privacy"]) != "real" || metadataString(clearance["status"]) != "cleared" || metadataString(clearance["basis"]) != "public corporate examples; no personal data" {
		t.Fatalf("guide privacy = %#v", guide)
	}
	review := metadataMap(doc["review"])
	cycle := metadataMap(review["cycle"])
	if metadataString(cycle["critic"]) != "existing critic note" {
		t.Fatalf("privacy update changed review fields: %#v", review)
	}
	if _, err := os.Stat(companion); err != nil {
		t.Fatalf("companion missing after update: %v", err)
	}
	if _, err := privacyClearance("real", " "); err == nil {
		t.Fatal("real privacy without a basis was accepted")
	}
	if _, err := privacyClearance("none", "not applicable"); err == nil {
		t.Fatal("none privacy with a clearance basis was accepted")
	}
	if err := setMaterialArtifactPrivacy(id, "deck", "none", ""); err == nil {
		t.Fatal("unsupported artifact name was accepted")
	}
}

func TestMaterialPrivacyAssetCommandCreatesAndUpdatesCompanion(t *testing.T) {
	root := setupContextProject(t)
	assetRel := "context/assets/chart.svg"
	asset := []byte("<svg>synthetic chart</svg>")
	writeCtxDoc(t, assetRel, string(asset))
	if out := string(execute(t, materialPrivacyAssetCmd, nil, assetRel, "personal", "--basis", "personal data removed")); out == "" {
		t.Fatal("asset privacy command returned no confirmation")
	}
	doc, found, err := metacomp.ReadTarget(root, assetRel)
	if err != nil || !found {
		t.Fatalf("ReadTarget() = (%v, %v, %v)", doc, found, err)
	}
	clearance := metadataMap(doc["privacy_clearance"])
	if metadataString(doc["privacy"]) != "personal" || metadataString(clearance["status"]) != "cleared" || metadataString(clearance["basis"]) != "personal data removed" {
		t.Fatalf("asset privacy = %#v", doc)
	}
	if err := setAssetPrivacy("context/assets/chart.svg", "none", ""); err != nil {
		t.Fatal(err)
	}
	doc, _, err = metacomp.ReadTarget(root, assetRel)
	if err != nil {
		t.Fatal(err)
	}
	if metadataString(doc["privacy"]) != "none" || doc["privacy_clearance"] != nil {
		t.Fatalf("none update did not remove clearance: %#v", doc)
	}
	for _, unsafe := range []string{"../outside.svg", "/absolute.svg", "context/assets/not-an-image.txt"} {
		if err := setAssetPrivacy(unsafe, "none", ""); err == nil {
			t.Errorf("unsafe asset path %q was accepted", unsafe)
		}
	}
}

func writePrivacyMaterialFixture(t *testing.T) (string, string) {
	t.Helper()
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	headerLine := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
	header, err := parseMaterialHeader(headerLine)
	if err != nil {
		t.Fatal(err)
	}
	id := "architect/architecture/alpha.it"
	guideRel := "context/renders/materials/architect/architecture/alpha.it.md"
	deckRel := "context/renders/materials/architect/architecture/alpha.it.slide.md"
	guideData := []byte(headerLine + "\n\n# Alpha\n\nGuide.\n")
	deckData := []byte(headerLine + "\n\n# Alpha deck\n")
	writeCtxDoc(t, guideRel, string(guideData))
	writeCtxDoc(t, deckRel, string(deckData))
	doc := metacomp.New(guideRel, metacomp.Digest(guideData))
	claims := headerClaims(header)
	doc["material"] = map[string]any{
		"concept": header.Concept, "syllabus": header.Syllabus, "lang": header.Lang,
		"classification": header.Classification,
		"artifacts": map[string]any{
			"guide":      map[string]any{"claims": claims},
			"slide_deck": map[string]any{"claims": claims},
		},
	}
	doc["related_artifacts"] = []any{map[string]any{
		"role": "slide_deck", "path": deckRel, "sha256": metacomp.Digest(deckData),
	}}
	doc["review"] = map[string]any{"cycle": map[string]any{"critic": "existing critic note"}}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, guideRel+metacomp.CompanionSuffix, string(encoded))
	return id, guideRel
}
