//go:build !wasm

package cmd

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/spf13/cobra"
)

var nsLookupCmd = &cobra.Command{
	Use:     "nslookup",
	Aliases: []string{"lookup", "look"},
	Short:   "NS lookup",
	Long:    `NS location IP`,
	Run: func(cmd *cobra.Command, args []string) {
		ns := getInputStringOrFlag(cmd, args, "ns", false)
		asJson := getBoolFlag(cmd, "json", false)

		ips, err := net.LookupIP(ns)
		exitWithError(cmd, err)

		res := make([]string, len(ips))
		for i, ip := range ips {
			val := ip.String()
			res[i] = val
		}

		if asJson {
			res, err := json.Marshal(res)
			exitWithError(cmd, err)
			outputBytes(cmd, res)
			return
		}

		outputString(cmd, strings.Join(res, "\n"))
	},
}

func init() {
	pf := nsLookupCmd.PersistentFlags()
	pf.StringP("ns", "t", "", "Target namespace")
	pf.BoolP("json", "j", false, "As JSON output")
	rootCmd.AddCommand(nsLookupCmd)
}
