package cmd

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"image/png"
	"time"

	"github.com/spf13/cobra"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

var totpCmd = &cobra.Command{
	Use:   "totp",
	Short: "TOTP",
	Long:  `Time-based One Time Password`,
}

func getAlgorithm(a string) otp.Algorithm {
	alg := otp.AlgorithmSHA1
	switch a {
	case "SHA1":
		alg = otp.AlgorithmSHA1
	case "SHA256":
		alg = otp.AlgorithmSHA256
	case "SHA512":
		alg = otp.AlgorithmSHA512
	case "MD5":
		alg = otp.AlgorithmMD5
	}
	return alg
}

func getDigits(d int) otp.Digits {
	dig := otp.DigitsSix
	switch d {
	case 6:
		dig = otp.DigitsSix
	case 8:
		dig = otp.DigitsEight
	}
	return dig
}

var totpUriCmd = &cobra.Command{
	Use:   "uri",
	Short: "Generate URI",
	Long:  `Generate URI`,
	Run: func(cmd *cobra.Command, args []string) {
		secret := getStringFlag(cmd, "secret", true)
		issuer := getStringFlag(cmd, "issuer", true)
		account := getStringFlag(cmd, "account", true)
		algorithm := getStringFlag(cmd, "algorithm", false)
		period := getUintFlag(cmd, "period", false)
		digits := getIntFlag(cmd, "digits", false)

		secretBytes, err := base32.StdEncoding.DecodeString(secret)
		exitWithError(cmd, err)

		alg := getAlgorithm(algorithm)
		dig := getDigits(digits)

		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      issuer,
			AccountName: account,
			Secret:      secretBytes,
			Algorithm:   alg,
			Period:      period,
			Digits:      dig,
		})
		exitWithError(cmd, err)

		outputString(cmd, key.URL())
	},
}

var totpImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Generate QR code Image",
	Long:  `Generate QR code Image`,
	Run: func(cmd *cobra.Command, args []string) {
		secret := getStringFlag(cmd, "secret", true)
		issuer := getStringFlag(cmd, "issuer", true)
		account := getStringFlag(cmd, "account", true)
		algorithm := getStringFlag(cmd, "algorithm", false)
		period := getUintFlag(cmd, "period", false)
		digits := getIntFlag(cmd, "digits", false)

		secretBytes, err := base32.StdEncoding.DecodeString(secret)
		exitWithError(cmd, err)

		alg := getAlgorithm(algorithm)
		dig := getDigits(digits)

		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      issuer,
			AccountName: account,
			Secret:      secretBytes,
			Algorithm:   alg,
			Period:      period,
			Digits:      dig,
		})
		exitWithError(cmd, err)

		// Convert TOTP key into a PNG
		var buf bytes.Buffer
		img, err := key.Image(200, 200)
		exitWithError(cmd, err)

		exitWithError(cmd, png.Encode(&buf, img))
		outputBytes(cmd, buf.Bytes())
	},
}

var totpCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "Generate Code",
	Long:  `Generate Code`,
	Run: func(cmd *cobra.Command, args []string) {
		secret := getStringFlag(cmd, "secret", true)
		algorithm := getStringFlag(cmd, "algorithm", false)
		period := getUintFlag(cmd, "period", false)
		digits := getIntFlag(cmd, "digits", false)

		alg := getAlgorithm(algorithm)
		dig := getDigits(digits)

		passcode, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
			Period:    period,
			Skew:      1,
			Digits:    dig,
			Algorithm: alg,
		})
		exitWithError(cmd, err)

		outputString(cmd, passcode)
	},
}

var totpVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify Code",
	Long:  `Verify Code`,
	Run: func(cmd *cobra.Command, args []string) {
		secret := getStringFlag(cmd, "secret", true)
		code := getStringFlag(cmd, "code", true)

		valid := totp.Validate(code, secret)
		if !valid {
			exitWithError(cmd, fmt.Errorf("invalid code"))
		}

		outputString(cmd, code)
	},
}

func init() {
	pf := totpCmd.PersistentFlags()
	pf.StringP("code", "c", "", "TOTP Code")
	pf.StringP("secret", "s", "", "TOTP Secret (Base 32)")
	pf.StringP("issuer", "r", "", "TOTP Issuer")
	pf.StringP("account", "a", "", "TOTP Account Name")
	pf.UintP("period", "p", 30, "TOTP Period")
	pf.IntP("digits", "d", 6, "TOTP digits (6, 8)")
	pf.StringP("algorithm", "l", "SHA1", "TOTP algorithm (SHA1, SHA256, SHA512, MD5)")

	totpCmd.AddCommand(totpCodeCmd)
	totpCmd.AddCommand(totpVerifyCmd)
	totpCmd.AddCommand(totpUriCmd)
	totpCmd.AddCommand(totpImageCmd)

	rootCmd.AddCommand(totpCmd)
}
