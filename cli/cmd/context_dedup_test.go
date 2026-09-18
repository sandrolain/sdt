package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizedBody(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  Hello   WORLD  ", "hello world"},
		{"Line\n\tbreak\r\nand  spaces", "line break and spaces"},
		{"Mixed CASE and\n\nblank lines", "mixed case and blank lines"},
	}
	for _, c := range cases {
		if got := normalizedBody(c.in); got != c.want {
			t.Errorf("normalizedBody(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNoteBody(t *testing.T) {
	withFM := `---
kind: notes
summary: s
---
Body
## Section
`
	if got := noteBody(withFM); got != "Body\n## Section\n" {
		t.Errorf("noteBody stripped %q, want %q", got, "Body\n## Section\n")
	}
	if got := noteBody("no frontmatter body"); got != "no frontmatter body" {
		t.Errorf("noteBody fallback = %q", got)
	}
}

func TestContentFingerprint(t *testing.T) {
	a := contentFingerprint("The same note body")
	b := contentFingerprint("The same note body")
	c := contentFingerprint("A different note body")
	if a != b || len(a) != 16 {
		t.Errorf("fingerprint not stable/16-hex: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("fingerprints must differ for different bodies: both %q", a)
	}
}

func TestJaccard(t *testing.T) {
	if got := jaccard(tokenSet("a b c"), tokenSet("a b c")); got != 1 {
		t.Errorf("identical sets: got %v", got)
	}
	if got := jaccard(tokenSet("a b c"), tokenSet("d e f")); got != 0 {
		t.Errorf("disjoint sets: got %v", got)
	}
	if got := jaccard(tokenSet(""), tokenSet("a")); got != 0 {
		t.Errorf("empty set: got %v", got)
	}
	if got := jaccard(tokenSet("a b c"), tokenSet("a b d")); got != 2.0/4.0 {
		t.Errorf("partial overlap: got %v", got)
	}
}

func TestLintDuplicateNotesExact(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/notes/one.md", "---\nkind: notes\nsummary: first\n---\nThe duplicated note body about topic X.\n")
	writeCtxDoc(t, "context/notes/two.md", "---\nkind: notes\nsummary: second\n---\nThe duplicated note body about topic X.\n")
	issues := lintDuplicateNotes([]string{"context/notes/one.md", "context/notes/two.md"})
	if len(issues) != 1 {
		t.Fatalf("expected 1 exact-duplicate issue, got %d", len(issues))
	}
	if issues[0].Priority != ctxLintSuggestion {
		t.Errorf("expected SUGGESTION, got %s", issues[0].Priority)
	}
	if !strings.Contains(issues[0].Message, "exact duplicate") || !strings.Contains(issues[0].Message, "one.md") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
	if strings.HasPrefix(issues[0].Path, "/") {
		// paths stay repo-relative
		t.Errorf("expected relative path, got %s", issues[0].Path)
	}
}

func TestLintDuplicateNotesNear(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/notes/one.md", "---\nkind: notes\nsummary: first\n---\nThe API returns a 429 rate limit when too many requests arrive quickly from the same client token.\n")
	writeCtxDoc(t, "context/notes/two.md", "---\nkind: notes\nsummary: second\n---\nThe API returns a 429 rate limit when too many requests arrive rapidly from the same client token.\n")
	issues := lintDuplicateNotes([]string{"context/notes/one.md", "context/notes/two.md"})
	if len(issues) != 1 {
		t.Fatalf("expected 1 near-duplicate issue, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Message, "near-duplicate") || !strings.Contains(issues[0].Message, "Jaccard") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestLintDuplicateNotesDistinct(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/notes/one.md", "---\nkind: notes\nsummary: first\n---\nDiscussion about nats clustering and leader election.\n")
	writeCtxDoc(t, "context/notes/two.md", "---\nkind: notes\nsummary: second\n---\nObservations about the go QR decoder benchmarks.\n")
	writeCtxDoc(t, "context/notes/three.md", "---\nkind: notes\nsummary: third\n---\nNotes on rate limiting backend timeouts.\n")
	if issues := lintDuplicateNotes([]string{"context/notes/one.md", "context/notes/two.md", "context/notes/three.md"}); len(issues) != 0 {
		t.Fatalf("expected no duplicate issues, got %v", issues)
	}
}

func TestLintDuplicateNotesSkipsMaps(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/notes/a.map.md", "---\nkind: notes\nsummary: map\n---\nMap of topic alpha with the same template payload.\n")
	writeCtxDoc(t, "context/notes/b.map.md", "---\nkind: notes\nsummary: map\n---\nMap of topic alpha with the same template payload.\n")
	if issues := lintDuplicateNotes([]string{"context/notes/a.map.md", "context/notes/b.map.md"}); len(issues) != 0 {
		t.Fatalf("expected generated maps skipped, got %v", issues)
	}
}

// TestLintDuplicateNotesNeverMerges guards the core contract: lint only reports
// and never modifies files, even when bodies match but sources conflict.
func TestLintDuplicateNotesNeverMerges(t *testing.T) {
	runInTempDir(t)
	body := "---\nkind: notes\nsummary: duplicated\n---\nIdentical note body regardless of provenance.\n"
	writeCtxDoc(t, "context/notes/one.md", body)
	// two.md: same body but a different, valid source — must never be collapsed.
	data := "---\nkind: notes\nsummary: duplicated\nsources:\n  - analysis/source.md\n---\nIdentical note body regardless of provenance.\n"
	writeCtxDoc(t, "context/notes/two.md", data)
	writeCtxDoc(t, "context/analysis/source.md", "---\nkind: analysis\nsummary: source\n---\nbody\n")

	issues := lintDuplicateNotes([]string{"context/notes/one.md", "context/notes/two.md"})
	if len(issues) != 1 {
		t.Fatalf("expected 1 exact-duplicate issue, got %d", len(issues))
	}
	if issues[0].Priority == ctxLintCritical {
		t.Error("duplicate lint must never be CRITICAL")
	}
	// both files untouched, source intact
	for _, f := range []string{"context/notes/one.md", "context/notes/two.md", "context/analysis/source.md"} {
		if len(readFileBytes(t, f)) == 0 {
			t.Errorf("file %s was touched", f)
		}
	}
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestContextLintDedupSuggestion exercises the dedup-before-write advisory end
// to end through the context lint command: near-duplicate notes surface as a
// SUGGESTION without any CRITICAL.
func TestContextLintDedupSuggestion(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/one.md", "---\nkind: notes\nsummary: first\n---\nThe API returns a 429 rate limit when too many requests arrive quickly from the same client token.\n")
	writeCtxDoc(t, "context/notes/two.md", "---\nkind: notes\nsummary: second\n---\nThe API returns a 429 rate limit when too many requests arrive rapidly from the same client token.\n")
	writeCtxDoc(t, "context/notes/three.md", "---\nkind: notes\nsummary: third\n---\nA completely different note about the QR decoder test corpus.\n")
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), ctxLintSuggestion) || !strings.Contains(string(out), "near-duplicate") {
		t.Errorf("expected a near-duplicate SUGGESTION, got:\n%s", out)
	}
	if strings.Contains(string(out), `"CRITICAL"`) {
		t.Errorf("dedup advisory must never be CRITICAL:\n%s", out)
	}
}
