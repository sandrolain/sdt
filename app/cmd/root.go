package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sdt",
	Short: "Smart Developer Tools",
	Long:  `Smart Developer Tools is a collection of CLI utilities for developers`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getInputString(args []string) (string, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		if len(byt) > 0 {
			return string(byt), nil
		}
	}

	if len(args) > 0 {
		return args[0], nil
	}

	return "", nil
}

func getInputBytes(args []string) ([]byte, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return byt, err
		}
		if len(byt) > 0 {
			return byt, nil
		}
	}

	if len(args) > 0 {
		return []byte(args[0]), nil
	}

	return []byte{}, nil
}

func getInputBytesRequired(args []string) ([]byte, error) {
	res, err := getInputBytes(args)
	if err != nil {
		return res, err
	}
	if len(res) == 0 {
		return res, fmt.Errorf("primary command input should not be empty")
	}
	return res, err
}

func exitWithError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func exitWithErrorF(f string, err error) {
	if err != nil {
		log.Fatalf(f, err)
	}
}

func getIntFlag(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	exitWithError(err)
	return val
}

func getUintFlag(cmd *cobra.Command, name string) uint {
	val, err := cmd.Flags().GetUint(name)
	exitWithError(err)
	return val
}

func getBoolFlag(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	exitWithError(err)
	return val
}

func getStringFlag(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	exitWithError(err)
	return val
}

func getStringArrayFlag(cmd *cobra.Command, name string) []string {
	val, err := cmd.Flags().GetStringArray(name)
	exitWithError(err)
	return val
}
