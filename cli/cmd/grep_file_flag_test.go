package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestGrepRefusesTheInputFileFlag is the D3 gate: `--file` is the global input
// flag, and on a search it used to be accepted while the walk covered the whole
// tree. The refusal names the flag that scopes.
func TestGrepRefusesTheInputFileFlag(t *testing.T) {
	newTree := func() (*cobra.Command, *cobra.Command) {
		root := &cobra.Command{Use: "sdt"}
		root.PersistentFlags().String("file", "", "Input File")
		child := &cobra.Command{Use: "grep"}
		root.AddCommand(child)
		return root, child
	}

	_, child := newTree()
	if err := grepInputFileRefusal(child); err != nil {
		t.Fatalf("no --file was passed, yet the guard refused: %v", err)
	}

	root, child := newTree()
	if err := root.PersistentFlags().Set("file", "context/index.md"); err != nil {
		t.Fatal(err)
	}
	err := grepInputFileRefusal(child)
	if err == nil {
		t.Fatal("--file was accepted on grep")
	}
	for _, want := range []string{"--dir", "--files-with-matches"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %s: %v", want, err)
		}
	}
}
