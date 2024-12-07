//go:build !wasm

package cmd

import (
	"github.com/gen2brain/beeep"
	"github.com/spf13/cobra"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Notification",
	Long:  `Desktop notification`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		title := getStringFlag(cmd, "title", false)
		exitWithError(cmd, beeep.Notify(title, str, ""))
	},
}

func init() {
	notifyCmd.PersistentFlags().StringP("title", "t", "Smart Developer Tools", "Rows as objects")

	rootCmd.AddCommand(notifyCmd)
}
