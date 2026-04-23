package cmd

import (
	"encoding/json"
	"testing"
)

func TestCountTokens_cl100k(t *testing.T) {
	// "Hello, world!" should produce a small number of tokens.
	count := CountTokens("Hello, world!", "cl100k")
	if count < 3 || count > 6 {
		t.Errorf("expected 3-6 tokens for 'Hello, world!', got %d", count)
	}
}

func TestCountTokens_p50k(t *testing.T) {
	count := CountTokens("Hello, world!", "p50k")
	if count < 3 || count > 6 {
		t.Errorf("expected 3-6 tokens for 'Hello, world!' (p50k), got %d", count)
	}
}

func TestCountTokens_llama(t *testing.T) {
	base := CountTokens("Hello, world!", "cl100k")
	llama := CountTokens("Hello, world!", "llama")
	if llama < base {
		t.Errorf("llama count (%d) should be >= cl100k count (%d)", llama, base)
	}
}

func TestCountTokens_empty(t *testing.T) {
	if n := CountTokens("", "cl100k"); n != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", n)
	}
}

func TestResolveModelFamily(t *testing.T) {
	cases := []struct {
		model  string
		family string
	}{
		{"gpt-4", "cl100k"},
		{"gpt-2", "p50k"},
		{"llama", "llama"},
		{"claude", "cl100k"},
		{"unknown-model", "cl100k"},
	}
	for _, c := range cases {
		got := resolveModelFamily(c.model)
		if got != c.family {
			t.Errorf("resolveModelFamily(%q) = %q, want %q", c.model, got, c.family)
		}
	}
}

func TestTokensCmd_text(t *testing.T) {
	out := execute(t, tokensCmd, []byte("Hello, world!"))
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestTokensCmd_json(t *testing.T) {
	out := execute(t, tokensCmd, []byte("Hello, world!"), "--format", "json")
	var result TokensResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if result.Tokens <= 0 {
		t.Errorf("expected tokens > 0, got %d", result.Tokens)
	}
}

func TestTokensCmd_model(t *testing.T) {
	out := execute(t, tokensCmd, []byte("Hello, world!"), "--model", "llama", "--format", "json")
	var result TokensResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if result.Model != "llama" {
		t.Errorf("expected model llama, got %s", result.Model)
	}
}
