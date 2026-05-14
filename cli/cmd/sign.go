package cmd

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

const (
	signAlgoRSASHA256   = "rsa-sha256"
	signAlgoRSASHA512   = "rsa-sha512"
	signAlgoECDSASHA256 = "ecdsa-sha256"
	signAlgoECDSASHA512 = "ecdsa-sha512"
	signAlgoED25519     = "ed25519"
)

// loadPrivateKey parses a PEM-encoded private key.
func loadPrivateKey(path string) (crypto.Signer, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- user-controlled key file
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key file")
	}
	switch block.Type {
	case pemTypeRSAPrivateKey:
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case pemTypePrivateKey: // PKCS#8
		key, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, parseErr
		}
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key does not implement crypto.Signer")
		}
		return signer, nil
	default:
		return nil, fmt.Errorf("unsupported PEM key type: %s", block.Type)
	}
}

// loadPublicKey parses a PEM-encoded public key.
func loadPublicKey(path string) (crypto.PublicKey, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- user-controlled key file
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key file")
	}
	switch block.Type {
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case pemTypePublicKey:
		return x509.ParsePKIXPublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported PEM key type: %s", block.Type)
	}
}

// hashForAlgo returns a digest and hash.Hash identifier for the given algo.
func hashForAlgo(algo string, data []byte) ([]byte, crypto.Hash, error) {
	switch algo {
	case signAlgoRSASHA256, signAlgoECDSASHA256:
		d := sha256.Sum256(data)
		return d[:], crypto.SHA256, nil
	case signAlgoRSASHA512, signAlgoECDSASHA512:
		d := sha512.Sum512(data)
		return d[:], crypto.SHA512, nil
	case signAlgoED25519:
		// Ed25519 hashes internally
		return data, 0, nil
	default:
		return nil, 0, fmt.Errorf("unsupported algorithm %q", algo)
	}
}

// signData signs data with the given private key and algorithm.
func signData(data []byte, signer crypto.Signer, algo string) ([]byte, error) {
	switch algo {
	case signAlgoED25519:
		ed25519Key, ok := signer.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not Ed25519")
		}
		return ed25519.Sign(ed25519Key, data), nil
	case signAlgoRSASHA256, signAlgoRSASHA512:
		digest, hashID, err := hashForAlgo(algo, data)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := signer.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not RSA")
		}
		return rsa.SignPKCS1v15(rand.Reader, rsaKey, hashID, digest)
	case signAlgoECDSASHA256, signAlgoECDSASHA512:
		digest, _, err := hashForAlgo(algo, data)
		if err != nil {
			return nil, err
		}
		ecKey, ok := signer.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not ECDSA")
		}
		return ecKey.Sign(rand.Reader, digest, nil)
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", algo)
	}
}

// verifyData verifies a signature against data.
func verifyData(data []byte, sig []byte, pub crypto.PublicKey, algo string) error {
	switch algo {
	case signAlgoED25519:
		edPub, ok := pub.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf("key is not Ed25519")
		}
		if !ed25519.Verify(edPub, data, sig) {
			return fmt.Errorf("signature verification failed")
		}
		return nil
	case signAlgoRSASHA256, signAlgoRSASHA512:
		digest, hashID, err := hashForAlgo(algo, data)
		if err != nil {
			return err
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("key is not RSA")
		}
		return rsa.VerifyPKCS1v15(rsaPub, hashID, digest, sig)
	case signAlgoECDSASHA256, signAlgoECDSASHA512:
		digest, _, err := hashForAlgo(algo, data)
		if err != nil {
			return err
		}
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("key is not ECDSA")
		}
		if !ecdsa.VerifyASN1(ecPub, digest, sig) {
			return fmt.Errorf("signature verification failed")
		}
		return nil
	default:
		return fmt.Errorf("unsupported algorithm %q", algo)
	}
}

var signRootCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign data with a private key",
	Long: `Sign input data using a PEM private key. Outputs a base64-encoded signature.

Supported algorithms:
  rsa-sha256    (default for RSA keys)
  rsa-sha512
  ecdsa-sha256  (default for ECDSA keys)
  ecdsa-sha512
  ed25519

Examples:
  echo -n "payload" | sdt sign --key private.pem
  echo -n "payload" | sdt sign --key private.pem --algo rsa-sha512
  sdt sign --key ed25519.pem --algo ed25519 --file payload.bin`,
	Run: func(cmd *cobra.Command, args []string) {
		keyPath := getStringFlag(cmd, "key", true)
		algo := getStringFlag(cmd, "algo", false)
		format := getFormat(cmd)

		if algo == "" {
			algo = signAlgoRSASHA256
		}

		data := getInputBytes(cmd, args)

		signer, err := loadPrivateKey(keyPath)
		exitWithError(cmd, err)

		sig, err := signData(data, signer, algo)
		exitWithError(cmd, err)

		b64 := base64.StdEncoding.EncodeToString(sig)

		type signResult struct {
			Algorithm string `json:"algorithm" yaml:"algorithm"`
			Signature string `json:"signature" yaml:"signature"`
		}

		switch format {
		case fmtJSON:
			out, merr := json.MarshalIndent(signResult{Algorithm: algo, Signature: b64}, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(signResult{Algorithm: algo, Signature: b64})
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, b64)
		}
	},
}

var verifyCmd = &cobra.Command{
	Use:   cmdVerify,
	Short: "Verify a signature against data using a public key",
	Long: `Verify that the input data matches the given base64-encoded signature
using the provided PEM public key.

Examples:
  echo -n "payload" | sdt verify --key public.pem --sig <base64>
  sdt verify --key public.pem --sig <base64> --algo rsa-sha512 --file payload.bin`,
	Run: func(cmd *cobra.Command, args []string) {
		keyPath := getStringFlag(cmd, "key", true)
		sigB64 := getStringFlag(cmd, "sig", true)
		algo := getStringFlag(cmd, "algo", false)
		format := getFormat(cmd)

		if algo == "" {
			algo = "rsa-sha256"
		}

		data := getInputBytes(cmd, args)

		sig, err := base64.StdEncoding.DecodeString(sigB64)
		exitWithError(cmd, err)

		pub, err := loadPublicKey(keyPath)
		exitWithError(cmd, err)

		verifyErr := verifyData(data, sig, pub, algo)

		type verifyResult struct {
			Valid   bool   `json:"valid"   yaml:"valid"`
			Message string `json:"message" yaml:"message"`
		}

		valid := verifyErr == nil
		msg := cmdValid
		if !valid {
			msg = verifyErr.Error()
		}

		switch format {
		case fmtJSON:
			out, merr := json.MarshalIndent(verifyResult{Valid: valid, Message: msg}, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(verifyResult{Valid: valid, Message: msg})
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if valid {
				outputString(cmd, "valid")
			} else {
				exitWithError(cmd, verifyErr)
			}
		}
	},
}

func init() {
	signRootCmd.Flags().String("key", "", "Path to PEM private key file (required)")
	signRootCmd.Flags().String("algo", "rsa-sha256", "Signing algorithm: rsa-sha256|rsa-sha512|ecdsa-sha256|ecdsa-sha512|ed25519")

	verifyCmd.Flags().String("key", "", "Path to PEM public key file (required)")
	verifyCmd.Flags().String("sig", "", "Base64-encoded signature to verify (required)")
	verifyCmd.Flags().String("algo", "rsa-sha256", "Signing algorithm: rsa-sha256|rsa-sha512|ecdsa-sha256|ecdsa-sha512|ed25519")

	rootCmd.AddCommand(signRootCmd)
	rootCmd.AddCommand(verifyCmd)
}
