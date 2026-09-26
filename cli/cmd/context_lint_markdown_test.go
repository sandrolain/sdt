package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintMarkdownBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		priority   string
		message    string
		issueCount int
	}{
		{name: "plain body"},
		{name: "h1 is warning", body: "# title\n", priority: ctxLintWarning, message: "markdown H1", issueCount: 1},
		{name: "h1 inside fenced block ignored", body: "```md\n# not a heading\n```\n"},
		{name: "h1 in blockquote", body: "> # title\n", priority: ctxLintWarning, message: "markdown H1", issueCount: 1},
		{name: "h1 in list", body: "- # title\n", priority: ctxLintWarning, message: "markdown H1", issueCount: 1},
		{name: "balanced backticks", body: "```go\nfmt.Println(1)\n```\n"},
		{name: "balanced empty fence", body: "```\n```\n"},
		{name: "unmatched empty fence", body: "```\n", priority: ctxLintCritical, message: "unbalanced fenced code block", issueCount: 1},
		{name: "balanced tildes", body: "~~~text\ncontent\n~~~\n"},
		{name: "CRLF fence", body: "```\r\ncontent\r\n```\r\n"},
		{name: "longer closer", body: "```\ncontent\n````\n"},
		{name: "unmatched backticks", body: "```go\ncontent\n", priority: ctxLintCritical, message: "unbalanced fenced code block", issueCount: 1},
		{name: "short closer does not balance", body: "````\ncontent\n```\nmore\n", priority: ctxLintCritical, message: "unbalanced fenced code block", issueCount: 1},
		{name: "different fence does not balance", body: "~~~\ncontent\n```\n", priority: ctxLintCritical, message: "unbalanced fenced code block", issueCount: 1},
		{name: "unmatched fence in blockquote", body: "> ```\n> content\n", priority: ctxLintCritical, message: "unbalanced fenced code block", issueCount: 1},
		{name: "balanced fence in blockquote", body: "> ```\n> content\n> ```\n"},
		{name: "unmatched fence in list", body: "- item\n  ```\n  content\n", priority: ctxLintCritical, message: "unbalanced fenced code block", issueCount: 1},
		{name: "balanced fence in list", body: "- item\n  ```\n  content\n  ```\n"},
		{name: "balanced fence in nested list", body: "- outer\n    - inner\n      ```\n      content\n      ```\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := lintMarkdownBody("fixture.md", []byte(tt.body))
			if len(issues) != tt.issueCount {
				t.Fatalf("issues = %#v, want %d", issues, tt.issueCount)
			}
			if tt.issueCount == 0 {
				return
			}
			if issues[0].Priority != tt.priority || !strings.Contains(issues[0].Message, tt.message) {
				t.Fatalf("issue = %#v, want %s containing %q", issues[0], tt.priority, tt.message)
			}
		})
	}
}

func TestFrontmatterBody(t *testing.T) {
	data := []byte("---\nsummary: '# not body'\n---\n# actual body\n")
	if got := string(frontmatterBody(data)); got != "# actual body\n" {
		t.Fatalf("frontmatterBody() = %q, want body only", got)
	}
}

func TestLintDocChecksMarkdownWithoutLegacyFrontmatter(t *testing.T) {
	runInTempDir(t)
	path := "context/tmp/legacy.md"
	writeCtxDoc(t, path, "# legacy heading\n")
	issues := lintDoc(path)
	if len(issues) != 2 || issues[0].Message != "missing YAML frontmatter" || !strings.Contains(issues[1].Message, "markdown H1") {
		t.Fatalf("lintDoc() issues = %#v, want missing-frontmatter and H1 warnings", issues)
	}
}

