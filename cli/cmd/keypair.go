package cmd

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"

	"github.com/spf13/cobra"
)

func generateKeyPair(cmd *cobra.Command) *[]string {
	pair := make([]string, 2)
	// generate key
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	exitWithError(cmd, err)
	publickey := &privatekey.PublicKey

	// dump private key to file
	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateB := new(bytes.Buffer)
	exitWithError(cmd, pem.Encode(privateB, privateKeyBlock))

	// dump public key to file
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publickey)
	exitWithError(cmd, err)
	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	publicB := new(bytes.Buffer)
	exitWithError(cmd, pem.Encode(publicB, publicKeyBlock))

	pair[0] = privateB.String()
	pair[1] = publicB.String()

	return &pair
}

var keypairCmd = &cobra.Command{
	Use:     "keypair",
	Aliases: []string{"kp"},
	Short:   "Key pair PEMs",
	Long:    `Generate key pair PEMs (x509 PKCS1/PKIX)`,
	Run: func(cmd *cobra.Command, args []string) {
		pair := generateKeyPair(cmd)
		res, err := json.Marshal(pair)
		exitWithError(cmd, err)
		outputBytes(cmd, res)
	},
}

func init() {
	rootCmd.AddCommand(keypairCmd)
}
