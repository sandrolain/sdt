package converter_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/cli/utils/converter"
)

const testBaseURL = "https://www.assistant-ui.com/docs/safe-content-frame"

func TestConvertLinksToLocalHandlesParentheses(t *testing.T) {
	in := "Read [the spec](https://e.test/wiki/Foo_(bar)) and [other](https://e.test/b)."
	got := converter.ConvertLinksToLocal(in, testBaseURL, map[string]string{
		"https://e.test/wiki/Foo_(bar)": "foo-bar.md",
	})

	want := "Read [the spec](foo-bar.md) and [other](https://e.test/b)."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertLinksToLocalPreservesNonLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"image", "![logo](https://e.test/logo.png)"},
		{"code span", "`[a](https://e.test/a)`"},
		{"reference link", "[a][ref]\n\n[ref]: https://e.test/a"},
		{"autolink", "<https://e.test/a>"},
		{"unmapped destination", "[a](https://e.test/other)"},
		{"different path", "[a](https://e.test/other)"},
		{"anchor only", "[a](#section)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := converter.ConvertLinksToLocal(c.in, testBaseURL, map[string]string{
				"https://e.test/a": "a.md",
			})
			if got != c.in {
				t.Errorf("got %q, want it unchanged", got)
			}
		})
	}
}

func TestConvertLinksToLocalKeepsTheFragment(t *testing.T) {
	in := "[a](https://e.test/a#section)"
	got := converter.ConvertLinksToLocal(in, testBaseURL, map[string]string{
		"https://e.test/a": "a.md",
	})

	if got != "[a](a.md#section)" {
		t.Errorf("got %q, want [a](a.md#section)", got)
	}
}

func TestConvertLinksToLocalResolvesRelativeAndQueryURLs(t *testing.T) {
	in := "[rel](./neighbor) [q](https://e.test/a?x=1)"
	got := converter.ConvertLinksToLocal(in, testBaseURL, map[string]string{
		"https://www.assistant-ui.com/docs/neighbor": "neighbor.md",
		"https://e.test/a?x=1":                       "a.md",
	})

	want := "[rel](neighbor.md) [q](a.md)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertLinksToLocalRewritesInsideATable(t *testing.T) {
	in := "| name | link |\n|---|---|\n| a | [a](https://e.test/a) |\n"
	got := converter.ConvertLinksToLocal(in, testBaseURL, map[string]string{
		"https://e.test/a": "a.md",
	})

	if !strings.Contains(got, "[a](a.md)") {
		t.Errorf("a link inside a table cell was not rewritten: %q", got)
	}
}

// TestConvertLinksToLocalLeavesACaptureByteIdentical proves the rewrite touches
// the resolved destination and nothing else: everything else in a real capture
// is byte-identical.
func TestConvertLinksToLocalLeavesACaptureByteIdentical(t *testing.T) {
	raw, err := os.ReadFile("testdata/capture.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	capture := string(raw)

	dest := firstResolvableDestination(capture)
	if dest == "" {
		t.Fatal("the fixture carries no absolute link to rewrite")
	}

	local := "safe-content-frame.md"
	got := converter.ConvertLinksToLocal(capture, testBaseURL, map[string]string{dest: local})

	want := strings.ReplaceAll(capture, "]("+dest+")", "]("+local+")")
	if got != want {
		t.Error("the capture changed outside the rewritten destination")
	}
	if got == capture {
		t.Error("the capture was not rewritten at all")
	}
}

// firstResolvableDestination returns the first absolute link destination the
// map can key on: no fragment, no trailing slash.
var absoluteLinkRe = regexp.MustCompile(`\]\((https?://[^)\s]+)\)`)

func firstResolvableDestination(markdown string) string {
	for _, m := range absoluteLinkRe.FindAllStringSubmatch(markdown, -1) {
		dest := m[1]
		if strings.Contains(dest, "#") || strings.HasSuffix(dest, "/") {
			continue
		}
		return dest
	}
	return ""
}
