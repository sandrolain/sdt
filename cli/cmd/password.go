package cmd

import (
	"strings"

	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
)

var passwordCmd = &cobra.Command{
	Use:     "password",
	Aliases: []string{"pwd"},
	Short:   "Generate password",
	Long:    `Generate password`,
	Run: func(cmd *cobra.Command, args []string) {
		num := getIntFlag(cmd, "number", false)
		len := getIntFlag(cmd, "length", false)
		dig := getIntFlag(cmd, "digits", false)
		sim := getIntFlag(cmd, "symbols", false)
		unt := getBoolFlag(cmd, "untyped", false)
		rep := getBoolFlag(cmd, "repeat", false)

		if dig < 0 {
			dig = len / 4
		}

		if sim < 0 {
			sim = len / 4
		}

		out := make([]string, num)

		for i := 0; i < num; i++ {
			res, err := password.Generate(len, dig, sim, unt, rep)
			exitWithError(cmd, err)
			out[i] = res
		}
		outputString(cmd, strings.Join(out, "\n"))
	},
}

func init() {
	fg := passwordCmd.PersistentFlags()
	fg.IntP("number", "n", 1, "Number of passwords")
	fg.IntP("length", "l", 16, "Password length")
	fg.IntP("digits", "d", -1, "Digits number")
	fg.IntP("symbols", "s", -1, "Symbols number")
	fg.BoolP("untyped", "u", false, "Only lowercase characters")
	fg.BoolP("repeat", "r", false, "Allow repeated characters")
	rootCmd.AddCommand(passwordCmd)
}
