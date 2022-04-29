package cmd

import (
	"strings"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "URL Tools",
	Long:  `URL Tools`,
}

var urlEncCmd = &cobra.Command{
	Use:   "enc",
	Short: "URL Encode",
	Long:  `URL Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		res := utils.URLEncode(str)
		res = strings.ReplaceAll(res, "+", "%20")
		cmd.OutOrStdout().Write([]byte(res))
	},
}

var urlEncFormCmd = &cobra.Command{
	Use:   "encform",
	Short: "Form URL Encode",
	Long:  `Form URL Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		res := utils.URLEncode(str)
		cmd.OutOrStdout().Write([]byte(res))
	},
}

var urlDecCmd = &cobra.Command{
	Use:   "dec",
	Short: "URL Decode",
	Long:  `URL Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		str, err := utils.URLDecode(str)
		exitWithError(err)
		cmd.OutOrStdout().Write([]byte(str))
	},
}

func init() {
	urlCmd.AddCommand(urlEncCmd)
	urlCmd.AddCommand(urlEncFormCmd)
	urlCmd.AddCommand(urlDecCmd)
	rootCmd.AddCommand(urlCmd)
}
