package cmd

import (
	"github.com/sandrolain/sdt/cli/utils"
	"github.com/spf13/cobra"
)

var jsonCmd = &cobra.Command{
	Use:   "json",
	Short: "JSON Tools",
	Long:  `JSON Tools`,
}

var jsonPrettyCmd = &cobra.Command{
	Use:   "pretty",
	Short: "Prettify JSON",
	Long:  `Prettify JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt, err := utils.PrettifyJSON(str)
		exitWithError(cmd, err)
		outputBytes(cmd, byt)
	},
}

var jsonMinifyCmd = &cobra.Command{
	Use:   "minify",
	Short: "Minify JSON",
	Long:  `Minify JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt, err := utils.MinifyJSON(str)
		exitWithError(cmd, err)
		outputBytes(cmd, byt)
	},
}

var jsonValidCmd = &cobra.Command{
	Use:   "valid",
	Short: "Validate JSON",
	Long:  `Validare JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		exitWithError(cmd, utils.ValidJSON(str))
		outputString(cmd, str)
	},
}

func init() {
	jsonCmd.AddCommand(jsonPrettyCmd)
	jsonCmd.AddCommand(jsonMinifyCmd)
	jsonCmd.AddCommand(jsonValidCmd)
	rootCmd.AddCommand(jsonCmd)
}
