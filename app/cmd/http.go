package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "QR code",
	Long:  `Generate QR code`,
	Run: func(cmd *cobra.Command, args []string) {
		byt, err := getInputBytes(cmd, args)
		exitWithError(err)

		method := getStringFlag(cmd, "method")
		url := getStringFlag(cmd, "url")
		typ := getStringFlag(cmd, "type")
		to := getIntFlag(cmd, "timeout")
		header := getStringArrayFlag(cmd, "header")

		client := http.Client{
			Timeout: time.Duration(to) * time.Second,
		}

		method = strings.ToUpper(method)

		var req *http.Request
		if method == "POST" || method == "PUT" {
			reader := bytes.NewReader(byt)
			req, err = http.NewRequest(method, url, reader)
		} else {
			req, err = http.NewRequest(method, url, nil)
		}
		exitWithError(err)

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

		exitWithError(err)

		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		exitWithError(err)

		fmt.Print(string(body))
	},
}

func init() {
	pf := httpCmd.PersistentFlags()
	pf.IntP("timeout", "t", 10, "Timeout (seconds)")
	pf.StringP("method", "m", "GET", "Method")
	pf.StringP("url", "u", "", "URL")
	pf.StringP("type", "y", "", "Request Content-Type")
	pf.StringArrayP("header", "e", []string{}, "Header")
	httpCmd.MarkPersistentFlagRequired("url")

	rootCmd.AddCommand(httpCmd)
}
