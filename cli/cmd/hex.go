package cmd

import (
	"encoding/hex"

	"github.com/spf13/cobra"
)

var hexCmd = &cobra.Command{
	Use:   "hex",
	Short: "Hex Encoding",
	Long:  `Hexadecimal Encoding`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		str := hex.EncodeToString(byt)
		outputString(cmd, str)
	},
}

var hexDecCmd = &cobra.Command{
	Use:   "dec",
	Short: "Hex Decoding",
	Long:  `Hexadecimal Decoding`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		byt, err := hex.DecodeString(str)
		exitWithError(cmd, err)
		outputBytes(cmd, byt)
	},
}

func init() {
	hexCmd.AddCommand(hexDecCmd)
	rootCmd.AddCommand(hexCmd)
}
