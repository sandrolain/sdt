package cmd

import (
	"fmt"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var b32Cmd = &cobra.Command{
	Use:   "b32",
	Short: "B32 Encode",
	Long:  `Base 32 Encode`,
	Run: func(cmd *cobra.Command, args []string) {
		byt, err := getInputBytes(args)
		exitWithError(err)

		fmt.Print(utils.Base32Encode(byt))
	},
}

var b32DecCmd = &cobra.Command{
	Use:   "dec",
	Short: "B32 Decode",
	Long:  `Base 32 Decode`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		byt, err := utils.Base32Decode(str)
		exitWithError(err)

		fmt.Print(string(byt))
	},
}

func init() {
	rootCmd.AddCommand(b32Cmd)
	b32Cmd.AddCommand(b32DecCmd)
}
