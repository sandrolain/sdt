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
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "why")))
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

	// 6. populate-wiki preview → plan, no write.
	planOut := string(execute(t, researchPopulateWikiCmd, nil, "--run", runID, "--brief", "why", "--max-pages", "3"))
	if !strings.Contains(planOut, "pages: 2") {
		t.Errorf("populate-wiki preview should plan 2 pages:\n%s", planOut)
	}
	wikiDir := filepath.Join(root, "context", "wiki")
	if entries, err := os.ReadDir(wikiDir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "a-") || strings.HasPrefix(e.Name(), "b-") {
				t.Errorf("preview must not write a page: %s", e.Name())
			}
		}
	}
}
