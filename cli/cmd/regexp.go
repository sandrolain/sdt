package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

var regeCmd = &cobra.Command{
	Use:     "regexp",
	Aliases: []string{"re"},
	Short:   "RegExp matching",
	Long:    `Regular Expression matching`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		exp := getStringFlag(cmd, "expression", true)

		re, err := regexp.Compile(exp)
		exitWithError(cmd, err)

		res := re.FindAllString(str, -1)
		if res == nil {
			exitWithError(cmd, fmt.Errorf(`input not match "%s"`, exp))
		}

		out, err := json.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	},
}

var regeReplaceCmd = &cobra.Command{
	Use:   "replace",
	Short: "RegExp replace",
	Long:  `Regular Expression replace`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		exp := getStringFlag(cmd, "expression", true)
		rep := getStringFlag(cmd, "replace", true)

		re, err := regexp.Compile(exp)
		exitWithError(cmd, err)

		out := re.ReplaceAllString(str, rep)
		outputString(cmd, out)
	},
}

func init() {
	regeCmd.PersistentFlags().StringP("expression", "e", "", "Expression")
	regeReplaceCmd.PersistentFlags().StringP("replace", "r", "", "Replace")
	regeCmd.AddCommand(regeReplaceCmd)
	rootCmd.AddCommand(regeCmd)
}
