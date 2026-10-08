package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/sandrolain/sdt/internal/semantic"
)

// TestContextSnapshotBuildNoQuery covers the query-free build path: the verb
// accepts zero positional args, builds the snapshot over the current index and
// persists a header matching the model.
func TestContextSnapshotBuildNoQuery(t *testing.T) {
	dir := runInTempDir(t)
	probeSemanticModel(t)
	writeCtxDoc(t, "context/notes/a.md", "---\nkind: notes\nsummary: alpha\n---\n\nsnapshotbuildtoken\n")

	ctxSearchIndex = nil
	out := string(execute(t, contextSnapshotBuildCmd, nil, "--semantic-model", "BASE2M"))
	if !strings.Contains(out, "snapshot") || !strings.Contains(out, "1 sections") {
		t.Fatalf("build output = %q, want a snapshot line with 1 section", out)
	}
	s := semantic.LoadSnapshot(dir)
	if !s.Matches(semantic.ModelBase2M, semantic.RecipeVersion, mdindex.ManifestVersion) {
		t.Fatalf("snapshot header mismatch after build: %+v", s)
	}
	if len(s.SectionVectors) != 1 {
		t.Fatalf("snapshot must hold 1 section, got %d", len(s.SectionVectors))
	}
	if _, err := os.Stat(semantic.SnapshotPath(dir)); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
}

// TestContextSnapshotBuildUnknownModel covers the degrade path: an unknown model
// errors without writing a snapshot.
func TestContextSnapshotBuildUnknownModel(t *testing.T) {
	dir := runInTempDir(t)
	writeCtxDoc(t, "context/notes/a.md", "---\nkind: notes\nsummary: alpha\n---\n\nbody\n")

	ctxSearchIndex = nil
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextSnapshotBuildCmd, nil, "--semantic-model", "NOPE"))
	})
	if _, err := os.Stat(semantic.SnapshotPath(dir)); !os.IsNotExist(err) {
		t.Errorf("failed build must not write a snapshot: %v", err)
	}
}

// TestRelatedHintNamesBuildVerb guards the bug: the hint must name the command
// that can actually build the snapshot.
func TestRelatedHintNamesBuildVerb(t *testing.T) {
	if !strings.Contains(ctxRelatedHint, "sdt context snapshot build") {
		t.Fatalf("hint must name the build verb, got %q", ctxRelatedHint)
	}
}
