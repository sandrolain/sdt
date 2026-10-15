//go:build !wasm

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/search"
)

// grepContextCmd builds a fresh grep command so a test can set the flags through
// the real parser rather than with a hand-made FlagSet.
func grepContextCmd(t *testing.T, args ...string) (*cobra.Command, error) {
	t.Helper()
	c := &cobra.Command{Use: "grep", Run: func(*cobra.Command, []string) {}}
	c.Flags().IntP("after", "A", 0, "")
	c.Flags().IntP("before", "B", 0, "")
	c.Flags().IntP("context", "C", 0, "")
	if err := c.ParseFlags(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return c, nil
}

// TestGrepContextFlagsResolveTheShorthandGroup pins the resolution rule the flag
// group lives or dies by: -C is both sides, and an explicit side beats it for
// that side only. GNU's rule, and the only one an agent's reflex will match.
func TestGrepContextFlagsResolveTheShorthandGroup(t *testing.T) {
	cases := []struct {
		args           []string
		before, after  int
		wantErrContain string
	}{
		{args: []string{"-A", "2"}, before: 0, after: 2},
		{args: []string{"-B", "3"}, before: 3, after: 0},
		{args: []string{"-C", "4"}, before: 4, after: 4},
		{args: []string{"-C", "4", "-A", "0"}, before: 4, after: 0},
		{args: []string{"-C", "4", "-B", "1"}, before: 1, after: 4},
		{args: []string{"-A", "1", "-B", "2"}, before: 2, after: 1},
		{args: nil, before: 0, after: 0},
		{args: []string{"-A", "-1"}, wantErrContain: "cannot be negative"},
		{args: []string{"-C", "-2"}, wantErrContain: "cannot be negative"},
	}
	for _, tc := range cases {
		c, err := grepContextCmd(t, tc.args...)
		if err != nil {
			t.Fatalf("parse %v: %v", tc.args, err)
		}
		before, after, err := grepContextFlags(c)
		if tc.wantErrContain != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErrContain) {
				t.Errorf("%v: error = %v, want one containing %q", tc.args, err, tc.wantErrContain)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: %v", tc.args, err)
			continue
		}
		if before != tc.before || after != tc.after {
			t.Errorf("%v: before=%d after=%d, want %d and %d", tc.args, before, after, tc.before, tc.after)
		}
	}
}

// TestGrepContextFlagsRejectNegativeNamesTheShorthand keeps the failure
// actionable: a caller who typed -A -1 has a bug, and the message must say which
// flag carried it rather than clamping in silence.
func TestGrepContextFlagsRejectNegativeNamesTheShorthand(t *testing.T) {
	c, _ := grepContextCmd(t, "-B", "-5")
	_, _, err := grepContextFlags(c)
	if err == nil {
		t.Fatal("a negative count must be rejected")
	}
	if !strings.Contains(err.Error(), "-B") || !strings.Contains(err.Error(), "--before") {
		t.Errorf("the message must name the flag both ways, got %q", err)
	}
}

// TestRenderGrepMatchesPrintsAnOverlappingLineOnce is the defect the merge
// exists for: with -A2 two adjacent matches share a line, and concatenating the
// blocks prints it twice — once as context and once as a match.
func TestRenderGrepMatchesPrintsAnOverlappingLineOnce(t *testing.T) {
	matches := []search.GrepMatch{
		{Path: "f.txt", Line: 1, Text: "M a", After: []search.GrepContext{{Line: 2, Text: "M b"}, {Line: 3, Text: "x"}}},
		{Path: "f.txt", Line: 2, Text: "M b", After: []search.GrepContext{{Line: 3, Text: "x"}}},
	}
	got := strings.Join(renderGrepMatches(matches, true), "\n")
	want := "f.txt:1:M a\nf.txt:2:M b\nf.txt-3-x"
	if got != want {
		t.Errorf("render =\n%s\nwant\n%s", got, want)
	}
	// The shared line keeps its match marker, which is what makes a context line
	// distinguishable from a match instead of trusting position.
	if strings.Count(got, "f.txt:2:M b") != 1 {
		t.Errorf("the shared line is not marked as a match exactly once:\n%s", got)
	}
}

