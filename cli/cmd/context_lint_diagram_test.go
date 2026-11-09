package cmd

import (
	"regexp"
	"strings"
	"testing"
)

// The diagram check's own tests, plus the test that binds the two copies of the
// keyword set: the Go map and the dialect module's table.

func lintDiagramBody(t *testing.T, body string) []ctxLintIssue {
	t.Helper()
	return lintDiagram("context/notes/probe.md", body)
}

func TestDiagramKeywordKnownAndUnknown(t *testing.T) {
	known := "```mermaid\nflowchart LR\n  A --> B\n```\n"
	if got := lintDiagramBody(t, known); len(got) != 0 {
		t.Fatalf("known keyword produced findings: %v", got)
	}

	unknown := "```mermaid\nflowchrt LR\n  A --> B\n```\n"
	got := lintDiagramBody(t, unknown)
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(got), got)
	}
	if got[0].Priority != ctxLintSuggestion {
		t.Errorf("priority = %s, want SUGGESTION (a diagram never hard-fails the lint)", got[0].Priority)
	}
	if !strings.Contains(got[0].Message, "not the diagram's syntax") {
		t.Errorf("finding does not state the check's limit: %q", got[0].Message)
	}
}

func TestDiagramKeywordSkipsDirectiveAndFrontmatter(t *testing.T) {
	cases := map[string]string{
		"init directive": "```mermaid\n%%{init: {'theme': 'forest'}}%%\nflowchart LR\n  A --> B\n```\n",
		"comment":        "```mermaid\n%% a comment\nflowchart LR\n  A --> B\n```\n",
		"frontmatter":    "```mermaid\n---\ntitle: A\n---\nflowchart LR\n  A --> B\n```\n",
		"blank lines":    "```mermaid\n\n\nflowchart LR\n  A --> B\n```\n",
	}
	for name, body := range cases {
		if got := lintDiagramBody(t, body); len(got) != 0 {
			t.Errorf("%s: findings on a legal block: %v", name, got)
		}
	}
}

func TestDiagramBlockWithoutTypeIsAFinding(t *testing.T) {
	body := "```mermaid\n\n%%{init: {}}%%\n```\n"
	got := lintDiagramBody(t, body)
	if len(got) != 1 || !strings.Contains(got[0].Message, "no type keyword") {
		t.Fatalf("want one no-type finding, got %v", got)
	}
}

func TestDiagramNonMermaidFencesAreIgnored(t *testing.T) {
	body := "```go\nfunc main() {}\n```\n\n```yaml\nkey: value\n```\n"
	if got := lintDiagramBody(t, body); len(got) != 0 {
		t.Fatalf("non-mermaid fences produced findings: %v", got)
	}
}

func TestDiagramIntentLineValid(t *testing.T) {
	body := "<!-- diagram: question=the flow; source=code; form=flowchart -->\n" +
		"```mermaid\nflowchart LR\n  A --> B\n```\n"
	if got := lintDiagramBody(t, body); len(got) != 0 {
		t.Fatalf("valid intent line produced findings: %v", got)
	}
}

func TestDiagramIntentLineFindings(t *testing.T) {
	cases := map[string]struct {
		line string
		want string
	}{
		"unknown key":   {"<!-- diagram: purpose=x; source=code -->", "outside the set"},
		"empty value":   {"<!-- diagram: question=; source=code -->", "has no value"},
		"not key=value": {"<!-- diagram: just words -->", "is not `key=value`"},
		"unknown form":  {"<!-- diagram: form=flowchrt -->", "is not a type keyword"},
		"form mismatch": {"<!-- diagram: form=sequenceDiagram -->", "disagrees with the block's `flowchart`"},
	}
	for name, c := range cases {
		body := c.line + "\n```mermaid\nflowchart LR\n  A --> B\n```\n"
		got := lintDiagramBody(t, body)
		if len(got) == 0 {
			t.Errorf("%s: no finding", name)
			continue
		}
		if !strings.Contains(strings.Join(messages(got), " "), c.want) {
			t.Errorf("%s: finding %q does not mention %q", name, strings.Join(messages(got), " | "), c.want)
		}
	}
}

func TestDiagramIntentLineAbsenceIsNotAFinding(t *testing.T) {
	body := "```mermaid\nflowchart LR\n  A --> B\n```\n"
	if got := lintDiagramBody(t, body); len(got) != 0 {
		t.Fatalf("absence of an intent line produced findings: %v", got)
	}
}

func TestDiagramIntentLineDetachesOnABlankLine(t *testing.T) {
	body := "<!-- diagram: form=sequenceDiagram -->\n\n```mermaid\nflowchart LR\n  A --> B\n```\n"
	if got := lintDiagramBody(t, body); len(got) != 0 {
		t.Fatalf("a detached intent line was read as attached: %v", got)
	}
}

// TestDiagramKeywordsMatchDialect binds the two copies of the keyword set: every
// keyword the dialect module's question → keyword table names must be in the Go
// map. The set's other half is pinned from the renderer's side by
// web/src/lib/diagramKeywords.test.ts.
func TestDiagramKeywordsMatchDialect(t *testing.T) {
	module := generatedInstruction(t, "diagram-mermaid.md")
	table := dialectKeywordTable(t, module)
	if len(table) == 0 {
		t.Fatal("no keyword column found in the dialect module's table")
	}
	for _, keyword := range table {
		if !ctxDiagramKeywords[keyword] {
			t.Errorf("dialect table names %q, which the lint's keyword set does not carry", keyword)
		}
	}
}

// dialectKeywordTable returns the backticked keywords of the question → keyword
// table, in the module's own order.
func dialectKeywordTable(t *testing.T, module string) []string {
	t.Helper()
	backtick := regexp.MustCompile("`([^`]+)`")
	var keywords []string
	inTable := false
	for _, line := range strings.Split(module, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| Question") || strings.HasPrefix(trimmed, "|Question") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := strings.Split(trimmed, "|")
		if len(cells) < 3 {
			continue
		}
		for _, word := range backtick.FindAllStringSubmatch(cells[2], -1) {
			keywords = append(keywords, word[1])
		}
	}
	return keywords
}

func messages(issues []ctxLintIssue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Message)
	}
	return out
}
