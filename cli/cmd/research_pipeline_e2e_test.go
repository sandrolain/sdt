package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

// TestResearchPipelineEndToEnd is the whole-pipeline acceptance check: it drives
// every verb offline (a fake search provider and an httptest fetch server) and
// asserts the recorded state end to end.
func TestResearchPipelineEndToEnd(t *testing.T) {
	root := researchTestProject(t)
	ts := pageServer(t)

	// 1. search → discovered (fake provider, no network).
	f := &fakeProvider{
		results: []research.SearchResult{
			{URL: ts.URL + "/a", Title: "Page A"},
			{URL: ts.URL + "/b", Title: "Page B"},
		},
		remaining: 50,
	}
	withFakeProvider(t, f)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "why", "--objective", "pipeline-e2e")))
	execute(t, researchSearchCmd, nil, "--run", runID, "--limit", "5", "--network", "--confirm")

	r, _ := research.Load(root, runID)
	if r.CountByStatus(research.StatusDiscovered) != 2 {
		t.Fatalf("after search, want 2 discovered: %+v", r.Sources)
	}
	if r.Budget.Results != 2 {
		t.Errorf("budget results = %d, want 2", r.Budget.Results)
	}

	// 2. fetch → fetched with provenance.
	execute(t, researchFetchCmd, nil, "--run", runID)
	r, _ = research.Load(root, runID)
	if r.CountByStatus(research.StatusFetched) != 2 {
		t.Fatalf("after fetch, want 2 fetched: %+v", r.Sources)
	}
	for _, s := range r.Sources {
		if s.SHA256 == "" || s.RawPath == "" {
			t.Errorf("source missing provenance: %+v", s)
		}
	}
	// Fetch writes only under the run's tmp raw/ — never into context/refs/.
	refsDir := filepath.Join(root, "context", "refs")
	if entries, err := os.ReadDir(refsDir); err == nil {
		for _, e := range entries {
			t.Errorf("fetch must not create anything under context/refs/: %s", e.Name())
		}
	}

	// 3. verify → verified, checkpoint recorded.
	execute(t, researchVerifyCmd, nil, "--run", runID)
	r, _ = research.Load(root, runID)
	if r.CountByStatus(research.StatusVerified) != 2 {
		t.Fatalf("after verify, want 2 verified: %+v", r.Sources)
	}
	if r.Checkpoint.Operation != "verify" {
		t.Errorf("checkpoint = %q", r.Checkpoint.Operation)
	}

	// 4. resume → idempotent, nothing refetched.
	before := map[string]string{}
	for _, s := range r.Sources {
		before[s.CanonicalURL] = s.SHA256
	}
	execute(t, researchResumeCmd, nil, "--run", runID)
	r, _ = research.Load(root, runID)
	if len(r.Sources) != 2 {
		t.Fatalf("resume changed the source count: %+v", r.Sources)
	}
	for _, s := range r.Sources {
		if before[s.CanonicalURL] != s.SHA256 {
			t.Errorf("resume refetched %s", s.CanonicalURL)
		}
	}

	// 5. synthesize → cited draft, stdout, no write.
	out := string(execute(t, researchSynthesizeCmd, nil, "--run", runID))
	if !strings.Contains(out, "2 cited source(s)") {
		t.Errorf("synthesize should cite both sources:\n%s", out)
	}

	// 6. populate-wiki brief → citation pack, read-only (no page written).
	briefOut := string(execute(t, researchPopulateWikiCmd, nil, "--run", runID, "--brief", "why"))
	if !strings.Contains(briefOut, "verified sources: 2") {
		t.Errorf("populate-wiki brief should list 2 verified sources:\n%s", briefOut)
	}
	if _, err := os.Stat(filepath.Join(root, "context", "wiki")); err == nil {
		t.Errorf("populate-wiki must not write a wiki page")
	}

	// 7. research archive apply → dated archive directory, <result>.md files and
	// the citation pack; no wiki page is written by any verb.
	r, _ = research.Load(root, runID)
	archiveDir := research.ArchiveRefDir(r)
	execOut := string(execute(t, researchArchiveCmd, nil, "--run", runID, "--apply", "--confirm"))

	r, _ = research.Load(root, runID)
	if r.ArchiveDir != archiveDir {
		t.Errorf("run archive_dir = %q, want %q", r.ArchiveDir, archiveDir)
	}
	absDir := filepath.Join(root, "context", filepath.FromSlash(archiveDir))
	files, err := os.ReadDir(absDir)
	if err != nil {
		t.Fatalf("dated archive directory missing at %s: %v", absDir, err)
	}
	if len(files) != 2 {
		t.Errorf("archive dir should hold 2 captures, got %d", len(files))
	}
	for _, s := range r.Sources {
		rel := research.RefRelPath(r, s)
		if _, err := os.Stat(filepath.Join(root, "context", filepath.FromSlash(rel))); err != nil {
			t.Errorf("archived capture %s missing: %v", rel, err)
		}
	}
	if !strings.Contains(execOut, archiveDir+"/") {
		t.Errorf("archive output must cite the %s subpath:\n%s", archiveDir, execOut)
	}
	if _, err := os.Stat(filepath.Join(root, "context", "wiki")); err == nil {
		t.Errorf("archive must not write a wiki page")
	}
}
