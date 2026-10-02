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

func lint(t *testing.T, content string) []ctxLintIssue {
	t.Helper()
	return lintMapDoc("x.map.md", content, []byte(content))
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

func TestLintMapDocKeepsProseAndQuotes(t *testing.T) {
	// Paragraphs and blockquotes are nodes now, so neither may warn.
	prose := "# root\n\n## a\n\nA sentence that used to disappear.\n\n- item text\n\n![alt](img.png)\n"
	if issues := lint(t, prose); hasMapIssue(issues, ctxLintWarning, "dropped by the parser") {
		t.Fatalf("a paragraph is a node: %#v", issues)
	}
	quote := "# root\n\n## a\n\n> quoted\n"
	if issues := lint(t, quote); hasMapIssue(issues, ctxLintWarning, "blockquote") {
		t.Fatalf("a blockquote is a node: %#v", issues)
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

func TestLintMapDocMarkmapKeyIsInert(t *testing.T) {
	// The renderer options channel went away with markmap: the key itself is the
	// finding now, not its unknown sub-keys.
	content := "---\ntitle: t\nmarkmap:\n  colorFreezeLevel: 2\n---\n# t\n"
	issues := lintMapDoc("x.map.md", content, []byte("# t\n"))
	if !hasMapIssue(issues, ctxLintWarning, "`markmap:` options channel was removed") {
		t.Fatalf("expected inert markmap key warning: %#v", issues)
	}
	clean := "---\ntitle: t\n---\n# t\n\n## a\n"
	if hasMapIssue(lintMapDoc("x.map.md", clean, []byte(clean)), ctxLintWarning, "markmap") {
		t.Fatal("a map without the key must not warn")
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
	if !hasMapIssue(lint(t, mixed), ctxLintWarning, "mixes a loose list") {
		t.Fatal("expected mixing warning")
	}
	ok := "# root\n\n## branch\n\n- a\n- b\n"
	if hasMapIssue(lint(t, ok), ctxLintWarning, "mixes a loose list") {
		t.Fatal("list under a branch is not mixing")
	}
}

func TestLintMapDocRelationship(t *testing.T) {
	paired := "# root\n\n## a [1]\n\n## b [^1](Cool)\n"
	if issues := lint(t, paired); hasMapIssue(issues, ctxLintWarning, "relationship") {
		t.Fatalf("a paired relationship must not warn: %#v", issues)
	}
	orphanSource := "# root\n\n## a [1]\n"
	if !hasMapIssue(lint(t, orphanSource), ctxLintWarning, "no [^1] target") {
		t.Fatal("expected an unpaired source warning")
	}
	orphanTarget := "# root\n\n## a [^2]\n"
	if !hasMapIssue(lint(t, orphanTarget), ctxLintWarning, "no [2] source") {
		t.Fatal("expected an unpaired target warning")
	}
	duplicate := "# root\n\n## a [3]\n\n## b [3]\n\n## c [^3]\n"
	if !hasMapIssue(lint(t, duplicate), ctxLintWarning, "declared by 2 topics") {
		t.Fatal("expected a duplicate-source warning")
	}
}

func TestLintMapDocWrapTitles(t *testing.T) {
	withMembers := "# root\n\n## a [B1]\n\n## b [B1]\n\n[B1]: Wrap\n"
	if issues := lint(t, withMembers); hasMapIssue(issues, ctxLintWarning, "no member topic") {
		t.Fatalf("a title with members must not warn: %#v", issues)
	}
	empty := "# root\n\n## a\n\n[B1]: Wrap\n"
	if !hasMapIssue(lint(t, empty), ctxLintWarning, "no member topic") {
		t.Fatal("expected a title-without-members warning")
	}
	indented := "# root\n\n## a\n\n  [S1]: Sum\n"
	if !hasMapIssue(lint(t, indented), ctxLintWarning, "summary title") {
		t.Fatal("an indented summary title is still a declaration")
	}
}

func TestLintMapDocSummaryParents(t *testing.T) {
	// A summary wraps siblings; topics under different parents are a mistake.
	siblings := "# root\n\n## p\n\n### a [S1]\n\n### b [S1]\n\n[S1]: Sum\n"
	if issues := lint(t, siblings); hasMapIssue(issues, ctxLintWarning, "different parents") {
		t.Fatalf("sibling summary must not warn: %#v", issues)
	}
	split := "# root\n\n## p\n\n### a [S1]\n\n## q\n\n### b [S1]\n"
	if !hasMapIssue(lint(t, split), ctxLintWarning, "different parents") {
		t.Fatal("expected a summary-across-parents warning")
	}
}

func TestLintMapDocMarkerOnTitleLine(t *testing.T) {
	content := "# root\n\n## a [B1]\n\n## b [B1]\n\n[B1]: Wrap [1]\n\n## c [1]\n"
	issues := lint(t, content)
	if !hasMapIssue(issues, ctxLintWarning, "is dropped") {
		t.Fatalf("expected a dropped-marker warning: %#v", issues)
	}
}

func TestLintMapDocMarkerScanFollowsTheParser(t *testing.T) {
	// A nested item's marker belongs to the nested topic, not its parent, and a
	// title line is a declaration rather than a topic.
	content := "# root\n\n- parent [S1]\n  - child [1]\n- other [^1]\n[S1]: Sum\n"
	issues := lint(t, content)
	if hasMapIssue(issues, ctxLintWarning, "relationship") {
		t.Fatalf("a nested marker must resolve to its own topic: %#v", issues)
	}
	if hasMapIssue(issues, ctxLintWarning, "no member topic") {
		t.Fatalf("the summary has a member topic: %#v", issues)
	}
}

func TestLintMarkdownBodyMapH1Exception(t *testing.T) {
	body := []byte("# root\n\n## a\n")
	if got := lintMarkdownBody("x.map.md", body, true, false); len(got) != 0 {
		t.Fatalf("map H1 should be exempt, got %#v", got)
	}
	if got := lintMarkdownBody("x.md", body, false, false); len(got) != 1 || !strings.Contains(got[0].Message, "markdown H1") {
		t.Fatalf("non-map H1 should warn, got %#v", got)
	}
}
