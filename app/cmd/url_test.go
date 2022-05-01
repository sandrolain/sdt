package cmd

import (
	"testing"
)

func TestUrlEncode(t *testing.T) {
	out, err := execute(t, urlEncCmd, []byte{}, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	exp := "hello%20world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestUrlEncodeForm(t *testing.T) {
	out, err := execute(t, urlEncFormCmd, []byte{}, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	exp := "hello+world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestUrlDecode(t *testing.T) {
	out, err := execute(t, urlDecCmd, []byte{}, "hello%20world")
	if err != nil {
		t.Fatal(err)
	}
	exp := "hello world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}
