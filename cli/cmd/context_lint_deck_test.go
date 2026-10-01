package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasDeckIssue(issues []ctxLintIssue, sev, substr string) bool {
	for _, it := range issues {
		if it.Priority == sev && strings.Contains(it.Message, substr) {
			return true
		}
	}
	return false
}

func deckFixture(t *testing.T, name string) (path, content, body string) {
	t.Helper()
	path = filepath.Join("testdata", "decks", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	fm, b := splitFrontmatterForTest(string(data))
	return path, fm + b, b
}

// splitFrontmatterForTest is a local stand-in so the test does not depend on the
// lint pipeline helper's signature.
func splitFrontmatterForTest(content string) (string, string) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", content
	}
	seen := false
	for i, l := range lines {
		if strings.TrimSpace(l) == "---" {
			if !seen {
				seen = true
				continue
			}
			return strings.Join(lines[:i+1], ""), strings.Join(lines[i+1:], "")
		}
	}
	return content, ""
}

// TestLintDeckClean: the committed calibration deck raises no deck warning.
func TestLintDeckClean(t *testing.T) {
	path, content, body := deckFixture(t, "portable-subset.slide.md")
	issues := lintDeck(path, content, []byte(body))
	for _, it := range issues {
		if it.Priority == ctxLintWarning || it.Priority == ctxLintCritical {
			t.Fatalf("clean deck raised %s: %s", it.Priority, it.Message)
		}
	}
}

// TestLintDeckDefects: each committed defective deck raises its intended
// warning.
func TestLintDeckDefects(t *testing.T) {
	tests := []struct {
		fixture string
		want    string
	}{
		{"bad-separator.slide.md", "no blank line before it"},
		{"bad-directive.slide.md", "unknown directive"},
		{"no-slides.slide.md", "usually a document"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			path, content, body := deckFixture(t, tt.fixture)
			issues := lintDeck(path, content, []byte(body))
			if !hasDeckIssue(issues, ctxLintWarning, tt.want) && !hasDeckIssue(issues, ctxLintSuggestion, tt.want) {
				t.Fatalf("fixture %s: missing %q in %#v", tt.fixture, tt.want, issues)
			}
		})
	}
}

// TestLintDeckDirectiveKnown: a built-in directive (with or without the spot
// `_` prefix) and a frontmatter directive are both silent.
func TestLintDeckDirectiveKnown(t *testing.T) {
	body := "# a\n\n<!-- _class: lead -->\n\n<!-- paginate: true -->\n\n---\n\n# b\n"
	issues := lintDeck("x.slide.md", body, []byte(body))
	if hasDeckIssue(issues, ctxLintWarning, "unknown directive") {
		t.Fatalf("known directives should be silent: %#v", issues)
	}
}

// TestLintDeckAssetResolves: an existing relative asset is silent; a missing
// one warns.
func TestLintDeckAssetResolves(t *testing.T) {
	path, content, body := deckFixture(t, "portable-subset.slide.md")
	issues := lintDeck(path, content, []byte(body))
	if hasDeckIssue(issues, ctxLintWarning, "does not resolve") {
		t.Fatalf("the fixture's asset resolves: %#v", issues)
	}
	bad := "# a\n\n![w:100 cap](gone.png)\n\n---\n\n# b\n"
	issues = lintDeck("x.slide.md", bad, []byte(bad))
	if !hasDeckIssue(issues, ctxLintWarning, "does not resolve") {
		t.Fatalf("missing asset should warn: %#v", issues)
	}
}
