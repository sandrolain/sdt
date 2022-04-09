package cmd

import (
	"fmt"
	"strings"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var jsonCmd = &cobra.Command{
	Use:   "json",
	Short: "JSON Tools",
	Long:  `JSON Tools`,
}

var jsonPrettyCmd = &cobra.Command{
	Use:   "pretty",
	Short: "Prettify JSON",
	Long:  `Prettify JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		byt, err := utils.PrettifyJSON(str)
		exitWithError(err)

		fmt.Print(string(byt))
	},
}

var jsonMinifyCmd = &cobra.Command{
	Use:   "minify",
	Short: "Minify JSON",
	Long:  `Minify JSON`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		byt, err := utils.MinifyJSON(str)
		exitWithError(err)

		fmt.Print(string(byt))
	},
}

var jsonValidCmd = &cobra.Command{
	Use:   "valid",
	Short: "Validate JWT",
	Long:  `Validare JWT`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		secret, err := cmd.Flags().GetBytesBase64("secret")
		exitWithError(err)
		issuer, err := cmd.Flags().GetString("issuer")
		exitWithError(err)

		parts := strings.Split(str, ".")
		if len(parts) != 3 {
			exitWithError(fmt.Errorf("invalid JWT parts number: %v", len(parts)))
		}

		err = utils.ValidateJWT(str, issuer, secret)
		exitWithError(err)
	},
}

func init() {
	jsonCmd.AddCommand(jsonPrettyCmd)
	jsonCmd.AddCommand(jsonMinifyCmd)
	jsonCmd.AddCommand(jsonValidCmd)

	rootCmd.AddCommand(jsonCmd)
}
