package cmd

import (
	"bytes"
	"testing"
)

func TestB32Encode(t *testing.T) {
	out := execute(t, b32Cmd, []byte{0xd3, 0xb9, 0x89, 0xca, 0x42, 0xfd, 0x34, 0xfa, 0x5a, 0xa7})
	exp := "2O4YTSSC7U2PUWVH"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestB32Decode(t *testing.T) {
	out := execute(t, b32DecCmd, []byte("2O4YTSSC7U2PUWVH"))
	exp := []byte{0xd3, 0xb9, 0x89, 0xca, 0x42, 0xfd, 0x34, 0xfa, 0x5a, 0xa7}
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}