func TestResolveContextLintPath(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/example.md", "---\nkind: plan\nsummary: example\n---\n")
	writeCtxDoc(t, "context/index.md", "---\nkind: index\nsummary: index\n---\n")
	writeCtxDoc(t, "context/tmp/scratch.md", "---\nkind: tmp\nsummary: scratch\n---\n")
	writeCtxDoc(t, "context/refs/external.md", "---\nkind: reference\nsummary: external\n---\n")
	writeCtxDoc(t, "context/wiki/page.md", "---\nkind: wiki\nsummary: wiki\nstatus: draft\n---\n")
	writeCtxDoc(t, "context/plan/nested/page.md", "---\nkind: plan\nsummary: nested\n---\n")

	absolute, err := filepath.Abs("context/plan/example.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"context/plan/example.md", "plan/example.md", absolute} {
		got, err := resolveContextLintPath(ref)
		if err != nil || got != "context/plan/example.md" {
			t.Errorf("resolveContextLintPath(%q) = %q, %v", ref, got, err)
		}
	}
	for _, ref := range []string{"index.md", "context/index.md"} {
		got, err := resolveContextLintPath(ref)
		if err != nil || got != sdtContextIndex {
			t.Errorf("resolveContextLintPath(%q) = %q, %v", ref, got, err)
		}
	}
	for _, ref := range []string{"../outside.md", "tmp/scratch.md", "refs/external.md", "wiki/page.md", "plan/nested/page.md"} {
		if got, err := resolveContextLintPath(ref); err == nil {
			t.Errorf("resolveContextLintPath(%q) = %q, want rejection", ref, got)
		}
	}
}

func TestContextLintSingleDocumentAndCorpusParity(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/plan/focus.md", "---\nkind: plan\nsummary: focus\n---\n# body heading\n")
	writeCtxDoc(t, "context/plan/other.md", "---\nkind: plan\nsummary: other\n---\n# unrelated heading\n")

	perDocOut := execute(t, contextLintCmd, nil, "context/plan/focus.md", "--format", "json")
	corpusOut := execute(t, contextLintCmd, nil, "--format", "json")
	var perDoc, corpusIssues []ctxLintIssue
	if err := json.Unmarshal(perDocOut, &perDoc); err != nil {
		t.Fatalf("invalid per-document JSON: %v\n%s", err, perDocOut)
	}
	if err := json.Unmarshal(corpusOut, &corpusIssues); err != nil {
		t.Fatalf("invalid corpus JSON: %v\n%s", err, corpusOut)
	}
	if len(perDoc) == 0 {
		t.Fatal("per-document lint returned no findings")
	}
	foundHeading := false
	for _, issue := range perDoc {
		if issue.Path != "context/plan/focus.md" {
			t.Fatalf("single-document lint included unrelated document: %#v", issue)
		}
		foundHeading = foundHeading || strings.Contains(issue.Message, "markdown H1")
	}
	if !foundHeading {
		t.Fatalf("per-document issues = %#v, want focus.md H1 finding", perDoc)
	}
	var matching []ctxLintIssue
	for _, issue := range corpusIssues {
		if issue.Path == "context/plan/focus.md" {
			matching = append(matching, issue)
		}
	}
	if len(matching) != len(perDoc) {
		t.Fatalf("corpus findings for focus.md = %#v, per-document = %#v", matching, perDoc)
	}
	used := make([]bool, len(matching))
	for _, got := range perDoc {
		found := false
		for i, want := range matching {
			if !used[i] && want.Priority == got.Priority && want.Message == got.Message {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("corpus findings for focus.md = %#v, per-document = %#v", matching, perDoc)
		}
	}

	multiOut := execute(t, contextLintCmd, nil, "plan/focus.md", "plan/other.md", "--format", "json")
	var multiple []ctxLintIssue
	if err := json.Unmarshal(multiOut, &multiple); err != nil {
		t.Fatalf("invalid multi-document JSON: %v\n%s", err, multiOut)
	}
	seen := map[string]bool{}
	for _, issue := range multiple {
		seen[issue.Path] = true
	}
	if !seen["context/plan/focus.md"] || !seen["context/plan/other.md"] {
		t.Fatalf("multi-document lint paths = %#v, want focus.md and other.md", seen)
	}
}

func TestContextLintRejectsOutsidePath(t *testing.T) {
	setupContextProject(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextLintCmd, nil, "../outside.md"))
	})
}
