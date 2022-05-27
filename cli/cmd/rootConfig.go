//go:build !wasm

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
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

		return string(must(ioutil.ReadFile(file)))
	}

	input := getBoolFlag(cmd, "input", false)

	if input {
		byt := must(ioutil.ReadAll(os.Stdin))
		if len(byt) > 0 {
			return string(byt)
		}
	}

	fi := must(os.Stdin.Stat())

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return ""
		}
		if len(byt) > 0 {
			return string(byt)
		}
	}

	if len(args) > 0 {
		return args[0]
	}

	return ""
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	file := getStringFlag(cmd, "file", false)

	if file != "" {
		exist := must(fileExists(file))
		if !exist {
			exitWithError(fmt.Errorf(`file "%s" not exist`, file))
		}
		return must(ioutil.ReadFile(file))
	}

	input := getBoolFlag(cmd, "input", false)

	if input {
		return must(ioutil.ReadAll(os.Stdin))
	}

	fi := must(os.Stdin.Stat())

	if fi.Mode()&os.ModeNamedPipe != 0 {
		byt := must(ioutil.ReadAll(os.Stdin))

		if len(byt) > 0 {
			return byt
		}
	}

	if len(args) > 0 {
		return []byte(args[0])
	}

	return []byte{}
}

func ExecuteByArgs(args []string, in []byte) ([]byte, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origIn := os.Stdin
	os.Stdin = r
	w.Write(in)
	w.Close()

	buf := new(bytes.Buffer)
	rootCmd.SetOutput(buf)
	rootCmd.SetArgs(args)

	err = rootCmd.Execute()
	os.Stdin = origIn
	rootCmd.SetOutput(os.Stdout)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
