package cmd

import (
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "URL Tools",
	Long:  `URL Tools`,
}

var urlEncCmd = &cobra.Command{
	Use:     "encode",
	Aliases: []string{"enc"},
	Short:   "URL Encode",
	Long:    `URL Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		res := url.QueryEscape(str)
		res = strings.ReplaceAll(res, "+", "%20")
		outputString(cmd, res)
	},
}

var urlEncFormCmd = &cobra.Command{
	Use:     "formencode",
	Aliases: []string{"formenc"},
	Short:   "Form URL Encode",
	Long:    `Form URL Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		res := url.QueryEscape(str)
		outputString(cmd, res)
	},
}

var urlDecCmd = &cobra.Command{
	Use:     "decode",
	Aliases: []string{"dec"},
	Short:   "URL Decode",
	Long:    `URL Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		str = must(url.QueryUnescape(str))
		outputString(cmd, str)
	},
}

func init() {
	urlCmd.AddCommand(urlEncCmd)
	urlCmd.AddCommand(urlEncFormCmd)
	urlCmd.AddCommand(urlDecCmd)
	rootCmd.AddCommand(urlCmd)
}
