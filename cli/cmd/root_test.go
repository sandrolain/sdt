package cmd

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/sandrolain/sdt/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// captureLogs redirects the shared logger to a buffer for the duration of the
// test and returns it, so assertions can read what the commands logged. The
// previous default logger is restored on cleanup.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := slog.Default()
	buf := new(bytes.Buffer)
	logging.Setup(logging.Options{Writer: buf, NoColor: true})
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buf
}

// resetCmdFlags recursively resets all flag values to their defaults so that
// shared cobra.Command instances don't accumulate state between Execute() calls.
func resetCmdFlags(cmd *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if !f.Changed {
			return
		}
		// StringArray/StringSlice values append on Set, so restoring the
		// "[]" DefValue would inject a literal entry; replace them instead.
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	cmd.Flags().VisitAll(reset)
	cmd.PersistentFlags().VisitAll(reset)
	for _, sub := range cmd.Commands() {
		resetCmdFlags(sub)
	}
}

func execute(t *testing.T, c *cobra.Command, in []byte, args ...string) []byte {
	t.Helper()

	uses := getUseArray(c)
	args = append(uses, args...)

	rc := c.Root()

	// Reset all flag state to avoid accumulation between Execute() calls.
	resetCmdFlags(rc)

	inr := bytes.NewReader(in)
	rc.SetIn(inr)

	origOut := rootCmd.OutOrStdout()

	buf := new(bytes.Buffer)
	rc.SetOut(buf)
	rc.SetArgs(args)

	err := rc.Execute()
	rc.SetIn(nil)
	rootCmd.SetOut(origOut)

	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func shouldExitWithCode(t *testing.T, code int, fn func() string) {
	exited := -1
	origExit := exit
	exit = func(exitCode int) {
		exited = exitCode
	}
	fn()
	exit = origExit
	if code != exited {
		t.Fatalf("expected exit code %v, got %v", code, exited)
	}
}
