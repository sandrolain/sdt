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
		str, err := getInputString(args)
		exitWithError(err)

		title, err := cmd.Flags().GetString("title")
		exitWithError(err)

		err = beeep.Notify(title, str, "")
		exitWithError(err)
	},
}

func init() {
	notifyCmd.PersistentFlags().StringP("title", "t", "Smart Developer Tools", "Rows as objects")

	rootCmd.AddCommand(notifyCmd)
}
