package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiffUnifiedIdentical(t *testing.T) {
	a := writeTempFile(t, "a.txt", "line1\nline2\nline3\n")
	b := writeTempFile(t, "b.txt", "line1\nline2\nline3\n")
	out := execute(t, diffCmd, nil, "--a", a, "--b", b)
	// Identical files: only header, no hunks
	s := string(out)
	if strings.Contains(s, "@@") {
		t.Errorf("expected no hunks for identical files, got: %s", s)
	}
}

func TestDiffUnifiedChanged(t *testing.T) {
	a := writeTempFile(t, "a.txt", "line1\nline2\nline3\n")
	b := writeTempFile(t, "b.txt", "line1\nLINE2\nline3\n")
	out := execute(t, diffCmd, nil, "--a", a, "--b", b)
	s := string(out)
	if !strings.Contains(s, "@@") {
		t.Errorf("expected hunk markers, got: %s", s)
	}
	if !strings.Contains(s, "-line2") {
		t.Errorf("expected removed line2, got: %s", s)
	}
	if !strings.Contains(s, "+LINE2") {
		t.Errorf("expected added LINE2, got: %s", s)
	}
}

func TestDiffJsonPatch(t *testing.T) {
	a := writeTempFile(t, "a.json", `{"name":"Alice","age":30}`)
	b := writeTempFile(t, "b.json", `{"name":"Bob","age":30}`)
	out := execute(t, diffCmd, nil, "--a", a, "--b", b, "--diff-format", "json-patch")
	var ops []jsonPatchOp
	if err := json.Unmarshal(out, &ops); err != nil {
		t.Fatalf("json-patch output invalid JSON: %v", err)
	}
	found := false
	for _, op := range ops {
		if op.Op == "replace" && strings.Contains(op.Path, "name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected replace op for 'name', got: %v", ops)
	}
}

func TestDiffMissingArgs(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, diffCmd, nil, "--a", "/tmp/x.txt")
		return ""
	})
}

func TestLCS(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	b := []string{"a", "c", "d", "e"}
	dp := lcs(a, b)
	// LCS length should be 3 (a, c, d)
	if dp[len(a)][len(b)] != 3 {
		t.Errorf("expected LCS length 3, got %d", dp[len(a)][len(b)])
	}
}
