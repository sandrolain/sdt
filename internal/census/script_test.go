package census

import "testing"

// TestSedScriptLocatesTheProgram walks the argument shapes sed accepts: a
// quoted program, the BSD empty backup suffix, a -i.bak suffix, -e, an
// unquoted program and a malformed tail.
func TestSedScriptLocatesTheProgram(t *testing.T) {
	for _, tc := range []struct{ name, cmd, want string }{
		{"not sed", "go test ./...", ""},
		{"quoted", `sed -i 's/a/b/' f`, "s/a/b/"},
		{"bsd empty suffix", `sed -i '' 's/^x.*/y/' f`, "s/^x.*/y/"},
		{"suffix on the flag", `sed -i.bak 's/a/b/' f`, "s/a/b/"},
		{"suffix without a program", "sed -i.bak", ""},
		{"dash e", `sed -i -e 's/a/b/' f`, "s/a/b/"},
		{"unquoted", `sed -i s/a/b/ f`, "s/a/b/"},
		{"unterminated", `sed -i 's/a/b/ f`, ""},
		{"nothing after the flag", "sed -i", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sedScript(tc.cmd); got != tc.want {
				t.Errorf("sedScript(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestSedPatternIsRegexp pins the split the analysis's table depends on.
func TestSedPatternIsRegexp(t *testing.T) {
	for _, tc := range []struct {
		name, cmd string
		want      bool
	}{
		{"no program", "sed -i", false},
		{"literal", `sed -i 's/foo/bar/' f`, false},
		{"metacharacter", `sed -i 's/^foo.*/bar/' f`, true},
		{"address range", `sed -i '1,5d' f`, true},
		{"regex address", `sed -i '/^a/d' f`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sedPatternIsRegexp(tc.cmd); got != tc.want {
				t.Errorf("sedPatternIsRegexp(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}
