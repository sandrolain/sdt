package cmd

import (
	"fmt"
	"os"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var hexCmd = &cobra.Command{
	Use:   "hex",
	Short: "Hex Encoding",
	Long:  `Hexadecimal Encoding`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		str := utils.HexEncode(byt)
		fmt.Print(str)
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
		os.Stdout.Write(byt)
	},
}

func init() {
	hexCmd.AddCommand(hexDecCmd)
	rootCmd.AddCommand(hexCmd)
}
