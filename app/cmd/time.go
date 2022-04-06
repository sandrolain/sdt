package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var timeCmd = &cobra.Command{
	Use:     "time",
	Aliases: []string{"tm"},
	Short:   "Time Tools",
	Long:    `Time Tools`,
}

func getTime(cmd *cobra.Command) time.Time {
	t, err := cmd.Flags().GetInt64("time")
	exitWithError(err)

	d, err := cmd.Flags().GetString("diff")
	exitWithError(err)

	var tm time.Time
	if t > 0 {
		tm = time.Unix(t, 0)
	} else {
		tm = time.Now()
	}

	if d != "" {
		diff, err := time.ParseDuration(d)
		exitWithError(err)
		tm = tm.Add(diff)
	}

	return tm
}

var timeUnixCmd = &cobra.Command{
	Use:   "unix",
	Short: "Unit time",
	Long:  `Format Unix time`,
	Run: func(cmd *cobra.Command, args []string) {
		tm := getTime(cmd)
		fmt.Print(tm.Unix())
	},
}

var timeIsoCmd = &cobra.Command{
	Use:   "iso",
	Short: "ISO 8601 time",
	Long:  `Format ISO 8601 time`,
	Run: func(cmd *cobra.Command, args []string) {
		tm := getTime(cmd)
		fmt.Print(tm.Format(time.RFC3339))
	},
}

var timeHttpCmd = &cobra.Command{
	Use:     "http",
	Aliases: []string{"gmt", "utc", "header"},
	Short:   "ISO 8601 time",
	Long:    `Format ISO 8601 time`,
	Run: func(cmd *cobra.Command, args []string) {
		tm := getTime(cmd)
		fmt.Print(tm.Format(http.TimeFormat))
	},
}

func init() {
	timeCmd.PersistentFlags().Int64P("time", "t", 0, "Unix time to format")
	timeCmd.PersistentFlags().StringP("diff", "d", "", "Difference to apply")

	timeCmd.AddCommand(timeUnixCmd)
	timeCmd.AddCommand(timeIsoCmd)
	timeCmd.AddCommand(timeHttpCmd)

	rootCmd.AddCommand(timeCmd)
}
