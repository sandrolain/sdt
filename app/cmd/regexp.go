package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

var regeCmd = &cobra.Command{
	Use:     "regexp",
	Aliases: []string{"ereg", "exp"},
	Short:   "RegExp matching",
	Long:    `Regular Expression matching`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		exp := getStringFlag(cmd, "expression", true)

		re := must(regexp.Compile(exp))

		res := re.FindAllString(str, -1)
		if res == nil {
			exitWithError(fmt.Errorf(`input not match "%s"`, exp))
		}

		out := must(json.Marshal(res))
		outputBytes(cmd, out)
	},
}

func init() {
	regeCmd.PersistentFlags().StringP("expression", "e", "", "Expression")
	rootCmd.AddCommand(regeCmd)
}
