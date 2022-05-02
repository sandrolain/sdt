package cmd

import (
	"testing"
)

func TestUrlEncode(t *testing.T) {
	out := execute(t, urlEncCmd, []byte{}, "hello world")
	exp := "hello%20world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestUrlEncodeForm(t *testing.T) {
	out := execute(t, urlEncFormCmd, []byte{}, "hello world")
	exp := "hello+world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestUrlDecode(t *testing.T) {
	out := execute(t, urlDecCmd, []byte{}, "hello%20world")
	exp := "hello world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}
