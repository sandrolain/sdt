package cmd

import (
	"github.com/spf13/cobra"

	"bytes"
	"strings"
)

func loadFileConfig() {
	return
}

var stdIn []byte

func getInputString(cmd *cobra.Command, args []string) string {
	flags := cmd.Flags()

	if flags.Lookup("input").Changed {
		return getStringFlag(cmd, "input", true)
	}

	if flags.Lookup("inb64").Changed {
		return string(getBytesBase64Flag(cmd, "inb64", true))
	}

	if len(args) > 0 {
		return strings.Join(args[:], "")
	}

	if len(stdIn) > 0 {
		return string(stdIn)
	}

	return ""
}

func getInputBytes(cmd *cobra.Command, args []string) []byte {
	flags := cmd.Flags()

	if flags.Lookup("input").Changed {
		return []byte(getStringFlag(cmd, "input", true))
	}

	if flags.Lookup("inb64").Changed {
		return getBytesBase64Flag(cmd, "inb64", true)
	}

	if len(args) > 0 {
		return []byte(strings.Join(args[:], ""))
	}

	if len(stdIn) > 0 {
		return stdIn
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
