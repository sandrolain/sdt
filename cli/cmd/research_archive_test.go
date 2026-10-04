package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

func TestResearchArchivePreviewWritesNothing(t *testing.T) {
	root, runID := synthesizedRun(t)

	out := string(execute(t, researchArchiveCmd, nil, "--run", runID))
	if !strings.Contains(out, "Archive plan") {
		t.Errorf("preview output unexpected:\n%s", out)
	}
	refsDir := filepath.Join(root, "context", "refs")
	if _, err := os.Stat(refsDir); err == nil {
		entries, _ := os.ReadDir(refsDir)
		if len(entries) != 0 {
			t.Errorf("preview must not archive anything: %v", entries)
		}
	}
}

func TestResearchArchiveApplyRequiresConfirm(t *testing.T) {
	_, runID := synthesizedRun(t)
	if code := runArchive(t, "--run", runID, "--apply"); code == 0 {
		t.Fatal("--apply without --confirm must be refused")
	}
}

func TestResearchArchiveApplyWritesCapturesAndCitationPack(t *testing.T) {
	root, runID := synthesizedRun(t)
	r, _ := research.Load(root, runID)
	verified := r.Sources[0]

	execute(t, researchArchiveCmd, nil, "--run", runID, "--apply", "--confirm")

	rel := research.RefRelPath(r, verified)
	if _, err := os.Stat(filepath.Join(root, "context", filepath.FromSlash(rel))); err != nil {
		t.Errorf("archived capture missing at %s: %v", rel, err)
	}
	// The archive never writes a wiki page.
	if _, err := os.Stat(filepath.Join(root, "context", "wiki")); err == nil {
		t.Errorf("archive must not create context/wiki/")
	}
	// The run records its archive directory and checkpoint.
	reloaded, _ := research.Load(root, runID)
	if reloaded.ArchiveDir != research.ArchiveRefDir(r) {
		t.Errorf("archive_dir = %q, want %q", reloaded.ArchiveDir, research.ArchiveRefDir(r))
	}
	if reloaded.Checkpoint.Operation != "archive" {
		t.Errorf("checkpoint = %q, want archive", reloaded.Checkpoint.Operation)
	}
	// The applied output prints the citation pack.
	out := string(execute(t, researchArchiveCmd, nil, "--run", runID, "--format", "text", "--apply", "--confirm"))
	if !strings.Contains(out, rel+"@") {
		t.Errorf("citation pack must cite %s@<sha8>:\n%s", rel, out)
	}
}

func TestResearchArchiveRefusesUnverifiedOnly(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")
	// Deliberately not verified.
	if code := runArchive(t, "--run", runID, "--apply", "--confirm"); code == 0 {
		t.Fatal("archiving with no verified source must fail")
	}
}

func TestResearchArchiveRefusesForeignArchiveDir(t *testing.T) {
	root, runID := synthesizedRun(t)
	r, _ := research.Load(root, runID)
	// Plant a foreign directory at the run's target archive name.
	foreign := filepath.Join(root, "context", "refs", research.ArchiveDirName(r))
	if err := os.MkdirAll(foreign, 0o750); err != nil {
		t.Fatal(err)
	}
	if code := runArchive(t, "--run", runID, "--apply", "--confirm"); code == 0 {
		t.Fatal("a pre-existing foreign archive directory must fail the apply")
	}
	entries, _ := os.ReadDir(foreign)
	if len(entries) != 0 {
		t.Errorf("refusal must not write into the foreign directory, found %d entries", len(entries))
	}
}

// runArchive executes research archive and reports its exit code.
func runArchive(t *testing.T, args ...string) (exited int) {
	t.Helper()
	origExit := exit
	exit = func(code int) {
		exited = code
		panic("exit")
	}
	defer func() {
		exit = origExit
		if r := recover(); r != nil && r != "exit" {
			panic(r)
		}
	}()
	execute(t, researchArchiveCmd, nil, args...)
	return exited
}
