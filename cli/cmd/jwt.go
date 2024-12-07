package cmd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gookit/color"
	"github.com/sandrolain/sdt/cli/utils"
	"github.com/spf13/cobra"
)

var jwtCmd = &cobra.Command{
	Use:   "jwt",
	Short: "JWT Tools",
	Long:  `JWT Tools`,
}

var jwtParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse JWT",
	Long:  `Parse JWT and return JWT parts`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		pretty := getBoolFlag(cmd, "pretty", false)

		parts := strings.Split(str, ".")
		if len(parts) != 3 {
			exitWithError(cmd, fmt.Errorf("invalid JWT parts number: %v", len(parts)))
		}

		out := make([][]byte, 6)

		out[0] = []byte(color.Info.Render("HEAD:\n\n"))
		byt, err := utils.Base64URLNoPaddingDecode(parts[0])
		exitWithError(cmd, err)
		if pretty {
			byt, err = utils.PrettifyJSON(string(byt))
			exitWithError(cmd, err)
		}
		out[1] = byt

		out[2] = []byte(color.Info.Render("\n\nCLAIMS:\n\n"))
		byt, err = utils.Base64URLNoPaddingDecode(parts[1])
		exitWithError(cmd, err)
		if pretty {
			byt, err = utils.PrettifyJSON(string(byt))
			exitWithError(cmd, err)
		}
		out[3] = byt

		out[4] = []byte(color.Info.Render("\n\nSIGNATURE:\n\n"))
		out[5] = []byte(parts[2])

		outputBytes(cmd, bytes.Join(out, []byte{}))
	},
}

var jwtClaimsCmd = &cobra.Command{
	Use:   "claims",
	Short: "Get JWT claims",
	Long:  `Parse JWT and return JWT claims`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		pretty := getBoolFlag(cmd, "pretty", false)

		parts := strings.Split(str, ".")
		if len(parts) != 3 {
			exitWithError(cmd, fmt.Errorf("invalid JWT parts number: %v", len(parts)))
		}

		byt, err := utils.Base64URLNoPaddingDecode(parts[1])
		exitWithError(cmd, err)
		if pretty {
			byt, err = utils.PrettifyJSON(string(byt))
			exitWithError(cmd, err)
		}
		outputBytes(cmd, byt)
	},
}

var jwtValidCmd = &cobra.Command{
	Use:   "valid",
	Short: "Validate JWT",
	Long:  `Validare JWT`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)

		secret := getBytesBase64Flag(cmd, "secret", true)
		issuer := getStringFlag(cmd, "issuer", false)

		parts := strings.Split(str, ".")
		if len(parts) != 3 {
			exitWithError(cmd, fmt.Errorf("invalid JWT parts number: %v", len(parts)))
		}

		err := utils.ValidateJWT(str, issuer, secret)
		exitWithError(cmd, err)
	},
}

func init() {
	jwtCmd.PersistentFlags().BoolP("pretty", "p", false, "Pretty print JSON structures")

	jwtValidCmd.PersistentFlags().BytesBase64P("secret", "s", nil, "Signature secret for JWT validation")
	jwtValidCmd.PersistentFlags().StringP("issuer", "r", "", "Issuer for JWT validation")

	jwtCmd.AddCommand(jwtParseCmd)
	jwtCmd.AddCommand(jwtClaimsCmd)
	jwtCmd.AddCommand(jwtValidCmd)

	rootCmd.AddCommand(jwtCmd)
}
