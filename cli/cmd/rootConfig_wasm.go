package cmd

import (
	"github.com/spf13/cobra"
)

func loadFileConfig() {
	return
}

func getInputString(cmd *cobra.Command, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	if len(args) > 0 {
		return []byte(args[0])
	}
	return []byte{}
}
