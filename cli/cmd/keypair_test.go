package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKeypairCmd(t *testing.T) {
	out := execute(t, keypairCmd, nil)
	var pair []string
	if err := json.Unmarshal(out, &pair); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if len(pair) != 2 {
		t.Fatalf("expected 2 elements (private, public), got %d", len(pair))
	}
	if !strings.Contains(pair[0], "RSA PRIVATE KEY") {
		t.Errorf("expected RSA PRIVATE KEY in first element, got: %s", pair[0][:50])
	}
	if !strings.Contains(pair[1], "PUBLIC KEY") {
		t.Errorf("expected PUBLIC KEY in second element, got: %s", pair[1][:50])
	}
}
