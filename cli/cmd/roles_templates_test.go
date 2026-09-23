package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// profileMaxCoreLines is the ≤ ~80 lines-per-profile-body contract (technical
// "about eighty"). Rendered core bodies must stay under it; overflow belongs in
// per-role reference files.
const profileMaxCoreLines = 82

// ninePointSections are the 9-point structure headings every role core must
// carry, in order.
var ninePointSections = []string{
	"### 1. Mission",
	"### 2. Responsibilities and non-responsibilities",
	"### 3. Role-relevant project context",
	"### 4. Conventions and constraints",
	"### 5. User preferences",
	"### 6. Working mode",
	"### 7. Handoff (capability contract)",
	"### 8. Definition of done",
	"### 9. Anti-patterns",
}

func TestRoleProfileFilesCoverRegister(t *testing.T) {
	now := time.Now()
	files := roleProfileFiles("sdt_test", now)
	// shared.md + one profile per register slug.
	if len(files) != len(roleRegister)+1 {
		t.Fatalf("expected %d role files (shared + %d roles), got %d", len(roleRegister)+1, len(roleRegister), len(files))
	}
	if files[0].name != sdtRolesSharedFile {
		t.Errorf("first file must be the shared rules file, got %q", files[0].name)
	}
	for i, r := range roleRegister {
		f := files[i+1]
		if f.name != r.Slug+sdtMarkdownExt {
			t.Errorf("file %d: expected %s, got %s", i, r.Slug+sdtMarkdownExt, f.name)
		}
		if !strings.Contains(f.body, "title: \""+r.Title+"\"") {
			t.Errorf("%s: profile must carry the register title %q", r.Slug, r.Title)
		}
		if !strings.Contains(f.body, "slug: "+r.Slug) {
			t.Errorf("%s: profile must carry its slug", r.Slug)
		}
	}
}

func TestRoleCoreTemplatePerSlug(t *testing.T) {
	// Every register slug must have an authored core; nothing renders empty.
	for _, r := range roleRegister {
		body, ok := instrRoleCoreTemplate(r)
		if !ok {
			t.Errorf("role %q has no authored core template", r.Slug)
			continue
		}
		if strings.TrimSpace(body) == "" {
			t.Errorf("role %q core is empty", r.Slug)
		}
	}
}

func TestRoleCoreNinePointStructure(t *testing.T) {
	// Coverage of the shared rules file: it must exist and carry the register.
	for _, r := range roleRegister {
		body, ok := instrRoleCoreTemplate(r)
		if !ok {
			continue
		}
		last := -1
		for _, heading := range ninePointSections {
			idx := strings.Index(body, heading)
			if idx < 0 {
				t.Errorf("%s: missing section %q", r.Slug, heading)
				continue
			}
			if idx < last {
				t.Errorf("%s: sections out of order at %q", r.Slug, heading)
			}
			last = idx
		}
	}
}

func TestRoleCoreLineBudget(t *testing.T) {
	for _, r := range roleRegister {
		body, ok := instrRoleCoreTemplate(r)
		if !ok {
			continue
		}
		lines := strings.Count(strings.TrimRight(body, "\n"), "\n") + 1
		if lines > profileMaxCoreLines {
			t.Errorf("%s: core body is %d lines, over the ~%d limit", r.Slug, lines, profileMaxCoreLines)
		}
	}
}

func TestRoleProfilesLayeredScopes(t *testing.T) {
	now := time.Now()
	for _, r := range roleRegister {
		body, ok := roleProfileBody(r, "sdt_test", now)
		if !ok {
			t.Fatalf("cannot render profile for %s", r.Slug)
		}
		for _, layer := range []string{roleLayerCore, roleLayerProject, roleLayerPreferences} {
			marker := roleSectionName(r.Slug, layer)
			if !hasSection(body, marker) {
				t.Errorf("%s: profile misses the %s marker scope %q", r.Slug, layer, marker)
			}
		}
		// The preferences block must be present and flagged user-owned.
		if !strings.Contains(body, "user-owned") {
			t.Errorf("%s: preferences scope must declare itself user-owned", r.Slug)
		}
	}
}

func TestRoleSharedTemplateCarriesRegister(t *testing.T) {
	shared := instrRoleSharedTemplate()
	for _, r := range roleRegister {
		if !strings.Contains(shared, "`"+r.Slug+"`") {
			t.Errorf("shared rules table missing role %q", r.Slug)
		}
		if !strings.Contains(shared, r.Title) {
			t.Errorf("shared rules table missing title for %q", r.Slug)
		}
	}
	for _, want := range []string{
		"closed and curated",
		"(assumption)",
		"never invent",
		"job > plan > global",
		"passed**",
		"**passed**",
		"Open assumptions",
		"handoff",
	} {
		if !strings.Contains(shared, want) {
			t.Errorf("shared rules missing %q", want)
		}
	}
}

// ── generation (Phase 3) ───────────────────────────────────────────────────────

func TestAgentRolesInitGenerates(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentRolesInitCmd, nil, "--project", "sdt_test")
	if !strings.Contains(string(out), "[created]") {
		t.Fatalf("expected created files in output:\n%s", out)
	}
	for _, name := range append([]string{sdtRolesSharedFile}, roleProfileFileNames()...) {
		path := filepath.Join(sdtRolesDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected generated file %s: %v", path, err)
		}
	}
	shared, err := os.ReadFile(filepath.Join(sdtRolesDir, sdtRolesSharedFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shared), "`pm`") || !strings.Contains(string(shared), "closed and curated") {
		t.Errorf("shared.md must carry the register:\n%s", shared)
	}
	for _, r := range roleRegister {
		data, err := os.ReadFile(filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt))
		if err != nil {
			t.Fatalf("read %s: %v", r.Slug, err)
		}
		for _, layer := range []string{roleLayerCore, roleLayerProject, roleLayerPreferences} {
			if !hasSection(string(data), roleSectionName(r.Slug, layer)) {
				t.Errorf("%s: missing %s scope", r.Slug, layer)
			}
		}
	}
}

