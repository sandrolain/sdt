package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var bcryptCmd = &cobra.Command{
	Use:   "bcrypt",
	Short: "Bcrypt password",
	Long:  `Generate Bcrypt hash for password`,
	Run: func(cmd *cobra.Command, args []string) {
		value := getInputBytesRequired(cmd, args)
		cost, err := cmd.Flags().GetInt("cost")
		exitWithError(err)
		bcrypt, err := bcrypt.GenerateFromPassword(value, cost)
		exitWithError(err)
		fmt.Print(string(bcrypt))
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
		exitWithError(err)
	},
}

func init() {
	bcryptCmd.PersistentFlags().IntP("cost", "c", 14, "Bcrypt generation cost")
	bcryptVerifyCmd.PersistentFlags().StringP("password", "p", "", "Password to verify")
	bcryptVerifyCmd.MarkPersistentFlagRequired("password")

	bcryptCmd.AddCommand(bcryptVerifyCmd)

	rootCmd.AddCommand(bcryptCmd)
}
