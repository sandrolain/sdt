package cmd

import (
	"github.com/sandrolain/sdt/cli/utils"
	"github.com/spf13/cobra"
)

var qrcodeCmd = &cobra.Command{
	Use:     "qrcode",
	Aliases: []string{"qr"},
	Short:   "QR code",
	Long:    `Generate QR code`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		size := getIntFlag(cmd, "size", false)
		png := must(utils.GenerateQRCodeImage(str, size))
		outputBytes(cmd, png)
	},
}

var qrcodeReadCmd = &cobra.Command{
	Use:   "read",
	Short: "QR code read",
	Long:  `Read QR code`,
	Run: func(cmd *cobra.Command, args []string) {
		in := getInputBytes(cmd, args)
		out := must(utils.ReadQRCodeImage(in))
		outputString(cmd, out)
	},
}

func init() {
	qrcodeCmd.PersistentFlags().IntP("size", "s", 256, "Image size")
	qrcodeCmd.AddCommand(qrcodeReadCmd)
	rootCmd.AddCommand(qrcodeCmd)
}
