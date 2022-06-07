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

		ips := must(net.LookupIP(ns))

		res := make([]string, len(ips))
		for i, ip := range ips {
			val := ip.String()
			res[i] = val
		}

		if asJson {
			outputBytes(cmd, must(json.Marshal(res)))
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
