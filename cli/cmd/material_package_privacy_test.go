//go:build !wasm

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sandrolain/sdt/internal/materialpackage"
	"github.com/sandrolain/sdt/internal/metacomp"
)

func TestPackagePrivacyReadsDocumentDeclaration(t *testing.T) {
	privacy, clearance, err := packagePrivacyFromDocument([]byte("---\nprivacy: personal\nprivacy_clearance:\n  status: cleared\n  basis: names removed\n---\nContent\n"))
	if err != nil {
		t.Fatal(err)
	}
	if privacy != materialpackage.PrivacyPersonal || clearance == nil || clearance.Status != materialpackage.PrivacyCleared || clearance.Basis != "names removed" {
		t.Fatalf("privacy = %q, clearance = %+v", privacy, clearance)
	}
	if _, _, err := packagePrivacyFromDocument([]byte("# no frontmatter\n")); err == nil {
		t.Fatal("document without a declaration should be rejected")
	}
	if _, _, err := packagePrivacyFromDocument([]byte("---\nkind: lesson\n---\nContent\n")); err == nil {
		t.Fatal("document without a privacy value should be rejected")
	}
}

func TestPackagePrivacyReadsAssetCompanion(t *testing.T) {
	root := t.TempDir()
	assetPath := filepath.Join(root, "context", "assets", "chart.svg")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o750); err != nil {
		t.Fatal(err)
	}
	asset := []byte("<svg/>")
	if err := os.WriteFile(assetPath, asset, 0o600); err != nil {
		t.Fatal(err)
	}
	doc := metacomp.New("context/assets/chart.svg", metacomp.Digest(asset))
	doc["privacy"] = "real"
	doc["privacy_clearance"] = map[string]any{"status": "cleared", "basis": "identifying labels removed"}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath+metacomp.CompanionSuffix, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	privacy, clearance, err := packagePrivacyFromAsset(root, "context/assets/chart.svg")
	if err != nil {
		t.Fatal(err)
	}
	if privacy != materialpackage.PrivacyReal || clearance == nil || clearance.Basis != "identifying labels removed" {
		t.Fatalf("privacy = %q, clearance = %+v", privacy, clearance)
	}
	if _, _, err := packagePrivacyFromAsset(root, "context/assets/missing.svg"); err == nil {
		t.Fatal("asset without a companion should be rejected")
	}
}
