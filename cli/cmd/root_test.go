package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetCmdFlags recursively resets all flag values to their defaults so that
// shared cobra.Command instances don't accumulate state between Execute() calls.
func resetCmdFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
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
