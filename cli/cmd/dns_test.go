package cmd

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
)

func TestDNSLookupIPRecordTypes(t *testing.T) {
	resolved := []net.IP{
		net.ParseIP("192.0.2.7"),
		net.ParseIP("2001:db8::7"),
		net.ParseIP("::ffff:192.0.2.8"),
	}
	lookup := func(host string) ([]net.IP, error) {
		if host != "example.test" {
			t.Fatalf("lookup host = %q, want example.test", host)
		}
		return resolved, nil
	}

	tests := []struct {
		recordType string
		want       []string
	}{
		{recordType: "A", want: []string{"192.0.2.7", "192.0.2.8"}},
		{recordType: "AAAA", want: []string{"2001:db8::7"}},
	}
	for _, test := range tests {
		t.Run(test.recordType, func(t *testing.T) {
			got, err := dnsLookupIPs("example.test", test.recordType, lookup)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("records = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDNSLookup_invalidType(t *testing.T) {
	_, err := dnsLookup("localhost", "UNKNOWN")
	if err == nil {
		t.Error("expected error for unsupported record type")
	}
}

func TestDNSCmd_text(t *testing.T) {
	out := execute(t, dnsCmd, nil, "--host", "localhost")
	_ = out // localhost may return empty on some systems; just verify no crash
}

func TestDNSCmd_json(t *testing.T) {
	out := execute(t, dnsCmd, nil, "--host", "localhost", "--format", "json")
	var result DNSResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if result.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %s", result.Host)
	}
	if result.Type != "A" {
		t.Errorf("expected type 'A', got %s", result.Type)
	}
}

func TestDNSCmd_yaml(t *testing.T) {
	out := execute(t, dnsCmd, nil, "--host", "localhost", "--format", "yaml")
	if !strings.Contains(string(out), "host") {
		t.Errorf("expected 'host' in yaml output, got: %s", out)
	}
}

func TestDNSCmd_missingHost(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, dnsCmd, nil)
		return ""
	})
}

func TestDNSCmd_invalidType(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, dnsCmd, nil, "--host", "localhost", "--type", "BOGUS")
		return ""
	})
}
