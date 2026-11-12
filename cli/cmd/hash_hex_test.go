package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

// TestSHA256HexMatchesKnownVector proves --hex prints the shasum shape: the
// hexadecimal digest of a known input, byte for byte.
func TestSHA256HexMatchesKnownVector(t *testing.T) {
	dir := runInTempDir(t)
	fixture := filepath.Join(dir, "p.txt")
	writeTestFile(t, fixture, "payload")
	out := string(execute(t, sha256Cmd, nil, "--file", "p.txt", "--hex"))
	want := "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"
	if strings.TrimSpace(out) != want {
		t.Errorf("sha256 --hex = %q, want %q", strings.TrimSpace(out), want)
	}
}

// TestSHA256DefaultIsRawDigest proves the default stays the raw bytes and --hex
// is the explicit opt-in, not a silent change of shape.
func TestSHA256DefaultIsRawDigest(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "p.txt"), "payload")
	out := execute(t, sha256Cmd, nil, "--file", "p.txt")
	sum := sha256.Sum256([]byte("payload"))
	if string(out) != string(sum[:]) {
		t.Errorf("default output = %x, want the raw digest", out)
	}
}

// TestSHA256HexOnFileViaFileFlag is the call the Toolbox row advertises:
// `sdt sha256 --file <f> --hex`, the global input flag end to end.
func TestSHA256HexOnFileViaFileFlag(t *testing.T) {
	dir := runInTempDir(t)
	writeTestFile(t, filepath.Join(dir, "p.txt"), "payload")
	out := string(execute(t, sha256Cmd, nil, "--file", "p.txt", "--hex", "--quiet"))
	sum := sha256.Sum256([]byte("payload"))
	if strings.TrimSpace(out) != hex.EncodeToString(sum[:]) {
		t.Errorf("hex = %q, want %x", strings.TrimSpace(out), hex.EncodeToString(sum[:]))
	}
}
