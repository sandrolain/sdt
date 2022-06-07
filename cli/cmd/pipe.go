package cmd

import (
	"github.com/spf13/cobra"
)

var andCmd = &cobra.Command{
	Use:     "pipe",
	Aliases: []string{":"},
	Short:   "run multiple <sdt> commands separated by -",
	Run: func(cmd *cobra.Command, args []string) {
		var cmdParts []string
		var cmdList [][]string
		for _, arg := range args {
			if arg == "-" || arg == ":" {
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
			var err error
			out, err = ExecuteByArgs(cmdParts, in)
			exitWithError(err)
			in = out
		}

		outputBytes(cmd, out)
	},
}

func init() {
	andCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(andCmd)
}
