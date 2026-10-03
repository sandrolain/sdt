package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

func TestResearchResumeFetchesPendingAndVerifies(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)

	// Simulate an interrupted run: two discovered sources, neither fetched.
	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	r.AddSource(research.Source{CanonicalURL: ts.URL + "/a", Status: research.StatusDiscovered})
	r.AddSource(research.Source{CanonicalURL: ts.URL + "/b", Status: research.StatusDiscovered})
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}

	execute(t, researchResumeCmd, nil, "--run", runID)

	r, _ = research.Load(root, runID)
	if r.CountByStatus(research.StatusVerified) != 2 {
		t.Fatalf("resume must fetch and verify both sources: %+v", r.Sources)
	}
	if r.Checkpoint.Operation != "resume" {
		t.Errorf("checkpoint = %q, want resume", r.Checkpoint.Operation)
	}
}

func TestResearchResumeSkipsFinishedWork(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")
	execute(t, researchVerifyCmd, nil, "--run", runID)

	before, _ := research.Load(root, runID)

	// Resume a fully finished run: no new fetch, no duplicate source, same hash.
	execute(t, researchResumeCmd, nil, "--run", runID)

	after, _ := research.Load(root, runID)
	if len(after.Sources) != len(before.Sources) {
		t.Errorf("resume created duplicate sources: before %d, after %d", len(before.Sources), len(after.Sources))
	}
	if after.Sources[0].SHA256 != before.Sources[0].SHA256 {
		t.Errorf("resume re-fetched an already fetched source (hash changed)")
	}
	if after.CountByStatus(research.StatusVerified) != 1 {
		t.Errorf("verified source state lost: %+v", after.Sources)
	}
}

func TestResearchResumeIsIdempotentOnReplay(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)

	r, _ := research.Load(root, runID)
	r.AddSource(research.Source{CanonicalURL: ts.URL + "/a", Status: research.StatusDiscovered})
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}

	execute(t, researchResumeCmd, nil, "--run", runID)
	first, _ := research.Load(root, runID)

	execute(t, researchResumeCmd, nil, "--run", runID)
	second, _ := research.Load(root, runID)

	if len(first.Sources) != len(second.Sources) || len(second.Sources) != 1 {
		t.Fatalf("replay changed the source set: first %d, second %d", len(first.Sources), len(second.Sources))
	}
	if first.Sources[0].SHA256 != second.Sources[0].SHA256 {
		t.Errorf("replay re-fetched completed work")
	}
}

func TestResearchResumeEmptyRunIsNoOp(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	// No sources at all: resume must succeed without doing anything.
	execute(t, researchResumeCmd, nil, "--run", runID)
}
