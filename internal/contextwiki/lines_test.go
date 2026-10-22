package contextwiki

import "testing"

func TestContentLines(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"one terminated line", "a\n", 1},
		{"two terminated lines", "a\nb\n", 2},
		{"unterminated final line counts", "a\nb", 2},
		{"blank file", "\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := contentLines(tc.content); got != tc.want {
				t.Errorf("contentLines(%q) = %d, want %d", tc.content, got, tc.want)
			}
		})
	}
}
