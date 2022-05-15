package cmd

import (
	"os"
	"os/exec"

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

		cmdPath := must(os.Executable())

		data := getInputBytes(cmd, []string{})

		var out []byte

		for _, cmdParts := range cmdList {
			cmdName := cmdParts[0]
			c := exec.Command(cmdPath, cmdParts...)
			p, err := c.StdinPipe()
			exitWithErrorF(cmdName+": %v", err)

			p.Write(data)
			p.Close()

			out, err = c.CombinedOutput()
			exitWithErrorF(cmdName+": %v\n\n"+string(out), err)

			data = out
		}
		outputBytes(cmd, out)
	},
}

func init() {
	andCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(andCmd)
}
