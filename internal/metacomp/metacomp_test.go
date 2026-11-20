package metacomp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTargetVerifiesAssociationAndExactBytes(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "context", "refs", "book.pdf")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	content := []byte{0, 1, 2, 255}
	if err := os.WriteFile(target, content, 0o600); err != nil {
		t.Fatal(err)
	}
	rel := "context/refs/book.pdf"
	doc := New(rel, Digest(content))
	doc["future_extension"] = map[string]any{"opaque": "preserved"}
	encoded, err := Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CompanionPath(target), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	got, found, err := ReadTarget(root, rel)
	if err != nil || !found {
		t.Fatalf("ReadTarget() = (%v, %v, %v)", got, found, err)
	}
	if got["future_extension"] == nil {
		t.Fatal("unknown extension was discarded")
	}
	if err := os.WriteFile(target, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadTarget(root, rel); err == nil {
		t.Fatal("tampered target was accepted")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadTarget(root, rel); err == nil {
		t.Fatal("missing target was accepted")
	}
}

func TestReadTargetRejectsTraversalAndUnknownVersion(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := New("../outside", Digest([]byte("x")))
	if err := ValidateTarget(root, "../outside", doc); err == nil {
		t.Fatal("path traversal was accepted")
	}
	doc = New("context/a", Digest([]byte("x")))
	doc["schema_version"] = "2.0"
	if err := ValidateTarget(root, "context/a", doc); err == nil {
		t.Fatal("unknown schema version was accepted")
	}
}
