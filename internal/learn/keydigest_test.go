package learn

import (
	"strings"
	"testing"
)

// TestKeyDigestFreezesTheVerdict is the contract in five assertions: the digest is
// stable, it moves for each of the four fields that decide a verdict, and it does
// not move for the text that only asks the question.
func TestKeyDigestFreezesTheVerdict(t *testing.T) {
	const (
		expected   = "deployment frequency"
		rubric     = "names all four"
		grading    = "keywords"
		distractor = "the raw deploy count"
	)
	base := KeyDigest(expected, rubric, grading, distractor)

	if base != KeyDigest(expected, rubric, grading, distractor) {
		t.Fatal("the same key produced two digests")
	}
	if len(base) != keyDigestLength {
		t.Errorf("digest length = %d, want %d", len(base), keyDigestLength)
	}
	if strings.ContainsFunc(base, func(r rune) bool { return r < '0' || r > 'f' }) {
		t.Errorf("digest %q is not lowercase hex", base)
	}

	cases := []struct{ name, got string }{
		{"expected", KeyDigest(expected+"s", rubric, grading, distractor)},
		{"rubric", KeyDigest(expected, rubric+" and units", grading, distractor)},
		{"grading", KeyDigest(expected, rubric, "exact", distractor)},
		{"distractor", KeyDigest(expected, rubric, grading, "the four are targets")},
	}
	for _, tc := range cases {
		if tc.got == base {
			t.Errorf("changing the %s left the digest unchanged", tc.name)
		}
	}

	// A field holding the separator cannot impersonate two fields.
	if KeyDigest("a"+keyDigestSeparator+"b", "", "", "") == KeyDigest("a", "b", "", "") {
		t.Error("the separator does not separate: two keys collided")
	}
	// Whitespace around a cell is not content.
	if KeyDigest("  "+expected+"  ", rubric, grading, distractor) != base {
		t.Error("surrounding whitespace moved the digest")
	}
}
