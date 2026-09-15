package corpus

import "testing"

func TestExcludedDirName(t *testing.T) {
	for _, name := range []string{"tmp", "scripts", "refs", "commands", "instructions", "sdtdocs"} {
		if !ExcludedDirName(name) {
			t.Errorf("ExcludedDirName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"wiki", "plan", "notes", "commandsx", "Tmp"} {
		if ExcludedDirName(name) {
			t.Errorf("ExcludedDirName(%q) = true, want false", name)
		}
	}
}

func TestExcludedPath(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		{"context/tmp/x.md", true},
		{"context/scripts/a.go", true},
		{"context/refs/repo/readme.md", true},
		{"context/commands/index.md", true},
		{"context/instructions/plan.md", true},
		{"context/sdtdocs/sdt_context_docs.md", true},
		{"context/README.md", true},
		{"context/./README.md", true},
		{`context\refs\a.md`, true},
		{"context/wiki/topic.md", false},
		{"context/wiki/commands.md", false},
		{"context/plan/20260915-x.md", false},
		{"context/notes/refs.md", false},
		{"context/instructions.md", false},
	}
	for _, c := range cases {
		if got := ExcludedPath(c.rel); got != c.want {
			t.Errorf("ExcludedPath(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}
