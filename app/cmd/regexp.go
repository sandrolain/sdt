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
		byt, err := getInputBytes(args)
		exitWithError(err)

		exp, err := cmd.Flags().GetString("expression")
		exitWithError(err)

		re, err := regexp.Compile(exp)
		exitWithError(err)

		if !re.Match(byt) {
			exitWithError(fmt.Errorf(`input not match "%s"`, exp))
		}
	},
}

func init() {
	regeCmd.PersistentFlags().StringP("expression", "e", "", "Expression")
	regeCmd.MarkPersistentFlagRequired("expression")

	rootCmd.AddCommand(regeCmd)
}
