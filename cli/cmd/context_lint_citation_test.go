package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// refSHA8 recomputes a referenced file's SHA-256 prefix independently of the
// production helper so the fixture's expected value is not tautological.
func refSHA8(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}

// citationIssues filters a lintDoc result down to the citation-check findings.
func citationIssues(issues []ctxLintIssue) []ctxLintIssue {
	var out []ctxLintIssue
	for _, it := range issues {
		if strings.Contains(it.Message, "citation") {
			out = append(out, it)
		}
	}
	return out
}

func TestLintBodyCitations(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/refs/source.md", "the source body\n")
	fresh := refSHA8(t, "context/refs/source.md")
	short := fresh[:6]

	cases := []struct {
		name string
		body string
		want int    // number of citation findings
		msg  string // substring of the single finding when want > 0
	}{
		{name: "citation-free doc untouched", body: "no references here\n", want: 0},
		{name: "fresh 8-hex prefix passes", body: "see refs/source.md@" + fresh + ":10-12 for the claim\n", want: 0},
		{name: "fresh 6-hex prefix passes", body: "see refs/source.md@" + short + " for the claim\n", want: 0},
		{name: "stale prefix warns", body: "see refs/source.md@deadbeef:10\n", want: 1,
			msg: "stale citation refs/source.md@deadbeef (actual " + fresh + ")"},
		{name: "bare form (no lines) parses and warns", body: "see refs/source.md@deadbeef\n", want: 1,
			msg: "stale citation refs/source.md@deadbeef"},
		{name: "range form parses and warns", body: "see refs/source.md@deadbeef:10-12\n", want: 1,
			msg: "stale citation refs/source.md@deadbeef"},
		{name: "missing target warns", body: "see refs/nope.md@deadbeef:1\n", want: 1,
			msg: "citation target not found: refs/nope.md"},
		{name: "sha256: prefix tolerated (unresolved)", body: "see refs/source.md@sha256:deadbeefcafe\n", want: 0},
		{name: "sha256: prefix tolerated even for a missing target", body: "see refs/nope.md@sha256:deadbeefcafe\n", want: 0},
		{name: "inline code span ignored", body: "quote `refs/source.md@deadbeef` as an example\n", want: 0},
		{name: "fenced block ignored", body: "```\nrefs/source.md@deadbeef\n```\n", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "---\nkind: notes\nsummary: s\nagent: opencode\n---\n" + tc.body
			writeTestFile(t, "context/notes/citing.md", content)
			got := citationIssues(lintDoc("context/notes/citing.md"))
			if len(got) != tc.want {
				t.Fatalf("citation findings = %#v, want %d", got, tc.want)
			}
			if tc.want == 0 {
				return
			}
			if got[0].Priority != ctxLintWarning {
				t.Errorf("citation findings must be WARNING, got %s", got[0].Priority)
			}
			if !strings.Contains(got[0].Message, tc.msg) {
				t.Errorf("message %q missing %q", got[0].Message, tc.msg)
			}
		})
	}
}

func TestLintCitationHints(t *testing.T) {
	for _, msg := range []string{
		"stale citation refs/x.md@fbf61784 (actual eab182ea)",
		"citation target not found: refs/x.md",
	} {
		if ctxLintHint(msg) == "" {
			t.Errorf("message %q has no curated remediation hint", msg)
		}
	}
}

// TestLintCitationWarningsDoNotFailRun proves a stale citation is advisory: the
// command completes (execute t.Fatals on a non-nil error) with a WARNING finding,
// so exit-code behaviour stays CRITICAL-only.
func TestLintCitationWarningsDoNotFailRun(t *testing.T) {
	setupContextProject(t)
	writeTestFile(t, "context/refs/source.md", "the source body\n")
	writeTestFile(t, "context/notes/citing.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\nsee refs/source.md@deadbeef:1\n")

	out := string(execute(t, contextLintCmd, nil, "context/notes/citing.md", "--format", "json"))
	if !strings.Contains(out, "stale citation refs/source.md@deadbeef") {
		t.Fatalf("expected a stale-citation WARNING without failure, got: %s", out)
	}
}

// TestLintCitationWikiWiring proves the check also runs over wiki pages, which
// the corpus-wide `sdt context lint` does not scan.
func TestLintCitationWikiWiring(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/refs/source.md", "the source body\n")
	fresh := refSHA8(t, "context/refs/source.md")
	page := "---\nkind: wiki\nid: page\ntitle: Page\nsummary: s\ntype: concept\nstatus: draft\ntags: [x]\n---\n" +
		"a claim `{#claim-1}` citing refs/source.md@deadbeef:3\n"
	writeTestFile(t, "context/wiki/page.md", page)

	found := false
	for _, it := range lintWikiLint() {
		if strings.Contains(it.Message, "stale citation refs/source.md@deadbeef (actual "+fresh+")") {
			found = true
		}
	}
	if !found {
		t.Fatal("wiki lint must verify body citations")
	}
}
