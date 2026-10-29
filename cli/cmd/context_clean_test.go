//go:build !wasm

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cleanFixture puts one file in the trash and one in tmp, so every case can
// assert what happened to the target it did not name.
func cleanFixture(t *testing.T) {
	t.Helper()
	writeTestFile(t, filepath.Join(sdtTrashDir, "gone.md"), "discarded\n")
	writeTestFile(t, filepath.Join(sdtTmpDir, "scratch.txt"), "scratch\n")
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// TestCleanRefusesWithoutATarget is the safety property the whole verb rests on:
// an invocation that names no directory removes nothing, and the refusal happens
// before anything is read.
func TestCleanRefusesWithoutATarget(t *testing.T) {
	runInTempDir(t)
	cleanFixture(t)

	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCleanCmd, nil))
	})
	if out := errBuf.String(); !strings.Contains(out, "--trash") || !strings.Contains(out, "--tmp") {
		t.Errorf("the refusal must name both flags, got %q", out)
	}
	if n := countFiles(t, sdtTrashDir); n != 1 {
		t.Errorf("a refused clean removed %d file(s) from the trash", 1-n)
	}
	if n := countFiles(t, sdtTmpDir); n != 1 {
		t.Errorf("a refused clean removed %d file(s) from tmp", 1-n)
	}
}

// TestCleanEmptiesOnlyTheNamedTarget is the negative cell that a destructive verb
// needs: after `--trash`, tmp is asserted byte-for-byte intact, and the reverse.
func TestCleanEmptiesOnlyTheNamedTarget(t *testing.T) {
	runInTempDir(t)
	cleanFixture(t)
	tmpBefore := mustReadFile(t, filepath.Join(sdtTmpDir, "scratch.txt"))

	out := string(execute(t, contextCleanCmd, nil, "--trash"))
	if !strings.Contains(out, "removed 1 file(s)") {
		t.Errorf("the report must count what it removed: %q", out)
	}
	if n := countFiles(t, sdtTrashDir); n != 0 {
		t.Errorf("the trash still holds %d file(s)", n)
	}
	if got := mustReadFile(t, filepath.Join(sdtTmpDir, "scratch.txt")); got != tmpBefore {
		t.Error("--trash touched context/tmp")
	}
	// The directory itself stays: the next write must not have to recreate it.
	if _, err := os.Stat(sdtTrashDir); err != nil {
		t.Errorf("the emptied directory must survive: %v", err)
	}

	cleanFixture(t)
	execute(t, contextCleanCmd, nil, "--tmp")
	if n := countFiles(t, sdtTmpDir); n != 0 {
		t.Errorf("--tmp left %d file(s)", n)
	}
	if n := countFiles(t, sdtTrashDir); n == 0 {
		t.Error("--tmp emptied the trash as well")
	}
}

// TestCleanRemovesRecursively pins the depth: both targets are wholesale discard
// areas, so a nested artifact is still a discarded file.
func TestCleanRemovesRecursively(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, filepath.Join(sdtTrashDir, "a", "b", "deep.md"), "x\n")
	execute(t, contextCleanCmd, nil, "--trash")
	if n := countFiles(t, sdtTrashDir); n != 0 {
		t.Errorf("a nested file survived: %d left", n)
	}
}

// TestCleanDryRunRemovesNothing holds the preview to its name and to its
// usefulness: it lists exactly what a real run would remove.
func TestCleanDryRunRemovesNothing(t *testing.T) {
	runInTempDir(t)
	cleanFixture(t)
	out := execute(t, contextCleanCmd, nil, "--trash", "--dry-run", "--format", "json")
	var res ctxCleanResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out)
	}
	if len(res.Targets) != 1 || res.Targets[0].Count != 1 {
		t.Fatalf("dry-run plan = %+v, want one target with one file", res.Targets)
	}
	if !res.DryRun {
		t.Error("the result must say it was a preview")
	}
	if !strings.HasSuffix(res.Targets[0].Files[0], "gone.md") {
		t.Errorf("dry-run files = %v", res.Targets[0].Files)
	}
	if n := countFiles(t, sdtTrashDir); n != 1 {
		t.Errorf("dry-run removed %d file(s)", 1-n)
	}
}

// TestCleanIsIdempotentOnAMissingOrEmptyTarget makes a scripted clean safe to
// repeat: the second run reports nothing removed and exits 0.
func TestCleanIsIdempotentOnAMissingOrEmptyTarget(t *testing.T) {
	runInTempDir(t)
	cleanFixture(t)
	execute(t, contextCleanCmd, nil, "--trash")

	out := string(execute(t, contextCleanCmd, nil, "--trash"))
	if !strings.Contains(out, "removed 0 file(s)") {
		t.Errorf("a second run must report nothing removed: %q", out)
	}
	// A target that was never created is the same case, not an error.
	out = string(execute(t, contextCleanCmd, nil, "--tmp"))
	if !strings.Contains(out, "removed") {
		t.Errorf("a target with nothing in it must still be reported: %q", out)
	}
}

