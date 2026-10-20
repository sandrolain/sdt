package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCaptureCmd runs a command and returns its stdout and its stderr, so the
// deprecation line can be asserted on the stream it belongs to.
func runCaptureCmd(t *testing.T, c *cobra.Command, args ...string) (string, string) {
	t.Helper()
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	out := string(execute(t, c, nil, args...))
	return out, errBuf.String()
}

// TestDeprecatedAliasesKeepTheCapability is the dedupe contract: an absorbed
// verb still does exactly what it did (identical stdout), and the alias says
// once, on stderr, which verb to use instead.
func TestDeprecatedAliasesKeepTheCapability(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nlinks: none\n---\nbody\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: active\nsources:\n  - analysis/a.md\n---\nbody\n")

	cases := []struct {
		name              string
		canonical, legacy *cobra.Command
		args              []string
	}{
		{"status vocab", contextVocabCmd, contextVocabLegacyCmd, nil},
		{"status sync", contextSyncCmd, contextSyncLegacyCmd, []string{"--dry-run"}},
		{"relations path", contextRelationsPathCmd, contextTraceCmd, []string{"plan/p.md"}},
		{"relations rank", contextRelationsRankCmd, contextResolveCmd, []string{"--type", "plan"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, canonicalErr := runCaptureCmd(t, tc.canonical, tc.args...)
			got, legacyErr := runCaptureCmd(t, tc.legacy, tc.args...)
			if got != want {
				t.Errorf("alias output differs from the canonical verb:\ncanonical: %q\nalias:     %q", want, got)
			}
			if canonicalErr != "" {
				t.Errorf("the canonical verb wrote to stderr: %q", canonicalErr)
			}
			if n := strings.Count(legacyErr, "deprecated"); n != 1 {
				t.Errorf("the alias must say `deprecated` exactly once on stderr, got %d: %q", n, legacyErr)
			}
		})
	}
}

// TestReadAbsorbsContextShow proves the two readers are one: the same document
// prints identically through `read -p` (path or corpus reference) and through
// the deprecated `show`, including a section.
func TestReadAbsorbsContextShow(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: a\nlinks: none\n---\n\n## One\n\nfirst\n\n## Two\n\nsecond\n")

	byShow, _ := runCaptureCmd(t, contextShowCmd, "context/analysis/a.md")
	byPath, _ := runCaptureCmd(t, fileReadCmd, "-p", "context/analysis/a.md")
	if byPath != byShow {
		t.Errorf("read -p and context show disagree:\nread: %q\nshow: %q", byPath, byShow)
	}

	byRef, _ := runCaptureCmd(t, fileReadCmd, "-p", "analysis/a")
	if byRef != byShow {
		t.Errorf("a corpus reference did not resolve to the document:\nref:  %q\npath: %q", byRef, byShow)
	}

	section, _ := runCaptureCmd(t, fileReadCmd, "-p", "analysis/a", "--section", "Two")
	if !strings.Contains(section, "second") || strings.Contains(section, "first") {
		t.Errorf("--section did not select exactly the section: %q", section)
	}
}

// TestCheckStateAbsorbsTheStateFlags proves the general `--state` value and the
// deprecated boolean produce the same document, and that the old spelling warns.
func TestCheckStateAbsorbsTheStateFlags(t *testing.T) {
	setupContextProject(t)
	const doc = "---\nkind: plan\nsummary: p\n---\n\n## Phase 1\n\n- [ ] one <!-- c1 -->\n- [ ] two <!-- c2 -->\n"
	writeCtxDoc(t, "context/plan/p.md", doc)

	if _, errOut := runCaptureCmd(t, contextCheckCmd, "plan/p.md", "--done", "c1"); !strings.Contains(errOut, "deprecated") {
		t.Errorf("the per-state flag must warn, got stderr %q", errOut)
	}
	legacy := mustReadFile(t, "context/plan/p.md")

	writeCtxDoc(t, "context/plan/p.md", doc)
	if _, errOut := runCaptureCmd(t, contextCheckCmd, "plan/p.md", "--state", "done", "c1"); errOut != "" {
		t.Errorf("--state must not write to stderr: %q", errOut)
	}
	general := mustReadFile(t, "context/plan/p.md")

	// The documents differ only by the uid this fixture lets the CLI stamp and
	// the refreshed `updated`; the checklist must be identical.
	if !strings.Contains(legacy, "- [x] one") || !strings.Contains(general, "- [x] one") {
		t.Errorf("the item was not ticked through both spellings:\nflag:  %q\nstate: %q", legacy, general)
	}
	if strings.Count(legacy, "- [x]") != strings.Count(general, "- [x]") {
		t.Errorf("--state and the state flag ticked a different number of items:\nflag:  %q\nstate: %q", legacy, general)
	}
}

