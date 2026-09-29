package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// analysisDoc builds an analysis body with `sections` filler sections and the
// given options block (already the inner text of `## Options considered`).
func analysisDoc(sections int, optionsSection string) string {
	var b strings.Builder
	b.WriteString("## Problem statement\n\ntext\n")
	if optionsSection != "" {
		b.WriteString("\n## Options considered\n\n" + optionsSection + "\n")
	}
	for i := range sections {
		fmt.Fprintf(&b, "\n## Section %d\n\ntext\n", i)
	}
	return b.String()
}

// lintAnalysisRules writes a document of the given kind and runs the per-file
// lint on it, returning the R3 option-outcome and R1 oversize findings.
func lintAnalysisRules(t *testing.T, kind, body string) (outcome, oversize []ctxLintIssue) {
	t.Helper()
	issues := lintAnalysisDocKind(t, kind, body)
	for _, i := range issues {
		switch {
		case strings.Contains(i.Message, "no recorded outcome"):
			outcome = append(outcome, i)
		case strings.Contains(i.Message, "more than one subject"), strings.Contains(i.Message, "split signal"):
			oversize = append(oversize, i)
		}
	}
	return outcome, oversize
}

func lintAnalysisDocKind(t *testing.T, kind, body string) []ctxLintIssue {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.md")
	content := "---\nkind: " + kind + "\nsummary: s\n---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return lintDoc(path)
}

func TestLintAnalysisOptionsOutcome(t *testing.T) {
	cases := []struct {
		name    string
		options string
		want    int // unrecorded options; 0 means the rule must stay silent
	}{
		{"recorded with the Outcome form", "### Option A: one\n- **Outcome:** accepted\n### Option B: two\n- **Outcome:** rejected (too costly)\n", 0},
		{"recorded with the terse bold form", "### Option A: one\n**Accepted**\n### Option B: two\n**Rejected**\n", 0},
		{"one of two recorded", "### Option A: one\n- **Outcome:** accepted\n### Option B: two\n- Pros: cheap\n", 1},
		{"none recorded", "### Option A: one\n- Pros: x\n- Cons: y\n### Option B: two\n- Pros: x\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := lintAnalysisRules(t, ctxTypeAnalysis, analysisDoc(1, c.options))
			if c.want == 0 {
				if len(got) != 0 {
					t.Fatalf("every option records an outcome, want no finding, got %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want one outcome finding, got %d: %v", len(got), got)
			}
			if !strings.Contains(got[0].Message, fmt.Sprintf("%d option(s)", c.want)) {
				t.Errorf("finding should count %d unrecorded option(s), got %q", c.want, got[0].Message)
			}
			if got[0].Priority != ctxLintSuggestion {
				t.Errorf("rule must stay advisory SUGGESTION, got %s", got[0].Priority)
			}
			// The hint names the R3 outcome vocabulary and the rule.
			if !strings.Contains(got[0].Hint, "accepted / rejected") || !strings.Contains(got[0].Hint, "R3") {
				t.Errorf("hint should name the R3 outcomes, got %q", got[0].Hint)
			}
		})
	}
}

func TestLintAnalysisOptionsScope(t *testing.T) {
	// An analysis that leads to no choice omits the section: no finding.
	if got, _ := lintAnalysisRules(t, ctxTypeAnalysis, analysisDoc(1, "")); len(got) != 0 {
		t.Errorf("no Options considered section must not fire: %v", got)
	}
	// The rule is analysis-only.
	if got, _ := lintAnalysisRules(t, ctxTypePlan, analysisDoc(1, "### Option A: one\n- Pros: x\n")); len(got) != 0 {
		t.Errorf("rule is analysis-only, fired on a plan: %v", got)
	}
}

