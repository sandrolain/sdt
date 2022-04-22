package cmd

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var clipboardCmd = &cobra.Command{
	Use:     "clipboard",
	Aliases: []string{"cb", "clip"},
	Short:   "Clipboard Tools",
	Long:    `Clipboard Tools`,
}

var clipboardReadCmd = &cobra.Command{
	Use:     "read",
	Aliases: []string{"cb", "clip"},
	Short:   "Read clipboard",
	Long:    `Read clipbaord value`,
	Run: func(cmd *cobra.Command, args []string) {
		value, err := clipboard.ReadAll()
		exitWithError(err)
		fmt.Print(value)
	},
}

var clipboardWriteCmd = &cobra.Command{
	Use:     "write",
	Aliases: []string{"cb", "clip"},
	Short:   "Write clipboard",
	Long:    `Read clipbaord value`,
	Run: func(cmd *cobra.Command, args []string) {
		value, err := getInputString(cmd, args)
		exitWithError(err)
		err = clipboard.WriteAll(value)
		exitWithError(err)
	},
}

func init() {
	clipboardCmd.AddCommand(clipboardReadCmd)
	clipboardCmd.AddCommand(clipboardWriteCmd)

	rootCmd.AddCommand(clipboardCmd)
}
