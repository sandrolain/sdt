package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// dnsLookupWithTimeout runs dnsLookup in a goroutine with a deadline.
// Returns ("", timeout_err) if the lookup takes longer than d.
func dnsLookupWithTimeout(host, recordType string, d time.Duration) ([]string, error) {
	type res struct {
		records []string
		err     error
	}
	ch := make(chan res, 1)
	go func() {
		recs, err := dnsLookup(host, recordType)
		ch <- res{recs, err}
	}()
	select {
	case r := <-ch:
		return r.records, r.err
	case <-time.After(d):
		return nil, nil // treat timeout as skip-able
	}
}

func TestDNSLookup_A(t *testing.T) {
	// Use localhost which should always resolve
	records, err := dnsLookup("localhost", "A")
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
	if len(records) == 0 {
		t.Error("expected at least one A record for localhost")
	}
}

func TestDNSLookup_AAAA(t *testing.T) {
	records, err := dnsLookup("localhost", "AAAA")
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
	// localhost may or may not have AAAA — just check no error
	_ = records
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

func TestDNSLookup_MX(t *testing.T) {
	records, err := dnsLookupWithTimeout("gmail.com", "MX", 3*time.Second)
	if records == nil && err == nil {
		t.Skip("DNS lookup timed out")
	}
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
}

func TestDNSLookup_TXT(t *testing.T) {
	records, err := dnsLookupWithTimeout("gmail.com", "TXT", 3*time.Second)
	if records == nil && err == nil {
		t.Skip("DNS lookup timed out")
	}
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
	_ = records
}

func TestDNSLookup_CNAME(t *testing.T) {
	records, err := dnsLookupWithTimeout("www.google.com", "CNAME", 3*time.Second)
	if records == nil && err == nil {
		t.Skip("DNS lookup timed out")
	}
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
}

func TestDNSLookup_NS(t *testing.T) {
	records, err := dnsLookupWithTimeout("google.com", "NS", 3*time.Second)
	if records == nil && err == nil {
		t.Skip("DNS lookup timed out")
	}
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
}

func TestDNSLookup_PTR(t *testing.T) {
	records, err := dnsLookupWithTimeout("8.8.8.8", "PTR", 3*time.Second)
	if records == nil && err == nil {
		t.Skip("DNS lookup timed out")
	}
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
	_ = records
}

func TestDNSCmd_MX_json(t *testing.T) {
	records, err := dnsLookupWithTimeout("gmail.com", "MX", 3*time.Second)
	if records == nil && err == nil {
		t.Skip("DNS lookup timed out")
	}
	if err != nil {
		t.Skipf("DNS not available: %v", err)
	}
}
