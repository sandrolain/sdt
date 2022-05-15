package cmd

import (
	"fmt"
	"testing"
)

func TestBytesDefault(t *testing.T) {
	out := execute(t, bytesCmd, []byte{})
	defBytesLen := 32
	if len(out) != defBytesLen {
		t.Fatalf("expecting bytes length of %v, got %v", defBytesLen, len(out))
	}
}

func TestBytesCustomLength(t *testing.T) {
	size := 64
	out := execute(t, bytesCmd, []byte{}, "--size", fmt.Sprint(size))
	if len(out) != size {
		t.Fatalf("expecting bytes length of %v, got %v", size, len(out))
	}
}

func TestDecimalEncoding(t *testing.T) {
	out := execute(t, decCmd, []byte{0xFF, 0x01, 0x99})
	exp := "255 1 153"
	if string(out) != exp {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, string(out))
	}

	out = execute(t, decCmd, []byte{0xFF, 0x01, 0x99}, "-s", ",")
	exp = "255,1,153"
	if string(out) != exp {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, string(out))
	}

	out = execute(t, decCmd, []byte{0xFF, 0x01, 0x99}, "-s", "")
	exp = "255001153"
	if string(out) != exp {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, string(out))
	}
}
