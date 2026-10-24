package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func comprehensionPage(id, artifact string, claims int) string {
	body := "---\nkind: wiki\nid: " + id + "\ntitle: " + id + "\ntype: concept\nstatus: active\nsummary: fixture\nsources:\n  - " + artifact + "\n---\n\n## Claims\n\n"
	for i := 1; i <= claims; i++ {
		body += "1. {#claim-" + string(rune('0'+i)) + "} Claim " + string(rune('0'+i)) + " (" + artifact + "@abcdef12:1-2)\n"
	}
	return body
}

func writeComprehensionFixture(t *testing.T, pagesPerBook int) {
	t.Helper()
	writeCtxDoc(t, "context/wiki/sources/book1.md", "---\nkind: wiki\nid: sources/book1\ntitle: Book 1\ntype: entity\nstatus: active\nsummary: one\nsources:\n  - refs/converted/book1.md\n---\n\n## Summary\n")
	writeCtxDoc(t, "context/wiki/sources/book2.md", "---\nkind: wiki\nid: sources/book2\ntitle: Book 2\ntype: entity\nstatus: active\nsummary: two\nsources:\n  - refs/converted/book2.md\n---\n\n## Summary\n")
	for i := 1; i <= pagesPerBook; i++ {
		writeCtxDoc(t, "context/wiki/architecture/a"+string(rune('0'+i))+".md", comprehensionPage("architecture/a"+string(rune('0'+i)), "refs/converted/book1.md", i))
		writeCtxDoc(t, "context/wiki/architecture/b"+string(rune('0'+i))+".md", comprehensionPage("architecture/b"+string(rune('0'+i)), "refs/converted/book2.md", i))
	}
}

func TestWikiComprehensionTwoBooks(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeComprehensionFixture(t, 2)
	out := execute(t, contextWikiComprehensionCmd, nil, "--format", "json")
	var samples []comprehensionSample
	if err := json.Unmarshal(out, &samples); err != nil {
		t.Fatalf("comprehension JSON is not readable: %v\n%s", err, out)
	}
	if len(samples) != 2 {
		t.Fatalf("samples = %d, want 2:\n%s", len(samples), out)
	}
	for _, s := range samples {
		if s.Dense == "" || s.Boundary == "" || s.DenseClaims < s.BoundaryClaims {
			t.Errorf("bad sample: %+v", s)
		}
	}
}

func TestWikiComprehensionOnePagePerBook(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeComprehensionFixture(t, 1)
	out := string(execute(t, contextWikiComprehensionCmd, nil, "--book", "sources/book1"))
	if !strings.Contains(out, "dense: architecture/a1") || !strings.Contains(out, "boundary: -") {
		t.Fatalf("one-page book was not handled:\n%s", out)
	}
}

func TestWikiComprehensionEmpty(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	out := string(execute(t, contextWikiComprehensionCmd, nil))
	if out != "" {
		t.Fatalf("empty corpus produced output:\n%s", out)
	}
}
