package cmd

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var gzipCmd = &cobra.Command{
	Use:     "gzip",
	Aliases: []string{"gz"},
	Short:   "Gzip",
	Long:    `Gzip`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)

		var b bytes.Buffer
		gz := gzip.NewWriter(&b)

		_, err := gz.Write(byt)
		exitWithError(err)

		err = gz.Flush()
		exitWithError(err)

		err = gz.Close()
		exitWithError(err)

		res := b.Bytes()
		os.Stdout.Write(res)
	},
}

var gunzipCmd = &cobra.Command{
	Use:     "gunzip",
	Aliases: []string{"guz"},
	Short:   "Gunzip",
	Long:    `Gunzip`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)

		b := bytes.NewBuffer(byt)

		var r io.Reader
		r, err := gzip.NewReader(b)
		exitWithError(err)

		var resB bytes.Buffer
		_, err = resB.ReadFrom(r)
		exitWithError(err)

		res := resB.Bytes()
		os.Stdout.Write(res)
	},
}

func init() {
	rootCmd.AddCommand(gzipCmd)
	rootCmd.AddCommand(gunzipCmd)
}
