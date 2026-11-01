package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// writeRenderPage writes one wiki page with a known `updated`, the stamp a render
// is compared with.
func writeRenderPage(t *testing.T, id, updated string) {
	t.Helper()
	fm := "---\nkind: wiki\nid: " + id + "\ntitle: " + id + "\ntype: concept\nstatus: active\n" +
		"summary: fixture\n" +
		"created: 2026-10-01T00:00:00Z\n" +
		"updated: " + updated + "\n" +
		"project: p\n---\n\n## Summary\n\nA page.\n"
	writeCtxDoc(t, "context/wiki/"+id+".md", fm)
}

// writeRender writes one render file with the given header line.
func writeRender(t *testing.T, id, header string) {
	t.Helper()
	writeCtxDoc(t, "context/renders/technical/"+id+".md", header+"\n\n## Definition\n\nDense.\n")
}

// renderRows decodes the command's JSON output.
func renderRows(t *testing.T, out []byte) []renderRow {
	t.Helper()
	var rows []renderRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("render report is not readable: %v\n%s", err, out)
	}
	return rows
}

// TestWikiRenderStatusVocabulary binds the four words: the report's own statuses
// are the contract a caller gates on.
func TestWikiRenderStatusVocabulary(t *testing.T) {
	want := []string{renderFresh, renderStale, renderMissing, renderOrphan}
	if got := renderStatuses(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("status vocabulary = %v, want %v", got, want)
	}
}

func TestWikiRenderFreshAndStale(t *testing.T) {	setupContextProject(t)
	stubWikiExit(t)
	writeRenderPage(t, "architecture/alpha", "2026-10-20T10:00:00Z")
	writeRenderPage(t, "architecture/beta", "2026-10-21T10:00:00Z")
	writeRender(t, "architecture/alpha", renderHeader("architecture/alpha", "2026-10-20T10:00:00Z"))
	writeRender(t, "architecture/beta", renderHeader("architecture/beta", "2026-10-01T10:00:00Z"))

	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json"))
	if len(rows) != 2 {
		t.Fatalf("want two renders, got %+v", rows)
	}
	if rows[0].Page != "architecture/alpha" || rows[0].Status != renderFresh {
		t.Errorf("alpha must be fresh, got %+v", rows[0])
	}
	if rows[1].Status != renderStale || rows[1].Updated != "2026-10-21T10:00:00Z" {
		t.Errorf("a moved page must report stale with its current stamp, got %+v", rows[1])
	}

	out := string(execute(t, contextWikiRenderCmd, nil))
	if !strings.Contains(out, "fresh architecture/alpha") ||
		!strings.Contains(out, "stale architecture/beta (2026-10-01T10:00:00Z → 2026-10-21T10:00:00Z)") {
		t.Errorf("text report lost a status or a stamp:\n%s", out)
	}
}

func TestWikiRenderSubpathID(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeRenderPage(t, "backend/auth", "2026-10-20T10:00:00Z")
	writeRender(t, "backend/auth", renderHeader("backend/auth", "2026-10-20T10:00:00Z"))

	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil, "--format", "json"))
	if len(rows) != 1 || rows[0].Page != "backend/auth" || rows[0].Status != renderFresh {
		t.Fatalf("a slashed page id must round-trip through the path, got %+v", rows)
	}
	if rows[0].Path != "context/renders/technical/backend/auth.md" {
		t.Errorf("path = %q", rows[0].Path)
	}
}

func TestWikiRenderMissingOrphanAndUnreadableHeader(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeRenderPage(t, "architecture/alpha", "2026-10-20T10:00:00Z")
	writeRenderPage(t, "architecture/beta", "2026-10-21T10:00:00Z")
	// A render whose page is gone.
	writeRender(t, "architecture/gone", renderHeader("architecture/gone", "2026-10-01T10:00:00Z"))
	// A render that cannot prove its freshness.
	writeRender(t, "architecture/beta", "# Dense\n\nNo header.")

	rows := renderRows(t, execute(t, contextWikiRenderCmd, nil,
		"architecture/alpha", "--format", "json"))
	byPage := map[string]renderRow{}
	for _, r := range rows {
		byPage[r.Page] = r
	}
	if byPage["architecture/alpha"].Status != renderMissing {
		t.Errorf("a page named with no render must report missing, got %+v", byPage["architecture/alpha"])
	}
	if byPage["architecture/gone"].Status != renderOrphan {
		t.Errorf("a render with no page must report orphan, got %+v", byPage["architecture/gone"])
	}
	if byPage["architecture/beta"].Status != renderStale || byPage["architecture/beta"].Stamp != "" {
		t.Errorf("a header that cannot be read is not fresh, got %+v", byPage["architecture/beta"])
	}

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiRenderCmd, nil, "--check"))
	})
}

func TestWikiRenderCheckGates(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeRenderPage(t, "architecture/alpha", "2026-10-20T10:00:00Z")
	writeRender(t, "architecture/alpha", renderHeader("architecture/alpha", "2026-10-20T10:00:00Z"))

	// A fresh set passes the gate, and the report alone never gates.
	execute(t, contextWikiRenderCmd, nil, "--check")
	out := string(execute(t, contextWikiRenderCmd, nil))

	writeRenderPage(t, "architecture/alpha", "2026-10-22T10:00:00Z")
	if got := string(execute(t, contextWikiRenderCmd, nil)); !strings.Contains(got, "stale") {
		t.Errorf("the report must still print a stale render, got:\n%s", got)
	}
	_ = out
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiRenderCmd, nil, "--check"))
	})
}

func TestWikiRenderRejectsUnknownPage(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiRenderCmd, nil, "architecture/invented"))
	})
}

func TestWikiRenderNoRendersIsHonest(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeRenderPage(t, "architecture/alpha", "2026-10-20T10:00:00Z")
	out := string(execute(t, contextWikiRenderCmd, nil))
	if !strings.Contains(out, "no renders under") {
		t.Errorf("an empty lane must say so, got %q", out)
	}
	// And the gate passes: nothing to quote, nothing stale.
	execute(t, contextWikiRenderCmd, nil, "--check")
}
