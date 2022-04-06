package cmd

import (
	"fmt"
	"os"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var bytesCmd = &cobra.Command{
	Use:   "bytes",
	Short: "Random Bytes",
	Long:  `Generate Random Bytes`,
	Run: func(cmd *cobra.Command, args []string) {
		size, err := cmd.Flags().GetInt("size")
		exitWithError(err)

		bytes := utils.RandomBytes(size)
		os.Stdout.Write(bytes)
	},
}

var bytesDecCmd = &cobra.Command{
	Use:   "dec",
	Short: "Dec Encoding",
	Long:  `Decimal Encoding`,
	Run: func(cmd *cobra.Command, args []string) {
		size, err := cmd.Flags().GetInt("size")
		exitWithError(err)

		bytes := utils.RandomBytes(size)
		for _, i := range bytes {
			fmt.Printf("%v ", i)
		}
	},
}

func init() {
	bytesCmd.PersistentFlags().IntP("size", "s", 32, "Size of random bytes sequence")
	bytesCmd.AddCommand(bytesDecCmd)
	rootCmd.AddCommand(bytesCmd)
}
