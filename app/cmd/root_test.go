package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func execute(t *testing.T, c *cobra.Command, in []byte, args ...string) ([]byte, error) {
	t.Helper()

	uses := getUseArray(c)
	args = append(uses, args...)

	rc := c.Root()

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origIn := os.Stdin
	os.Stdin = r
	w.Write(in)
	w.Close()

	buf := new(bytes.Buffer)
	rc.SetOutput(buf)
	rc.SetArgs(args)

	err = rc.Execute()
	os.Stdin = origIn

	return buf.Bytes(), err
}
