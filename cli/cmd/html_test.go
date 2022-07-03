package cmd

import (
	"testing"
)

func TestHtmlEncode(t *testing.T) {
	out := execute(t, htmlEncCmd, []byte("<hello>"))
	exp := "&lt;hello&gt;"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestHtmlDecode(t *testing.T) {
	out := execute(t, htmlDecCmd, []byte("&gt; hello &lt; world!"))
	exp := "> hello < world!"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}