func TestAgentRolesInitParityWithRender(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "sdt_test")
	for _, r := range roleRegister {
		path := filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		created := parseFrontmatterField(string(data), "created")
		ts, perr := time.Parse(time.RFC3339, created)
		if perr != nil {
			t.Fatalf("%s: unparsable created %q: %v", r.Slug, created, perr)
		}
		want, ok := renderRoleProfile(r, "sdt_test", ts)
		if !ok {
			t.Fatalf("%s: cannot render", r.Slug)
		}
		if string(data) != want {
			t.Errorf("%s: on-disk profile differs from renderRoleProfile (drift)\n got:\n%s\nwant:\n%s", r.Slug, data, want)
		}
	}
}

func TestAgentRolesInitNonDestructive(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	path := filepath.Join(sdtRolesDir, "pm.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(data) + "\n<!-- user note outside markers -->\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	out := execute(t, agentRolesInitCmd, nil, "--project", "p")
	if !strings.Contains(string(out), "[skipped]") {
		t.Errorf("expected skipped on re-run without --force:\n%s", out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != edited {
		t.Errorf("non-force run must not touch an existing profile:\n%s", after)
	}
}

func TestAgentRolesInitForcePreservesPreferences(t *testing.T) {
	runInTempDir(t)
	execute(t, agentRolesInitCmd, nil, "--project", "p")
	path := filepath.Join(sdtRolesDir, "backend.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	prefName := roleSectionName("backend", roleLayerPreferences)
	coreName := roleSectionName("backend", roleLayerCore)
	userPrefs := "## User preferences\n\n- always use table-driven tests\n- never mock the database\n"
	edited := agentReplaceSection(string(data), prefName, userPrefs)
	edited = agentReplaceSection(edited, coreName, "CORRUPTED CORE")
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	execute(t, agentRolesInitCmd, nil, "--project", "p", "--force")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "always use table-driven tests") {
		t.Errorf("--force overwrote the user preferences scope:\n%s", got)
	}
	if strings.Contains(string(got), "CORRUPTED CORE") {
		t.Errorf("--force did not refresh the generated core layer:\n%s", got)
	}
	core, ok := instrRoleCoreTemplate(roleDescriptor{Slug: "backend", Title: "Backend engineer"})
	if !ok {
		t.Fatal("backend core template missing")
	}
	if !strings.Contains(string(got), strings.TrimSpace(core)) {
		t.Errorf("--force did not restore the authored core body:\n%s", got)
	}
}

// TestGeneratedRolesMatchTemplates guards against drift between the role
// templates and the generated files under context/roles/ (shared.md + one
// profile per register slug). For the profiles, the generated core/project
// layers must equal a fresh render for the same project/created timestamp; the
// user-owned preferences scope is deliberately excluded, exactly as the check
// does. A failure means a template changed without `sdt agent roles init --force`.
//
// context/ is gitignored (the repo tracks code only), so the generated
// workspace is absent on a fresh CI checkout; the guard is local-only and
// skips when the directory is missing.
func TestGeneratedRolesMatchTemplates(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, sdtRolesDir)); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present (context/ is gitignored); drift guard is local-only", sdtRolesDir)
	}
	// Fact derivation scans the current directory (repo evidence), so the guard
	// must run from the repo root exactly like `sdt agent roles check`.
	t.Chdir(root)

	// Shared rules file: single-marker generated body.
	sharedPath := filepath.Join(sdtRolesDir, sdtRolesSharedFile)
	data, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read generated %s: %v", sharedPath, err)
	}
	name := agentGeneratedMarkerName(filepath.Base(sdtRolesDir), sdtRolesSharedFile)
	if string(data) != agentRenderGenerated(name, instrRoleSharedTemplate()) {
		t.Errorf("drift in %s: template differs from the committed generated file (run sdt agent roles init --force)", sharedPath)
	}

	// Role profiles: generated core/project layers must match a fresh render.
	for _, r := range roleRegister {
		path := filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read generated %s: %v", path, err)
		}
		content := string(data)
		if got := parseFrontmatterField(content, "slug"); got != r.Slug {
			t.Errorf("drift in %s: frontmatter slug %q, register has %q", path, got, r.Slug)
		}
		created := parseFrontmatterField(content, "created")
		ts, perr := time.Parse(time.RFC3339, created)
		if perr != nil {
			t.Errorf("drift in %s: unparsable created %q: %v", path, created, perr)
			continue
		}
		project := parseFrontmatterField(content, "project")
		want, ok := roleProfileBody(r, project, ts)
		if !ok {
			t.Fatalf("%s: cannot render profile for %s", path, r.Slug)
		}
		for _, layer := range []string{roleLayerCore, roleLayerProject} {
			name := roleSectionName(r.Slug, layer)
			gotBody, gotOK := sectionBody(content, name)
			if !gotOK {
				t.Errorf("drift in %s: missing %s layer marker", path, layer)
				continue
			}
			if wantBody, wantOK := sectionBody(want, name); !wantOK || gotBody != wantBody {
				t.Errorf("drift in %s: %s layer differs from the template (run sdt agent roles init --force)", path, layer)
			}
		}
	}
}

// roleProfileFileNames returns the profile file names in register order.

func roleProfileFileNames() []string {
	names := make([]string, 0, len(roleRegister))
	for _, r := range roleRegister {
		names = append(names, r.Slug+sdtMarkdownExt)
	}
	return names
}
