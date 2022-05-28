package cmd

import (
	"github.com/spf13/cobra"

	"bytes"
	"os"
)

func loadFileConfig() {
	return
}

func getInputString(cmd *cobra.Command, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

var stdIn []byte

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	if len(stdIn) > 0 {
		return stdIn
	}
	if len(args) > 0 {
		return []byte(args[0])
	}
	return []byte{}
}

func ExecuteByArgs(args []string, in []byte) ([]byte, error) {
	stdIn = in
	origOut := rootCmd.OutOrStdout()
	buf := new(bytes.Buffer)
	rootCmd.SetOutput(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	stdIn = []byte{}
	rootCmd.SetOutput(origOut)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
