package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// runRoot runs the root command the way Execute does — pre-run included, then
// runAndFinish — capturing stdout and stderr separately.
func runRoot(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	prevOut, prevErr, prevIn := rootCmd.OutOrStdout(), rootCmd.ErrOrStderr(), rootCmd.InOrStdin()
	resetCmdFlags(rootCmd)
	out, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	rootCmd.SetOut(out)
	rootCmd.SetErr(errBuf)
	rootCmd.SetIn(bytes.NewReader(nil))
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(prevOut)
		rootCmd.SetErr(prevErr)
		rootCmd.SetIn(prevIn)
	})

	err := runAndFinish(rootCmd)
	return out.String(), errBuf.String(), err
}

var doneLine = regexp.MustCompile(`^sdt-done ok=(?:true|false) exit=\d+ reason=[a-z][a-z-]* wall=\d+\.\d{3}s$`)

// doneLines returns the sdt-done lines in stderr, so a test can assert there is
// exactly one.
func doneLines(stderr string) []string {
	var found []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "sdt-done ") {
			found = append(found, line)
		}
	}
	return found
}

func TestProgressDoneLineHuman(t *testing.T) {
	out, stderr, err := runRoot(t, "b64", "--input", "hello", "--progress", "human")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "aGVsbG8=" {
		t.Fatalf("stdout = %q, want the b64 result alone", out)
	}
	lines := doneLines(stderr)
	if len(lines) != 1 {
		t.Fatalf("expected exactly one sdt-done line, got %d in %q", len(lines), stderr)
	}
	if !doneLine.MatchString(lines[0]) {
		t.Fatalf("done line = %q, want the documented shape", lines[0])
	}
	if !strings.Contains(lines[0], "ok=true exit=0 reason=done") {
		t.Fatalf("done line = %q, want ok=true exit=0 reason=done", lines[0])
	}
}

func TestProgressDoneLineJSON(t *testing.T) {
	_, stderr, err := runRoot(t, "b64", "--input", "hello", "--progress", "json")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if lines := doneLines(stderr); len(lines) != 1 {
		t.Fatalf("expected exactly one sdt-done line, got %q", stderr)
	}
	if strings.Contains(stderr, "sdt-bar") {
		t.Fatalf("json mode must not draw the human bar: %q", stderr)
	}
}

func TestProgressModesSilent(t *testing.T) {
	for _, args := range [][]string{
		{"b64", "--input", "hello", "--progress", "none"},
		{"b64", "--input", "hello"}, // auto on a piped stderr
		{"b64", "--input", "hello", "--quiet"},
	} {
		_, stderr, err := runRoot(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if stderr != "" {
			t.Fatalf("%v: expected silence on stderr, got %q", args, stderr)
		}
	}
}

func TestProgressRejectsUnknownMode(t *testing.T) {
	_, _, err := runRoot(t, "b64", "--input", "hello", "--progress", "verbose")
	if err == nil || !strings.Contains(err.Error(), "invalid --progress") {
		t.Fatalf("expected an invalid --progress error, got %v", err)
	}
}

func TestProgressDoneLineOnError(t *testing.T) {
	exited := -1
	prevExit := exit
	exit = func(code int) { exited = code }
	t.Cleanup(func() { exit = prevExit })

	_, stderr, _ := runRoot(t, "b64", "dec", "--input", "not base64!!", "--progress", "human")
	if exited != 1 {
		t.Fatalf("exit code = %d, want 1", exited)
	}
	lines := doneLines(stderr)
	if len(lines) != 1 {
		t.Fatalf("expected exactly one sdt-done line, got %d in %q", len(lines), stderr)
	}
	if !strings.Contains(lines[0], "ok=false exit=1 reason=error") {
		t.Fatalf("done line = %q, want ok=false exit=1 reason=error", lines[0])
	}
}

// TestProgressKeepsStdoutIdentical pins the rule that narration never reaches
// stdout: the result is byte-identical whether the narration is on or off, for
// a text result and for a --format json one.
func TestProgressKeepsStdoutIdentical(t *testing.T) {
	cases := [][]string{
		{"b64", "--input", "hello"},
		{"context", "vocab", "--format", "json"},
	}
	for _, base := range cases {
		quietOut, _, err := runRoot(t, base...)
		if err != nil {
			t.Fatalf("%v: %v", base, err)
		}
		noisyArgs := append(append([]string{}, base...), "--progress", "human")
		noisyOut, _, err := runRoot(t, noisyArgs...)
		if err != nil {
			t.Fatalf("%v: %v", noisyArgs, err)
		}
		if quietOut != noisyOut {
			t.Fatalf("%v: stdout changed with narration on: %q vs %q", base, quietOut, noisyOut)
		}
	}
}

func TestExitWithJunkAndDecision(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
		code int
	}{
		{"junk", func() { exitWithJunk(rootCmd, "pages-failed") }, 3},
		{"decision", func() { exitWithDecision(rootCmd, "unfinished", []string{"run: sdt context resume x"}) }, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exited := -1
			prevExit := exit
			exit = func(code int) { exited = code }
			t.Cleanup(func() { exit = prevExit })

			var stderr bytes.Buffer
			prevErr := rootCmd.ErrOrStderr()
			rootCmd.SetErr(&stderr)
			t.Cleanup(func() { rootCmd.SetErr(prevErr) })
			progressReporter = nil

			tc.fn()
			if exited != tc.code {
				t.Fatalf("exit = %d, want %d", exited, tc.code)
			}
			if tc.code == 4 {
				if !strings.Contains(stderr.String(), "sdt-decision reason=unfinished") {
					t.Fatalf("decision header missing: %q", stderr.String())
				}
				if !strings.Contains(stderr.String(), "sdt context resume x") {
					t.Fatalf("decision options missing: %q", stderr.String())
				}
			}
		})
	}
}
