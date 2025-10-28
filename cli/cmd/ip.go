package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func validIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	re := regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	return re.MatchString(ipAddress)
}

var ipInfoCmd = &cobra.Command{
	Use:     "ipinfo",
	Aliases: []string{"ip"},
	Short:   "IP location",
	Long:    `IP location info`,
	Run: func(cmd *cobra.Command, args []string) {
		ip := getInputStringOrFlag(cmd, args, "ip", false)
		asJson := getBoolFlag(cmd, "json", false)
		url := "https://ipapi.co/json"
		if ip != "" {
			if !validIP4(ip) {
				ips, err := net.LookupIP(ip)
				exitWithError(cmd, err)
				ip = ips[0].To4().String()
			}
			url = fmt.Sprintf("https://ipapi.co/%s/json", ip)
		}

		client := http.Client{
			Timeout: time.Duration(5) * time.Second,
		}
		req, err := http.NewRequest("GET", url, nil)
		exitWithError(cmd, err)

		req.Header.Set("User-Agent", "sdt/"+version)
		res, err := client.Do(req)
		exitWithError(cmd, err)

		body, err := io.ReadAll(res.Body)
		exitWithError(cmd, err)

		err = res.Body.Close()
		exitWithError(cmd, err)

		if asJson {
			outputBytes(cmd, body)
			return
		}

		var data map[string]interface{}
		exitWithError(cmd, json.Unmarshal(body, &data))

		tableString := &strings.Builder{}
		table := tablewriter.NewWriter(tableString)
		table.Header("Property", "Value")

		for k, v := range data {
			k = strings.ReplaceAll(k, "_", " ")
			exitWithError(cmd, table.Append(k, fmt.Sprintf("%v", v)))
		}
		exitWithError(cmd, table.Render())

		outputBytes(cmd, []byte(tableString.String()))
	},
}

func init() {
	pf := ipInfoCmd.PersistentFlags()
	pf.StringP("ip", "t", "", "Target IP (default: client IP)")
	pf.BoolP("json", "j", false, "As JSON output")
	rootCmd.AddCommand(ipInfoCmd)
}
