//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// budgetTree builds a working tree carrying the declared surface set: an
// AGENTS.md with an instructions block and a write-once project block, the
// generated index, and the two essential-tier directories. It points the
// process at it so ctxBudgetSurfaces walks the tree.
func budgetTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(agentTargetDefault, "# T\n\n"+sectionBlock(agentSectionNameInstructions, "alpha\nbeta gamma\n")+
		"\n"+sectionBlock("project", "project conventions\n")+"\n")
	write(ctxBudgetIndexFile, "one\ntwo\nthree\n")
	write(sdtArchitectureDir+"/a.md", "alpha alpha\n")
	write(sdtArchitectureDir+"/b.md", "beta\n")
	write(sdtDecisionsDir+"/0001-x.md", "decision body\n")
	t.Chdir(root)
	return root
}

func TestContextBudgetMeasuresDeclaredSurfaces(t *testing.T) {
	budgetTree(t)
	rep := ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, false)

	want := []string{"instructions", "index", "architecture", "decisions"}
	if len(rep.Surfaces) != len(want) {
		t.Fatalf("surfaces = %d, want %d: %+v", len(rep.Surfaces), len(want), rep.Surfaces)
	}
	for i, name := range want {
		if rep.Surfaces[i].Name != name {
			t.Errorf("surface %d = %q, want %q", i, rep.Surfaces[i].Name, name)
		}
	}

	byName := map[string]ctxBudgetSurface{}
	for _, s := range rep.Surfaces {
		byName[s.Name] = s
	}

	// The instructions surface counts the generated block as it is read — markers
	// included, since that whole span is what the agent loads — and excludes the
	// write-once project block beside it.
	wholeFile := ctxBudgetReadFile(agentTargetDefault)
	gotBytes := byName["instructions"].Bytes
	bodyBytes := len("alpha\nbeta gamma\n")
	if gotBytes <= bodyBytes {
		t.Errorf("instructions bytes = %d, want more than the bare body (%d): the block as read includes its markers",
			gotBytes, bodyBytes)
	}
	if gotBytes >= wholeFile.Bytes {
		t.Errorf("instructions bytes = %d, want less than the whole file (%d): the project block must be excluded",
			gotBytes, wholeFile.Bytes)
	}
	if got := byName["index"].Bytes; got != len("one\ntwo\nthree\n") {
		t.Errorf("index bytes = %d, want the whole file", got)
	}
	if got := byName["architecture"].Files; got != 2 {
		t.Errorf("architecture files = %d, want 2", got)
	}
	if got := byName["decisions"].Files; got != 1 {
		t.Errorf("decisions files = %d, want 1", got)
	}
}

func TestContextBudgetTotalsAndEstimate(t *testing.T) {
	budgetTree(t)
	rep := ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, false)

	var sumBytes, sumLines, sumWords int
	for _, s := range rep.Surfaces {
		sumBytes += s.Bytes
		sumLines += s.Lines
		sumWords += s.Words
	}
	if rep.TotalBytes != sumBytes || rep.TotalLines != sumLines || rep.TotalWords != sumWords {
		t.Errorf("totals = (%d,%d,%d), want the sum of the surfaces (%d,%d,%d)",
			rep.TotalBytes, rep.TotalLines, rep.TotalWords, sumBytes, sumLines, sumWords)
	}
	if rep.EstimatedTokens != rep.TotalBytes/ctxBudgetBytesPerToken {
		t.Errorf("estimated tokens = %d, want totalBytes/%d", rep.EstimatedTokens, ctxBudgetBytesPerToken)
	}
	if !strings.Contains(rep.TokenBasis, "not a model tokenizer count") {
		t.Errorf("token basis = %q, must label the figure as an estimate", rep.TokenBasis)
	}

	// A different divisor must change the estimate and the stated basis.
	rep4 := ctxBudgetBuildReport(3, 8, false)
	if rep4.EstimatedTokens != rep4.TotalBytes/8 {
		t.Errorf("with divisor 8, tokens = %d, want totalBytes/8", rep4.EstimatedTokens)
	}
	if rep4.BytesPerToken != 8 {
		t.Errorf("bytesPerToken = %d, want the override 8", rep4.BytesPerToken)
	}

	// A non-positive divisor falls back to the documented default rather than
	// dividing by zero.
	if got := ctxBudgetBuildReport(3, 0, false).BytesPerToken; got != ctxBudgetBytesPerToken {
		t.Errorf("bytesPerToken for divisor 0 = %d, want the default %d", got, ctxBudgetBytesPerToken)
	}
}