// TestStatusSetReasonIsTheClosePath proves the close capability moved into
// `status set`: a terminal status plus `--reason` records the override note, and
// the `close` alias still does the same thing while saying it is deprecated.
func TestStatusSetReasonIsTheClosePath(t *testing.T) {
	setupContextProject(t)
	const body = "---\nkind: tasks\nsummary: t\nstatus: pending\n---\n\n## Phase 1\n\n- [ ] one <!-- c1 -->\n"
	writeCtxDoc(t, "context/tasks/t.md", body)

	if _, errOut := runCaptureCmd(t, contextStatusSetCmd, "context/tasks/t.md", "--status", "completed", "--reason", "shipping later"); errOut != "" {
		t.Errorf("status set wrote to stderr: %q", errOut)
	}
	viaSet := mustReadFile(t, "context/tasks/t.md")
	if !strings.Contains(viaSet, "shipping later") || !strings.Contains(viaSet, "status: completed") {
		t.Errorf("status set did not record the close: %q", viaSet)
	}

	writeCtxDoc(t, "context/tasks/t2.md", strings.Replace(body, "summary: t", "summary: t2", 1))
	if _, errOut := runCaptureCmd(t, contextCloseCmd, "context/tasks/t2.md", "--allow-unfinished", "--reason", "shipping later"); !strings.Contains(errOut, "deprecated") {
		t.Errorf("close must warn, got stderr %q", errOut)
	}
	viaClose := mustReadFile(t, "context/tasks/t2.md")
	if !strings.Contains(viaClose, "status: completed") || !strings.Contains(viaClose, "shipping later") {
		t.Errorf("close lost the capability it is the alias of: %q", viaClose)
	}
}

// TestDeprecatedCommandsStayHidden proves the alias is out of the help surface:
// a deprecated verb is not advertised, and its canonical one is not deprecated.
func TestDeprecatedCommandsStayHidden(t *testing.T) {
	for _, c := range []*cobra.Command{contextCloseCmd, contextVocabLegacyCmd, contextSyncLegacyCmd, contextRelatedCmd, contextResolveCmd, contextTraceCmd, contextShowCmd} {
		if !c.Hidden {
			t.Errorf("%s is still advertised in help", c.Name())
		}
	}
	for _, c := range []*cobra.Command{contextVocabCmd, contextSyncCmd, contextRelationsPathCmd, contextRelationsRankCmd, contextRelationsSimilarCmd, fileReadCmd, contextStatusSetCmd, contextCheckCmd} {
		if c.Deprecated != "" {
			t.Errorf("the canonical verb %s is marked deprecated", c.Name())
		}
	}
}

// TestRelationsParentReadsTheRecordedEdge proves the new `relations parent`
// view reads the typed relation, not a guess.
func TestRelationsParentReadsTheRecordedEdge(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\nlinks: none\n---\nbody\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: active\nsources:\n  - analysis/a.md\n---\nbody\n")

	out, _ := runCaptureCmd(t, contextRelationsParentCmd, "plan/p.md")
	if !strings.Contains(out, "analysis/a.md") {
		t.Errorf("relations parent did not resolve the analysis: %q", out)
	}
	leaf, _ := runCaptureCmd(t, contextRelationsParentCmd, "analysis/a.md")
	if !strings.Contains(leaf, "no parent recorded") {
		t.Errorf("a root document must say it has no parent: %q", leaf)
	}
}
