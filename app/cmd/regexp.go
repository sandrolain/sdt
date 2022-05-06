package cmd

import (
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
		byt := getInputBytes(cmd, args)
		exp := getStringFlag(cmd, "expression", true)

		re := must(regexp.Compile(exp))

		if !re.Match(byt) {
			exitWithError(fmt.Errorf(`input not match "%s"`, exp))
		}
	},
}

func init() {
	regeCmd.PersistentFlags().StringP("expression", "e", "", "Expression")

	rootCmd.AddCommand(regeCmd)
}
