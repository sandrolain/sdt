//go:build !wasm

package cmd

import (
	"strings"
	"testing"
)

// The four cases below are the pinned twin of web/src/lib/slidesSource.test.ts:
// the Go normalizeDeckSource must keep the viewer's rule exactly, or the deck a
// viewer renders and the deck material status judges diverge. A change here is
// a change there, in the same commit.

func TestDeckNormalizeMatchesSlidesSourceTwin(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"drops a leading material header before the deck frontmatter",
			"<!-- material: concept=wiki/x; claims=c-1@abc -->\n\n---\ntheme: default\npaginate: true\n---\n\n# One\n",
			"---\ntheme: default\npaginate: true\n---\n\n# One\n",
		},
		{
			"leaves a leading speaker note before a separator alone",
			"<!-- a note -->\n\n# One\n\n---\n\n# Two\n",
			"<!-- a note -->\n\n# One\n\n---\n\n# Two\n",
		},
		{
			// `---` followed by a blank line is a slide separator, not a
			// frontmatter block; the leading note must stay (it is slide 1's
			// note).
			"does not mistake a separator for frontmatter",
			"<!-- a note -->\n\n---\n\n# One\n",
			"<!-- a note -->\n\n---\n\n# One\n",
		},
		{
			"leaves corpus frontmatter at the start alone",
			"---\nkind: notes\n---\n\n<!-- a note -->\n\n# One\n",
			"---\nkind: notes\n---\n\n<!-- a note -->\n\n# One\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDeckSource(tc.source); got != tc.want {
				t.Errorf("normalizeDeckSource:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestDeckStructureIssue pins the structural rules a render needs (plan D6),
// read on the normalized source the way the viewer renders the deck: the
// frontmatter first, a section after it, and no frontmatter key leaking into
// a slide as text.
func TestDeckStructureIssue(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"frontmatter first",
			"---\ntheme: default\npaginate: true\n---\n\n# One\n\n---\n\n# Two\n",
			"",
		},
		{
			// The material deck shape (decision/0038): the machine header
			// comment first, the frontmatter after it; normalizeDeckSource is
			// the viewer's rule and the deck renders.
			"header before the frontmatter is the viewer's rescue",
			"<!-- material: concept=wiki/x; claims=c-1@abc -->\n\n---\ntheme: default\npaginate: true\n---\n\n# One\n",
			"",
		},
		{
			"no frontmatter at all",
			"# One\n\n---\n\n# Two\n",
			"the frontmatter is not the first block",
		},
		{
			// The displacement a rescue cannot fix: the comment is followed by
			// a separator, not the frontmatter, so the keys are slide text and
			// no frontmatter opens the document.
			"frontmatter displaced beyond the header",
			"<!-- material: concept=wiki/x; claims=c-1@abc -->\n\n---\n\ntheme: default\npaginate: true\n---\n\n# One\n",
			"the frontmatter is not the first block",
		},
		{
			"frontmatter never closes",
			"---\ntheme: default\npaginate: true\n\n# One\n",
			"the frontmatter block never closes",
		},
		{
			"no section after the frontmatter",
			"---\ntheme: default\n---\n",
			"followed by no slide section",
		},
		{
			"frontmatter key as slide text",
			"---\ntheme: default\n---\n\n# One\n\n---\n\npaginate: true\n",
			"the frontmatter key paginate: appears as slide text",
		},
		{
			"size key as slide text",
			"---\ntheme: default\n---\n\n# One\n\n---\n\nsize: 16:9\n",
			"the frontmatter key size: appears as slide text",
		},
		{
			// A deck whose frontmatter opens the file passes even with a
			// directive-looking prose line that only mentions a key mid-sentence.
			"prose mentioning a key mid-line passes",
			"---\ntheme: default\n---\n\n# One\n\nThe theme: default line above is prose.\n",
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deckStructureIssue(tc.source)
			if tc.want == "" {
				if got != "" {
					t.Errorf("deckStructureIssue = %q, want no issue", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("deckStructureIssue = %q, want mention of %q", got, tc.want)
			}
		})
	}
}