// TestCleanBothTargetsInOneRun covers the one invocation the flags allow
// together.
func TestCleanBothTargetsInOneRun(t *testing.T) {
	runInTempDir(t)
	cleanFixture(t)
	out := execute(t, contextCleanCmd, nil, "--trash", "--tmp", "--format", "json")
	var res ctxCleanResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Targets) != 2 {
		t.Fatalf("targets = %+v, want both", res.Targets)
	}
	if res.Targets[0].Dir != sdtTrashDir || res.Targets[1].Dir != sdtTmpDir {
		t.Errorf("target order = %s, %s", res.Targets[0].Dir, res.Targets[1].Dir)
	}
	for _, tg := range res.Targets {
		if tg.Count != 1 {
			t.Errorf("%s: count = %d, want 1", tg.Dir, tg.Count)
		}
	}
}

// ageFile moves a file's mtime back so a cutoff can select it.
func ageFile(t *testing.T, path string, age time.Duration) {
	t.Helper()
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

// TestCleanOlderThanSparesFreshWork: a cutoff narrows the sweep to the files it
// selects, and the fresh file the filter exists to spare survives.
func TestCleanOlderThanSparesFreshWork(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, filepath.Join(sdtTmpDir, "old.txt"), "old\n")
	writeTestFile(t, filepath.Join(sdtTmpDir, "fresh.txt"), "fresh\n")
	ageFile(t, filepath.Join(sdtTmpDir, "old.txt"), 40*24*time.Hour)

	out := string(execute(t, contextCleanCmd, nil, "--tmp", "--older-than", "30d", "--dry-run"))
	if !strings.Contains(out, "old.txt") || strings.Contains(out, "fresh.txt") {
		t.Fatalf("the cutoff did not narrow the plan:\n%s", out)
	}

	execute(t, contextCleanCmd, nil, "--tmp", "--older-than", "30d")
	if _, err := os.Stat(filepath.Join(sdtTmpDir, "old.txt")); !os.IsNotExist(err) {
		t.Errorf("the old file was not removed")
	}
	if _, err := os.Stat(filepath.Join(sdtTmpDir, "fresh.txt")); err != nil {
		t.Errorf("the fresh file was removed: %v", err)
	}
}

// TestCleanCensusReportsAnOrphanTask: a task file whose plan resolves to nothing
// is residue, and the census reports it without deleting it.
func TestCleanCensusReportsAnOrphanTask(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, filepath.Join(sdtTasksDir, "20260101-000000-gone-phase-1.md"),
		"---\nkind: tasks\nstatus: completed\nsources:\n  - plan/20260101-000000-gone.md\n---\n\n- [x] step\n")
	ageFile(t, filepath.Join(sdtTasksDir, "20260101-000000-gone-phase-1.md"), 40*24*time.Hour)

	out := string(execute(t, contextCleanCmd, nil, "--census", "--older-than", "30d"))
	if !strings.Contains(out, "orphan tasks") || !strings.Contains(out, "gone-phase-1") {
		t.Fatalf("the census missed the orphan task:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(sdtTasksDir, "20260101-000000-gone-phase-1.md")); err != nil {
		t.Errorf("the census deleted without --apply: %v", err)
	}

	execute(t, contextCleanCmd, nil, "--census", "--older-than", "30d", "--apply")
	if _, err := os.Stat(filepath.Join(sdtTasksDir, "20260101-000000-gone-phase-1.md")); !os.IsNotExist(err) {
		t.Errorf("--apply did not remove the orphan task")
	}
}

// TestCleanCensusLeavesARecord: an old analysis is history, never a candidate,
// even when it is older than every cutoff.
func TestCleanCensusLeavesARecord(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, filepath.Join(sdtAnalysisDir, "20200101-000000-old.md"),
		"---\nkind: analysis\nsummary: an old record\n---\n\n## Problem statement\n")
	ageFile(t, filepath.Join(sdtAnalysisDir, "20200101-000000-old.md"), 900*24*time.Hour)

	out := string(execute(t, contextCleanCmd, nil, "--census", "--older-than", "30d"))
	if strings.Contains(out, "20200101-000000-old") {
		t.Fatalf("a record was reported as residue:\n%s", out)
	}
}
