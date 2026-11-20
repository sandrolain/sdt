package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

const ipInfoEndpoint = "https://ipapi.co"

// IPClassification reports address properties available from local parsing.
// These properties do not identify a machine's public egress address.
type IPClassification struct {
	Address            string `json:"address"              yaml:"address"`
	Version            string `json:"version"              yaml:"version"`
	Private            bool   `json:"private"              yaml:"private"`
	Loopback           bool   `json:"loopback"             yaml:"loopback"`
	LinkLocalUnicast   bool   `json:"link_local_unicast"   yaml:"link_local_unicast"`
	LinkLocalMulticast bool   `json:"link_local_multicast" yaml:"link_local_multicast"`
	Multicast          bool   `json:"multicast"            yaml:"multicast"`
	Unspecified        bool   `json:"unspecified"          yaml:"unspecified"`
}

func classifyIP(address string) (IPClassification, error) {
	ip := net.ParseIP(strings.TrimSpace(address))
	if ip == nil {
		return IPClassification{}, fmt.Errorf("%q is not an IP address", address)
	}

	version := "IPv6"
	if ip.To4() != nil {
		version = "IPv4"
	}
	return IPClassification{
		Address:            ip.String(),
		Version:            version,
		Private:            ip.IsPrivate(),
		Loopback:           ip.IsLoopback(),
		LinkLocalUnicast:   ip.IsLinkLocalUnicast(),
		LinkLocalMulticast: ip.IsLinkLocalMulticast(),
		Multicast:          ip.IsMulticast(),
		Unspecified:        ip.IsUnspecified(),
	}, nil
}

func resolveIPTarget(target string, lookup func(string) ([]net.IP, error)) (net.IP, error) {
	target = strings.TrimSpace(target)
	if ip := net.ParseIP(target); ip != nil {
		return ip, nil
	}

	addresses, err := lookup(target)
	if err != nil {
		return nil, fmt.Errorf("resolve target %q: %w", target, err)
	}
	for _, address := range addresses {
		if address.To4() != nil || address.To16() != nil {
			return address, nil
		}
	}
	return nil, fmt.Errorf("target %q did not resolve to an IP address", target)
}

func requestIPInfo(target string, client *http.Client, endpoint string, lookup func(string) ([]net.IP, error)) ([]byte, error) {
	ip, err := resolveIPTarget(target, lookup)
	if err != nil {
		return nil, err
	}

	requestURL := strings.TrimRight(endpoint, "/") + "/" + url.PathEscape(ip.String()) + "/json"
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sdt/"+version)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close() //nolint:errcheck // Closing a GET response body after reading it has no useful recovery path.
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("ipinfo provider returned HTTP %s", res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read ipinfo provider response: %w", err)
	}
	return body, nil
}

func ipInfoTarget(cmd *cobra.Command, args []string) (string, error) {
	if target := getStringFlag(cmd, "ip", false); target != "" {
		return target, nil
	}
	for _, name := range []string{editFileFlag, inputStringFlag, inputBase64Flag} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return getInputString(cmd, args), nil
		}
	}
	if len(args) > 0 {
		return getInputString(cmd, args), nil
	}
	if input, ok := cmd.InOrStdin().(*os.File); ok {
		info, err := input.Stat()
		if err != nil {
			return "", fmt.Errorf("inspect target stdin: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return "", nil
		}
	}
	return getInputString(cmd, args), nil
}

var ipInfoCmd = &cobra.Command{
	Use:     "ipinfo",
	Aliases: []string{"ip"},
	Short:   "Classify an IP locally or look up an explicit target",
	Long: `Classify an IP address locally with --local, or retrieve provider-backed
geolocation for an explicit IP address or hostname. No target is inferred from
the machine's public egress address.

Examples:
  sdt ipinfo --local --ip 192.168.1.1
  sdt ipinfo --ip 203.0.113.7 --format json`,
	Run: func(cmd *cobra.Command, args []string) {
		target, err := ipInfoTarget(cmd, args)
		exitWithError(cmd, err)
		if strings.TrimSpace(target) == "" {
			exitWithError(cmd, fmt.Errorf("target required: use --ip, an input source, or --local with an explicit target"))
			return
		}

		if getBoolFlag(cmd, "local", false) {
			result, err := classifyIP(target)
			exitWithError(cmd, err)
			switch getFormat(cmd) {
			case fmtJSON:
				out, merr := json.MarshalIndent(result, "", "  ")
				exitWithError(cmd, merr)
				outputBytes(cmd, out)
			case fmtYAML:
				out, merr := yaml.Marshal(result)
				exitWithError(cmd, merr)
				outputBytes(cmd, out)
			default:
				classes := make([]string, 0, 6)
				if result.Private {
					classes = append(classes, "private")
				}
				if result.Loopback {
					classes = append(classes, "loopback")
				}
				if result.LinkLocalUnicast || result.LinkLocalMulticast {
					classes = append(classes, "link-local")
				}
				if result.Multicast {
					classes = append(classes, "multicast")
				}
				if result.Unspecified {
					classes = append(classes, "unspecified")
				}
				if len(classes) == 0 {
					classes = append(classes, "none of private, loopback, link-local, multicast, or unspecified")
				}
				outputString(cmd, fmt.Sprintf("%s (%s): %s", result.Address, result.Version, strings.Join(classes, ", ")))
			}
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		body, err := requestIPInfo(target, client, ipInfoEndpoint, net.LookupIP)
		exitWithError(cmd, err)
		outputBytes(cmd, body)
	},
}

func init() {
	ipInfoCmd.Flags().StringP("ip", "t", "", "Target IP address or hostname (required for geolocation)")
	ipInfoCmd.Flags().Bool("local", false, "Classify a literal IP locally without a provider request")
	rootCmd.AddCommand(ipInfoCmd)
}
