package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type versionInfo struct {
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit"  yaml:"commit"`
	Date    string `json:"date"    yaml:"date"`
}

func SetVersion(v string, c string, d string) {
	version = v
	commit = c
	date = d
}

var (
	version    = "dev"
	commit     = "none"
	date       = "unknown"
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Version will output the current build information",
		Long:  ``,
		Run: func(cmd *cobra.Command, _ []string) {
			shortened, err := cmd.Flags().GetBool("short")
			exitWithError(cmd, err)
			if shortened {
				outputString(cmd, version)
				return
			}

			info := versionInfo{Version: version, Commit: commit, Date: date}
			format := getFormat(cmd)
			switch format {
			case "yaml":
				out, err := yaml.Marshal(info)
				exitWithError(cmd, err)
				outputBytes(cmd, out)
			default:
				out, err := json.Marshal(info)
				exitWithError(cmd, err)
				outputString(cmd, fmt.Sprintf("%s\n", out))
			}
		},
	}
)

func init() {
	versionCmd.PersistentFlags().BoolP("short", "s", false, "Print just the version number.")
	rootCmd.AddCommand(versionCmd)
}
