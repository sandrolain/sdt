//go:build !wasm

package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/corpus"
)

// TestNoSurfaceNamesTheRemovedGraveyard pins the merge of `context/deprecated`
// into the trash at every surface that used to name it. The directory itself is
// what the decision removed, so a search of the source would be the weaker test:
// what matters is that nothing generates, creates, verifies or ignores it any
// more, and that all four now say trash.
func TestNoSurfaceNamesTheRemovedGraveyard(t *testing.T) {
	runInTempDir(t)
	ensureWorkDirs(false)

	if _, err := os.Stat(sdtTrashDir); err != nil {
		t.Errorf("the trash is part of the layout and must be created with it: %v", err)
	}
	if _, err := os.Stat("context/deprecated"); !os.IsNotExist(err) {
		t.Errorf("context/deprecated must not exist (err=%v)", nil)
	}
	for _, d := range agentVerifyWorkDirs {
		if strings.Contains(d, "deprecated") {
			t.Errorf("the verify list still names %s", d)
		}
	}
	if !strings.Contains(strings.Join(agentVerifyWorkDirs, " "), sdtTrashDir) {
		t.Errorf("the verify list must name %s", sdtTrashDir)
	}
	for _, e := range gitIgnoreWorkEntries {
		if strings.Contains(e, "deprecated") {
			t.Errorf("the gitignore entries still name %s", e)
		}
	}
	if !strings.Contains(strings.Join(gitIgnoreWorkEntries, " "), sdtTrashDir) {
		t.Errorf("a bootstrapped project must ignore %s/", sdtTrashDir)
	}

	readme := sdtWorkReadmeContent()
	if strings.Contains(readme, "context/deprecated") {
		t.Error("the generated workspace README still names the removed graveyard")
	}
	if !strings.Contains(readme, "context/trash") {
		t.Error("the generated workspace README must name the trash")
	}
}

// TestTrashIsExcludedFromTheCorpus pins the served-knowledge side of the
// exclusion, which is the one that decides whether `context search` can answer
// with a discarded document.
func TestTrashIsExcludedFromTheCorpus(t *testing.T) {
	if !corpus.ExcludedPath("context/trash/notes/x.md") {
		t.Error("a trashed document must be excluded from the served corpus")
	}
	if !corpus.ExcludedPath(sdtTrashDir) {
		t.Error("the trash directory must be excluded")
	}
	if corpus.ExcludedPath("context/notes/x.md") {
		t.Error("a live document must stay served")
	}
}
