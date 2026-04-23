package cmd

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestComputeHMAC_sha256(t *testing.T) {
	// Known vector: key="key", data="The quick brown fox jumps over the lazy dog"
	// HMAC-SHA256 = f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8
	data := []byte("The quick brown fox jumps over the lazy dog")
	key := []byte("key")
	got, err := computeHMAC(data, key, "sha256")
	if err != nil {
		t.Fatal(err)
	}
	want := "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
	if hex.EncodeToString(got) != want {
		t.Errorf("got %x, want %s", got, want)
	}
}

func TestComputeHMAC_sha512(t *testing.T) {
	data := []byte("hello")
	key := []byte("secret")
	got, err := computeHMAC(data, key, "sha512")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 64 {
		t.Errorf("expected 64 bytes for sha512, got %d", len(got))
	}
}

func TestComputeHMAC_sha384(t *testing.T) {
	data := []byte("hello")
	key := []byte("secret")
	got, err := computeHMAC(data, key, "sha384")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 48 {
		t.Errorf("expected 48 bytes for sha384, got %d", len(got))
	}
}

func TestComputeHMAC_badAlgo(t *testing.T) {
	_, err := computeHMAC([]byte("data"), []byte("key"), "md5")
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}

func TestHMACCmd_text(t *testing.T) {
	out := execute(t, hmacCmd, []byte("hello"), "--key", "secret")
	result := strings.TrimSpace(string(out))
	if len(result) != 64 {
		t.Errorf("expected 64 hex chars for sha256, got %q (%d)", result, len(result))
	}
}

func TestHMACCmd_json(t *testing.T) {
	out := execute(t, hmacCmd, []byte("hello"), "--key", "secret", "--format", "json")
	var result HMACResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if result.Algorithm != "sha256" {
		t.Errorf("expected algorithm sha256, got %s", result.Algorithm)
	}
	if result.Key != "secret" {
		t.Errorf("expected key 'secret', got %s", result.Key)
	}
}

func TestHMACCmd_sha512(t *testing.T) {
	out := execute(t, hmacCmd, []byte("data"), "--key", "k", "--algo", "sha512")
	result := strings.TrimSpace(string(out))
	if len(result) != 128 {
		t.Errorf("expected 128 hex chars for sha512, got %d", len(result))
	}
}

func TestHMACCmd_missingKey(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, hmacCmd, []byte("data"))
		return ""
	})
}
