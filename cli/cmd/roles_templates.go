package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

// ── role profile templates (role core + shared rules file) ────────────────────
//
// Profile generation is base-first: the role core is the stable, project-agnostic
// layer; the project layer and the user preferences are separate marker scopes
// that regeneration (--force) and the user own respectively. Every layer marker
// is scoped `roles/<slug>/<layer>` so the existing marker-merge machinery can
// refresh a single layer. The shared rules file (context/roles/shared.md) holds
// the rules common to every profile; profiles reference it instead of restating
// it (no duplication).

// sdtRolesSharedFile is the shared-rules document every role profile references.
const sdtRolesSharedFile = "shared.md"

// roleLayerCore / roleLayerProject / roleLayerPrefs are the three marker scopes
// per profile, in generation order. The preferences scope is user-owned and is
// never rewritten by --force (see agentWriteRoleFile in the init phase).

const (
	roleLayerCore        = "core"
	roleLayerProject     = "project"
	roleLayerPreferences = "preferences"
)

// roleSectionName returns the marker name for a role profile layer.
func roleSectionName(role, layer string) string {
	return filepath.Join(filepath.Base(sdtRolesDir), role, layer)
}

// ── shared rules file ─────────────────────────────────────────────────────────

// instrRoleSharedTemplate renders context/roles/shared.md from the closed
// register so the role table never drifts from the Go constant.
func instrRoleSharedTemplate() string {
	data := struct {
		Rows []roleDescriptor
	}{Rows: roleRegister}
	return templates.Must("roles/shared.md.tmpl", data, roleFuncMap())
}

// ── role core templates ────────────────────────────────────────────────────────

// instrRoleCoreTemplate renders the role-core layer body (9-point structure)
// for one role from its embedded roles/core/<slug>.md.tmpl file. A missing file
// (unknown/not-yet-authored slug) reports ok=false, exactly like the old map
// lookup. Points tied to the project (3 context, 4 conventions) and the user
// (5 preferences) are layer boundaries, resolved by the project and preferences
// scopes respectively.

func instrRoleCoreTemplate(r roleDescriptor) (string, bool) {
	name := "roles/core/" + r.Slug + ".md.tmpl"
	if !templates.Exists(name) {
		return "", false
	}
	return templates.Must(name, r, nil), true
}

// roleSharedFrontmatter is the frontmatter for role profile files.
func roleFrontmatter(r roleDescriptor, project string, now time.Time) string {
	return strings.Join([]string{
		"---",
		"kind: role",
		"title: \"" + r.Title + "\"",
		"slug: " + r.Slug,
		"summary: \"Generated role profile for " + r.Slug + " (closed register); shared rules in context/roles/shared.md.\"",
		"created: \"" + now.UTC().Format(time.RFC3339) + "\"",
		"project: " + project,
		"---",
	}, "\n") + "\n"
}

// rolePreferencesStub is the initial user-owned preferences body. --force never
// touches this scope.
func rolePreferencesStub() string {
	return "## User preferences\n\n(_user-owned_: `sdt agent roles init --force` never overwrites this block; the\nuser edits it directly.)\n"
}

// renderRoleProfile renders the full generated profile document for one role in
// its three layered marker scopes, using the same marker-merge helpers as the
// writer so the drift guard and the on-disk result never diverge. Facts are
// derived from repo evidence on every render, so the profile stays in sync with
// the repository.
func renderRoleProfile(r roleDescriptor, project string, now time.Time) (string, bool) {
	return renderRoleProfileFacts(r, project, now, deriveRoleProjectFacts())
}

// renderRoleProfileFacts is the facts-aware render used by the writer so a
// single derive pass (plus optional user answers) feeds every profile.
func renderRoleProfileFacts(r roleDescriptor, project string, now time.Time, facts roleProjectFacts) (string, bool) {
	core, ok := instrRoleCoreTemplate(r)
	if !ok {
		return "", false
	}
	content := roleFrontmatter(r, project, now)
	content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerCore), core, true)
	content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerProject), roleProjectLayer(r, facts), true)
	content = agentAppendIfMissing(content, roleSectionName(r.Slug, roleLayerPreferences), rolePreferencesStub())
	return content, true
}

// roleProfileBody is the render entry used by tests and the generated-file set.
func roleProfileBody(r roleDescriptor, project string, now time.Time) (string, bool) {
	return renderRoleProfile(r, project, now)
}

// ── generation into context/roles/ ─────────────────────────────────────────────

// agentWriteRoleFile writes or refreshes one role profile. It is non-destructive
// by default (an existing profile is preserved unless force is set). With force
// the generated `core` and `project` layers are refreshed in place, while the
// user-owned `preferences` scope is never rewritten: a missing preferences scope
// is appended, a present one is left untouched. Frontmatter and any user content
// outside the markers survive.

func agentWriteRoleFile(path string, r roleDescriptor, project string, now time.Time, force bool, facts roleProjectFacts) FileResult {
	res := FileResult{Path: path}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir, user-chosen output
	if err != nil && !os.IsNotExist(err) {
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	exists := err == nil
	if exists && !force {
		res.Status = statusSkipped
		res.Reason = "file already exists (use --force to overwrite)"
		return res
	}

	var content string
	if exists {
		core, ok := instrRoleCoreTemplate(r)
		if !ok {
			res.Status = statusError
			res.Reason = "no authored core template for role " + r.Slug
			return res
		}
		content = string(data)
		content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerCore), core, true)
		content, _ = agentMergeBlock(content, roleSectionName(r.Slug, roleLayerProject), roleProjectLayer(r, facts), true)
		content = agentAppendIfMissing(content, roleSectionName(r.Slug, roleLayerPreferences), rolePreferencesStub())
	} else {
		rendered, ok := renderRoleProfileFacts(r, project, now, facts)
		if !ok {
			res.Status = statusError
			res.Reason = "no authored core template for role " + r.Slug
			return res
		}
		content = rendered
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G703 G306 -- user-chosen output
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if exists {
		res.Status = statusUpdated
	} else {
		res.Status = statusCreated
	}
	return res
}

// writeRoleFiles generates the shared rules file and every role profile under
// context/roles/. The shared file is single-scope (agentWriteGeneratedFile); the
// profiles use the three-layer writer that preserves user preferences.

func writeRoleFiles(project string, force bool, facts roleProjectFacts) []FileResult {
	now := time.Now()
	var results []FileResult
	sharedPath := filepath.Join(sdtRolesDir, sdtRolesSharedFile)
	results = append(results, agentWriteGeneratedFile(sharedPath, agentGeneratedMarkerName(filepath.Base(sdtRolesDir), sdtRolesSharedFile), instrRoleSharedTemplate(), force))
	for _, r := range roleRegister {
		path := filepath.Join(sdtRolesDir, r.Slug+sdtMarkdownExt)
		results = append(results, agentWriteRoleFile(path, r, project, now, force, facts))
	}
	return results
}

// ── generated file set ─────────────────────────────────────────────────────────

// roleProfileFiles returns the generated files under context/roles/: the shared
// rules file plus one profile per register slug, in register order. Used by the
// init phase and by the drift guard.

func roleProfileFiles(project string, now time.Time) []instructionFile {
	files := []instructionFile{{name: filepath.Base(sdtRolesSharedFile), body: instrRoleSharedTemplate()}}
	for _, r := range roleRegister {
		if body, ok := roleProfileBody(r, project, now); ok {
			files = append(files, instructionFile{name: r.Slug + sdtMarkdownExt, body: body})
		}
	}
	return files
}
