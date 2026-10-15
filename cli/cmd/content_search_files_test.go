//go:build !wasm

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestGrepFilesWithMatchesCommandEndToEnd is the fallback the mode replaces:
// `grep -rln | head -5`, one process instead of two.
func TestGrepFilesWithMatchesCommandEndToEnd(t *testing.T) {
	contentTree(t)
	out := string(execute(t, grepCmd, nil, "needle", "--files-with-matches", "--limit", "0"))
	// The fixture carries "needle" in a source file, an "ignored" log, a binary,
	// two corpus documents and a dependency — every text file the walk reaches.
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		t.Fatalf("no paths reported:\n%s", out)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		if strings.Contains(l, ":") {
			t.Errorf("a path line carries more than the path: %q", l)
		}
		if seen[l] {
			t.Errorf("path reported twice: %q", l)
		}
		seen[l] = true
	}
	if !seen["src/keep.txt"] {
		t.Errorf("src/keep.txt carries a match and is missing:\n%s", out)
	}
}

// TestGrepFilesWithMatchesShortShorthandMatchesGNU keeps the reflex working: the
// fallback was `grep -rl`, so `-l` must be the same mode.
func TestGrepFilesWithMatchesShortShorthandMatchesGNU(t *testing.T) {
	contentTree(t)
	shorthand := string(execute(t, grepCmd, nil, "needle", "-l", "--limit", "0"))
	longhand := string(execute(t, grepCmd, nil, "needle", "--files-with-matches", "--limit", "0"))
	if shorthand != longhand {
		t.Errorf("-l and --files-with-matches disagree:\n%s\n---\n%s", shorthand, longhand)
	}
}

// TestGrepFilesWithMatchesRejectsMeaninglessFlags is the "rejected, not ignored"
// rule: every refused flag describes how to render a match, and this mode
// reports files. A silently ignored flag is worse than no flag, because the
// caller believes it took effect.
func TestGrepFilesWithMatchesRejectsMeaninglessFlags(t *testing.T) {
	contentTree(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"needle", "-l", "--max-count", "2"}, "--max-count"},
		{[]string{"needle", "-l", "-A", "2"}, "--after"},
		{[]string{"needle", "-l", "-B", "2"}, "--before"},
		{[]string{"needle", "-l", "-C", "2"}, "--context"},
	}
	for _, tc := range cases {
		shouldExitWithCode(t, 1, func() string {
			execute(t, grepCmd, nil, tc.args...)
			return ""
		})
	}
	// The message must name the flag, not merely refuse. exitWithError logs
	// through the root command's err writer.
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, grepCmd, nil, "needle", "-l", "--max-count", "2"))
	})
	if out := errBuf.String(); !strings.Contains(out, "--max-count") || !strings.Contains(out, "--files-with-matches") {
		t.Errorf("the refusal must name both flags, got %q", out)
	}
}

// TestGrepFilesWithMatchesAcceptsTheDefaultCap keeps the refusal from being
// pedantic: `--max-count 0` is the explicit default, not a cap.
func TestGrepFilesWithMatchesAcceptsTheDefaultCap(t *testing.T) {
	contentTree(t)
	out := string(execute(t, grepCmd, nil, "needle", "-l", "--max-count", "0"))
	if !strings.Contains(out, "src/keep.txt") {
		t.Errorf("--max-count 0 is the default and must be accepted:\n%s", out)
	}
}

// TestGrepFilesWithMatchesAnswerIsEmptyWhenNothingMatches pins the unambiguous
// zero: nothing printed, and the mode exits cleanly.
func TestGrepFilesWithMatchesAnswerIsEmptyWhenNothingMatches(t *testing.T) {
	contentTree(t)
	out := string(execute(t, grepCmd, nil, "no-such-token-anywhere", "-l"))
	if strings.TrimSpace(out) != "" {
		t.Errorf("a zero answer prints nothing, got %q", out)
	}
}

// nonEmptyLines splits output into its non-empty lines.
func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
