package cmd

import (
	"github.com/sandrolain/sdt/cli/utils"
	"github.com/spf13/cobra"
)

var b64Cmd = &cobra.Command{
	Use:   "b64",
	Short: "B64 Encode",
	Long:  `Base 64 Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		outputString(cmd, utils.Base64Encode(byt))
	},
}

var b64DecCmd = &cobra.Command{
	Use:   "dec",
	Short: "B64 Decode",
	Long:  `Base 64 Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt, err := utils.Base64Decode(str)
		exitWithError(cmd, err)
		outputBytes(cmd, byt)
	},
}

var b64UrlCmd = &cobra.Command{
	Use:   "b64url",
	Short: "B64 URL Encode",
	Long:  `Base 64 URL Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		outputString(cmd, utils.Base64URLEncode(byt))
	},
}

var b64UrlDecCmd = &cobra.Command{
	Use:   "dec",
	Short: "B64 URL Decode",
	Long:  `Base 64 URL Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt, err := utils.Base64URLDecode(str)
		exitWithError(cmd, err)
		outputBytes(cmd, byt)
	},
}

func init() {
	rootCmd.AddCommand(b64Cmd)
	b64Cmd.AddCommand(b64DecCmd)
	rootCmd.AddCommand(b64UrlCmd)
	b64UrlCmd.AddCommand(b64UrlDecCmd)
}
