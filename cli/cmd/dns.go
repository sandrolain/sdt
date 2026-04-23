package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// DNSResult holds the results of a DNS lookup.
type DNSResult struct {
	Host    string   `json:"host"    yaml:"host"`
	Type    string   `json:"type"    yaml:"type"`
	Records []string `json:"records" yaml:"records"`
}

// dnsLookup performs the DNS lookup of the given type for the host.
func dnsLookup(host, recordType string) ([]string, error) {
	switch strings.ToUpper(recordType) {
	case "A", "AAAA":
		ips, err := net.LookupHost(host)
		if err != nil {
			return nil, err
		}
		return ips, nil
	case "MX":
		records, err := net.LookupMX(host)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(records))
		for _, mx := range records {
			out = append(out, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
		}
		return out, nil
	case "TXT":
		records, err := net.LookupTXT(host)
		if err != nil {
			return nil, err
		}
		return records, nil
	case "CNAME":
		cname, err := net.LookupCNAME(host)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil
	case "NS":
		records, err := net.LookupNS(host)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(records))
		for _, ns := range records {
			out = append(out, ns.Host)
		}
		return out, nil
	case "PTR":
		names, err := net.LookupAddr(host)
		if err != nil {
			return nil, err
		}
		return names, nil
	default:
		return nil, fmt.Errorf("unsupported DNS record type %q; supported: A, AAAA, MX, TXT, CNAME, NS, PTR", recordType)
	}
}

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS lookup for a host",
	Long: `Perform a DNS lookup for the given host and record type.

Supported record types: A (default), AAAA, MX, TXT, CNAME, NS, PTR

PTR performs a reverse DNS lookup (pass an IP address as --host).

Examples:
  sdt dns --host example.com
  sdt dns --host example.com --type MX
  sdt dns --host example.com --type TXT --format json
  sdt dns --host 8.8.8.8 --type PTR`,
	Run: func(cmd *cobra.Command, args []string) {
		host := getStringFlag(cmd, "host", true)
		recordType := getStringFlag(cmd, "type", false)
		if recordType == "" {
			recordType = "A"
		}
		format := getFormat(cmd)

		records, err := dnsLookup(host, recordType)
		exitWithError(cmd, err)

		result := DNSResult{
			Host:    host,
			Type:    strings.ToUpper(recordType),
			Records: records,
		}

		switch format {
		case fmtJSON:
			out, merr := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(result)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			for _, r := range records {
				outputString(cmd, r)
			}
		}
	},
}

func init() {
	dnsCmd.Flags().String("host", "", "Hostname or IP address to look up (required)")
	dnsCmd.Flags().String("type", "A", "DNS record type: A|AAAA|MX|TXT|CNAME|NS|PTR")
	rootCmd.AddCommand(dnsCmd)
}
