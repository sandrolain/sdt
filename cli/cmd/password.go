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

		out := make([]string, num)

		for i := 0; i < num; i++ {
			res := must(password.Generate(len, dig, sim, unt, rep))
			out[i] = res
		}
		outputString(cmd, strings.Join(out, "\n"))
	},
}

func init() {
	fg := passwordCmd.PersistentFlags()
	fg.IntP("number", "n", 1, "Number of passwords")
	fg.IntP("length", "l", 32, "Password length")
	fg.IntP("digits", "d", 8, "Digits number")
	fg.IntP("symbols", "s", 8, "Symbols number")
	fg.BoolP("untyped", "u", false, "Only lowercase characters")
	fg.BoolP("repeat", "r", false, "Allow repeated characters")
	rootCmd.AddCommand(passwordCmd)
}
