package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// TestLintCitationMissResolvesByDigest proves the miss path of the citation
// lint (option B of the refs-custody analysis): when the cited path does not
// resolve, the run walks context/refs/ once and reports a moved artifact as a
// resolvable SUGGESTION, a genuinely missing digest as the existing not-found
// WARNING, and a digest collision by naming both files, never guessing.
func TestLintCitationMissResolvesByDigest(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		body     string
		priority string
		msg      string
	}{
		{
			name: "moved target resolves by digest as SUGGESTION",
			files: map[string]string{
				"context/refs/2026/evolved.md": "evolutionary architecture body\n",
			},
			body:     "see refs/old/deleted.md@%s:1-4 for the claim\n",
			priority: ctxLintSuggestionPriority,
			msg:      "citation target moved, still resolvable (was refs/old/deleted.md, now refs/2026/evolved.md)",
		},
		{
			name: "genuinely missing digest stays not-found",
			files: map[string]string{
				"context/refs/live.md": "the source body\n",
			},
			body:     "see refs/gone.md@deadbeef:1\n",
			priority: ctxLintWarning,
			msg:      "citation target not found: refs/gone.md",
		},
		{
			name: "digest collision names both files and guesses nothing",
			files: map[string]string{
				"context/refs/copy-a.md": "identical bytes\n",
				"context/refs/copy-b.md": "identical bytes\n",
			},
			body:     "see refs/where/was-it.md@%s:1\n",
			priority: ctxLintWarning,
			msg:      "citation digest collision: 2 files match",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runInTempDir(t)
			var digest string
			for path, content := range tc.files {
				writeTestFile(t, path, content)
				digest = refSHA8(t, path)
			}
			body := tc.body
			if strings.Contains(body, "%s") {
				body = fmt.Sprintf(body, digest)
			}
			writeTestFile(t, "context/notes/citing.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\n"+body)

			got := citationIssues(lintDoc("context/notes/citing.md"))
			if len(got) != 1 {
				t.Fatalf("citation findings = %#v, want exactly one", got)
			}
			if got[0].Priority != tc.priority {
				t.Errorf("priority = %s, want %s", got[0].Priority, tc.priority)
			}
			if !strings.Contains(got[0].Message, tc.msg) {
				t.Errorf("message %q missing %q", got[0].Message, tc.msg)
			}
		})
	}
}

// TestLintCitationCollisionNamesBothFiles pins the collision message naming
// every matching path, so the report never silently picks one.
func TestLintCitationCollisionNamesBothFiles(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/refs/copy-a.md", "identical bytes\n")
	writeTestFile(t, "context/refs/copy-b.md", "identical bytes\n")
	digest := refSHA8(t, "context/refs/copy-a.md")
	writeTestFile(t, "context/notes/citing.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\nsee refs/lost.md@"+digest+":1\n")

	got := citationIssues(lintDoc("context/notes/citing.md"))
	if len(got) != 1 {
		t.Fatalf("citation findings = %#v, want exactly one", got)
	}
	for _, path := range []string{"refs/copy-a.md", "refs/copy-b.md"} {
		if !strings.Contains(got[0].Message, path) {
			t.Errorf("collision message %q does not name %s", got[0].Message, path)
		}
	}
}

// TestLintCitationMissCacheStaysWithinTheRun proves the digest walk is paid
// once: a second miss in the same run reads the cache, and a fresh run (the
// reset each lint invocation performs) rebuilds the index instead of reading a
// previous run's tree.
func TestLintCitationMissCacheStaysWithinTheRun(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, "context/refs/moved.md", "the moved artifact\n")
	digest := refSHA8(t, "context/refs/moved.md")
	writeTestFile(t, "context/notes/citing.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\nsee refs/old.md@"+digest+":1\n")

	got := citationIssues(lintDoc("context/notes/citing.md"))
	if len(got) != 1 || got[0].Priority != ctxLintSuggestionPriority {
		t.Fatalf("first miss should resolve as moved, got %#v", got)
	}

	// Same run: the cached index still answers.
	got = citationIssues(lintDoc("context/notes/citing.md"))
	if len(got) != 1 || got[0].Priority != ctxLintSuggestionPriority {
		t.Fatalf("cached index should resolve as moved, got %#v", got)
	}

	// A new run: the cache reset re-reads the tree, so a change made between
	// runs is visible.
	resetCtxRefDigestCache()
	writeTestFile(t, "context/refs/moved.md", "the moved artifact, rewritten\n")
	got = citationIssues(lintDoc("context/notes/citing.md"))
	if len(got) != 1 || got[0].Priority != ctxLintWarning || !strings.Contains(got[0].Message, "citation target not found: refs/old.md") {
		t.Fatalf("fresh run must re-read refs/, got %#v", got)
	}
}

func TestLintCitationHints(t *testing.T) {
	for _, msg := range []string{
		"stale citation refs/x.md@fbf61784 (actual eab182ea)",
		"citation target not found: refs/x.md",
		"citation target moved, still resolvable (was refs/old.md, now refs/new.md)",
		"citation digest collision: 2 files match deadbeef (refs/a.md, refs/b.md)",
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
