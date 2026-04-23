package cmd

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// HMACResult is the structured output for the hmac command.
type HMACResult struct {
	Algorithm string `json:"algorithm" yaml:"algorithm"`
	Hex       string `json:"hex"       yaml:"hex"`
	Key       string `json:"key"       yaml:"key"`
}

// computeHMAC computes an HMAC with the given algorithm over data using key.
func computeHMAC(data []byte, key []byte, algo string) ([]byte, error) {
	var h func() hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New
	case "sha512":
		h = sha512.New
	case "sha384":
		h = sha512.New384
	default:
		return nil, fmt.Errorf("unsupported algorithm %q; supported: sha256, sha512, sha384", algo)
	}
	mac := hmac.New(h, key)
	mac.Write(data)
	return mac.Sum(nil), nil
}

var hmacCmd = &cobra.Command{
	Use:   "hmac",
	Short: "Compute HMAC of input using a secret key",
	Long: `Compute HMAC (Hash-based Message Authentication Code) of input data.

Useful for verifying webhook signatures and message authenticity.

Algorithms: sha256 (default), sha512, sha384

Examples:
  echo -n "payload" | sdt hmac --key "secret"
  echo -n "payload" | sdt hmac --key "secret" --algo sha512
  echo -n "payload" | sdt hmac --key "secret" --format json
  sdt hmac --key "secret" --algo sha256 --file payload.bin`,
	Run: func(cmd *cobra.Command, args []string) {
		key := getStringFlag(cmd, "key", true)
		algo := getStringFlag(cmd, "algo", false)
		if algo == "" {
			algo = "sha256"
		}
		format := getFormat(cmd)

		data := getInputBytes(cmd, args)
		mac, err := computeHMAC(data, []byte(key), algo)
		exitWithError(cmd, err)

		hexStr := hex.EncodeToString(mac)

		switch format {
		case fmtJSON:
			result := HMACResult{Algorithm: algo, Hex: hexStr, Key: key}
			out, merr := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			result := HMACResult{Algorithm: algo, Hex: hexStr, Key: key}
			out, merr := yaml.Marshal(result)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, hexStr)
		}
	},
}

func init() {
	hmacCmd.Flags().String("key", "", "Secret key for HMAC (required)")
	hmacCmd.Flags().String("algo", "sha256", "Hash algorithm: sha256|sha512|sha384")
	rootCmd.AddCommand(hmacCmd)
}
