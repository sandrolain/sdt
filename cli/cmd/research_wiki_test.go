package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

func TestResearchPopulateWikiPreviewWritesNothing(t *testing.T) {
	root, runID := synthesizedRun(t)
	wikiDir := filepath.Join(root, "context", "wiki")

	out := string(execute(t, researchPopulateWikiCmd, nil, "--run", runID, "--brief", "why"))
	if !strings.Contains(out, "Wiki promotion plan") {
		t.Errorf("preview output unexpected:\n%s", out)
	}
	if _, err := os.Stat(wikiDir); err == nil {
		entries, _ := os.ReadDir(wikiDir)
		for _, e := range entries {
			if strings.Contains(e.Name(), "a-") || strings.Contains(e.Name(), "b-") {
				t.Errorf("preview must not write a wiki page: %s", e.Name())
			}
		}
	}
}

func TestResearchPopulateWikiApplyRequiresConfirm(t *testing.T) {
	_, runID := synthesizedRun(t)
	if code := runPopulate(t, "--run", runID, "--brief", "why", "--apply"); code == 0 {
		t.Fatal("--apply without --confirm must be refused")
	}
}

func TestResearchPopulateWikiApplyWritesPage(t *testing.T) {
	root, runID := synthesizedRun(t)
	r, _ := research.Load(root, runID)
	verified := r.Sources[0]
	id := research.SourceSlug(verified.CanonicalURL)

	execute(t, researchPopulateWikiCmd, nil, "--run", runID, "--brief", "why", "--apply", "--confirm")

	path := research.WikiPagePath(root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("wiki page not written at %s: %v", path, err)
	}
	body := string(data)
	for _, want := range []string{"kind: wiki", "status: draft", "## Claims", "refs/"} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q:\n%s", want, body)
		}
	}
	// The claim cites the run's dated archive subpath, and the capture exists.
	wantRel := research.RefRelPath(r, verified)
	if !strings.Contains(body, wantRel) {
		t.Fatalf("claim must cite %q:\n%s", wantRel, body)
	}
	if _, err := os.Stat(filepath.Join(root, "context", filepath.FromSlash(wantRel))); err != nil {
		t.Errorf("cited refs capture missing at %s: %v", wantRel, err)
	}
	// The run records its archive directory.
	reloaded, _ := research.Load(root, runID)
	if reloaded.ArchiveDir != research.ArchiveRefDir(r) {
		t.Errorf("archive_dir = %q, want %q", reloaded.ArchiveDir, research.ArchiveRefDir(r))
	}
}

func TestResearchPopulateWikiRefusesUnverifiedOnly(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")
	// Deliberately not verified.
	if code := runPopulate(t, "--run", runID, "--brief", "why", "--apply", "--confirm"); code == 0 {
		t.Fatal("promoting with no verified source must fail")
	}
}

func TestResearchPopulateWikiRefusesForeignArchiveDir(t *testing.T) {
	root, runID := synthesizedRun(t)
	r, _ := research.Load(root, runID)
	// Plant a foreign directory at the run's target archive name.
	foreign := filepath.Join(root, "context", "refs", research.ArchiveDirName(r))
	if err := os.MkdirAll(foreign, 0o750); err != nil {
		t.Fatal(err)
	}
	if code := runPopulate(t, "--run", runID, "--brief", "why", "--apply", "--confirm"); code == 0 {
		t.Fatal("a pre-existing foreign archive directory must fail the apply")
	}
	entries, _ := os.ReadDir(foreign)
	if len(entries) != 0 {
		t.Errorf("refusal must not write into the foreign directory, found %d entries", len(entries))
	}
}

// runPopulate executes populate-wiki and reports its exit code.
func runPopulate(t *testing.T, args ...string) (exited int) {
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
	execute(t, researchPopulateWikiCmd, nil, args...)
	return exited
}
