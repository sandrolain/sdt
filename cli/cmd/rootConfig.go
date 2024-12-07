//go:build !wasm

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

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
			exitWithError(nil, err)
		}
	}
}

func getInputString(cmd *cobra.Command, args []string) string {
	flags := cmd.Flags()

	if flags.Lookup("file").Changed {
		file := getStringFlag(cmd, "file", false)

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}

		//#nosec G304 -- implementation of generic utility
		res, err := os.ReadFile(file)
		exitWithError(cmd, err)
		return string(res)
	}

	if flags.Lookup("input").Changed {
		return getStringFlag(cmd, "input", true)
	}

	if flags.Lookup("inb64").Changed {
		return string(getBytesBase64Flag(cmd, "inb64", true))
	}

	if len(args) > 0 {
		return strings.Join(args[:], "")
	}
	res, err := io.ReadAll(cmd.InOrStdin())
	exitWithError(cmd, err)
	return string(res)
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	flags := cmd.Flags()

	if flags.Lookup("file").Changed {
		file := getStringFlag(cmd, "file", false)

		exist, err := fileExists(file)
		exitWithError(cmd, err)
		if !exist {
			exitWithError(cmd, fmt.Errorf(`file "%s" not exist`, file))
		}
		//#nosec G304 -- implementation of generic utility
		res, err := os.ReadFile(file)
		exitWithError(cmd, err)
		return res
	}

	if flags.Lookup("input").Changed {
		return []byte(getStringFlag(cmd, "input", true))
	}

	if flags.Lookup("inb64").Changed {
		return getBytesBase64Flag(cmd, "inb64", true)
	}

	if len(args) > 0 {
		return []byte(strings.Join(args[:], ""))
	}

	res, err := io.ReadAll(cmd.InOrStdin())
	exitWithError(cmd, err)
	return res
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
