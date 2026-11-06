package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// termsPage is a page carrying a `## Terms` section with the given entries.
func termsPage(entries ...string) *contextwiki.Page {
	body := "\n## Summary\n\nA page.\n\n## Claims\n\n" +
		"1. {#claim-1} <!-- c-aaaaaaaa --> a fact (refs/spec.md@abc:1)\n\n" +
		"## Terms\n\n" + strings.Join(entries, "\n") + "\n\n## Notes\n\nA note.\n"
	return &contextwiki.Page{Path: "context/wiki/architecture/alpha.md", ID: "architecture/alpha", Body: body}
}

// lintTermsOf runs the term rule over one page and returns its findings.
func lintTermsOf(p *contextwiki.Page) []string {
	b := &wikiIssueBuilder{}
	lintWikiTerms(b, []*contextwiki.Page{p})
	out := make([]string, 0, len(b.issues))
	for _, issue := range b.issues {
		out = append(out, issue.Message)
	}
	return out
}

// TestWikiLintTermsMechanicalHalf proves the rule's checkable half: an entry is a
// claim with an anchor, an id and a span; a page without the section is silent.
func TestWikiLintTermsMechanicalHalf(t *testing.T) {
	cases := []struct {
		name string
		page *contextwiki.Page
		want string
	}{
		{"claim-backed", termsPage("- {#claim-2} <!-- c-bbbbbbbb --> **Term** — a definition. (refs/spec.md@abc:2-3)"), ""},
		{"no anchor", termsPage("- **Term** — a definition. (refs/spec.md@abc:2-3)"), "no claim anchor"},
		{"no id", termsPage("- {#claim-2} **Term** — a definition. (refs/spec.md@abc:2-3)"), "no stable id"},
		{"no span", termsPage("- {#claim-2} <!-- c-bbbbbbbb --> **Term** — a definition."), "no source span"},
		{"no section", &contextwiki.Page{Path: "context/wiki/architecture/beta.md", ID: "architecture/beta",
			Body: "\n## Summary\n\nA page.\n\n## Claims\n\n1. {#claim-1} <!-- c-aaaaaaaa --> a fact (refs/spec.md@abc:1)\n"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(lintTermsOf(tc.page), "\n")
			switch {
			case tc.want == "" && got != "":
				t.Errorf("want no finding, got %s", got)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Errorf("findings do not mention %q: %s", tc.want, got)
			}
		})
	}
}
