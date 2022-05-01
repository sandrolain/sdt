package cmd

import (
	"bytes"
	"testing"
)

func TestHexEncode(t *testing.T) {
	out, err := execute(t, hexCmd, []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatal(err)
	}
	exp := "010203"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestHexDecode(t *testing.T) {
	out, err := execute(t, hexDecCmd, []byte("010203"))
	if err != nil {
		t.Fatal(err)
	}
	exp := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}
