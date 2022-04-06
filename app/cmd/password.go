package cmd

import (
	"fmt"

	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
)

var passwordCmd = &cobra.Command{
	Use:   "password",
	Short: "Generate password",
	Long:  `Generate password`,
	Run: func(cmd *cobra.Command, args []string) {
		num := getIntFlag(cmd, "number")
		len := getIntFlag(cmd, "length")
		dig := getIntFlag(cmd, "digits")
		sim := getIntFlag(cmd, "symbols")
		unt := getBoolFlag(cmd, "untyped")
		rep := getBoolFlag(cmd, "repeat")

		for i := 0; i < num; i++ {
			if i > 0 {
				fmt.Print("\n")
			}
			res, err := password.Generate(len, dig, sim, unt, rep)
			exitWithError(err)
			fmt.Print(res)
		}
	},
}

func init() {
	passwordCmd.PersistentFlags().IntP("number", "n", 1, "Number of passwords")
	passwordCmd.PersistentFlags().IntP("length", "l", 32, "Password length")
	passwordCmd.PersistentFlags().IntP("digits", "d", 8, "Digits number")
	passwordCmd.PersistentFlags().IntP("symbols", "s", 8, "Symbols number")
	passwordCmd.PersistentFlags().BoolP("untyped", "u", false, "Only lowercase characters")
	passwordCmd.PersistentFlags().BoolP("repeat", "r", false, "Allow repeated characters")

	rootCmd.AddCommand(passwordCmd)
}
