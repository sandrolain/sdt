package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func writeOrphanPage(t *testing.T, id, relations, body string) {
	t.Helper()
	fm := "---\nkind: wiki\nid: " + id + "\ntitle: " + id + "\ntype: concept\nstatus: active\nsummary: fixture\n"
	if relations != "" {
		fm += "relations: " + relations + "\n"
	}
	fm += "sources:\n  - refs/converted/book.md\n---\n\n## Summary\n\n" + body + "\n"
	writeCtxDoc(t, "context/wiki/"+id+".md", fm)
}

func TestWikiOrphansFindsOrphan(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryIndexPage)
	writeOrphanPage(t, "architecture/orphan", "", "No graph edge.")
	out := string(execute(t, contextWikiOrphansCmd, nil))
	if !strings.Contains(out, "architecture/orphan") || !strings.Contains(out, "no typed relation") {
		t.Fatalf("orphan not reported:\n%s", out)
	}
}

func TestWikiOrphansAcceptsRelationOrIndexLink(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryIndexPage)
	writeOrphanPage(t, "architecture/related", `{refers_to: "[[architecture/orphan|orphan]]"}`, "Related.")
	writeOrphanPage(t, "architecture/linked", "", "See [[sources/book|book]].")
	out := string(execute(t, contextWikiOrphansCmd, nil))
	if out != "" {
		t.Fatalf("linked/related pages were reported:\n%s", out)
	}
}

func TestWikiOrphansJSON(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/sources/book.md", inventoryIndexPage)
	writeOrphanPage(t, "architecture/orphan", "", "No graph edge.")
	out := execute(t, contextWikiOrphansCmd, nil, "--format", "json")
	var findings []orphanFinding
	if err := json.Unmarshal(out, &findings); err != nil {
		t.Fatalf("orphans JSON is not readable: %v\n%s", err, out)
	}
	if len(findings) != 1 || findings[0].Page != "wiki/architecture/orphan.md" {
		t.Errorf("unexpected findings: %+v", findings)
	}
}

func TestWikiAuditCountsOrphans(t *testing.T) {
	setupContextProject(t)
	writeCovRunFixture(t)
	writeOrphanPage(t, "architecture/orphan", "", "No graph edge.")
	out := string(execute(t, contextWikiAuditCmd, nil,
		"sources/book", "--artifact", "refs/converted/book-abcdef123456.md"))
	if !strings.Contains(out, "orphans: 1") {
		t.Fatalf("audit did not count the orphans:\n%s", out)
	}
}
