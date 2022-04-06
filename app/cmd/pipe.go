package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var andCmd = &cobra.Command{
	Use:   "pipe",
	Short: "run multiple <sdt> commands separated by -",
	Run: func(cobraCmd *cobra.Command, args []string) {
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

		cmdPath, err := os.Executable()
		exitWithError(err)

		data, err := getInputBytes([]string{})
		exitWithError(err)

		var out []byte

		for _, cmdParts := range cmdList {
			cmdName := cmdParts[0]
			c := exec.Command(cmdPath, cmdParts...)
			p, err := c.StdinPipe()
			exitWithErrorF(cmdName+": %v", err)

			p.Write(data)
			p.Close()

			out, err = c.Output()
			exitWithErrorF(cmdName+": %v\n\n"+string(out), err)

			data = out
		}
		fmt.Print(string(out))

		// i := os.Stdin
		// for _, cmdParts := range cmdList {
		// 	r, w, err := os.Pipe()
		// 	exitWithError(err)

		// 	os.Stdin = i
		// 	rootCmd.SetOut(w)
		// 	rootCmd.SetArgs(cmdParts)

		// 	err = rootCmd.Execute()
		// 	exitWithError(err)

		// 	scanner := bufio.NewScanner(r)
		// 	byt := scanner.Bytes()

		// 	file, err := os.CreateTemp(os.TempDir(), "wd")
		// 	exitWithError(err)
		// 	file.Write(byt)
		// 	file.Close()

		// 	file, err = os.OpenFile(file.Name(), os.O_RDONLY, 0755)
		// 	exitWithError(err)

		// 	s, _ := file.Stat()
		// 	fmt.Printf("%+v", s)

		// 	defer os.Remove(file.Name())

		// 	i = file
		// }
	},
}

func init() {
	andCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(andCmd)
}
