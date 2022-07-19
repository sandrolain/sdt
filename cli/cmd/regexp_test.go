package cmd

import (
	"testing"
)

func TestRegExp(t *testing.T) {
	data := "Hello World"
	out := execute(t, regeCmd, []byte(data), "-e", "[^\\s]+")

	str := string(out)
	exp := `["Hello","World"]`

	if str != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", data, str)
	}
}

func TestRegExpReplace(t *testing.T) {
	data := "Hello World"
	out := execute(t, regeReplaceCmd, []byte(data), "-e", "W[^\\s]+", "-r", "Pippo")

	str := string(out)
	exp := `Hello Pippo`

	if str != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", data, str)
	}
}
