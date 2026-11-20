package cmd

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClassifyIP(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		version     string
		private     bool
		loopback    bool
		linkLocal   bool
		unspecified bool
	}{
		{name: "IPv4 private", input: "192.168.1.1", version: "IPv4", private: true},
		{name: "IPv4 loopback", input: "127.0.0.1", version: "IPv4", loopback: true},
		{name: "IPv6 link local", input: "fe80::1", version: "IPv6", linkLocal: true},
		{name: "IPv6 unspecified", input: "::", version: "IPv6", unspecified: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classifyIP(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Version != test.version || got.Private != test.private || got.Loopback != test.loopback ||
				(got.LinkLocalUnicast || got.LinkLocalMulticast) != test.linkLocal || got.Unspecified != test.unspecified {
				t.Fatalf("classification = %+v", got)
			}
		})
	}

	if _, err := classifyIP("example.test"); err == nil {
		t.Fatal("hostname was accepted as a local IP address")
	}
}

func TestIPInfoRequiresExplicitTarget(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, ipInfoCmd, nil)
		return ""
	})
}

func TestIPInfoLocalJSON(t *testing.T) {
	out := execute(t, ipInfoCmd, nil, "--local", "--ip", "192.168.1.1", "--format", "json")
	var result IPClassification
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if result.Address != "192.168.1.1" || result.Version != "IPv4" || !result.Private {
		t.Fatalf("classification = %+v", result)
	}
}

func TestRequestIPInfoUsesExplicitTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/203.0.113.7/json" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "sdt/") {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		_, _ = w.Write([]byte(`{"ip":"203.0.113.7"}`))
	}))
	defer server.Close()

	body, err := requestIPInfo("203.0.113.7", server.Client(), server.URL, func(string) ([]net.IP, error) {
		t.Fatal("literal IP unexpectedly triggered DNS resolution")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ip":"203.0.113.7"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestRequestIPInfoResolvesExplicitHostname(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2001:db8::7/json" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := requestIPInfo("example.test", server.Client(), server.URL, func(host string) ([]net.IP, error) {
		if host != "example.test" {
			t.Fatalf("lookup host = %q", host)
		}
		return []net.IP{net.ParseIP("2001:db8::7")}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestIPInfoReportsProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := requestIPInfo("203.0.113.7", server.Client(), server.URL, net.LookupIP)
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("error = %v, want provider status", err)
	}
}
