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

	want := []string{"instructions", "project", "index", "architecture", "decisions"}
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

// TestContextBudgetProjectSurfaceIsTheAuthoredBlock pins F1's half of the report:
// the project block is measured where it is, so the instructions line and this
// one together are the whole every-turn file, and neither swallows the other.
func TestContextBudgetProjectSurfaceIsTheAuthoredBlock(t *testing.T) {
	budgetTree(t)
	rep := ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, false)

	var project ctxBudgetSurface
	started := 0
	for _, s := range rep.Surfaces {
		if s.Name == agentSectionNameProject {
			project = s
		}
		if s.Group == ctxBudgetGroupStart {
			started += s.Words
		}
	}
	if project.Name == "" {
		t.Fatal("the project block is missing from the report")
	}
	if project.Words == 0 {
		t.Error("the project block carries words in the fixture: a zero count means it was not measured")
	}
	if project.Bytes >= ctxBudgetReadFile(agentTargetDefault).Bytes {
		t.Errorf("project bytes = %d, want less than the whole file: the instructions block must be excluded", project.Bytes)
	}
	// The start subtotal is what a session pays before its first lookup, so it is
	// the sum of the surfaces in that group — block, project block and essentials.
	if rep.Start.Words != started {
		t.Errorf("start subtotal = %d words, want the sum of its surfaces (%d)", rep.Start.Words, started)
	}
}

// TestContextBudgetProjectSurfaceIgnoresAProseMention is the regression the real
// AGENTS.md produced: its instructions block names the project marker in prose, and
// a first-match search would start there and swallow the tail of the file — the
// report then claimed a project block larger than the file it lives in.
func TestContextBudgetProjectSurfaceIgnoresAProseMention(t *testing.T) {
	dir := t.TempDir()
	instructions := sectionBlock(agentSectionNameInstructions,
		"the write-once `<!-- sdt:begin:project -->` block below holds the conventions\n")
	project := sectionBlock(agentSectionNameProject, "stack and commands\n")
	writeTestFile(t, filepath.Join(dir, agentTargetDefault), "# T\n\n"+instructions+"\n"+project+"\n")
	t.Chdir(dir)

	got := ctxBudgetProjectSurface(3)
	if got.Name != agentSectionNameProject {
		t.Fatalf("project surface = %q, want %q", got.Name, agentSectionNameProject)
	}
	if want := ctxBudgetMeasure(project).Words; got.Words != want {
		t.Errorf("project words = %d, want %d: the prose mention is not the block", got.Words, want)
	}
}

// TestContextBudgetWithoutAProjectBlock proves the absence is a zero surface, not an
// error: a project that has not filled the block yet still gets a report.
func TestContextBudgetWithoutAProjectBlock(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, agentTargetDefault), "# T\n\n"+
		sectionBlock(agentSectionNameInstructions, "only the generated half\n")+"\n")
	t.Chdir(dir)

	if got := ctxBudgetProjectSurface(3); got.Name != "" {
		t.Fatalf("a file without the project block must contribute no surface: %+v", got)
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

// TestContextBudgetSeparatesStartFromLookup is the honesty gate: the report may
// not add the block a session begins with to the documents it never reads unless
// the work touches them.
func TestContextBudgetSeparatesStartFromLookup(t *testing.T) {
	root := budgetTree(t)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(sdtEssentialsFile)), []byte("digest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := ctxBudgetBuildReport(3, ctxBudgetBytesPerToken, false)

	group := map[string]string{}
	for _, s := range rep.Surfaces {
		group[s.Name] = s.Group
	}
	for _, want := range []string{"instructions", "essentials"} {
		if group[want] != ctxBudgetGroupStart {
			t.Errorf("%s group = %q, want %q", want, group[want], ctxBudgetGroupStart)
		}
	}
	for _, want := range []string{"index", "architecture", "decisions"} {
		if group[want] != ctxBudgetGroupLookup {
			t.Errorf("%s group = %q, want %q", want, group[want], ctxBudgetGroupLookup)
		}
	}
	if rep.Start.Words == 0 || rep.Lookup.Words == 0 {
		t.Fatalf("both groups must be measured: start %d words, lookup %d words", rep.Start.Words, rep.Lookup.Words)
	}
	if rep.Start.Words+rep.Lookup.Words != rep.TotalWords {
		t.Errorf("group words %d+%d do not add up to the total %d",
			rep.Start.Words, rep.Lookup.Words, rep.TotalWords)
	}

	text := ctxBudgetRenderText(rep)
	for _, want := range []string{"paid at the start of a session", "read on demand (lookup)", "subtotal:"} {
		if !strings.Contains(text, want) {
			t.Errorf("text report missing %q:\n%s", want, text)
		}
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
		"agent surface",
		"paid at the start of a session",
		"read on demand (lookup)",
		"instructions:",
		"index:",
		"architecture:",
		"decisions:",
		"subtotal:",
		"estimates:",
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
	seen := 0
	for i, l := range lines {
		name := strings.TrimSpace(l)
		if !strings.HasPrefix(name, "instructions:") && !strings.HasPrefix(name, "index:") {
			continue
		}
		seen++
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				continue
			}
			if strings.HasPrefix(next, "    ") {
				t.Errorf("single-file surface %q repeats its file in a largest-entry block", name)
			}
			break
		}
	}
	if seen != 2 {
		t.Errorf("found %d single-file surfaces, want instructions and index", seen)
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
