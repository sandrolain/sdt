package cmd

import (
	"fmt"
	"testing"
)

func TestBytesDefault(t *testing.T) {
	out, err := execute(t, bytesCmd, []byte{})
	if err != nil {
		t.Fatal(err)
	}
	defBytesLen := 32
	if len(out) != defBytesLen {
		t.Fatalf("expecting bytes length of %v, got %v", defBytesLen, len(out))
	}
}

func TestBytesCustomLength(t *testing.T) {
	size := 64
	out, err := execute(t, bytesCmd, []byte{}, "--size", fmt.Sprint(size))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != size {
		t.Fatalf("expecting bytes length of %v, got %v", size, len(out))
	}
}

func TestDecimalEncoding(t *testing.T) {
	out, err := execute(t, decCmd, []byte{0xFF, 0x01, 0x99})
	if err != nil {
		t.Fatal(err)
	}
	exp := "255 1 153"
	if string(out) != exp {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, string(out))
	}

	out, err = execute(t, decCmd, []byte{0xFF, 0x01, 0x99}, "-s", ",")
	if err != nil {
		t.Fatal(err)
	}
	exp = "255,1,153"
	if string(out) != exp {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, string(out))
	}

	out, err = execute(t, decCmd, []byte{0xFF, 0x01, 0x99}, "-s", "")
	if err != nil {
		t.Fatal(err)
	}
	exp = "255001153"
	if string(out) != exp {
		t.Fatalf("expecting \"%v\", got \"%v\"", exp, string(out))
	}
}
