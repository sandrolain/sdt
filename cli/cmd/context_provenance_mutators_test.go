package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// provenanceDoc writes one closable analysis the document mutators can act on.
func provenanceDoc(t *testing.T, status string) string {
	t.Helper()
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/prov.md",
		"---\nkind: analysis\nuid: "+testUIDv7(71)+"\nsummary: A doc\nstatus: "+status+
			"\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n## Body\nold text\n")
	return "analysis/prov.md"
}

// assertProvenance fails unless all three stamps reached the frontmatter.
func assertProvenance(t *testing.T, rel string) {
	t.Helper()
	got := readDoc(t, rel)
	for _, want := range []string{"agent: opencode", "model: deepseek", "session: s1"} {
		if !strings.Contains(got, want) {
			t.Errorf("provenance %q missing:\n%s", want, got)
		}
	}
}

// The provenance flags are the F20 contract: every mutator that writes a corpus
// document records who performed the mutation, not only the checklist verbs.
func TestDocumentMutatorsAcceptProvenance(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"context edit", contextEditCmd},
		{"context set", contextSetCmd},
		{"context unset", contextUnsetCmd},
		{"context touch", contextTouchCmd},
		{"context close", contextCloseCmd},
		{"context check", contextCheckCmd},
		{"context task add", contextTaskAddCmd},
		{"context task review", contextTaskReviewCmd},
		{"context task deviation add", contextDeviationAddCmd},
	} {
		for _, flag := range []string{ctxFieldAgent, ctxFieldModel, ctxFieldSession} {
			if tc.cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s does not accept --%s", tc.name, flag)
			}
		}
	}
}

func TestContextSetStampsProvenance(t *testing.T) {
	doc := provenanceDoc(t, "active")
	execute(t, contextSetCmd, nil, doc, "summary", "A doc",
		"--agent", "opencode", "--model", "deepseek", "--session", "s1")
	assertProvenance(t, doc)
}

func TestContextUnsetStampsProvenance(t *testing.T) {
	doc := provenanceDoc(t, "active")
	execute(t, contextUnsetCmd, nil, doc, "summary",
		"--agent", "opencode", "--model", "deepseek", "--session", "s1")
	assertProvenance(t, doc)
}

func TestContextEditStampsProvenance(t *testing.T) {
	doc := provenanceDoc(t, "active")
	execute(t, contextEditCmd, nil, doc, "--input", "## Body\nnew text\n",
		"--agent", "opencode", "--model", "deepseek", "--session", "s1")
	assertProvenance(t, doc)
	if got := readDoc(t, doc); !strings.Contains(got, "new text") {
		t.Errorf("edit did not write the body:\n%s", got)
	}
}

func TestContextCloseStampsProvenance(t *testing.T) {
	doc := provenanceDoc(t, "active")
	execute(t, contextCloseCmd, nil, doc,
		"--agent", "opencode", "--model", "deepseek", "--session", "s1")
	assertProvenance(t, doc)
}

func TestContextTouchStampsProvenance(t *testing.T) {
	doc := provenanceDoc(t, "active")
	execute(t, contextTouchCmd, nil, doc,
		"--agent", "opencode", "--model", "deepseek", "--session", "s1")
	assertProvenance(t, doc)
}

// One value, two spellings: writing the `agent` key while passing --agent is
// refused rather than letting the stamp silently overwrite the argument.
func TestContextSetRefusesProvenanceCollision(t *testing.T) {
	doc := provenanceDoc(t, "active")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextSetCmd, nil, doc, ctxFieldAgent, "explicit", "--agent", "opencode"))
	})
	if got := readDoc(t, doc); strings.Contains(got, "agent:") {
		t.Errorf("a refused write must leave the frontmatter alone:\n%s", got)
	}
}
