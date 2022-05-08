package cmd

import (
	"github.com/spf13/cobra"

	qrcode "github.com/skip2/go-qrcode"
)

var qrcodeCmd = &cobra.Command{
	Use:   "qrcode",
	Short: "QR code",
	Long:  `Generate QR code`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		size := getIntFlag(cmd, "size", false)
		png := must(qrcode.Encode(str, qrcode.Medium, size))
		outputBytes(cmd, png)
	},
}

func init() {
	qrcodeCmd.PersistentFlags().IntP("size", "s", 256, "Image size")
	rootCmd.AddCommand(qrcodeCmd)
}