func TestLintAnalysisOversize(t *testing.T) {
	clean := "### Option A: one\n- **Outcome:** accepted\n"

	// Under both bounds: silent.
	if _, over := lintAnalysisRules(t, ctxTypeAnalysis, analysisDoc(ctxAnalysisOversizedSections-3, clean)); len(over) != 0 {
		t.Errorf("analysis under the bounds must not fire: %v", over)
	}
	// Past the section bound.
	_, over := lintAnalysisRules(t, ctxTypeAnalysis, analysisDoc(ctxAnalysisOversizedSections+1, ""))
	if len(over) != 1 {
		t.Fatalf("want one oversize finding, got %v", over)
	}
	if !strings.Contains(over[0].Message, fmt.Sprintf("%d sections", ctxAnalysisOversizedSections+2)) {
		t.Errorf("section finding should count sections, got %q", over[0].Message)
	}
	// Past the option bound, sections still few.
	options := strings.Repeat("### Option: x\n- **Outcome:** rejected\n", ctxAnalysisOversizedOptions+1)
	_, over = lintAnalysisRules(t, ctxTypeAnalysis, analysisDoc(1, options))
	if len(over) != 1 {
		t.Fatalf("want one oversize finding, got %v", over)
	}
	if !strings.Contains(over[0].Message, fmt.Sprintf("%d options", ctxAnalysisOversizedOptions+1)) {
		t.Errorf("option finding should count options, got %q", over[0].Message)
	}
	// The split gate is the user's: the hint must say so.
	if !strings.Contains(over[0].Hint, "ask the user") || !strings.Contains(over[0].Hint, "R1") {
		t.Errorf("hint should carry the ask-first gate, got %q", over[0].Hint)
	}
	if over[0].Priority != ctxLintSuggestion {
		t.Errorf("rule must stay advisory SUGGESTION, got %s", over[0].Priority)
	}
}

func TestCtxSectionExtraction(t *testing.T) {
	body := "## One\n\ntext\n```sh\n## not a section\n```\n## Two\n\n### Sub\n\ntext\n## Three\n"

	sec, ok := ctxSection(body, "One")
	if !ok {
		t.Fatal("section One not found")
	}
	if strings.Contains(sec, "not a section") {
		t.Errorf("fenced code must be excluded from a section body: %q", sec)
	}
	if strings.Contains(sec, "## Two") {
		t.Errorf("section must end at the next H2: %q", sec)
	}
	if _, ok := ctxSection(body, "Missing"); ok {
		t.Error("absent section must report ok=false")
	}

	// A trailing section runs to the end of the document, and a nested `###`
	// stays inside its parent.
	last, ok := ctxSection(body, "Three")
	if !ok || last != "## Three\n" {
		t.Errorf("trailing section = %q (ok=%v)", last, ok)
	}
	two, ok := ctxSection(body, "Two")
	if !ok || !strings.Contains(two, "### Sub") || strings.Contains(two, "## Three") {
		t.Errorf("nested heading must stay inside its parent: %q", two)
	}
}

func TestStripFencedCode(t *testing.T) {
	body := "a\n```\n## x\n```\nb\n~~~\n## y\n"
	got := stripFencedCode(body)
	if strings.Contains(got, "## x") || strings.Contains(got, "## y") {
		t.Errorf("fenced content must be blanked: %q", got)
	}
	if !strings.Contains(got, "a\n") || !strings.Contains(got, "b\n") {
		t.Errorf("content outside fences must survive: %q", got)
	}
	if n := strings.Count(got, "\n"); n != strings.Count(body, "\n") {
		t.Errorf("line count must be preserved: %d vs %d", n, strings.Count(body, "\n"))
	}
}

func TestCtxSectionCountIgnoresSubheadingsAndFences(t *testing.T) {
	// 3 H2 sections; the `###` and the fenced `## ` must not be counted.
	body := "## A\n### A1\n## B\n```\n## fake\n```\n## C\n"
	if n := ctxSectionCount(body); n != 3 {
		t.Errorf("ctxSectionCount = %d, want 3", n)
	}
}
