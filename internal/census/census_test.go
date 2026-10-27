package census

import "testing"

func TestClassify(t *testing.T) {
	for _, tc := range []struct{ name, want, cmd string }{
		{"one replace", "one literal pair", "python3 - <<'PY'\np='a.md'\nt=p.read_text()\np.write_text(t.replace('x','y'))\nPY"},
		{"two replaces", "batch of literal pairs", "python3 - <<'PY'\nt=t.replace('a','b').replace('c','d')\nPY"},
		{"insert", "position / slice", "python3 - <<'PY'\nlines.insert(3,'x')\nPY"},
		{"slice", "position / slice", "python3 - <<'PY'\nt = t[:i] + 'x' + t[i:]\nPY"},
		{"python regex", "python regex", "python3 - <<'PY'\nt=re.sub(r'a+','b',t)\nPY"},
		{"whole body", "whole-body rewrite", "python3 - <<'PY'\nopen('a.md','w').write(body)\nPY"},
		{"cat", "whole-file write", "cat > a.md <<'EOF'\nhello\nEOF"},
		{"sed literal", "sed literal", "sed -i 's/foo/bar/' a.md"},
		{"sed regex", "sed regex or address", "sed -i 's/^foo.*/bar/' a.md"},
		{"sed address", "sed regex or address", "sed -i '1,5d' a.md"},
		{"bsd sed regex", "sed regex or address", "sed -i '' 's/^foo.*/bar/' a.md"},
		{"bsd sed literal", "sed literal", "sed -i '' 's/foo/bar/' a.md"},
		{"perl", "perl regex", "perl -pi -e 's/a/b/' a.md"},
		{"other", "other", "go test ./..."},
		{"empty", "other", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.cmd); got != tc.want {
				t.Errorf("Classify(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestShapesIsTheClosedVocabulary keeps the vocabulary in the analysis's order:
// a reordered list would silently change the report's rows.
func TestShapesIsTheClosedVocabulary(t *testing.T) {
	want := []string{
		"one literal pair", "batch of literal pairs", "position / slice",
		"whole-body rewrite", "whole-file write", "sed regex or address",
		"python regex", "perl regex", "sed literal", "other",
	}
	if len(Shapes) != len(want) {
		t.Fatalf("Shapes has %d entries, want %d", len(Shapes), len(want))
	}
	for i := range want {
		if Shapes[i] != want[i] {
			t.Errorf("Shapes[%d] = %q, want %q", i, Shapes[i], want[i])
		}
	}
}
