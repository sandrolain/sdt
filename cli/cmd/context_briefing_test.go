package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestContextBriefingScaffold(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, contextNewCmd, nil, "--type", "briefing", "--title", "Auth subsystem", "--summary", "how auth fits")

	path := filepath.Join(dir, "context", "briefing", "auth-subsystem.md")
	content := mustReadFile(t, path)
	for _, want := range []string{
		"kind: briefing",
		"subject: auth-subsystem",
		"## Artifacts", "## Map", "## Roles", "## Rules", "## Derived facts", "## Delta",
		"Derive, don't trust",
		"2026-10-11 — <replace this dated example with the first second-verified change>",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("scaffold missing %q:\n%s", want, content)
		}
	}
}

func TestAgentInitBriefingScaffoldHasDatedDeltaExample(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p")
	content := mustReadFile(t, filepath.Join(dir, "context", "briefing", "onboarding.md"))
	want := "2026-10-11 — <replace this dated example with the first second-verified change>"
	if !strings.Contains(content, want) {
		t.Fatalf("agent init briefing scaffold missing dated Delta example %q:\n%s", want, content)
	}
	uid := parseFrontmatterField(content, ctxFrontmatterUID)
	if !validUIDv7(uid) {
		t.Errorf("agent init briefing scaffold uid = %q, want a canonical UUIDv7", uid)
	}
	for _, issue := range lintDoc(filepath.Join(dir, "context", "briefing", "onboarding.md")) {
		if strings.Contains(issue.Message, "Delta") {
			t.Errorf("agent init scaffold trips its own Delta lint: %#v", issue)
		}
	}
}

func TestBriefingLint(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join(dir, "context", "briefing", "auth.md")

	// A derived fact with no deriving command and an undated delta.
	writeCtxDoc(t, "context/briefing/auth.md",
		"---\nkind: briefing\nsubject: auth\nstatus: draft\nsummary: s\nsources:\n  - x\n---\n\n## Derived facts\n\n- the current version is 3\n\n## Delta\n\nthings changed\n")
	issues := lintDoc(path)
	var gotMissingCmd, gotUndated bool
	for _, i := range issues {
		if strings.Contains(i.Message, "deriving command") {
			gotMissingCmd = true
		}
		if strings.Contains(i.Message, "Delta") {
			gotUndated = true
		}
	}
	if !gotMissingCmd {
		t.Errorf("expected a derived-fact-without-command finding, got %v", issues)
	}
	if !gotUndated {
		t.Errorf("expected an undated-delta finding, got %v", issues)
	}

	// A well-formed briefing passes the briefing checks.
	writeCtxDoc(t, "context/briefing/auth.md",
		"---\nkind: briefing\nsubject: auth\nstatus: draft\nsummary: s\nsources:\n  - x\n---\n\n## Derived facts\n\n- the current version is obtained with `sdt version`\n\n## Delta\n\n2026-10-09: initial briefing\n")
	for _, i := range lintDoc(path) {
		if strings.Contains(i.Message, "deriving command") || strings.Contains(i.Message, "Delta") {
			t.Errorf("well-formed briefing flagged: %v", i)
		}
	}
}
