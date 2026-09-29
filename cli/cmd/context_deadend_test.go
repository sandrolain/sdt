package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextNewNotesObjectiveAndNoteType(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "de",
		"--objective", "harness-improvements", "--note-type", "dead-end")
	content := mustReadFile(t, filepath.Join(dir, "context", "notes", "20260806-070000-de.md"))
	for _, want := range []string{"kind: notes", "objective: harness-improvements", "note_type: dead-end"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in notes frontmatter:\n%s", want, content)
		}
	}
}

func TestReindexGroupsDeadEndNotesUnderObjective(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: the initiative analysis\nobjective: obj-x\n---\nbody\n")
	writeCtxDoc(t, "context/notes/de.md", "---\nkind: notes\nsummary: tried X, failed\nobjective: obj-x\nnote_type: dead-end\n---\nbody\n")
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: ordinary note\n---\nbody\n")

	content, err := buildIndex()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "### obj-x") {
		t.Errorf("expected an obj-x objective bucket:\n%s", content)
	}
	if !strings.Contains(content, "**dead-end** tried X, failed") {
		t.Errorf("expected the dead-end marker in the objective bucket:\n%s", content)
	}
	if got := strings.Count(content, "notes/de.md"); got != 1 {
		t.Errorf("dead-end note must appear exactly once, found %d:\n%s", got, content)
	}
	if !strings.Contains(content, "notes/n.md") {
		t.Errorf("ordinary note should stay in the general list:\n%s", content)
	}
}

func TestContextStatusDeadEndRow(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/de.md", "---\nkind: notes\nsummary: dead end\nobjective: obj-x\nnote_type: dead-end\n---\nbody\n")

	var found *ctxStatusEntry
	for _, r := range ctxStatusRows() {
		if r.Type == "dead-ends" {
			row := r
			found = &row
		}
	}
	if found == nil {
		t.Fatal("expected a dead-ends status row")
	}
	if found.Count != 1 || !strings.Contains(found.Next, "obj-x (1)") {
		t.Errorf("unexpected dead-ends row: %+v", *found)
	}
}

func TestContextStatusPostponedRow(t *testing.T) {
	setupContextProject(t)
	postponed := "---\nkind: analysis\nsummary: s\nobjective: obj-x\nstatus: postponed\n---\nbody\n"
	writeCtxDoc(t, "context/analysis/p.md", postponed)
	writeCtxDoc(t, "context/analysis/q.md", strings.Replace(postponed, "obj-x", "obj-y", 1))
	// Non-postponed analyses and a postponed non-analysis never count.
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nobjective: obj-x\nstatus: active\n---\nbody\n")
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: s\nstatus: postponed\n---\nbody\n")

	var found *ctxStatusEntry
	for _, r := range ctxStatusRows() {
		if r.Type == "postponed" {
			row := r
			found = &row
		}
	}
	if found == nil {
		t.Fatal("expected a postponed status row")
	}
	if found.Count != 2 {
		t.Errorf("postponed count = %d, want 2: %+v", found.Count, *found)
	}
	// Objectives are sorted so the next-step hint is stable.
	if !strings.Contains(found.Next, "obj-x (1), obj-y (1)") {
		t.Errorf("postponed next step should list sorted objectives, got %+v", *found)
	}
	if !strings.Contains(found.Next, "revival condition") {
		t.Errorf("postponed next step should name the revival condition, got %+v", *found)
	}
}

func TestContextStatusPostponedRowHiddenWhenZero(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: completed\n---\nbody\n")

	for _, r := range ctxStatusRows() {
		if r.Type == "postponed" {
			t.Fatalf("no postponed analysis exists, row must stay hidden: %+v", r)
		}
	}
}
