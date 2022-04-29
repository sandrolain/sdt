package cmd

import (
	"fmt"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var hexCmd = &cobra.Command{
	Use:   "hex",
	Short: "Hex Encoding",
	Long:  `Hexadecimal Encoding`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		fmt.Printf("byt: %v\n", byt)
		str := utils.HexEncode(byt)
		cmd.OutOrStdout().Write([]byte(str))
	},
}

var hexDecCmd = &cobra.Command{
	Use:   "dec",
	Short: "Hex Decoding",
	Long:  `Hexadecimal Decoding`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt, err := utils.HexDecode(str)
		exitWithError(err)
		cmd.OutOrStdout().Write(byt)
	},
}

func init() {
	hexCmd.AddCommand(hexDecCmd)
	rootCmd.AddCommand(hexCmd)
}
