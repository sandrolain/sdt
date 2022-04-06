package cmd

import (
	"fmt"
	"strings"

	"github.com/sandrolain/sdt/app/utils"
	"github.com/spf13/cobra"
)

var jwtCmd = &cobra.Command{
	Use:   "jwt",
	Short: "jwt Tools",
	Long:  `jwt Tools`,
}

var jwtParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse JWT",
	Long:  `Parse JWT and return JWT parts`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		pretty, err := cmd.Flags().GetBool("pretty")
		exitWithError(err)

		parts := strings.Split(str, ".")
		if len(parts) != 3 {
			exitWithError(fmt.Errorf("invalid JWT parts number: %v", len(parts)))
		}

		fmt.Println("HEAD:")
		byt, err := utils.Base64URLNoPaddingDecode(parts[0])
		exitWithError(err)
		if pretty {
			byt, err = utils.PrettifyJSON(string(byt))
			exitWithError(err)
		}
		fmt.Println(string(byt))

		fmt.Println("\nCLAIMS:")
		byt, err = utils.Base64URLNoPaddingDecode(parts[1])
		exitWithError(err)
		if pretty {
			byt, err = utils.PrettifyJSON(string(byt))
			exitWithError(err)
		}
		fmt.Println(string(byt))

		fmt.Println("\nSIGNATURE:")
		fmt.Println(parts[2])

	},
}

var jwtClaimsCmd = &cobra.Command{
	Use:   "claims",
	Short: "Get JWT claims",
	Long:  `Parse JWT and return JWT claims`,
	Run: func(cmd *cobra.Command, args []string) {
		str, err := getInputString(args)
		exitWithError(err)

		pretty, err := cmd.Flags().GetBool("pretty")
		exitWithError(err)

		parts := strings.Split(str, ".")
		if len(parts) != 3 {
			exitWithError(fmt.Errorf("invalid JWT parts number: %v", len(parts)))
		}

		byt, err := utils.Base64URLNoPaddingDecode(parts[1])
		exitWithError(err)
		if pretty {
			byt, err = utils.PrettifyJSON(string(byt))
			exitWithError(err)
		}
		fmt.Println(string(byt))
	},
}

var jwtValidCmd = &cobra.Command{
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
	jwtCmd.PersistentFlags().BoolP("pretty", "p", false, "Pretty print JSON structures")

	jwtValidCmd.PersistentFlags().BytesBase64P("secret", "s", nil, "Signature secret for JWT validation")
	jwtValidCmd.PersistentFlags().StringP("issuer", "i", "", "Issuer for JWT validation")
	jwtValidCmd.MarkPersistentFlagRequired("secret")

	jwtCmd.AddCommand(jwtParseCmd)
	jwtCmd.AddCommand(jwtClaimsCmd)
	jwtCmd.AddCommand(jwtValidCmd)

	rootCmd.AddCommand(jwtCmd)
}
