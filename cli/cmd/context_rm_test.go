//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRmMovesTheFileIntoTheTrash is the soft delete in three assertions: the file
// is gone from its directory, it is in the trash with identical bytes, and the
// generated index no longer names it.
func TestRmMovesTheFileIntoTheTrash(t *testing.T) {
	runInTempDir(t)
	const path = "context/notes/20260920-130000-scratch.md"
	fixtureDoc(t, path, "notes", "", "")
	before := mustReadFile(t, path)

	out := string(execute(t, contextRmCmd, nil, path))
	if !strings.Contains(out, "no document references it") {
		t.Errorf("the report must say nothing referenced it: %q", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the file must leave its directory (err=%v)", err)
	}
	moved := filepath.Join(sdtTrashDir, "notes", "20260920-130000-scratch.md")
	if after := mustReadFile(t, moved); after != before {
		t.Errorf("bytes changed in the move:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if idx, err := os.ReadFile(sdtContextIndex); err == nil && strings.Contains(string(idx), "scratch") {
		t.Error("the regenerated index still names the discarded document")
	}
}

// TestRmReportsBothReferenceShapes is the cell the rewriter would fail: the
// sanctioned `context set` writes an inline list, while older documents carry a
// block list, and a report must see both.
func TestRmReportsBothReferenceShapes(t *testing.T) {
	runInTempDir(t)
	const target = "context/notes/20260920-130000-target.md"
	fixtureDoc(t, target, "notes", "", "")
	// The shape `sdt context set links "['...']"` produces.
	const inline = "context/notes/20260920-130000-inline.md"
	fixtureDoc(t, inline, "notes", "", "links: ['notes/20260920-130000-target.md']\n")
	// The shape an older document carries.
	const block = "context/notes/20260920-130000-block.md"
	fixtureDoc(t, block, "notes", "", "links:\n  - notes/20260920-130000-target.md\n")
	// A path that merely begins the same way is not a reference.
	const decoy = "context/notes/20260920-130000-decoy.md"
	fixtureDoc(t, decoy, "notes", "", "links: ['notes/20260920-130000-target.md.bak']\n")

	out := execute(t, contextRmCmd, nil, target, "--format", "json")
	var res ctxRmResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out)
	}
	joined := strings.Join(res.Referrers, "\n")
	for _, want := range []string{inline, block} {
		if !strings.Contains(joined, want) {
			t.Errorf("%s names the discarded document and is missing from the report:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, decoy) {
		t.Errorf("a longer path that merely begins the same way is not a reference:\n%s", joined)
	}
}

// TestRmZeroReferrersIsReportedNotEmpty keeps the report unambiguous: an empty
// list says the scan ran and found nothing, which an absent field cannot say.
func TestRmZeroReferrersIsReportedNotEmpty(t *testing.T) {
	runInTempDir(t)
	const path = "context/notes/20260920-130000-lonely.md"
	fixtureDoc(t, path, "notes", "", "")
	out := execute(t, contextRmCmd, nil, path, "--format", "json")
	var res ctxRmResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	if res.Referrers == nil {
		t.Fatalf("referrers must be an empty list, not absent:\n%s", out)
	}
	if len(res.Referrers) != 0 {
		t.Errorf("referrers = %v, want none", res.Referrers)
	}
}

// TestRmDryRunMovesNothing holds the preview to its name.
func TestRmDryRunMovesNothing(t *testing.T) {
	runInTempDir(t)
	const path = "context/notes/20260920-130000-preview.md"
	fixtureDoc(t, path, "notes", "", "")
	before := mustReadFile(t, path)

	out := string(execute(t, contextRmCmd, nil, path, "--dry-run"))
	if !strings.Contains(out, "would move") {
		t.Errorf("dry-run must report the would-be move: %q", out)
	}
	if after := mustReadFile(t, path); after != before {
		t.Error("dry-run moved the file")
	}
	if _, err := os.Stat(sdtTrashDir); !os.IsNotExist(err) {
		t.Error("dry-run created the trash directory")
	}
}

// TestRmDoesNotOverwriteANameAlreadyInTheTrash guards the one way a discard could
// still lose data: a second file with the same relative path.
func TestRmDoesNotOverwriteANameAlreadyInTheTrash(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	const path = "context/notes/20260920-130000-twice.md"
	fixtureDoc(t, path, "notes", "", "")
	execute(t, contextRmCmd, nil, path)

	fixtureDoc(t, path, "notes", "", "extra: second\n")
	execute(t, contextRmCmd, nil, path)

	entries, err := os.ReadDir(filepath.Join(sdtTrashDir, "notes"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("both discarded files must survive, got %v", names)
	}
	var stamped string
	for _, e := range entries {
		if strings.Contains(e.Name(), "20261015-120000") {
			stamped = e.Name()
		}
	}
	if stamped == "" {
		t.Errorf("the collision must land with a timestamp: %v", entries)
	}
}

// TestRemovableDocumentRefusesAPathOutsideTheCorpus keeps the destination
// computation from inventing one.
func TestRemovableDocumentRefusesAPathOutsideTheCorpus(t *testing.T) {
	if _, err := ctxTrashDestination("plan/20260101-000000-x.md"); err == nil {
		t.Error("a path outside the corpus must be refused")
	}
	dst, err := ctxTrashDestination("context/notes/x.md")
	if err != nil || dst != filepath.Join(sdtTrashDir, "notes", "x.md") {
		t.Errorf("destination = %q err=%v, want the trash with the relative path kept", dst, err)
	}
}
