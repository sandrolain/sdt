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

func generateKeyPair() *[]string {
	pair := make([]string, 2)
	// generate key
	privatekey := must(rsa.GenerateKey(rand.Reader, 2048))
	publickey := &privatekey.PublicKey

	// dump private key to file
	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateB := new(bytes.Buffer)
	exitWithError(pem.Encode(privateB, privateKeyBlock))

	// dump public key to file
	publicKeyBytes := must(x509.MarshalPKIXPublicKey(publickey))
	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	publicB := new(bytes.Buffer)
	exitWithError(pem.Encode(publicB, publicKeyBlock))

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
		pair := generateKeyPair()
		res := must(json.Marshal(pair))
		outputBytes(cmd, res)
	},
}

func init() {
	rootCmd.AddCommand(keypairCmd)
}
