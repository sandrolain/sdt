package cmd

import (
	"encoding/json"
	"testing"
)

func TestStringUppercase(t *testing.T) {
	out := execute(t, upperCaseCmd, []byte{}, "Hello World")
	exp := "HELLO WORLD"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestStringLowercase(t *testing.T) {
	out := execute(t, lowerCaseCmd, []byte{}, "Hello World")
	exp := "hello world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestStringTitlecase(t *testing.T) {
	out := execute(t, titleCaseCmd, []byte{}, "hello world")
	exp := "Hello World"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
	out = execute(t, titleCaseCmd, []byte{}, "HELLO WORLD")
	exp = "Hello World"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestStringEscape(t *testing.T) {
	out := execute(t, escapeCmd, []byte{}, "hello \"world\"")
	exp := "hello \\\"world\\\""
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestStringUnEscape(t *testing.T) {
	out := execute(t, unescapeCmd, []byte{}, "hello \\\"world\\\"")
	exp := "hello \"world\""
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestStringReplaceSpace(t *testing.T) {
	out := execute(t, replaceSpaceCmd, []byte{}, "hello world")
	exp := "helloworld"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
	out = execute(t, replaceSpaceCmd, []byte{}, "hello world", "-r=_")
	exp = "hello_world"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestStringCount(t *testing.T) {
	out := execute(t, countCmd, []byte{}, "hello world\nciao mondo\nfoo bar baz")
	var res struct {
		Lines int `json:"lines"`
		Words int `json:"words"`
		Chars int `json:"characters"`
	}
	err := json.Unmarshal(out, &res)
	if err != nil {
		t.Fatal(err)
	}

	if res.Lines != 3 {
		t.Fatalf("expecting %v lines, got %v", 3, res.Lines)
	}
	if res.Words != 7 {
		t.Fatalf("expecting %v words, got %v", 7, res.Words)
	}
	if res.Chars != 34 {
		t.Fatalf("expecting %v characters, got %v", 34, res.Chars)
	}
}
