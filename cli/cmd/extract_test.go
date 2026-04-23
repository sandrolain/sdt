package cmd

import (
	"encoding/json"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	in := []byte("Visit https://example.com and http://foo.bar/path?x=1")
	out := execute(t, extractCmd, in, "--type", "urls")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 URLs, got %d: %v", len(result), result)
	}
}

func TestExtractEmails(t *testing.T) {
	in := []byte("Contact alice@example.com or bob@foo.org for info.")
	out := execute(t, extractCmd, in, "--type", "emails")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 emails, got %d: %v", len(result), result)
	}
}

func TestExtractIPs(t *testing.T) {
	in := []byte("Server at 192.168.1.1 and backup at 10.0.0.254.")
	out := execute(t, extractCmd, in, "--type", "ips")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 IPs, got %d: %v", len(result), result)
	}
}

func TestExtractDates(t *testing.T) {
	in := []byte("Created on 2024-01-15, reviewed on January 3, 2025.")
	out := execute(t, extractCmd, in, "--type", "dates")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Errorf("expected dates, got none")
	}
}

func TestExtractNoType(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, extractCmd, nil)
		return ""
	})
}

func TestExtractUnknownType(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, extractCmd, nil, "--type", "unknown")
		return ""
	})
}

func TestExtractEmpty(t *testing.T) {
	in := []byte("no urls here")
	out := execute(t, extractCmd, in, "--type", "urls")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}
