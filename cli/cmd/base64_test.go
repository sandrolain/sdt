package cmd

import (
	"bytes"
	"testing"
)

func TestB64Encode(t *testing.T) {
	out := execute(t, b64Cmd, []byte{0xd3, 0xb9, 0x89, 0xca, 0x42, 0xfd, 0x34, 0xfa, 0x5a, 0xa7})
	exp := "07mJykL9NPpapw=="
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestB64Decode(t *testing.T) {
	out := execute(t, b64DecCmd, []byte("07mJykL9NPpapw=="))
	exp := []byte{0xd3, 0xb9, 0x89, 0xca, 0x42, 0xfd, 0x34, 0xfa, 0x5a, 0xa7}
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}

func TestB64UrlEncode(t *testing.T) {
	out := execute(t, b64UrlCmd, []byte{0xf9, 0xf9, 0xf9, 0xf9, 0xf9, 0xf9, 0xf9, 0xf9})
	exp := "-fn5-fn5-fk="
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestB64UrlDecode(t *testing.T) {
	out := execute(t, b64UrlDecCmd, []byte("-fn5-fn5-fk="))
	exp := []byte{0xf9, 0xf9, 0xf9, 0xf9, 0xf9, 0xf9, 0xf9, 0xf9}
	if !bytes.Equal(out, exp) {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, out)
	}
}
