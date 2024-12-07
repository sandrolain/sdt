package cmd

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var httpCmd = &cobra.Command{
	Use:     "http",
	Aliases: []string{"request", "req"},
	Short:   "HTTP client",
	Long:    `Make an HTTP request`,
	Run: func(cmd *cobra.Command, args []string) {
		byt := getInputBytes(cmd, args)

		method := getStringFlag(cmd, "method", false)
		url := getStringFlag(cmd, "url", true)
		typ := getStringFlag(cmd, "type", false)
		to := getIntFlag(cmd, "timeout", false)
		header := getStringArrayFlag(cmd, "header", false)

		client := http.Client{
			Timeout: time.Duration(to) * time.Second,
		}

		method = strings.ToUpper(method)

		var req *http.Request
		var err error
		if method == "POST" || method == "PUT" {
			reader := bytes.NewReader(byt)
			req, err = http.NewRequest(method, url, reader)
		} else {
			req, err = http.NewRequest(method, url, nil)
		}
		exitWithError(cmd, err)

		ua := req.UserAgent()

		if len(header) > 0 {
			for _, h := range header {
				parts := strings.Split(h, ":")
				val := ""
				if len(parts) > 1 {
					val = strings.TrimSpace(parts[1])
				}
				req.Header.Add(parts[0], val)
			}
		}

		if ua == req.UserAgent() {
			req.Header.Set("User-Agent", "sdt/"+version)
		}

		if typ != "" {
			req.Header.Set("Content-Type", typ)
		}

		res, err := client.Do(req)
		exitWithError(cmd, err)

		body, err := io.ReadAll(res.Body)
		exitWithError(cmd, err)

		err = res.Body.Close()
		exitWithError(cmd, err)

		outputBytes(cmd, body)
	},
}

func init() {
	pf := httpCmd.PersistentFlags()
	pf.IntP("timeout", "t", 10, "Timeout (seconds)")
	pf.StringP("method", "m", "GET", "Method")
	pf.StringP("url", "u", "", "URL")
	pf.StringP("type", "y", "", "Request Content-Type")
	pf.StringArrayP("header", "e", []string{}, "Header")

	rootCmd.AddCommand(httpCmd)
}
