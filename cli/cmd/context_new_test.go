//go:build !wasm

package cmd

import (
	"strings"
	"testing"
)

// TestNewAnalysisStartsDraft pins the lifecycle at the point it is written: a new
// analysis is `draft` — started, not yet concluded — and `active` is the state it
// reaches when it is concluded. The registry table test pins the field; this pins
// what the CLI does with it, which is the claim a reader of the instruction module
// acts on.
func TestNewAnalysisStartsDraft(t *testing.T) {
	runInTempDir(t)

	out := string(execute(t, contextNewCmd, nil, "--type", "analysis", "--title", "Lifecycle", "--slug", "lifecycle", "--summary", "s"))
	path := strings.TrimSpace(out)
	if path == "" {
		t.Fatalf("no path in output: %q", out)
	}
	if got := frontmatterField(mustReadFile(t, path), ctxMapStatus); got != ctxWikiStatusDraft {
		t.Errorf("a new analysis is %q, want %q", got, ctxWikiStatusDraft)
	}

	// The explicit flag still wins, so the two states stay reachable without a
	// second write.
	out = string(execute(t, contextNewCmd, nil, "--type", "analysis", "--title", "Concluded", "--slug", "concluded", "--summary", "s", "--status", ctxWikiStatusActive))
	if got := frontmatterField(mustReadFile(t, strings.TrimSpace(out)), ctxMapStatus); got != ctxWikiStatusActive {
		t.Errorf("explicit --status active wrote %q", got)
	}

	// A concluded analysis has to be reachable in one step, and its own vocabulary
	// is unchanged by where it starts.
	for _, want := range []string{ctxWikiStatusDraft, ctxWikiStatusActive, taskFileStatusCompleted, statusPostponed, statusArchived} {
		if !ctxStatusInVocab(ctxTypeLookupOrFatal(t, ctxTypeAnalysis), want) {
			t.Errorf("analysis vocabulary no longer carries %q", want)
		}
	}
}

// ctxTypeLookupOrFatal keeps the assertion above readable.
func ctxTypeLookupOrFatal(t *testing.T, kind string) ctxDocType {
	t.Helper()
	tp, ok := ctxTypeLookup(kind)
	if !ok {
		t.Fatalf("registry missing %q", kind)
	}
	return tp
}

// TestNewPreservesDocumentSuffix pins the suffix modifier at creation: a
// `.slide` or `.map` slug is a file-name suffix over the base kind, not a
// kebab-case slug, so the dot survives and the file is `<date>-<slug>.md`.
func TestNewPreservesDocumentSuffix(t *testing.T) {
	runInTempDir(t)
	for _, tc := range []struct{ slug, want string }{
		{"deck.slide", "-deck.slide.md"},
		{"deck.map", "-deck.map.md"},
		{"deck", "-deck.md"},
	} {
		out := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
			"--type", "notes", "--title", "Deck", "--slug", tc.slug, "--summary", "s")))
		if !strings.HasSuffix(out, tc.want) {
			t.Errorf("--slug %q wrote %q, want suffix %q", tc.slug, out, tc.want)
		}
	}
}

// TestContextPathPreservesDocumentSuffix keeps the dry-run path verb in step
// with the write path.
func TestContextPathPreservesDocumentSuffix(t *testing.T) {
	runInTempDir(t)
	out := strings.TrimSpace(string(execute(t, contextPathCmd, nil,
		"--type", "notes", "--slug", "deck.slide")))
	if !strings.HasSuffix(out, "-deck.slide.md") {
		t.Errorf("context path wrote %q, want a -deck.slide.md suffix", out)
	}
}
