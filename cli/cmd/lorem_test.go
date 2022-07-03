package cmd

import (
	"fmt"
	"strings"
	"testing"
)

func TestLoremWord(t *testing.T) {
	min := 6
	max := 10
	out := execute(t, loremCmd, []byte{}, "--type", "word", "--min", fmt.Sprint(min), "--max", fmt.Sprint(max))
	outlen := len(string(out))
	if outlen < min || outlen > max {
		t.Fatalf("expecting word to be of length between %v and %v, got %v", min, max, outlen)
	}
}

func TestLoremSentence(t *testing.T) {
	min := 6
	max := 10
	out := execute(t, loremCmd, []byte{}, "--type", "sentence", "--min", fmt.Sprint(min), "--max", fmt.Sprint(max))
	outlen := len(strings.Split(string(out), " "))
	if outlen < min || outlen > max {
		t.Fatalf("expecting word to be of length between %v and %v, got %v", min, max, outlen)
	}
}

func TestLoremParagraph(t *testing.T) {
	min := 6
	max := 10
	out := execute(t, loremCmd, []byte{}, "--type", "paragraph", "--min", fmt.Sprint(min), "--max", fmt.Sprint(max))
	outlen := len(strings.Split(string(out), "."))
	if outlen < min || outlen > max {
		t.Fatalf("expecting word to be of length between %v and %v, got %v", min, max, outlen)
	}
}
