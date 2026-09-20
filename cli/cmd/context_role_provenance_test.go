package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextNewNotesAgentRoleFrontmatter(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	execute(t, contextNewCmd, nil, "--type", "notes", "--slug", "prov", "--agent", "opencode", "--role", "reviewer")
	content := mustReadFile(t, filepath.Join(dir, "context", "notes", "20260806-070000-prov.md"))
	for _, want := range []string{"kind: notes", "agent: opencode", "role: reviewer"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in notes frontmatter:\n%s", want, content)
		}
	}
}

func TestLintNotesMissingAgentSuggestion(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/without.md", "---\nkind: notes\nsummary: s\n---\nbody\n")
	writeCtxDoc(t, "context/notes/with.md", "---\nkind: notes\nsummary: s\nagent: opencode\n---\nbody\n")

	issues := lintDoc("context/notes/without.md")
	found := false
	for _, it := range issues {
		if strings.Contains(it.Message, "notes entry missing `agent`") {
			found = true
			if it.Priority != ctxLintSuggestion {
				t.Errorf("expected SUGGESTION, got %s", it.Priority)
			}
		}
	}
	if !found {
		t.Errorf("expected a missing-agent SUGGESTION, got %v", issues)
	}
	if ctxLintHint("notes entry missing `agent` provenance (record who produced it)") == "" {
		t.Error("expected a curated hint for the missing-agent suggestion")
	}

	for _, it := range lintDoc("context/notes/with.md") {
		if strings.Contains(it.Message, "notes entry missing `agent`") {
			t.Errorf("did not expect a missing-agent issue when agent is set: %v", it)
		}
	}
}

func TestContextListFilterByAgentRole(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/a.md", "---\nkind: notes\nsummary: a\nagent: opencode\nrole: reviewer\n---\nbody\n")
	writeCtxDoc(t, "context/notes/b.md", "---\nkind: notes\nsummary: b\nagent: human\nrole: architect\n---\nbody\n")
	writeCtxDoc(t, "context/notes/c.md", "---\nkind: notes\nsummary: c\nagent: opencode\nrole: architect\n---\nbody\n")

	byAgent := string(execute(t, contextListCmd, nil, "--type", "notes", "--agent", "opencode"))
	if !strings.Contains(byAgent, "a.md") || !strings.Contains(byAgent, "c.md") || strings.Contains(byAgent, "b.md") {
		t.Errorf("unexpected --agent filter result:\n%s", byAgent)
	}

	byRole := string(execute(t, contextListCmd, nil, "--type", "notes", "--role", "architect"))
	if !strings.Contains(byRole, "b.md") || !strings.Contains(byRole, "c.md") || strings.Contains(byRole, "a.md") {
		t.Errorf("unexpected --role filter result:\n%s", byRole)
	}

	both := string(execute(t, contextListCmd, nil, "--type", "notes", "--agent", "opencode", "--role", "architect"))
	if !strings.Contains(both, "c.md") || strings.Contains(both, "a.md") || strings.Contains(both, "b.md") {
		t.Errorf("unexpected combined filter result:\n%s", both)
	}
}
