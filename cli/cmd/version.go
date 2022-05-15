package cmd

import (
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
		Run: func(cmd *cobra.Command, _ []string) {
			res := goVersion.FuncWithOutput(shortened, version, commit, date, output)
			outputString(cmd, res)
		},
	}
)

func init() {
	versionCmd.PersistentFlags().BoolVarP(&shortened, "short", "s", false, "Print just the version number.")
	versionCmd.PersistentFlags().StringVarP(&output, "output", "o", "json", "Output format. One of 'yaml' or 'json'.")
	rootCmd.AddCommand(versionCmd)
}
