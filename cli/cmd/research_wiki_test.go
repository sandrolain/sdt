package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResearchPopulateWikiIsReadOnly(t *testing.T) {
	root, runID := synthesizedRun(t)

	out := string(execute(t, researchPopulateWikiCmd, nil, "--run", runID, "--brief", "why"))
	for _, want := range []string{"Wiki brief", "brief: why", "refs/"} {
		if !strings.Contains(out, want) {
			t.Errorf("brief missing %q:\n%s", want, out)
		}
	}
	// The CLI must never write a wiki page.
	if _, err := os.Stat(filepath.Join(root, "context", "wiki")); err == nil {
		e, _ := os.ReadDir(filepath.Join(root, "context", "wiki"))
		if len(e) != 0 {
			t.Errorf("populate-wiki must not write a wiki page: %v", e)
		}
	}
}

func TestResearchPopulateWikiListsOnlyVerified(t *testing.T) {
	root, runID := synthesizedRun(t)
	// A second, unverified source must not appear in the brief.
	ts := pageServer(t)
	execute(t, researchFetchCmd, nil, "--run", runID, ts.URL+"/b")

	raw := string(execute(t, researchPopulateWikiCmd, nil, "--run", runID, "--format", "json"))
	var view struct {
		RunID   string           `json:"run_id"`
		Sources []map[string]any `json:"sources"`
	}
	if err := json.Unmarshal([]byte(raw), &view); err != nil {
		t.Fatalf("populate-wiki --format json is not valid JSON: %v\n%s", err, raw)
	}
	if view.RunID != runID {
		t.Errorf("run_id = %q, want %q", view.RunID, runID)
	}
	if len(view.Sources) != 1 {
		t.Errorf("only the verified source must be listed, got %d: %s", len(view.Sources), raw)
	}
	_ = root
}
