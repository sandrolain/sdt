//go:build !wasm

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countFixture writes a file whose line count is known by construction.
func countFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "counted.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestReadCountReplacesWcLines is the fallback this flag removes: `wc -l`.
func TestReadCountReplacesWcLines(t *testing.T) {
	path := countFixture(t, "one\ntwo\nthree\n")
	out := execute(t, fileReadCmd, nil, "-p", path, "--count")
	if got := string(out); got != "3\n" {
		t.Errorf("read --count = %q, want %q", got, "3\n")
	}
}

// TestReadCountComposesWithTheRange is the decision the phase turns on: a count
// and a slice must agree about what the range contains. `--count` with `--lines`
// counts the selection rather than contradicting it.
func TestReadCountComposesWithTheRange(t *testing.T) {
	path := countFixture(t, "one\ntwo\nthree\nfour\n")
	out := execute(t, fileReadCmd, nil, "-p", path, "--count", "--lines", "2:3")
	if got := string(out); got != "2\n" {
		t.Errorf("read --count --lines 2:3 = %q, want %q", got, "2\n")
	}
	// The bound clamps, so the clamped selection counts what it printed.
	out = execute(t, fileReadCmd, nil, "-p", path, "--count", "--lines", "3:999")
	if got := string(out); got != "2\n" {
		t.Errorf("clamped count = %q, want %q", got, "2\n")
	}
	// A range past the end is an error here for the same reason it is one
	// without --count: the two must not disagree about the range.
	shouldExitWithCode(t, 1, func() string {
		execute(t, fileReadCmd, nil, "-p", path, "--count", "--lines", "99:120")
		return ""
	})
}

// TestReadCountIsTheSameNumberInEveryFormat keeps the three formats from
// disagreeing, since an agent reads json and a human reads text.
func TestReadCountIsTheSameNumberInEveryFormat(t *testing.T) {
	path := countFixture(t, "one\ntwo\n")
	out := execute(t, fileReadCmd, nil, "-p", path, "--count", "--format", "json")
	var payload struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out)
	}
	if payload.Count != 2 {
		t.Errorf("json count = %d, want 2", payload.Count)
	}
	out = execute(t, fileReadCmd, nil, "-p", path, "--count", "--format", "yaml")
	if !strings.Contains(string(out), "count: 2") {
		t.Errorf("yaml count missing:\n%s", out)
	}
	// An empty file has no lines, and a zero line count is an answer.
	out = execute(t, fileReadCmd, nil, "-p", countFixture(t, ""), "--count")
	if got := string(out); got != "0\n" {
		t.Errorf("empty file count = %q, want %q", got, "0\n")
	}
}

// TestGrepCountReplacesGrepCount is the other fallback: the number of matching
// lines, with no rows to read around.
func TestGrepCountReplacesGrepCount(t *testing.T) {
	contentTree(t)
	out := string(execute(t, grepCmd, nil, "needle", "--count", "--include", "keep.txt"))
	if strings.TrimSpace(out) != "1" {
		t.Errorf("grep --count = %q, want 1", out)
	}
	// `grep -c ""` counts every line of a file, which is read --count, not a
	// grep with an empty pattern; an empty pattern still matches every line
	// here, and the count says how many there are.
	out = string(execute(t, grepCmd, nil, "^", "--count", "--include", "keep.txt"))
	if strings.TrimSpace(out) != "2" {
		t.Errorf("counting every line of a two-line file = %q, want 2", out)
	}
}

// TestGrepCountZeroIsReportedNotOmitted keeps the zero answer explicit in the
// structured surface, where an absent field would read as "not asked".
func TestGrepCountZeroIsReportedNotOmitted(t *testing.T) {
	contentTree(t)
	out := execute(t, grepCmd, nil, "no-such-token-anywhere", "--count", "--format", "json")
	var payload struct {
		Count *int `json:"count"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out)
	}
	if payload.Count == nil {
		t.Fatalf("a zero count must be present:\n%s", out)
	}
	if *payload.Count != 0 {
		t.Errorf("count = %d, want 0", *payload.Count)
	}
}

// TestGrepCountRefusesTheFlagsItCannotHonour extends the "rejected, not ignored"
// rule to the count mode: --limit would bound a fact, and context flags describe
// lines this mode does not print.
func TestGrepCountRefusesTheFlagsItCannotHonour(t *testing.T) {
	contentTree(t)
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"needle", "-c", "-l"}, []string{"--files-with-matches", "--count"}},
		{[]string{"needle", "-c", "--limit", "5"}, []string{"--limit", "--count"}},
		{[]string{"needle", "-c", "-A", "1"}, []string{"--after", "--count"}},
	}
	for _, tc := range cases {
		shouldExitWithCode(t, 1, func() string {
			execute(t, grepCmd, nil, tc.args...)
			return ""
		})
		var errBuf bytes.Buffer
		rootCmd.SetErr(&errBuf)
		t.Cleanup(func() { rootCmd.SetErr(nil) })
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, grepCmd, nil, tc.args...))
		})
		if out := errBuf.String(); !strings.Contains(out, tc.want[0]) || !strings.Contains(out, tc.want[1]) {
			t.Errorf("%v: refusal must name both flags, got %q", tc.args, out)
		}
	}
	// --max-count is the exception and must be accepted: it stops the scan of a
	// file, so it bounds a count the way `grep -m N -c` does.
	out := string(execute(t, grepCmd, nil, "needle", "-c", "--max-count", "1", "--include", "keep.txt"))
	if strings.TrimSpace(out) != "1" {
		t.Errorf("--max-count with --count = %q, want the clipped count", out)
	}
}

// TestGrepCountModeFlagsAppearInTheSchema keeps the wave-1 discovery surface
// holding for the two modes this wave added.
func TestGrepCountModeFlagsAppearInTheSchema(t *testing.T) {
	var schemas []CommandSchema
	collectSchemas(rootCmd, &schemas)
	for i := range schemas {
		if schemas[i].Command != "grep" {
			continue
		}
		for _, name := range []string{"count", "files-with-matches"} {
			prop, ok := schemas[i].Flags.Properties[name]
			if !ok {
				t.Fatalf("--%s is missing from the grep schema", name)
			}
			if prop.Type != "boolean" {
				t.Errorf("--%s type = %q, want boolean", name, prop.Type)
			}
		}
		return
	}
	t.Fatal("grep is not in the schema")
}
