package cmd

import (
	//#nosec G501 -- implementation of generic utility
	"crypto/md5"
	//#nosec G505 -- implementation of generic utility
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"

	"github.com/spf13/cobra"
)

// hashDigest prints one hash verb's digest: --hex is the shasum shape an
// agent compares with, the default stays the raw bytes.
func hashDigest(cmd *cobra.Command, digest []byte) {
	if getBoolFlag(cmd, "hex", false) {
		outputString(cmd, hex.EncodeToString(digest)+"\n")
		return
	}
	outputBytes(cmd, digest)
}

var sha1Cmd = &cobra.Command{
	Use:   "sha1",
	Short: "SHA-1",
	Long: `Generate SHA-1.

` + "`--hex`" + ` prints the hexadecimal form (the ` + "`shasum`" + ` shape); the default is
the raw digest bytes.`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		//#nosec G401 -- implementation of generic utility
		res := sha1.Sum(byt)
		hashDigest(cmd, res[:])
	},
}

var sha256Cmd = &cobra.Command{
	Use:   "sha256",
	Short: "SHA-256",
	Long: `Generate SHA-256.

` + "`--hex`" + ` prints the hexadecimal form (the ` + "`shasum`" + ` shape); the default is
the raw digest bytes.

EXAMPLE — ` + "`sdt sha256 fixture.bin --hex`" + ``,
	Example: `  sdt sha256 fixture.bin --hex`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		h := sha256.New()
		h.Write(byt)
		hashDigest(cmd, h.Sum(nil))
	},
}

var sha384Cmd = &cobra.Command{
	Use:   "sha384",
	Short: "SHA-384",
	Long: `Generate SHA-384.

` + "`--hex`" + ` prints the hexadecimal form (the ` + "`shasum`" + ` shape); the default is
the raw digest bytes.`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		h := sha512.New384()
		h.Write(byt)
		hashDigest(cmd, h.Sum(nil))
	},
}

var sha512Cmd = &cobra.Command{
	Use:   "sha512",
	Short: "SHA-512",
	Long: `Generate SHA-512.

` + "`--hex`" + ` prints the hexadecimal form (the ` + "`shasum`" + ` shape); the default is
the raw digest bytes.`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		h := sha512.New()
		h.Write(byt)
		hashDigest(cmd, h.Sum(nil))
	},
}

var md5Cmd = &cobra.Command{
	Use:   "md5",
	Short: "MD5",
	Long: `Generate MD5.

` + "`--hex`" + ` prints the hexadecimal form; the default is the raw digest bytes.`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)
		//#nosec G401 -- implementation of generic utility
		res := md5.Sum(byt)
		hashDigest(cmd, res[:])
	},
}

func init() {
	for _, c := range []*cobra.Command{sha1Cmd, sha256Cmd, sha384Cmd, sha512Cmd, md5Cmd} {
		c.Flags().Bool("hex", false, "Print the hexadecimal digest (the shasum shape) instead of the raw bytes")
		rootCmd.AddCommand(c)
	}
}
