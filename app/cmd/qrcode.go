package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	qrcode "github.com/skip2/go-qrcode"
)

var qrcodeCmd = &cobra.Command{
	Use:   "qrcode",
	Short: "QR code",
	Long:  `Generate QR code`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)

		size, err := cmd.Flags().GetInt("size")
		exitWithError(err)

		png, err := qrcode.Encode(str, qrcode.Medium, size)
		exitWithError(err)

		fmt.Print(string(png))
	},
}

func init() {
	qrcodeCmd.PersistentFlags().IntP("size", "s", 256, "Image size")

	rootCmd.AddCommand(qrcodeCmd)
}
