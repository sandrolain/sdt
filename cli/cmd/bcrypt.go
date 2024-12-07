package cmd

import (
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var bcryptCmd = &cobra.Command{
	Use:   "bcrypt",
	Short: "Bcrypt password",
	Long:  `Generate Bcrypt hash for password`,
	Run: func(cmd *cobra.Command, args []string) {
		value := getInputBytesRequired(cmd, args)
		cost := getIntFlag(cmd, "cost", false)
		bcrypt, err := bcrypt.GenerateFromPassword(value, cost)
		exitWithError(cmd, err)
		outputBytes(cmd, bcrypt)
	},
}

var bcryptVerifyCmd = &cobra.Command{
	Use:     "verify",
	Aliases: []string{"check", "compare"},
	Short:   "Verify Bcrypt",
	Long:    `Verify password to match Bcrypt hash`,
	Run: func(cmd *cobra.Command, args []string) {
		value := getInputBytesRequired(cmd, args)
		pwd := getStringFlag(cmd, "password", true)
		err := bcrypt.CompareHashAndPassword(value, []byte(pwd))
		exitWithError(cmd, err)
	},
}

func init() {
	bcryptCmd.PersistentFlags().IntP("cost", "c", 14, "Bcrypt generation cost")
	bcryptVerifyCmd.PersistentFlags().StringP("password", "p", "", "Password to verify")
	bcryptCmd.AddCommand(bcryptVerifyCmd)
	rootCmd.AddCommand(bcryptCmd)
}
