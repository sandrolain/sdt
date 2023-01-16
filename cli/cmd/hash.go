package cmd

import (
	//#nosec G501 -- implementation of generic utility
	"crypto/md5"
	//#nosec G505 -- implementation of generic utility
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"

	"github.com/spf13/cobra"
)

var sha1Cmd = &cobra.Command{
	Use:   "sha1",
	Short: "SHA-1",
	Long:  `Generate SHA-1`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		//#nosec G401 -- implementation of generic utility
		res := sha1.Sum(byt)
		outputBytes(cmd, res[:])
	},
}

var sha256Cmd = &cobra.Command{
	Use:   "sha256",
	Short: "SHA-256",
	Long:  `Generate SHA-256`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		h := sha256.New()
		h.Write(byt)
		res := h.Sum(nil)
		outputBytes(cmd, res)
	},
}

var sha384Cmd = &cobra.Command{
	Use:   "sha384",
	Short: "SHA-384",
	Long:  `Generate SHA-384`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		h := sha512.New384()
		h.Write(byt)
		res := h.Sum(nil)
		outputBytes(cmd, res)
	},
}

var sha512Cmd = &cobra.Command{
	Use:   "sha512",
	Short: "SHA-512",
	Long:  `Generate SHA-512`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		h := sha512.New()
		h.Write(byt)
		res := h.Sum(nil)
		outputBytes(cmd, res)
	},
}

var md5Cmd = &cobra.Command{
	Use:   "md5",
	Short: "MD5",
	Long:  `Generate MD5`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		//#nosec G401 -- implementation of generic utility
		res := md5.Sum(byt)
		outputBytes(cmd, res[:])
	},
}

func init() {
	rootCmd.AddCommand(sha1Cmd)
	rootCmd.AddCommand(sha256Cmd)
	rootCmd.AddCommand(sha384Cmd)
	rootCmd.AddCommand(sha512Cmd)
	rootCmd.AddCommand(md5Cmd)
}
