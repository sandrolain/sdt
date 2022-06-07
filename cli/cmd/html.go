package cmd

import (
	"html"

	"github.com/spf13/cobra"
)

var htmlCmd = &cobra.Command{
	Use:   "html",
	Short: "HTML Tools",
	Long:  `HTML Tools`,
}

var htmlEncCmd = &cobra.Command{
	Use:     "encode",
	Aliases: []string{"enc"},
	Short:   "HTML Encode",
	Long:    `HTML Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		res := html.EscapeString(str)
		outputString(cmd, res)
	},
}

var htmlDecCmd = &cobra.Command{
	Use:     "decode",
	Aliases: []string{"dec"},
	Short:   "HTML Decode",
	Long:    `HTML Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		str = html.UnescapeString(str)
		outputString(cmd, str)
	},
}

func init() {
	htmlCmd.AddCommand(htmlEncCmd)
	htmlCmd.AddCommand(htmlDecCmd)
	rootCmd.AddCommand(htmlCmd)
}
