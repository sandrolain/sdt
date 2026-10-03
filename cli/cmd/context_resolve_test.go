package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeResolveCandidate writes a minimal analysis document with the given
// status/updated so the ranking can be exercised deterministically.
func writeResolveCandidate(t *testing.T, slug, status, updated string) {
	t.Helper()
	writeTestFile(t, filepath.Join(sdtAnalysisDir, slug+".md"), `---
kind: analysis
title: "`+slug+`"
summary: "s"
status: `+status+`
updated: "`+updated+`"
project: p
---
`)
}

// TestContextResolveRanksByStatusThenRecency locks the ranking: a --status
// match sorts first, then most-recently updated, with a stable path tie-break.
func TestContextResolveRanksByStatusThenRecency(t *testing.T) {
	runInTempDir(t)
	writeResolveCandidate(t, "20260101-000000-old-active", "active", "2026-01-01T00:00:00Z")
	writeResolveCandidate(t, "20260201-000000-new-active", "active", "2026-02-01T00:00:00Z")
	writeResolveCandidate(t, "20260301-000000-draft", "draft", "2026-03-01T00:00:00Z")

	cands, err := resolveCandidates(mustType(t, ctxTypeAnalysis), "active")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 3 {
		t.Fatalf("got %d candidates, want 3", len(cands))
	}
	// Both active first (newest first), then the draft despite its fresher date.
	got := []string{cands[0].Status, cands[1].Status, cands[2].Status}
	if got[0] != "active" || got[1] != "active" || got[2] != "draft" {
		t.Errorf("ranking = %v, want active, active, draft", got)
	}
	if !strings.Contains(cands[0].Path, "new-active") {
		t.Errorf("first candidate = %q, want the newer active one", cands[0].Path)
	}
}

// TestContextResolveEmptyAndUnknown checks the empty-result and the invalid
// type/status paths.
func TestContextResolveEmptyAndUnknown(t *testing.T) {
	runInTempDir(t)
	cands, err := resolveCandidates(mustType(t, ctxTypePlan), "active")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 0 {
		t.Errorf("empty dir returned %d candidates", len(cands))
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextResolveCmd, nil, "--type", "nope"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextResolveCmd, nil, "--type", "analysis", "--status", "bogus"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextResolveCmd, nil, "--type", "worklog", "--status", "active"))
	})
}

// TestContextResolveJSONShape checks the --format json projection.
func TestContextResolveJSONShape(t *testing.T) {
	runInTempDir(t)
	writeResolveCandidate(t, "20260101-000000-one", "active", "2026-01-01T00:00:00Z")
	out := execute(t, contextResolveCmd, nil, "--type", "analysis", "--status", "active", "--format", "json")
	var cands []ctxResolveCandidate
	if err := json.Unmarshal(out, &cands); err != nil {
		t.Fatalf("json output not parseable: %v\n%s", err, out)
	}
	if len(cands) != 1 || cands[0].Status != "active" || cands[0].Type != ctxTypeAnalysis {
		t.Errorf("unexpected candidates: %+v", cands)
	}
}

// TestContextResolveWritesNothing is the read-only guard: resolve must not
// create, remove or modify any file, nor leave a cache entry.
func TestContextResolveWritesNothing(t *testing.T) {
	dir := runInTempDir(t)
	writeResolveCandidate(t, "20260101-000000-one", "active", "2026-01-01T00:00:00Z")

	before := snapshotFiles(t, dir)
	execute(t, contextResolveCmd, nil, "--type", "analysis", "--status", "active")
	after := snapshotFiles(t, dir)
	if before != after {
		t.Errorf("resolve changed the filesystem:\nbefore=%s\nafter=%s", before, after)
	}
	if _, err := os.Stat(".sdt/cache"); !os.IsNotExist(err) {
		t.Errorf("resolve wrote a cache entry: %v", err)
	}
}

// mustType resolves a ctxDocType or fails the test.
func mustType(t *testing.T, kind string) ctxDocType {
	t.Helper()
	d, ok := ctxTypeLookup(kind)
	if !ok {
		t.Fatalf("unknown type %q", kind)
	}
	return d
}

// snapshotFiles returns a stable listing of every path+size under dir, used to
// prove a command did not touch the tree.
func snapshotFiles(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		b.WriteString(path)
		b.WriteByte('|')
		b.WriteString(info.ModTime().Format("20060102T150405.000000000"))
		b.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
