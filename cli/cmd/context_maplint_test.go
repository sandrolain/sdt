package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasMapIssue(issues []ctxLintIssue, sev, substr string) bool {
	for _, it := range issues {
		if it.Priority == sev && strings.Contains(it.Message, substr) {
			return true
		}
	}
	return false
}

func TestLintMapDocRoot(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string // expected warning substring; "" means none
	}{
		{"single H1 root", "---\nkind: notes\n---\n# root\n\n## a\n", ""},
		{"frontmatter title fallback", "---\ntitle: root\n---\n## a\n\n## b\n", ""},
		{"two H1 roots", "# root\n\n# second\n\n## a\n", "H1 roots"},
		{"no root", "## a\n\n## b\n", "no root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := lintMapDoc("x.map.md", tt.content, []byte(tt.content))
			got := hasMapIssue(issues, ctxLintWarning, tt.want)
			if tt.want == "" && got {
				t.Fatalf("unexpected root warning: %#v", issues)
			}
			if tt.want != "" && !got {
				t.Fatalf("missing root warning %q: %#v", tt.want, issues)
			}
		})
	}
}

func TestLintMapDocDroppedParagraph(t *testing.T) {
	// A plain document-level paragraph is dropped by the parser.
	issues := lintMapDoc("x.map.md", "# root\n\ntext that disappears\n", []byte("# root\n\ntext that disappears\n"))
	if !hasMapIssue(issues, ctxLintWarning, "dropped by the parser") {
		t.Fatalf("expected dropped-paragraph warning: %#v", issues)
	}
	// A paragraph inside a list item is a node; an image-only paragraph is a leaf.
	ok := "# root\n\n## a\n\n- item text\n\n![alt](img.png)\n"
	issues = lintMapDoc("x.map.md", ok, []byte(ok))
	if hasMapIssue(issues, ctxLintWarning, "dropped by the parser") {
		t.Fatalf("list-item/image paragraph should not warn: %#v", issues)
	}
}

func TestLintMapDocBlockquote(t *testing.T) {
	issues := lintMapDoc("x.map.md", "# root\n\n> quoted\n", []byte("# root\n\n> quoted\n"))
	if !hasMapIssue(issues, ctxLintWarning, "blockquote") {
		t.Fatalf("expected blockquote warning: %#v", issues)
	}
}

func TestLintMapDocBudgets(t *testing.T) {
	two := "# root\n\n## a\n\n### x\n\n### y\n\n## b\n\n### x\n\n### y\n"
	if !hasMapIssue(lintMapDoc("x.map.md", two, []byte(two)), ctxLintSuggestion, "main branches") {
		t.Fatal("expected branch-count suggestion for 2 branches")
	}
	deep := "# root\n\n## a\n\n### b\n\n#### c\n\n##### deep\n"
	if !hasMapIssue(lintMapDoc("x.map.md", deep, []byte(deep)), ctxLintSuggestion, "depth") {
		t.Fatal("expected depth suggestion")
	}
	thin := "# root\n\n## a\n\n## b\n\n## c\n"
	if !hasMapIssue(lintMapDoc("x.map.md", thin, []byte(thin)), ctxLintSuggestion, "children") {
		t.Fatal("expected children suggestion for empty branches")
	}
}

func TestLintMapDocOptions(t *testing.T) {
	bad := "---\ntitle: t\nmarkmap:\n  colorFreezeLevel: 2\n  bogus: 1\n---\n# t\n"
	if !hasMapIssue(lintMapDoc("x.map.md", bad, []byte("# t\n")), ctxLintWarning, "bogus") {
		t.Fatal("expected unknown markmap option warning")
	}
	good := "---\ntitle: t\nmarkmap:\n  colorFreezeLevel: 2\n  maxWidth: 260\n---\n# t\n"
	if hasMapIssue(lintMapDoc("x.map.md", good, []byte("# t\n")), ctxLintWarning, "markmap") {
		t.Fatal("known markmap options should not warn")
	}
}

func TestLintMapDocLinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "base.md"), []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := "# root\n\n## a\n\n- [[base.md#x]]\n- [[missing.md#y]]\n"
	issues := lintMapDoc(filepath.Join(dir, "base.map.md"), content, []byte(content))
	if !hasMapIssue(issues, ctxLintWarning, "missing.md") {
		t.Fatalf("expected unresolved-link warning: %#v", issues)
	}
	if hasMapIssue(issues, ctxLintWarning, "base.md") {
		t.Fatalf("resolvable link must not warn: %#v", issues)
	}
}

func TestLintMapDocMixing(t *testing.T) {
	mixed := "# root\n\n- loose\n\n## branch\n"
	if !hasMapIssue(lintMapDoc("x.map.md", mixed, []byte(mixed)), ctxLintWarning, "mixes a loose list") {
		t.Fatal("expected mixing warning")
	}
	ok := "# root\n\n## branch\n\n- a\n- b\n"
	if hasMapIssue(lintMapDoc("x.map.md", ok, []byte(ok)), ctxLintWarning, "mixes a loose list") {
		t.Fatalf("list under a branch is not mixing: %#v", ok)
	}
}

func TestLintMarkdownBodyMapH1Exception(t *testing.T) {
	body := []byte("# root\n\n## a\n")
	if got := lintMarkdownBody("x.map.md", body, true); len(got) != 0 {
		t.Fatalf("map H1 should be exempt, got %#v", got)
	}
	if got := lintMarkdownBody("x.md", body, false); len(got) != 1 || !strings.Contains(got[0].Message, "markdown H1") {
		t.Fatalf("non-map H1 should warn, got %#v", got)
	}
}
