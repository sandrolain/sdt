package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── agent roles check (Phase 5) ───────────────────────────────────────────────

func TestRolesCheckCleanAfterInit(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	if findings := roleCheckFindings(); len(findings) != 0 {
		t.Fatalf("expected no findings on a fresh init, got %+v", findings)
	}
}

func TestRolesCheckCleanCommand(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	out := string(execute(t, agentRolesCheckCmd, nil))
	if out != "" {
		t.Errorf("expected empty text output on a clean run, got:\n%s", out)
	}
}

func TestRolesCheckMissingProfile(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	if err := os.Remove(filepath.Join(sdtRolesDir, "backend.md")); err != nil {
		t.Fatal(err)
	}
	findings := roleCheckFindings()
	if len(findings) == 0 {
		t.Fatal("expected findings for a missing profile")
	}
	found := false
	for _, f := range findings {
		if f.Priority == ctxLintCritical && strings.Contains(f.Message, "missing role profile") && strings.Contains(f.Path, "backend.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a CRITICAL missing-profile finding for backend.md, got %+v", findings)
	}
}

func TestRolesCheckUnregisteredProfile(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	writeTestFile(t, filepath.Join(sdtRolesDir, "ghost.md"), "---\nkind: role\nslug: ghost\ntitle: Ghost\ncreated: \"2026-01-01T00:00:00Z\"\nproject: p\n---\n")
	findings := roleCheckFindings()
	found := false
	for _, f := range findings {
		if f.Priority == ctxLintCritical && strings.Contains(f.Message, "unregistered role profile") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a CRITICAL unregistered-profile finding, got %+v", findings)
	}
}

func TestRolesCheckDrift(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	path := filepath.Join(sdtRolesDir, "pm.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := agentReplaceSection(string(data), roleSectionName("pm", roleLayerCore), "CORRUPTED CORE")
	if err := os.WriteFile(path, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := roleCheckFindings()
	found := false
	for _, f := range findings {
		if f.Priority == ctxLintCritical && strings.Contains(f.Message, "drifted") && strings.Contains(f.Path, "pm.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a CRITICAL drift finding for pm.md, got %+v", findings)
	}
}

func TestRolesCheckPreferencesNotDrift(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	path := filepath.Join(sdtRolesDir, "backend.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// User-owned preferences scope may diverge freely; it must NOT count as
	// drift.
	edited := agentReplaceSection(string(data), roleSectionName("backend", roleLayerPreferences), "## User preferences\n\n- always use table-driven tests\n")
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range roleCheckFindings() {
		if strings.Contains(f.Path, "backend.md") {
			t.Errorf("editing user preferences must not produce findings, got %+v", f)
		}
	}
}

func TestRolesCheckOverlap(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	// Deliberately overlapping fixture: backend and frontend both declare the
	// repo path `shared` in their project layers.
	for _, slug := range []string{"backend", "frontend"} {
		path := filepath.Join(sdtRolesDir, slug+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body := "## Project layer\n\n- Owned repo paths: `shared` (repo)\n"
		edited := agentReplaceSection(string(data), roleSectionName(slug, roleLayerProject), body)
		if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	findings := roleCheckFindings()
	found := false
	for _, f := range findings {
		if f.Priority == ctxLintCritical && strings.Contains(f.Message, "owned path overlap") && f.Path == "shared" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a CRITICAL overlap finding for `shared`, got %+v", findings)
	}
}

func TestRolesCheckJSON(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	out := execute(t, agentRolesCheckCmd, nil, "--format", "json")
	if strings.TrimSpace(string(out)) != "[]" && strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected an empty findings array on a clean run, got:\n%s", out)
	}
}

// ── mirror in context lint ────────────────────────────────────────────────────

func TestLintMirrorsRoleFindingsAsSuggestion(t *testing.T) {
	setupContextProject(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	// Corrupt a core layer: the deterministic finding mirrors into lint as a
	// SUGGESTION, never a failure.
	path := filepath.Join(sdtRolesDir, "pm.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := agentReplaceSection(string(data), roleSectionName("pm", roleLayerCore), "CORRUPTED CORE")
	if err := os.WriteFile(path, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	out := string(execute(t, contextLintCmd, nil))
	found := false
	for _, f := range lintRoleProfiles() {
		if f.Priority == ctxLintSuggestion && strings.Contains(f.Message, "drifted") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected lint to mirror the drift as a SUGGESTION, got %+v", lintRoleProfiles())
	}
	if !strings.Contains(out, "[SUGGESTION]") {
		t.Errorf("expected SUGGESTION issues in context lint output:\n%s", out)
	}
}

func TestLintRoleFrontmatterUnknown(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/odd.md", "---\nkind: notes\nsummary: s\nagent: opencode\nrole: sdet\n---\nbody\n")
	issues := lintDoc("context/notes/odd.md")
	found := false
	for _, it := range issues {
		if strings.Contains(it.Message, "unknown role sdet") {
			found = true
			if it.Priority != ctxLintSuggestion {
				t.Errorf("expected SUGGESTION for an unknown role, got %s", it.Priority)
			}
		}
	}
	if !found {
		t.Errorf("expected an unknown-role SUGGESTION, got %v", issues)
	}
	if ctxLintHint("unknown role sdet in `role:` frontmatter") == "" {
		t.Error("expected a curated hint for the unknown-role suggestion")
	}
}

func TestLintRoleFrontmatterKnown(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/notes/ok.md", "---\nkind: notes\nsummary: s\nagent: opencode\nrole: reviewer\n---\nbody\n")
	for _, it := range lintDoc("context/notes/ok.md") {
		if strings.Contains(it.Message, "unknown role") {
			t.Errorf("did not expect an unknown-role issue for a registered role: %v", it)
		}
	}
}

// ── mirror in agent doctor ────────────────────────────────────────────────────

func TestDoctorSurfacesRoleHealth(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")
	// No profiles yet: doctor reports the role check without failing the shell.
	out := string(execute(t, agentDoctorCmd, nil))
	if !strings.Contains(out, "roles") {
		t.Errorf("expected a roles check in doctor output:\n%s", out)
	}

	// Generate profiles: doctor role check becomes ok.
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	out = string(execute(t, agentDoctorCmd, nil))
	if !strings.Contains(out, "[ok  ] roles") {
		t.Errorf("expected a clean roles check after init:\n%s", out)
	}
	_ = dir
}
