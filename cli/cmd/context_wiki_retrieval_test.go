package cmd

import (
	"strings"
	"testing"
)

// wikiRetrievalFixture writes one draft, one active and one archived concept
// page sharing a token, so each default can be read off the result set.
func wikiRetrievalFixture(t *testing.T) {
	t.Helper()
	runInTempDir(t)
	for _, tc := range []struct{ slug, status string }{
		{"draft", "draft"},
		{"live", "active"},
		{"old", "archived"},
	} {
		writeCtxDoc(t, "context/wiki/"+tc.slug+".md",
			"---\nkind: wiki\nid: "+tc.slug+"\ntitle: "+tc.slug+"\ntype: concept\nstatus: "+tc.status+"\nsummary: a "+tc.slug+" page\n---\n\n## Summary\n\nquasar retrieval\n")
	}
}

// TestContextSearchWikiDefaultsToDraftAndActive: a draft concept page is one
// being written, so a wiki search finds it without --all, while the corpus
// default (active only) and archived pages keep their behaviour.
func TestContextSearchWikiDefaultsToDraftAndActive(t *testing.T) {
	wikiRetrievalFixture(t)

	ctxSearchIndex = nil
	wiki := string(execute(t, contextSearchCmd, nil, "quasar", "--type", "wiki"))
	for _, want := range []string{"context/wiki/draft.md", "context/wiki/live.md"} {
		if !strings.Contains(wiki, want) {
			t.Errorf("wiki search misses %s:\n%s", want, wiki)
		}
	}
	if strings.Contains(wiki, "context/wiki/old.md") {
		t.Errorf("archived wiki page must stay out by default:\n%s", wiki)
	}

	ctxSearchIndex = nil
	all := string(execute(t, contextSearchCmd, nil, "quasar", "--type", "wiki", "--all"))
	if !strings.Contains(all, "context/wiki/old.md") {
		t.Errorf("--all should include the archived page:\n%s", all)
	}

	ctxSearchIndex = nil
	corpus := string(execute(t, contextSearchCmd, nil, "quasar"))
	if !strings.Contains(corpus, "context/wiki/live.md") {
		t.Errorf("the corpus search should still find an active wiki page:\n%s", corpus)
	}
	if strings.Contains(corpus, "context/wiki/draft.md") {
		t.Errorf("the corpus default must remain active-only:\n%s", corpus)
	}

	ctxSearchIndex = nil
	archived := string(execute(t, contextSearchCmd, nil, "quasar", "--type", "wiki", "--status", "archived"))
	if !strings.Contains(archived, "context/wiki/old.md") || strings.Contains(archived, "context/wiki/live.md") {
		t.Errorf("an explicit --status must win over the default:\n%s", archived)
	}
}

// TestContextListAcceptsWiki: the wiki lane has its own linters, not its own
// listing; `list` accepts the kind like every other, and prints paths only.
func TestContextListAcceptsWiki(t *testing.T) {
	wikiRetrievalFixture(t)
	out := string(execute(t, contextListCmd, nil, "--type", "wiki"))
	for _, want := range []string{"context/wiki/draft.md", "context/wiki/live.md", "context/wiki/old.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("list --type wiki misses %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "quasar") {
		t.Errorf("list prints paths, never summaries:\n%s", out)
	}
}
