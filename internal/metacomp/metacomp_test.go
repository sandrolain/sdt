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

func TestDecodeRequiresMapping(t *testing.T) {
	for _, input := range []string{"", "null", "[]", "broken: ["} {
		if _, err := Decode([]byte(input)); err == nil {
			t.Errorf("Decode(%q) succeeded", input)
		}
	}
	doc, err := Decode([]byte("schema_version: '1.0'\ncustom: keep\n"))
	if err != nil || doc["custom"] != "keep" {
		t.Fatalf("Decode extension: %#v, %v", doc, err)
	}
}

func TestValidateTargetChecksRelatedArtifactsAndIdentity(t *testing.T) {
	root := t.TempDir()
	targetPath, relatedPath := "context/a.md", "context/b.bin"
	target, related := []byte("a"), []byte{0, 255}
	for path, data := range map[string][]byte{targetPath: target, relatedPath: related} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	doc := New(targetPath, Digest(target))
	doc["related_artifacts"] = []any{map[string]any{"role": "source", "path": relatedPath, "sha256": Digest(related)}}
	if err := ValidateTarget(root, targetPath, doc); err != nil {
		t.Fatalf("valid association: %v", err)
	}
	for name, mutate := range map[string]func(Document){
		"target mapping":   func(d Document) { d["target"] = "bad" },
		"path mismatch":    func(d Document) { d["target"] = map[string]any{"path": "context/other", "sha256": Digest(target)} },
		"digest uppercase": func(d Document) { d["target"] = map[string]any{"path": targetPath, "sha256": "ABC"} },
		"related sequence": func(d Document) { d["related_artifacts"] = "bad" },
		"related item":     func(d Document) { d["related_artifacts"] = []any{"bad"} },
		"related role": func(d Document) {
			d["related_artifacts"] = []any{map[string]any{"role": " ", "path": relatedPath, "sha256": Digest(related)}}
		},
		"related path": func(d Document) {
			d["related_artifacts"] = []any{map[string]any{"role": "source", "path": "../outside", "sha256": Digest(related)}}
		},
		"related digest": func(d Document) {
			d["related_artifacts"] = []any{map[string]any{"role": "source", "path": relatedPath, "sha256": Digest(target)}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := New(targetPath, Digest(target))
			mutate(changed)
			if err := ValidateTarget(root, targetPath, changed); err == nil {
				t.Fatal("invalid association accepted")
			}
		})
	}
}

func TestStringMapAcceptsStringKeysOnly(t *testing.T) {
	if _, ok := stringMap(map[string]any{"x": 1}); !ok {
		t.Fatal("string map rejected")
	}
	converted, ok := stringMap(map[any]any{"x": 1})
	if !ok || converted["x"] != 1 {
		t.Fatalf("converted = %#v, %v", converted, ok)
	}
	if _, ok := stringMap(map[any]any{1: "x"}); ok {
		t.Fatal("non-string key accepted")
	}
	if _, ok := stringMap("scalar"); ok {
		t.Fatal("scalar accepted")
	}
}

func TestReadTargetRejectsEscapingCompanionSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "context", "item")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "companion")
	if err := os.WriteFile(outside, []byte("schema_version: '1.0'"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, CompanionPath(target)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := ReadTarget(root, "context/item"); err == nil {
		t.Fatal("escaping companion symlink accepted")
	}
}
