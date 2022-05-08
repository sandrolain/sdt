package cmd

import (
	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var b32Cmd = &cobra.Command{
	Use:   "b32",
	Short: "B32 Encode",
	Long:  `Base 32 Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		outputString(cmd, utils.Base32Encode(byt))
	},
}

var b32DecCmd = &cobra.Command{
	Use:   "dec",
	Short: "B32 Decode",
	Long:  `Base 32 Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt := must(utils.Base32Decode(str))
		outputBytes(cmd, byt)
	},
}

func init() {
	rootCmd.AddCommand(b32Cmd)
	b32Cmd.AddCommand(b32DecCmd)
}
