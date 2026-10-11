package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// goldenUpdate reports whether the byte-goldens are rewritten instead of
// compared. It is the documented regeneration path: with UPDATE_GOLDEN=1 a
// deliberate template edit refreshes the fixtures in one command, instead of a
// throwaway test that writes the function output to the fixture path by hand.
func goldenUpdate() bool { return os.Getenv("UPDATE_GOLDEN") == "1" }

// checkGolden compares got against the committed byte-golden name, or rewrites
// the fixture when UPDATE_GOLDEN=1 is set. The fixtures lock the exact bytes of
// the generated instruction-surface documents; a template edit breaks the match
// and requires regenerating the fixture deliberately rather than silently
// changing output on the next init.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	fixture := filepath.Join("testdata", "golden", name)
	if goldenUpdate() {
		if err := os.WriteFile(fixture, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if got != string(data) {
		t.Errorf("%s drifted from its committed golden bytes; regenerate with UPDATE_GOLDEN=1 go test ./cli/cmd -run TestInstructionTemplateBytesGolden", name)
	}
}

// TestInstructionTemplateBytesGolden locks the generated documents that the
// static round-trip and per-template tests cannot restate exactly:
// instructions/project.md (identity-composed), agents/instructions (the
// AGENTS.md instructions block) and workspace/readme (registry-driven Commands
// section appended to the static shell).
func TestInstructionTemplateBytesGolden(t *testing.T) {
	cases := []struct {
		name string
		got  string
	}{
		{"project-instruction.md", instrProjectTemplate("p", "g")},
		{"agents-instructions.md", agentBlockInstructions("p", "g")},
		{"work-readme.md", sdtWorkReadmeContent()},
	}
	for _, tc := range cases {
		checkGolden(t, tc.name, tc.got)
	}
}

// parseBlockIdentity recovers the project/group pair a generated AGENTS.md
// instructions block was written with, so the drift guard can re-render with
// the same identity instead of assuming a fixed value.
func parseBlockIdentity(body string) (project, group string) {
	re := regexp.MustCompile(`(?m)^Project: (\S+)(?: · Group: (\S+))?$`)
	if m := re.FindStringSubmatch(body); m != nil {
		project = m[1]
		if len(m) > 2 {
			group = m[2]
		}
	}
	return project, group
}

// TestGeneratedAGENTSBlocksMatchTemplates guards against drift between the
// agents/instructions.md.tmpl template and the committed AGENTS.md instructions
// block — the only machine-refreshable AGENTS.md surface. AGENTS.md is tracked
// by the repo, so unlike the context/-based guards this one also runs on CI. A
// failure means a template changed without `sdt agent init --force`.
//
// The project block is deliberately NOT byte-compared: it is write-once and
// user-owned (rule 5 fills its sections from project evidence), so the seed
// template and the on-disk block legitimately differ.
func TestGeneratedAGENTSBlocksMatchTemplates(t *testing.T) {
	path := filepath.Join("..", "..", agentTargetDefault)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s not present: %v", path, err)
	}
	content := string(data)

	// Marker regions are line-anchored so the inline backticked mentions of
	// `<!-- sdt:begin:project -->` inside the instructions prose never match.
	re := regexp.MustCompile(
		`(?ms)^` + regexp.QuoteMeta(sectionBeginMarker(agentSectionNameInstructions)) + `.*?^` + regexp.QuoteMeta(sectionEndMarker(agentSectionNameInstructions)),
	)
	raw := re.FindString(content)
	if raw == "" {
		t.Fatal("AGENTS.md missing instructions block")
	}

	// Identity is recovered from the committed block itself so the guard
	// re-renders with the same project/group instead of assuming a fixed value.
	body := strings.TrimSpace(raw[len(sectionBeginMarker(agentSectionNameInstructions)) : len(raw)-len(sectionEndMarker(agentSectionNameInstructions))])
	project, group := parseBlockIdentity(body)
	want := sectionBlock(agentSectionNameInstructions, agentBlockInstructions(project, group))
	if strings.TrimSpace(raw) != strings.TrimSpace(want) {
		t.Errorf("AGENTS.md instructions block drifted from the template (run sdt agent init --force)")
	}

	// The project block must still be present (user-owned content may differ).
	if !strings.Contains(content, sectionBeginMarker(agentSectionNameProject)) ||
		!strings.Contains(content, sectionEndMarker(agentSectionNameProject)) {
		t.Errorf("AGENTS.md missing the write-once project block")
	}
}