// TestRenderGrepMatchesSeparatesNonAdjacentBlocks keeps the blocks readable: a
// gap between them is the `--` separator a grep reader expects.
func TestRenderGrepMatchesSeparatesNonAdjacentBlocks(t *testing.T) {
	matches := []search.GrepMatch{
		{Path: "f.txt", Line: 1, Text: "M1", After: []search.GrepContext{{Line: 2, Text: "b"}}},
		{Path: "f.txt", Line: 9, Text: "M2", Before: []search.GrepContext{{Line: 8, Text: "h"}}},
	}
	got := strings.Join(renderGrepMatches(matches, true), "\n")
	want := "f.txt:1:M1\nf.txt-2-b\n--\nf.txt-8-h\nf.txt:9:M2"
	if got != want {
		t.Errorf("render =\n%s\nwant\n%s", got, want)
	}
	// Without context the separator has no purpose and must not appear.
	flat := strings.Join(renderGrepMatches([]search.GrepMatch{
		{Path: "f.txt", Line: 1, Text: "M1"},
		{Path: "f.txt", Line: 9, Text: "M2"},
	}, false), "\n")
	if strings.Contains(flat, "--") {
		t.Errorf("a context-free render must carry no separator:\n%s", flat)
	}
	if flat != "f.txt:1:M1\nf.txt:9:M2" {
		t.Errorf("context-free render =\n%s", flat)
	}
}

// TestGrepCommandContextFlagsEndToEnd runs the flags through the verb, so the
// resolution and the renderer are checked as the caller meets them.
func TestGrepCommandContextFlagsEndToEnd(t *testing.T) {
	contentTree(t)
	// Matches are sorted by path, so the assertions address src/keep.txt — the
	// fixture's "alpha\nneedle line\n" — rather than assuming position.
	matchIn := func(res search.GrepResults, path string) *search.GrepMatch {
		t.Helper()
		for i := range res.Matches {
			if res.Matches[i].Path == path {
				return &res.Matches[i]
			}
		}
		t.Fatalf("%s is not among the %d matches", path, len(res.Matches))
		return nil
	}

	res := runGrep(t, "needle", "-B", "1", "--limit", "0")
	m := matchIn(res, "src/keep.txt")
	if len(m.Before) != 1 || m.Before[0].Text != "alpha" {
		t.Errorf("-B 1 did not carry the preceding line: %+v", m.Before)
	}
	// -C is both sides; the file's match is last, so after is empty and the
	// before side proves the shorthand reached the scanner.
	res = runGrep(t, "needle", "-C", "1", "--limit", "0")
	if m = matchIn(res, "src/keep.txt"); len(m.Before) != 1 || m.Before[0].Text != "alpha" {
		t.Errorf("-C 1 did not carry the before side: %+v", m.Before)
	}
}

// TestGrepCommandContextRendersAsBlocks checks the text surface, not only the
// structured one: a context line and a match line must be told apart.
func TestGrepCommandContextRendersAsBlocks(t *testing.T) {
	contentTree(t)
	out := string(execute(t, grepCmd, nil, "needle", "-B", "1", "--include", "keep.txt"))
	if !strings.Contains(out, "keep.txt:2:needle line") {
		t.Errorf("the match line is not marked with ':'\n%s", out)
	}
	if !strings.Contains(out, "keep.txt-1-alpha") {
		t.Errorf("the context line is not marked with '-'\n%s", out)
	}
}

// TestGrepContextFlagsAppearInTheSchema is the wave-1 surface still holding: a
// flag an agent cannot discover is a flag that does not exist.
func TestGrepContextFlagsAppearInTheSchema(t *testing.T) {
	var schemas []CommandSchema
	collectSchemas(rootCmd, &schemas)
	var found *CommandSchema
	for i := range schemas {
		if schemas[i].Command == "grep" {
			found = &schemas[i]
			break
		}
	}
	if found == nil {
		t.Fatal("grep is not in the schema")
	}
	for _, name := range []string{"after", "before", "context"} {
		if _, ok := found.Flags.Properties[name]; !ok {
			t.Errorf("--%s is missing from the schema", name)
		}
	}
}
