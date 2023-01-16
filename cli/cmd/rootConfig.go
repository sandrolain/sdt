//go:build !wasm

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func loadFileConfig() {
	viper.SetConfigName("sdt")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			exitWithError(err)
		}
	}
}

func getInputString(cmd *cobra.Command, args []string) string {
	file := getStringFlag(cmd, "file", false)

	if file != "" {
		exist := must(fileExists(file))
		if !exist {
			exitWithError(fmt.Errorf(`file "%s" not exist`, file))
		}

		//#nosec G304 -- implementation of generic utility
		return string(must(os.ReadFile(file)))
	}

	if len(args) > 0 {
		return args[0]
	}

	byt := must(io.ReadAll(cmd.InOrStdin()))
	return string(byt)
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	file := getStringFlag(cmd, "file", false)

	if file != "" {
		exist := must(fileExists(file))
		if !exist {
			exitWithError(fmt.Errorf(`file "%s" not exist`, file))
		}
		//#nosec G304 -- implementation of generic utility
		return must(os.ReadFile(file))
	}

	if len(args) > 0 {
		return []byte(args[0])
	}

	byt := must(io.ReadAll(cmd.InOrStdin()))
	return byt
}

func ExecuteByArgs(args []string, in []byte) ([]byte, error) {
	inr := bytes.NewReader(in)
	rootCmd.SetIn(inr)

	origOut := rootCmd.OutOrStdout()

	buf := new(bytes.Buffer)
	rootCmd.SetOutput(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	rootCmd.SetIn(nil)
	rootCmd.SetOutput(origOut)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
