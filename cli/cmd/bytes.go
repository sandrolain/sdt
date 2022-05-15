package cmd

import (
	"fmt"
	"strings"

	"github.com/sandrolain/sdt/cli/utils"
	"github.com/spf13/cobra"
)

var bytesCmd = &cobra.Command{
	Use:   "bytes",
	Short: "Random Bytes",
	Long:  `Generate Random Bytes`,
	Run: func(cmd *cobra.Command, args []string) {
		size := getIntFlag(cmd, "size", false)
		byt := utils.RandomBytes(size)
		outputBytes(cmd, byt)
	},
}

var decCmd = &cobra.Command{
	Use:   "dec",
	Short: "Decimal Encoding",
	Long:  `Decimal Encoding`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		sep := getStringFlag(cmd, "separator", false)
		res := make([]string, len(byt))
		if sep == "" {
			for i, b := range byt {
				res[i] = fmt.Sprintf("%03d", b)
			}
		} else {
			for i, b := range byt {
				res[i] = fmt.Sprint(b)
			}
		}
		outputString(cmd, strings.Join(res, sep))
	},
}

func init() {
	bytesCmd.PersistentFlags().IntP("size", "s", 32, "Size of random bytes sequence")
	rootCmd.AddCommand(bytesCmd)
	decCmd.PersistentFlags().StringP("separator", "s", " ", "Separator string")
	rootCmd.AddCommand(decCmd)
}
