package cmd

import (
	"bytes"
	"compress/gzip"
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

		must(gz.Write(byt))
		exitWithError(gz.Flush())
		exitWithError(gz.Close())

		res := b.Bytes()
		must(os.Stdout.Write(res))
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
		r := must(gzip.NewReader(b))

		var resB bytes.Buffer
		must(resB.ReadFrom(r))

		res := resB.Bytes()

		must(os.Stdout.Write(res))
	},
}

func init() {
	rootCmd.AddCommand(gzipCmd)
	rootCmd.AddCommand(gunzipCmd)
}
