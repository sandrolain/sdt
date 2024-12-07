package cmd

import (
	"bytes"
	"compress/gzip"

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
		exitWithError(cmd, err)
		exitWithError(cmd, gz.Flush())
		exitWithError(cmd, gz.Close())

		res := b.Bytes()
		outputBytes(cmd, res)
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
		r, err := gzip.NewReader(b)
		exitWithError(cmd, err)

		var resB bytes.Buffer
		_, err = resB.ReadFrom(r)
		exitWithError(cmd, err)

		res := resB.Bytes()

		outputBytes(cmd, res)
	},
}

func init() {
	rootCmd.AddCommand(gzipCmd)
	rootCmd.AddCommand(gunzipCmd)
}
