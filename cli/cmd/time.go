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
	t := getInt64Flag(cmd, "time", false)
	m := getBoolFlag(cmd, "millis", false)
	d := getStringFlag(cmd, "diff", false)

	var tm time.Time
	if t > 0 {
		if m {
			tm = time.UnixMilli(t)
		} else {
			tm = time.Unix(t, 0)
		}
	} else {
		tm = time.Now()
	}

	if d != "" {
		diff := must(time.ParseDuration(d))
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
		outputString(cmd, fmt.Sprint(tm.Unix()))
	},
}

var timeIsoCmd = &cobra.Command{
	Use:   "iso",
	Short: "ISO 8601 time",
	Long:  `Format ISO 8601 time`,
	Run: func(cmd *cobra.Command, args []string) {
		tm := getTime(cmd)
		tm = tm.UTC()
		outputString(cmd, tm.Format(time.RFC3339))
	},
}

var timeHttpCmd = &cobra.Command{
	Use:     "http",
	Aliases: []string{"gmt", "utc", "header"},
	Short:   "HTTP time",
	Long:    `Format HTTP time`,
	Run: func(cmd *cobra.Command, args []string) {
		tm := getTime(cmd)
		tm = tm.UTC()
		outputString(cmd, tm.Format(http.TimeFormat))
	},
}

func init() {
	pf := timeCmd.PersistentFlags()
	pf.Int64P("time", "t", 0, "Unix time to format")
	pf.BoolP("millis", "m", false, "Unix time with milliseconds")
	pf.StringP("diff", "d", "", "Difference to apply")

	timeCmd.AddCommand(timeUnixCmd)
	timeCmd.AddCommand(timeIsoCmd)
	timeCmd.AddCommand(timeHttpCmd)

	rootCmd.AddCommand(timeCmd)
}
