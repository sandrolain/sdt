package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// startEchoServer starts a TCP listener on a random port and returns the port + cleanup func.
func startEchoServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test TCP server: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestCheckPort_open(t *testing.T) {
	port := startEchoServer(t)
	result := checkPort("127.0.0.1", port, 3*time.Second)
	if !result.Open {
		t.Errorf("expected port %d to be open, got error: %s", port, result.Error)
	}
	if result.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}
}

func TestCheckPort_closed(t *testing.T) {
	// Port 1 is almost never open and connection should be refused quickly
	result := checkPort("127.0.0.1", 1, 1*time.Second)
	if result.Open {
		t.Skip("port 1 was unexpectedly open; skipping")
	}
	if result.Error == "" {
		t.Error("expected error message for closed port")
	}
}

func TestPortCmd_json_open(t *testing.T) {
	port := startEchoServer(t)
	out := execute(t, portCmd, nil,
		"--host", "127.0.0.1",
		"--port", strings.TrimSpace(string([]byte(fmt.Sprintf("%d", port)))),
		"--format", "json",
	)
	var result PortResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if !result.Open {
		t.Errorf("expected port %d open, got: %s", port, result.Error)
	}
}

func TestPortCmd_text_closed(t *testing.T) {
	out := execute(t, portCmd, nil,
		"--host", "127.0.0.1",
		"--port", "1",
		"--timeout", "500ms",
	)
	if !strings.Contains(string(out), "closed") {
		t.Errorf("expected 'closed' in output, got: %s", out)
	}
}

func TestPortCmd_yaml(t *testing.T) {
	port := startEchoServer(t)
	portStr := strings.TrimSpace(string([]byte(fmt.Sprintf("%d", port))))
	out := execute(t, portCmd, nil,
		"--host", "127.0.0.1",
		"--port", portStr,
		"--format", "yaml",
	)
	if !strings.Contains(string(out), "open") {
		t.Errorf("expected 'open' in yaml output, got: %s", out)
	}
}

func TestPortCmd_missingHost(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, portCmd, nil, "--port", "80")
		return ""
	})
}

func TestPortCmd_missingPort(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, portCmd, nil, "--host", "localhost")
		return ""
	})
}

func TestPortCmd_badTimeout(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, portCmd, nil, "--host", "localhost", "--port", "80", "--timeout", "notaduration")
		return ""
	})
}
