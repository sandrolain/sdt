package cmd

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/semantic"
)

// probeSemanticModel skips the test when the small BASE2M model cannot load
// (offline with no cached model). The CLI tests pin --semantic-model BASE2M so
// they stay offline-friendly; the CLI default (BASE8M) is measured manually.
func probeSemanticModel(t *testing.T) {
	t.Helper()
	if _, err := semantic.New(context.Background(), semantic.ModelBase2M); err != nil {
		t.Skipf("BASE2M model unavailable (offline?): %v", err)
	}
}

func TestContextSearchSemanticSnapshotReuse(t *testing.T) {
	dir := runInTempDir(t)
	probeSemanticModel(t)

	body := "---\nkind: analysis\nsummary: s\nstatus: active\n---\n\n## Hybrid\n\nsemanticvetokendoc\n"
	writeCtxDoc(t, "context/analysis/a.md", body)

	// First run: `--semantic` emits the doc and persists the snapshot.
	ctxSearchIndex = nil
	out := string(execute(t, contextSearchCmd, nil, "semanticvetokendoc", "--semantic", "--semantic-model", "BASE2M"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("first semantic run:\n%s", out)
	}
	if st, err := os.Stat(semantic.SnapshotPath(dir)); err != nil || st.IsDir() {
		t.Fatalf("snapshot missing after first run: %v", err)
	}
	s := semantic.LoadSnapshot(dir)
	if !s.Matches(semantic.ModelBase2M, semantic.RecipeVersion, mdindex.ManifestVersion) {
		t.Fatalf("first-run snapshot header mismatch: %+v", s)
	}
	if len(s.SectionVectors) != 1 || len(s.DocHashes) != 1 {
		t.Fatalf("first-run snapshot must capture 1 section/1 doc, got %d/%d", len(s.SectionVectors), len(s.DocHashes))
	}
	firstHash := s.DocHashes["context/analysis/a.md"]
	if firstHash == "" {
		t.Fatal("first-run snapshot must record the doc content hash")
	}

	// Second run, unchanged corpus: the snapshot survives and stays reusable.
	ctxSearchIndex = nil
	out = string(execute(t, contextSearchCmd, nil, "semanticvetokendoc", "--semantic", "--semantic-model", "BASE2M"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("second semantic run:\n%s", out)
	}
	s = semantic.LoadSnapshot(dir)
	if !s.Matches(semantic.ModelBase2M, semantic.RecipeVersion, mdindex.ManifestVersion) {
		t.Fatalf("second-run snapshot header mismatch: %+v", s)
	}
	if s.DocHashes["context/analysis/a.md"] != firstHash {
		t.Error("unchanged corpus must keep the same doc hash in the snapshot")
	}

	// Third run, corpus changed: the snapshot is refreshed with the new hash.
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\n---\n\n## Hybrid\n\nsemanticvetokendocv2\n")
	ctxSearchIndex = nil
	out = string(execute(t, contextSearchCmd, nil, "semanticvetokendocv2", "--semantic", "--semantic-model", "BASE2M"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Fatalf("third semantic run:\n%s", out)
	}
	s = semantic.LoadSnapshot(dir)
	if s.DocHashes["context/analysis/a.md"] == firstHash {
		t.Error("changed corpus must update the doc hash in the snapshot")
	}
	if !reflect.DeepEqual(s.DocHashes, semantic.LoadSnapshot(dir).DocHashes) {
		t.Error("snapshot read-back must match the persisted state")
	}
}

func TestContextSearchSemanticDegradesCleanly(t *testing.T) {
	dir := runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\n---\n\n## Hybrid\n\nsingletoken\n")

	// Unknown model → explicit error, exactly as before Phase 4.
	ctxSearchIndex = nil
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextSearchCmd, nil, "singletoken", "--semantic", "--semantic-model", "NOPE"))
	})
	if _, err := os.Stat(semantic.SnapshotPath(dir)); !os.IsNotExist(err) {
		t.Errorf("failed semantic run must not write a partial snapshot: %v", err)
	}

	// Offline/absent model degrades to the exact same error without a snapshot.
	ctxSearchIndex = nil
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextSearchCmd, nil, "singletoken", "--semantic", "--semantic-model", "NOPE2"))
	})
	if _, err := os.Stat(semantic.SnapshotPath(dir)); !os.IsNotExist(err) {
		t.Errorf("second failed run must not write a partial snapshot: %v", err)
	}
}
