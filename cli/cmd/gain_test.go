package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/usage"
)

// newUsageProject makes a throwaway project the usage log can resolve, and runs
// the test from inside it so nothing is written into this repository's own log.
func newUsageProject(t *testing.T) string {
	t.Helper()
	// TestMain turns the log off for the suite; this helper is how a test turns
	// it back on.
	t.Setenv(usage.EnvOff, "")
	dir := t.TempDir()
	config := "project: test\ngroup: test\n"
	if err := os.WriteFile(filepath.Join(dir, ".sdt.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

// usageLines returns the records written under root, one string per line.
func usageLines(t *testing.T, root string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(usage.Dir(root), "usage-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// TestUsageRecordHoldsNoArguments is the privacy contract: the record is a
// command path and four numbers, and the input never reaches the file.
func TestUsageRecordHoldsNoArguments(t *testing.T) {
	root := newUsageProject(t)

	out, _, err := runRoot(t, "b64", "--input", "supersecret", "--progress", "none")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "c3VwZXJzZWNyZXQ=" {
		t.Fatalf("stdout = %q", out)
	}

	lines := usageLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected one record, got %d: %q", len(lines), lines)
	}
	if strings.Contains(lines[0], "supersecret") {
		t.Fatalf("the record carries the argument: %s", lines[0])
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("record is not JSON: %v", err)
	}
	var keys []string
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"cmd", "exit", "ms", "out", "ts"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("record keys = %v, want %v", keys, want)
	}
	var cmd string
	if err := json.Unmarshal(record["cmd"], &cmd); err != nil {
		t.Fatal(err)
	}
	if cmd != "b64" {
		t.Fatalf("cmd = %q, want the command path", cmd)
	}
}

func TestUsageOffByEnvAndConfig(t *testing.T) {
	t.Run("SDT_USAGE=off", func(t *testing.T) {
		root := newUsageProject(t)
		t.Setenv(usage.EnvOff, "off")
		if _, _, err := runRoot(t, "b64", "--input", "hi", "--progress", "none"); err != nil {
			t.Fatalf("run: %v", err)
		}
		if lines := usageLines(t, root); len(lines) != 0 {
			t.Fatalf("expected no record, got %q", lines)
		}
	})
	t.Run("usage.enabled false", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(usage.EnvOff, "")
		config := "project: test\ngroup: test\nusage:\n  enabled: false\n"
		if err := os.WriteFile(filepath.Join(root, ".sdt.yaml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		if _, _, err := runRoot(t, "b64", "--input", "hi", "--progress", "none"); err != nil {
			t.Fatalf("run: %v", err)
		}
		if lines := usageLines(t, root); len(lines) != 0 {
			t.Fatalf("expected no record, got %q", lines)
		}
	})
}

// TestUsageFailureLeavesTheCommandIntact pins the rule that the log is a
// measurement: a directory it cannot write never changes the result or the
// exit status.
func TestUsageFailureLeavesTheCommandIntact(t *testing.T) {
	root := newUsageProject(t)
	// .sdt is a regular file, so the usage directory cannot be created under it.
	if err := os.WriteFile(filepath.Join(root, ".sdt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := runRoot(t, "b64", "--input", "hello", "--progress", "none")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "aGVsbG8=" {
		t.Fatalf("stdout = %q, want the result unchanged", out)
	}
}

func TestGainExcludesItselfAndInternalCalls(t *testing.T) {
	root := newUsageProject(t)
	zero := 0
	entries := []usage.Entry{
		{TS: "2026-10-30T06:00:00Z", Cmd: "context search", Out: 120, Hits: &zero},
		{TS: "2026-10-30T06:00:01Z", Cmd: "gain", Out: 500},
		{TS: "2026-10-30T06:00:02Z", Cmd: "context status set", Source: usage.SourceInternal, Out: 40},
	}
	for _, e := range entries {
		if err := usage.Append(root, e); err != nil {
			t.Fatal(err)
		}
	}

	out, _, err := runRoot(t, "gain", "--since", "all", "--json")
	if err != nil {
		t.Fatalf("gain: %v", err)
	}
	var payload gainPayload
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("gain --json is not JSON: %v\n%s", err, out)
	}
	if payload.Calls != 1 {
		t.Fatalf("calls = %d, want the recorded call only", payload.Calls)
	}
	if payload.ExcludedSelf != 1 || payload.ExcludedInternal != 1 {
		t.Fatalf("excluded = %d self, %d internal", payload.ExcludedSelf, payload.ExcludedInternal)
	}
	if payload.HitsReported != 1 || payload.HitsFound != 0 {
		t.Fatalf("a reported zero must count as a miss: %+v", payload)
	}

	allOut, _, err := runRoot(t, "gain", "--since", "all", "--all", "--json")
	if err != nil {
		t.Fatalf("gain --all: %v", err)
	}
	var allPayload gainPayload
	if err := json.Unmarshal([]byte(allOut), &allPayload); err != nil {
		t.Fatal(err)
	}
	if allPayload.Calls < 3 {
		t.Fatalf("--all calls = %d, want the excluded rows back", allPayload.Calls)
	}
}

func TestGainTextStatesNoCounterfactual(t *testing.T) {
	newUsageProject(t)
	out, _, err := runRoot(t, "gain")
	if err != nil {
		t.Fatalf("gain: %v", err)
	}
	if !strings.Contains(out, "No counterfactual saving is computed") {
		t.Fatalf("the report must refuse the counterfactual:\n%s", out)
	}
	if !strings.Contains(out, "window") {
		t.Fatalf("the report must name its window:\n%s", out)
	}
}

func TestGainRejectsUnparsableWindow(t *testing.T) {
	newUsageProject(t)
	var stderr string
	shouldExitWithCode(t, 1, func() string {
		_, stderr, _ = runRoot(t, "gain", "--since", "whenever")
		return ""
	})
	if !strings.Contains(stderr, "invalid --since") {
		t.Fatalf("expected an invalid --since error, got %q", stderr)
	}
}
