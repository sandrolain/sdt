package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstructionWriterRefreshesFrontmatter is the defect this writer exists
// for: an instruction module's header is generated, so a --force refresh must
// adopt the template's title and summary, not keep the old ones.
func TestInstructionWriterRefreshesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "material.md")
	name := agentGeneratedMarkerName("instructions", "material.md")
	old := "---\nkind: instructions\ntitle: Old title\nsummary: Old summary\n---\n\n" + sectionBlock(name, "old body") + "\n\n- a user trigger: keep me\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "---\nkind: instructions\ntitle: New title\nsummary: New summary\n---\n\nnew body"

	res := agentWriteInstructionFile(path, name, body, true)
	if res.Status != statusUpdated {
		t.Fatalf("status = %q, want %q", res.Status, statusUpdated)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "title: New title") || strings.Contains(got, "Old title") {
		t.Errorf("frontmatter was not refreshed:\n%s", got)
	}
	if !strings.Contains(got, "new body") || strings.Contains(got, "old body") {
		t.Errorf("body was not refreshed:\n%s", got)
	}
	if !strings.Contains(got, "a user trigger: keep me") {
		t.Errorf("user content outside the markers was lost:\n%s", got)
	}

	skipped := agentWriteInstructionFile(path, name, body, false)
	if skipped.Status != statusSkipped {
		t.Errorf("without force status = %q, want %q", skipped.Status, statusSkipped)
	}
}
