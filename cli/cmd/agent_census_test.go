package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func censusFixture(t *testing.T) string {
	t.Helper()
	dir := runInTempDir(t)
	lines := `{"ts":"2026-10-01T00:00:00Z","command":"sed -i 's/a/b/' x"}
{"ts":"2026-10-02T00:00:00Z","command":"cat > x <<'EOF'\nhi\nEOF"}
{"ts":"2026-09-01T00:00:00Z","command":"go test ./..."}
`
	writeTestFile(t, filepath.Join(dir, "calls.jsonl"), lines)
	return "calls.jsonl"
}

func TestAgentCensusCountsTheShapes(t *testing.T) {
	file := censusFixture(t)
	out := string(execute(t, agentCensusCmd, nil, "--file", file))
	for _, want := range []string{"sed literal", "whole-file write", "total"} {
		if !strings.Contains(out, want) {
			t.Errorf("report misses %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "total                     3") && !strings.Contains(out, "total") {
		t.Errorf("unexpected report:\n%s", out)
	}
}

func TestAgentCensusWindowExcludes(t *testing.T) {
	file := censusFixture(t)
	out := execute(t, agentCensusCmd, nil, "--file", file, "--since", "2026-10-01", "--format", "json")
	var res struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json answer: %v\n%s", err, out)
	}
	if res.Total != 2 {
		t.Errorf("window total = %d, want 2", res.Total)
	}
}

func TestAgentCensusRefusesAMalformedLine(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "bad.jsonl"), "{\"ts\":\"2026-10-01T00:00:00Z\",\"command\":\"x\"}\nnot json\n")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentCensusCmd, nil, "--file", "bad.jsonl"))
	})
}

func TestAgentCensusJSON(t *testing.T) {
	file := censusFixture(t)
	out := execute(t, agentCensusCmd, nil, "--file", file, "--format", "json")
	var res struct {
		Window struct {
			Since string `json:"since"`
		} `json:"window"`
		Total  int `json:"total"`
		Shapes []struct {
			Name  string  `json:"name"`
			Count int     `json:"count"`
			Share float64 `json:"share"`
		} `json:"shapes"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json answer: %v\n%s", err, out)
	}
	if res.Total != 3 {
		t.Errorf("total = %d, want 3", res.Total)
	}
	seen := map[string]int{}
	for _, s := range res.Shapes {
		seen[s.Name] = s.Count
	}
	if seen["sed literal"] != 1 || seen["whole-file write"] != 1 || seen["other"] != 1 {
		t.Errorf("shapes = %+v", seen)
	}
}

func TestAgentCensusSinceAfterUntilRefused(t *testing.T) {
	file := censusFixture(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentCensusCmd, nil, "--file", file, "--since", "2026-10-02", "--until", "2026-10-01"))
	})
}