func TestContextBudgetTopEntriesBoundedAndDeterministic(t *testing.T) {
	tree := budgetTree(t)
	write := func(rel, content string) {
		p := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(sdtArchitectureDir+"/big.md", strings.Repeat("x\n", 50))
	write(sdtArchitectureDir+"/mid.md", strings.Repeat("y\n", 20))

	var arch ctxBudgetSurface
	for _, s := range ctxBudgetBuildReport(2, ctxBudgetBytesPerToken, false).Surfaces {
		if s.Name == "architecture" {
			arch = s
		}
	}
	if len(arch.Largest) != 2 {
		t.Fatalf("largest entries = %d, want 2 (the --top bound)", len(arch.Largest))
	}
	if arch.Largest[0].Bytes <= arch.Largest[1].Bytes {
		t.Errorf("largest entries are not ordered by size: %+v", arch.Largest)
	}
	// Same tree, same order: the report is deterministic.
	second := ctxBudgetBuildReport(2, ctxBudgetBytesPerToken, false)
	for _, s := range second.Surfaces {
		if s.Name != "architecture" {
			continue
		}
		for j := range s.Largest {
			if s.Largest[j].Path != arch.Largest[j].Path {
				t.Errorf("entry %d = %q, want %q (deterministic order)", j, s.Largest[j].Path, arch.Largest[j].Path)
			}
		}
	}

	// --top 0 lists none, so the report stays small whatever the surface size.
	for _, s := range ctxBudgetBuildReport(0, ctxBudgetBytesPerToken, false).Surfaces {
		if len(s.Largest) != 0 {
			t.Errorf("surface %q listed %d entries with --top 0", s.Name, len(s.Largest))
		}
	}
}

func TestContextBudgetOptionalInstructionModules(t *testing.T) {
	budgetTree(t)
	if err := os.MkdirAll(sdtInstrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdtInstrDir, "cli.md"), []byte("module body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var with, without bool
	for _, s := range ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, false).Surfaces {
		with = with || s.Name == "instruction-modules"
	}
	for _, s := range ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, true).Surfaces {
		without = without || s.Name == "instruction-modules"
	}
	if with {
		t.Error("instruction modules measured without --include-modules; they load on demand")
	}
	if !without {
		t.Error("--include-modules did not add the instruction-modules surface")
	}
}

func TestContextBudgetMissingSurfaceDegradesToSilence(t *testing.T) {
	// An empty tree has no AGENTS.md, no index and no directories: the report is
	// informational, so it must produce a report rather than fail.
	t.Chdir(t.TempDir())
	rep := ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, false)
	if rep.TotalBytes != 0 {
		t.Errorf("total bytes on an empty tree = %d, want 0", rep.TotalBytes)
	}
	if len(rep.Surfaces) != 3 { // instructions, architecture, decisions (no index file)
		t.Errorf("surfaces = %d, want 3", len(rep.Surfaces))
	}
	if rep.EstimatedTokens != 0 {
		t.Errorf("estimated tokens = %d, want 0", rep.EstimatedTokens)
	}
}

func TestContextBudgetTextOutput(t *testing.T) {
	budgetTree(t)
	out := string(execute(t, contextBudgetCmd, nil))
	for _, want := range []string{
		"always-loaded surface",
		"instructions:",
		"index:",
		"architecture:",
		"decisions:",
		"total:",
		"estimate:",
		"not a model tokenizer count",
		"never fails a build and is not a gate",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}

	// A single-file surface names its file on the surface line, so the
	// largest-entry block is not repeated underneath it.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	instructions, index := -1, -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "instructions:") {
			instructions = i
		}
		if strings.HasPrefix(strings.TrimSpace(l), "index:") {
			index = i
		}
	}
	if instructions < 0 || index < 0 || index != instructions+1 {
		t.Errorf("single-file surfaces are not adjacent single lines (instructions at %d, index at %d)", instructions, index)
	}
}

func TestContextBudgetJSONAndYAMLShape(t *testing.T) {
	budgetTree(t)
	out := execute(t, contextBudgetCmd, nil, "--format", "json", "--top", "1")
	var rep ctxBudgetReport
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("decode budget output: %v\n%s", err, out)
	}
	if len(rep.Surfaces) == 0 {
		t.Fatal("json report carries no surface")
	}
	for _, s := range rep.Surfaces {
		if s.Name == "" || s.Bytes < 0 || s.Lines < 0 || s.Words < 0 {
			t.Errorf("surface %+v carries an empty name or a negative measure", s)
		}
	}
	if rep.BytesPerToken == 0 {
		t.Error("json report does not carry the divisor behind the estimate")
	}

	y := execute(t, contextBudgetCmd, nil, "--format", "yaml")
	var yrep ctxBudgetReport
	if err := yaml.Unmarshal(y, &yrep); err != nil {
		t.Fatalf("decode budget yaml: %v\n%s", err, y)
	}
	if yrep.TotalBytes != rep.TotalBytes {
		t.Errorf("yaml total bytes = %d, json = %d", yrep.TotalBytes, rep.TotalBytes)
	}
}

func TestContextBudgetNeverGates(t *testing.T) {
	// A surface far larger than any realistic corpus still exits zero: the
	// report is informational (analysis D1) and never a gate.
	tree := budgetTree(t)
	if err := os.WriteFile(filepath.Join(tree, filepath.FromSlash(ctxBudgetIndexFile)),
		[]byte(strings.Repeat("line of text here\n", 200000)), 0o644); err != nil {
		t.Fatal(err)
	}
	out := execute(t, contextBudgetCmd, nil, "--format", "json")
	var rep ctxBudgetReport
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("a large surface broke the report: %v\n%s", err, out)
	}
	if rep.TotalBytes == 0 {
		t.Error("a large surface reported zero bytes")
	}
}

func TestCtxBudgetMeasure(t *testing.T) {
	tests := []struct {
		name    string
		content string
		bytes   int
		lines   int
		words   int
	}{
		{"empty", "", 0, 0, 0},
		{"one line no newline", "alpha", 5, 1, 1},
		{"one line newline", "alpha\n", 6, 1, 1},
		{"two lines", "alpha\nbeta\n", 11, 2, 2},
		{"blank lines count", "a\n\nb\n", 5, 3, 2},
		{"tabs and runs separate words", "a \t b\n", 6, 1, 2},
		{"crlf is one line", "a\r\nb\r\n", 6, 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctxBudgetMeasure(tt.content)
			if got.Bytes != tt.bytes || got.Lines != tt.lines || got.Words != tt.words {
				t.Errorf("ctxBudgetMeasure(%q) = %+v, want bytes %d lines %d words %d",
					tt.content, got, tt.bytes, tt.lines, tt.words)
			}
		})
	}
}
