package cmd

import (
	"github.com/spf13/cobra"
)

var echoCmd = &cobra.Command{
	Use:   "echo",
	Short: "Echo",
	Long:  `Output the Input`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		outputBytes(cmd, byt)
	},
}

func init() {
	rootCmd.AddCommand(echoCmd)
}
