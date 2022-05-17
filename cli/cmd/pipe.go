package cmd

import (
	"github.com/spf13/cobra"
)

var andCmd = &cobra.Command{
	Use:   "pipe",
	Short: "run multiple <sdt> commands separated by -",
	Run: func(cmd *cobra.Command, args []string) {
		var cmdParts []string
		var cmdList [][]string
		for _, arg := range args {
			if arg == "-" {
				if len(cmdParts) > 0 {
					cmdList = append(cmdList, cmdParts)
					cmdParts = []string{}
				}
			} else {
				cmdParts = append(cmdParts, arg)
			}
		}
		cmdList = append(cmdList, cmdParts)

		in := getInputBytes(cmd, []string{})

		var out []byte

		for _, cmdParts := range cmdList {
			out = executeByArgs(cmdParts, in)
			in = out
		}

		outputBytes(cmd, out)
	},
}

func init() {
	andCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(andCmd)
}
