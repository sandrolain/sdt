package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEditBatchManifest writes a batch manifest and returns its path.
func writeEditBatchManifest(t *testing.T, content string) string {
	t.Helper()
	dir := runInTempDir(t)
	p := filepath.Join(dir, "batch.yaml")
	writeTestFile(t, p, content)
	return p
}

// TestEditBatchAppliesManyFiles is the happy path: two files, two pairs each,
// the count assertion honoured and the diffs printed.
func TestEditBatchAppliesManyFiles(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "a.go"), "alpha one beta\nalpha two\n")
	writeTestFile(t, filepath.Join(dir, "b.go"), "keep drop\n")
	manifest := "files:\n" +
		"  - file: a.go\n" +
		"    pairs:\n" +
		"      - old: alpha\n" +
		"        new: ALPHA\n" +
		"        count: 2\n" +
		"      - old: beta\n" +
		"        new: BETA\n" +
		"  - file: b.go\n" +
		"    pairs:\n" +
		"      - old: drop\n" +
		"        new: \"\"\n"
	out := string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
	if !strings.Contains(out, "+ALPHA one BETA") || !strings.Contains(out, "-keep drop") {
		t.Errorf("batch must print the diffs, got:\n%s", out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ALPHA one BETA\nALPHA two\n" {
		t.Errorf("a.go = %q", got)
	}
	got, err = os.ReadFile(filepath.Join(dir, "b.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep \n" {
		t.Errorf("b.go = %q, want the empty replacement to delete", got)
	}
}

// TestEditBatchCountGuardRefuses proves the per-pair assertion: count 0 is a
// guard that refuses when the literal occurs, count N demands exactly N.
func TestEditBatchCountGuardRefuses(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "f.txt"), "x\n")
	manifest := "files:\n" +
		"  - file: f.txt\n" +
		"    pairs:\n" +
		"      - old: x\n" +
		"        new: y\n" +
		"        count: 0\n"
	out := ""
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
		return out
	})
	if !strings.Contains(out, "expected 0 occurrence(s), found 1") {
		t.Errorf("guard pair must refuse with the counts, got:\n%s", out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x\n" {
		t.Errorf("guard pair must not write: %q", got)
	}
	manifest = "files:\n" +
		"  - file: f.txt\n" +
		"    pairs:\n" +
		"      - old: x\n" +
		"        new: y\n" +
		"        count: 2\n"
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
	})
}

// TestEditBatchPartialIsExitThree proves decision 0037's exit 3: some files
// applied, some refused, the refused one byte-identical.
func TestEditBatchPartialIsExitThree(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "good.txt"), "a\n")
	writeTestFile(t, filepath.Join(dir, "bad.txt"), "b\n")
	manifest := "files:\n" +
		"  - file: good.txt\n" +
		"    pairs:\n" +
		"      - old: a\n" +
		"        new: A\n" +
		"  - file: bad.txt\n" +
		"    pairs:\n" +
		"      - old: nope\n" +
		"        new: B\n"
	out := ""
	shouldExitWithCode(t, 3, func() string {
		out = string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
		return out
	})
	if !strings.Contains(out, "expected 1 occurrence(s), found 0") {
		t.Errorf("refusal must name the pair, got:\n%s", out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "good.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A\n" {
		t.Errorf("good.txt must have applied: %q", got)
	}
	got, err = os.ReadFile(filepath.Join(dir, "bad.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "b\n" {
		t.Errorf("bad.txt must be untouched: %q", got)
	}
}

// TestEditBatchNothingAppliedIsExitOne proves the other exit: when nothing
// applied the caller cannot mistake the run for a success.
func TestEditBatchNothingAppliedIsExitOne(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "f.txt"), "a\n")
	manifest := "files:\n" +
		"  - file: f.txt\n" +
		"    pairs:\n" +
		"      - old: nope\n" +
		"        new: A\n"
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
	})
}

// TestEditBatchJSONAnswer proves the machine-readable shape: per-file,
// per-pair facts, applied and refused counts.
func TestEditBatchJSONAnswer(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "f.txt"), "x\n")
	manifest := "files:\n" +
		"  - file: f.txt\n" +
		"    pairs:\n" +
		"      - old: x\n" +
		"        new: y\n"
	out := execute(t, editCmd, []byte(manifest), "--batch", "-", "--format", "json")
	var payload editBatchResult
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json answer does not parse: %v\n%s", err, out)
	}
	if len(payload.Files) != 1 || !payload.Files[0].Changed || payload.Applied != 1 || payload.Refused != 0 {
		t.Fatalf("json answer = %+v", payload)
	}
	if len(payload.Files[0].Pairs) != 1 || payload.Files[0].Pairs[0].Found != 1 || payload.Files[0].Pairs[0].Replaced != 1 {
		t.Errorf("pair answer = %+v", payload.Files[0].Pairs)
	}
}

// TestEditBatchRefusesGuardsAndMixedModes covers the file guards the single
// form has: symlink, binary, corpus, and a mode flag passed with --batch.
func TestEditBatchRefusesGuardsAndMixedModes(t *testing.T) {
	dir := runInTempDir(t)
	realFile := filepath.Join(dir, "real.txt")
	writeTestFile(t, realFile, "x\n")
	if err := os.Symlink(realFile, filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "b.bin"), "a\x00b")
	manifest := "files:\n" +
		"  - file: link.txt\n" +
		"    pairs:\n" +
		"      - old: x\n" +
		"        new: y\n" +
		"  - file: b.bin\n" +
		"    pairs:\n" +
		"      - old: a\n" +
		"        new: y\n"
	out := ""
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
		return out
	})
	if !strings.Contains(out, "is a symlink") || !strings.Contains(out, "binary or not valid UTF-8") {
		t.Errorf("guards must name their refusals, got:\n%s", out)
	}

	// A mode flag beside --batch is a caller bug, refused with the mode named.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, []byte(manifest), "--batch", "-", "--old", "x"))
	})

	// An empty manifest is refused, not a silent no-op.
	empty := writeEditBatchManifest(t, "files: []\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, nil, "--batch", empty))
	})
}

// TestEditBatchDryRunWritesNothing proves the review step over the manifest:
// every diff printed, every file untouched.
func TestEditBatchDryRunWritesNothing(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "f.txt"), "one\n")
	manifest := "files:\n" +
		"  - file: f.txt\n" +
		"    pairs:\n" +
		"      - old: one\n" +
		"        new: two\n"
	out := string(execute(t, editCmd, []byte(manifest), "--batch", "-", "--dry-run"))
	if !strings.Contains(out, "+two") {
		t.Errorf("dry run must print the diff, got:\n%s", out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\n" {
		t.Errorf("dry run wrote the file: %q", got)
	}
}

// TestEditBatchJSONManifest proves JSON is accepted beside YAML: the leading
// byte picks the parser.
func TestEditBatchJSONManifest(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "f.txt"), "x\n")
	manifest := `{"files":[{"file":"f.txt","pairs":[{"old":"x","new":"y"}]}]}`
	execute(t, editCmd, []byte(manifest), "--batch", "-")
	got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "y\n" {
		t.Errorf("file = %q", got)
	}
}

// TestEditBatchRefusesTheCorpus keeps the corpus to one mutator, as the
// single-file form does.
func TestEditBatchRefusesTheCorpus(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/x.md", "---\nkind: notes\nsummary: s\n---\nbody\n")
	manifest := writeEditBatchManifest(t, "files:\n"+
		"  - file: context/notes/x.md\n"+
		"    pairs:\n"+
		"      - old: body\n"+
		"        new: b\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, editCmd, []byte(manifest), "--batch", "-"))
	})
}
