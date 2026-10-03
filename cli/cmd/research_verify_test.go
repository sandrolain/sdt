package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/research"
)

func TestResearchVerifyMarksVerified(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")

	execute(t, researchVerifyCmd, nil, "--run", runID)

	r, err := research.Load(root, runID)
	if err != nil {
		t.Fatal(err)
	}
	if r.CountByStatus(research.StatusVerified) != 1 {
		t.Fatalf("source not verified: %+v", r.Sources)
	}
	if r.Checkpoint.Operation != "verify" {
		t.Errorf("checkpoint = %q, want verify", r.Checkpoint.Operation)
	}
}

func TestResearchVerifyTamperedHashExitsNonZero(t *testing.T) {
	root := researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")

	r, _ := research.Load(root, runID)
	abs := filepath.Join(research.RunDir(root, runID), filepath.FromSlash(r.Sources[0].RawPath))
	if err := os.WriteFile(abs, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	if code := runVerify(t, "--run", runID); code == 0 {
		t.Fatal("a rejected source must exit non-zero")
	}
	r, _ = research.Load(root, runID)
	if r.CountByStatus(research.StatusRejected) != 1 {
		t.Errorf("source must be rejected: %+v", r.Sources)
	}
	if !strings.Contains(r.Sources[0].Error, "sha256 mismatch") {
		t.Errorf("reason not recorded: %q", r.Sources[0].Error)
	}
}

func TestResearchVerifyJSON(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/a")

	raw := string(execute(t, researchVerifyCmd, nil, "--run", runID, "--format", "json"))
	var res struct {
		RunID    string `json:"run_id"`
		Checked  int    `json:"checked"`
		Verified int    `json:"verified"`
		Rejected int    `json:"rejected"`
		Outcomes []struct {
			Status string `json:"status"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("verify --format json invalid: %v\n%s", err, raw)
	}
	if res.Checked != 1 || res.Verified != 1 || res.Rejected != 0 {
		t.Errorf("result = %+v", res)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Status != research.StatusVerified {
		t.Errorf("outcomes = %+v", res.Outcomes)
	}
}

func TestResearchVerifyNothingChecked(t *testing.T) {
	researchTestProject(t)
	runID := strings.TrimSpace(string(execute(t, researchInitCmd, nil, "--query", "q")))
	if code := runVerify(t, "--run", runID); code == 0 {
		t.Fatal("verify with nothing fetched must exit non-zero")
	}
}

// runVerify executes the verify command and reports the exit code it triggered.
func runVerify(t *testing.T, args ...string) (exited int) {
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
	execute(t, researchVerifyCmd, nil, args...)
	return exited
}
