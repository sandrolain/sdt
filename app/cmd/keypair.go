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

type KeyPair struct {
	PrivateKey string `json:"private"`
	PublicKey  string `json:"public"`
}

func generateKeyPair() (*KeyPair, error) {
	pair := KeyPair{}
	// generate key
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	publickey := &privatekey.PublicKey

	// dump private key to file
	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateB := new(bytes.Buffer)
	err = pem.Encode(privateB, privateKeyBlock)
	if err != nil {
		return nil, err
	}

	// dump public key to file
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publickey)
	if err != nil {
		return nil, err
	}
	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	publicB := new(bytes.Buffer)
	err = pem.Encode(publicB, publicKeyBlock)
	if err != nil {
		return nil, err
	}

	pair.PrivateKey = privateB.String()
	pair.PublicKey = publicB.String()

	return &pair, nil
}

var keypairCmd = &cobra.Command{
	Use:     "keypair",
	Aliases: []string{"kp"},
	Short:   "Key pair PEMs",
	Long:    `Generate key pair PEMs`,
	Run: func(cmd *cobra.Command, args []string) {
		pair := must(generateKeyPair())
		res := must(json.Marshal(pair))
		outputBytes(cmd, res)
	},
}

func init() {
	rootCmd.AddCommand(keypairCmd)
}
