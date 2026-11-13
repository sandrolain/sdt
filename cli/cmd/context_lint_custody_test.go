package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// custodySHA256 computes a fixture file's full SHA-256 hex independently of the
// production helper, so the expected pin is not tautological.
func custodySHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// custodySheet writes a source-sheet fixture under context/sources/. The
// caller's `extra` carries every non-default key, including `status`.
func custodySheet(t *testing.T, name, extra string) {
	t.Helper()
	writeTestFile(t, filepath.Join(sdtSourcesDir, name),
		"---\nkind: source\ntitle: t\nsummary: s\nurl: https://example.com/a\n"+extra+"\n---\nbody\n")
}

// TestLintCustodyCounts exercises all four counts on one fixture tree: a cited
// artifact with a sheet stays silent, one without a sheet is counted; a gone
// sheet whose capture_path still exists is counted; a sheet whose sha256
// disagrees with the file it pins is counted; a gone sheet without a restore
// command is counted.
func TestLintCustodyCounts(t *testing.T) {
	runInTempDir(t)

	// Two artifacts: one with a sheet, one cited but never sheeted.
	writeTestFile(t, "context/refs/captured/guide.md", "the captured bytes\n")
	writeTestFile(t, "context/refs/orphan/notes.md", "the orphaned bytes\n")
	writeTestFile(t, "context/refs/gone-live.md", "the sheet says gone, the file disagrees\n")
	writeTestFile(t, "context/refs/drifted.md", "the file a stale sheet pins\n")
	writeTestFile(t, "context/notes/citing.md",
		"---\nkind: notes\nsummary: s\nagent: opencode\n---\n"+
			"see refs/captured/guide.md@"+refSHA8(t, "context/refs/captured/guide.md")+":1 and refs/orphan/notes.md@"+refSHA8(t, "context/refs/orphan/notes.md")+":1\n")

	capturedSHA := custodySHA256(t, "context/refs/captured/guide.md")
	custodySheet(t, "captured.md", "status: used\ncapture_path: refs/captured/guide.md\nsha256: "+capturedSHA)
	// The gone transition whose file is still there (count 2) and no restore
	// recorded (count 4).
	custodySheet(t, "gone-live.md", "status: gone\ncapture_path: refs/gone-live.md")
	// A pin that disagrees with the file (count 3): sha256 of different bytes.
	custodySheet(t, "drifted.md", "status: used\ncapture_path: refs/drifted.md\nsha256: "+strings.Repeat("a", 64))
	// A gone sheet with a restore command: silent on count 4.
	custodySheet(t, "gone-ok.md", "status: gone\ncapture_path: refs/gone-ok.md\nrestore: re-crawl https://example.com/a")
	// A gone sheet whose capture_path is genuinely absent: silent on count 2.
	custodySheet(t, "gone-absent.md", "status: gone\ncapture_path: refs/gone-absent.md\nrestore: re-crawl https://example.com/b")

	report := lintCustodyReport()
	if report.CitedNoSheet != 1 {
		t.Errorf("CitedNoSheet = %d, want 1 (refs/orphan/notes.md has no sheet)", report.CitedNoSheet)
	}
	if report.GoneWithLiveFile != 1 {
		t.Errorf("GoneWithLiveFile = %d, want 1 (refs/gone-live.md exists under a gone sheet)", report.GoneWithLiveFile)
	}
	if report.DigestDisagreement != 1 {
		t.Errorf("DigestDisagreement = %d, want 1 (refs/drifted.md's bytes are not the pinned sha256)", report.DigestDisagreement)
	}
	if report.GoneNoRestore != 1 {
		t.Errorf("GoneNoRestore = %d, want 1 (gone-live.md has no restore)", report.GoneNoRestore)
	}
	if report.SheetsRead != 5 || report.DocsScanned == 0 {
		t.Errorf("cost fields drifted: SheetsRead=%d DocsScanned=%d", report.SheetsRead, report.DocsScanned)
	}
	if report.Duration == "" {
		t.Error("the report must state its own duration")
	}
}

// TestLintCustodyCoverageByDigest proves a moved artifact stays covered: a
// citation whose path is gone but whose cited digest matches a sheet's sha256
// is not counted as sheet-less, while a cited digest no sheet pins is.
func TestLintCustodyCoverageByDigest(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/refs/moved-now.md", "the moved artifact\n")
	writeTestFile(t, "context/refs/unpinned.md", "unpinned bytes\n")
	writeTestFile(t, "context/notes/citing.md",
		"---\nkind: notes\nsummary: s\nagent: opencode\n---\n"+
			"see refs/moved-before.md@"+refSHA8(t, "context/refs/moved-now.md")+":1 and refs/unpinned.md@"+refSHA8(t, "context/refs/unpinned.md")+":1\n")

	custodySheet(t, "moved.md", "status: used\ncapture_path: refs/moved-now.md\nsha256: "+custodySHA256(t, "context/refs/moved-now.md"))

	report := lintCustodyReport()
	if report.CitedNoSheet != 1 {
		t.Errorf("CitedNoSheet = %d, want 1: the moved artifact is covered through its sheet sha256, the unpinned one is not", report.CitedNoSheet)
	}
}

// TestLintCustodyIgnoresNonClaims proves the cited-artifact scan applies the
// same rules the citation lint does: code spans, fenced blocks and the
// unresolved `@sha256:` form never become counted artifacts.
func TestLintCustodyIgnoresNonClaims(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/notes/citing.md",
		"---\nkind: notes\nsummary: s\nagent: opencode\n---\n"+
			"quote `refs/example.md@deadbeef` as an example\n\n"+
			"```\nrefs/fenced.md@deadbeef:1\n```\n\n"+
			"planned form refs/planned.md@sha256:deadbeefcafe is unresolved\n")

	report := lintCustodyReport()
	if report.CitedNoSheet != 0 {
		t.Errorf("CitedNoSheet = %d, want 0: no non-claim citation is an artifact", report.CitedNoSheet)
	}
}

// TestLintCustodyFlagWiring proves the opt-in flag prints the summary on the
// lint command without turning the counts into findings (the baseline cannot
// absorb the class because the counts never enter the issue set).
func TestLintCustodyFlagWiring(t *testing.T) {
	setupContextProject(t)
	writeTestFile(t, "context/refs/live.md", "bytes\n")
	writeTestFile(t, "context/notes/citing.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\nsee refs/live.md@"+refSHA8(t, "context/refs/live.md")+":1\n")
	custodySheet(t, "live.md", "status: used\ncapture_path: refs/live.md\nsha256: "+custodySHA256(t, "context/refs/live.md"))

	out := string(execute(t, contextLintCmd, nil, "--custody"))
	if !strings.Contains(out, "[custody] cited artifacts without a sheet: 0") {
		t.Fatalf("custody summary missing from output: %s", out)
	}
	if !strings.Contains(out, "[custody] cost: ") {
		t.Fatalf("custody cost line missing from output: %s", out)
	}

	// Without the flag the summary is not printed.
	out = string(execute(t, contextLintCmd, nil))
	if strings.Contains(out, "[custody]") {
		t.Fatalf("custody summary printed without --custody: %s", out)
	}
}
