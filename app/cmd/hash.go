package cmd

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"os"

	"github.com/spf13/cobra"
)

var sha1Cmd = &cobra.Command{
	Use:   "sha1",
	Short: "SHA-1",
	Long:  `Generate SHA-1`,
	Run: func(cmd *cobra.Command, args []string) {
		byt, err := getInputBytes(args)
		exitWithError(err)

		res := sha1.Sum(byt)
		os.Stdout.Write(res[:])
	},
}

var sha256Cmd = &cobra.Command{
	Use:   "sha256",
	Short: "SHA-256",
	Long:  `Generate SHA-256`,
	Run: func(cmd *cobra.Command, args []string) {
		byt, err := getInputBytes(args)
		exitWithError(err)

		h := sha256.New()
		h.Write(byt)
		res := h.Sum(nil)
		os.Stdout.Write(res)
	},
}

var sha384Cmd = &cobra.Command{
	Use:   "sha384",
	Short: "SHA-384",
	Long:  `Generate SHA-384`,
	Run: func(cmd *cobra.Command, args []string) {
		byt, err := getInputBytes(args)
		exitWithError(err)

		h := sha512.New384()
		h.Write(byt)
		res := h.Sum(nil)
		os.Stdout.Write(res)
	},
}

var sha512Cmd = &cobra.Command{
	Use:   "sha512",
	Short: "SHA-512",
	Long:  `Generate SHA-512`,
	Run: func(cmd *cobra.Command, args []string) {
		byt, err := getInputBytes(args)
		exitWithError(err)

		h := sha512.New()
		h.Write(byt)
		res := h.Sum(nil)
		os.Stdout.Write(res)
	},
}

func init() {
	rootCmd.AddCommand(sha1Cmd)
	rootCmd.AddCommand(sha256Cmd)
	rootCmd.AddCommand(sha384Cmd)
	rootCmd.AddCommand(sha512Cmd)
}
