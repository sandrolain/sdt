package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	goVersion "go.hein.dev/go-version"
)

func SetVersion(v string, c string, d string) {
	version = v
	commit = c
	date = d
}

var (
	shortened  = false
	version    = "dev"
	commit     = "none"
	date       = "unknown"
	output     = "json"
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Version will output the current build information",
		Long:  ``,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("shortened: %v\n", shortened)
			resp := goVersion.FuncWithOutput(shortened, version, commit, date, output)
			fmt.Print(resp)
			return
		},
	}
)

func init() {
	versionCmd.PersistentFlags().BoolVarP(&shortened, "short", "s", false, "Print just the version number.")
	versionCmd.PersistentFlags().StringVarP(&output, "output", "o", "json", "Output format. One of 'yaml' or 'json'.")
	rootCmd.AddCommand(versionCmd)
}
