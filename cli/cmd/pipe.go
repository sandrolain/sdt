package cmd

import (
	"github.com/spf13/cobra"
)

var andCmd = &cobra.Command{
	Use:     "pipe",
	Aliases: []string{":"},
	Short:   "run multiple <sdt> commands separated by -",
	Run: func(cmd *cobra.Command, args []string) {
		areInputArgs := true
		var inputArgs []string
		var cmdParts []string
		var cmdList [][]string
		for _, arg := range args {
			if arg == "-" || arg == ":" {
				if len(cmdParts) > 0 {
					if areInputArgs {
						inputArgs = cmdParts
					} else {
						cmdList = append(cmdList, cmdParts)
					}
					cmdParts = []string{}
				}
				areInputArgs = false
			} else {
				cmdParts = append(cmdParts, arg)
			}
		}
		cmdList = append(cmdList, cmdParts)

		in := getInputBytes(cmd, inputArgs)

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
