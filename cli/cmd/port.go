package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// PortResult holds the output for the port command.
type PortResult struct {
	Host      string  `json:"host"               yaml:"host"`
	Port      int     `json:"port"               yaml:"port"`
	Open      bool    `json:"open"               yaml:"open"`
	LatencyMs float64 `json:"latency_ms"         yaml:"latency_ms"`
	Error     string  `json:"error,omitempty"    yaml:"error,omitempty"`
}

// checkPort attempts a TCP connection to host:port with the given timeout.
func checkPort(host string, port int, timeout time.Duration) PortResult {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)
	ms := float64(elapsed.Microseconds()) / 1000.0

	result := PortResult{
		Host:      host,
		Port:      port,
		LatencyMs: ms,
	}
	if err != nil {
		result.Open = false
		result.Error = err.Error()
	} else {
		conn.Close()
		result.Open = true
	}
	return result
}

var portCmd = &cobra.Command{
	Use:   "port",
	Short: "Check if a TCP port is open on a host",
	Long: `Attempt a TCP connection to the given host and port.

Reports whether the port is open, and the connection latency.

Examples:
  sdt port --host localhost --port 80
  sdt port --host db.internal --port 5432 --timeout 2s
  sdt port --host example.com --port 443 --format json`,
	Run: func(cmd *cobra.Command, args []string) {
		host := getStringFlag(cmd, "host", true)
		port := getIntFlag(cmd, "port", true)
		timeoutStr := getStringFlag(cmd, "timeout", false)
		format := getFormat(cmd)

		if timeoutStr == "" {
			timeoutStr = "5s"
		}

		timeout, err := time.ParseDuration(timeoutStr)
		exitWithError(cmd, err)

		result := checkPort(host, port, timeout)

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
			if result.Open {
				outputString(cmd, fmt.Sprintf("open (%.2f ms)", result.LatencyMs))
			} else {
				outputString(cmd, fmt.Sprintf("closed: %s", result.Error))
			}
		}
	},
}

func init() {
	portCmd.Flags().String("host", "", "Hostname or IP address to check (required)")
	portCmd.Flags().Int("port", 0, "TCP port number to check (required)")
	portCmd.Flags().String("timeout", "5s", "Connection timeout (e.g. 2s, 500ms)")
	rootCmd.AddCommand(portCmd)
}
