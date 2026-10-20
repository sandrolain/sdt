package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/templates"
)

// Generation profiles.
//
// `sdt agent init` writes a default `core` set and nothing else: a project that
// never draws a diagram, a slide deck or a map should not be handed the modules
// that describe them, because every written module is one more file the agent
// may read and one more row in the always-loaded on-action table. A domain is
// one `--with` flag away, declared in `.sdt.yaml` so `refresh` reuses it.
//
// The register is closed and ordered: `--with` names a slug here or is refused,
// and the stored list keeps this order, so the same selection always renders the
// same set.

// agentProfileCore is the always-generated profile.
const agentProfileCore = "core"

// agentProfile is one generation profile: an optional slice of the generated
// set. `core` declares no modules — it is every module no other profile claims.
type agentProfile struct {
	Slug    string
	Title   string
	Guide   string
	Modules []string
	Skills  []string
}

// agentProfileRegister is the closed set, in the order the stored list keeps.
var agentProfileRegister = []agentProfile{
	{Slug: agentProfileCore, Title: "Core", Guide: "the lifecycle, the document contracts and the CLI"},
	{Slug: "knowledge", Title: "Knowledge", Guide: "briefings, distillation, learning and the wiki",
		Modules: []string{"briefing", "distillation", "ingestion", "learning", "wiki"}},
	{Slug: "ui", Title: "UI/UX", Guide: "layout, components, tokens, motion and accessibility",
		Modules: []string{"ui", "ui-accessibility", "ui-adapters", "ui-components", "ui-layout", "ui-motion", "ui-performance", "ui-taste", "ui-tokens"},
		Skills:  []string{"sdt-ui-ux"}},
	{Slug: "vector", Title: "Vector", Guide: "icons, logos and SVG authoring",
		Modules: []string{"vector", "vector-svg", "vector-tools"},
		Skills:  []string{"sdt-vector-svg"}},
	{Slug: "slides", Title: "Slides", Guide: "slide decks and the Marp dialect",
		Modules: []string{"slides", "slides-marp"},
		Skills:  []string{"sdt-slide-deck"}},
	{Slug: "mindmap", Title: "Mind maps", Guide: "mind maps and the map markdown dialect",
		Modules: []string{"mindmap", "mindmap-markmap"},
		Skills:  []string{"sdt-mind-map"}},
	{Slug: "diagram", Title: "Diagrams", Guide: "diagram types and the Mermaid dialect",
		Modules: []string{"diagram", "diagram-mermaid"}},
	{Slug: "browser", Title: "Browser", Guide: "navigation and rendered-layout verification",
		Modules: []string{"browser", "browser-tools"},
		Skills:  []string{"sdt-browser-verification"}},
	{Slug: "roles", Title: "Roles", Guide: "the role profiles under context/roles/"},
}

// agentProfileBySlug resolves a slug against the register.
func agentProfileBySlug(slug string) (agentProfile, bool) {
	for _, p := range agentProfileRegister {
		if p.Slug == slug {
			return p, true
		}
	}
	return agentProfile{}, false
}

// resolveProfiles validates a requested selection and returns it normalised:
// unknown slugs refused, `core` always present, register order kept, no
// duplicates. An empty request resolves to core alone.
func resolveProfiles(requested []string) ([]string, error) {
	want := map[string]bool{agentProfileCore: true}
	for _, raw := range requested {
		for _, slug := range strings.Split(raw, ",") {
			slug = strings.TrimSpace(slug)
			if slug == "" {
				continue
			}
			if _, ok := agentProfileBySlug(slug); !ok {
				return nil, fmt.Errorf("unknown profile %q (known: %s)", slug, strings.Join(agentProfileSlugs(), ", "))
			}
			want[slug] = true
		}
	}
	out := make([]string, 0, len(want))
	for _, p := range agentProfileRegister {
		if want[p.Slug] {
			out = append(out, p.Slug)
		}
	}
	return out, nil
}

// agentProfileSlugs lists the register's slugs, for help and error text.
func agentProfileSlugs() []string {
	out := make([]string, 0, len(agentProfileRegister))
	for _, p := range agentProfileRegister {
		out = append(out, p.Slug)
	}
	return out
}

// selectedProfiles returns the selection as a set for the template and the
// doctor.
func selectedProfiles(profiles []string) map[string]bool {
	set := map[string]bool{agentProfileCore: true}
	for _, p := range profiles {
		set[p] = true
	}
	return set
}

// profileSelected reports whether a profile is in the selection.
func profileSelected(profiles []string, slug string) bool {
	return selectedProfiles(profiles)[slug]
}

// moduleProfile returns the profile that owns an instruction module, or "" for a
// core module. Ownership is derived from the register, so a module can belong to
// one profile only.
func moduleProfile(module string) string {
	for _, p := range agentProfileRegister {
		for _, m := range p.Modules {
			if m == module {
				return p.Slug
			}
		}
	}
	return ""
}

// moduleSelected reports whether an instruction module is written for a
// selection: a core module always, an owned one only through its profile.
func moduleSelected(module string, profiles []string) bool {
	owner := moduleProfile(module)
	if owner == "" {
		return true
	}
	return profileSelected(profiles, owner)
}

// skillProfile returns the profile that owns a skill directory, or "" for a core
// skill.
func skillProfile(dir string) string {
	for _, p := range agentProfileRegister {
		for _, s := range p.Skills {
			if s == dir {
				return p.Slug
			}
		}
	}
	return ""
}

// stubSelected reports whether a trigger is written for a selection: it is when
// its durable contract module is.
func stubSelected(s commandStub, profiles []string) bool {
	if len(profiles) == 0 {
		return true
	}
	return moduleSelected(s.contract, profiles)
}

// currentProfiles returns the project's stored selection, or core alone when
// there is no config or the stored value cannot be resolved.
func currentProfiles() []string {
	if cfg := loadProjectConfig(sdtConfigFile); cfg != nil {
		if resolved, err := resolveProfiles(cfg.Instructions.Profiles); err == nil {
			return resolved
		}
	}
	return []string{agentProfileCore}
}

// agentProfileSummary renders the selection for the doctor and the CLI output:
// "core, ui (9 modules)".
func agentProfileSummary(profiles []string) string {
	parts := make([]string, 0, len(profiles))
	for _, slug := range profiles {
		p, ok := agentProfileBySlug(slug)
		if !ok {
			continue
		}
		if len(p.Modules) == 0 && len(p.Skills) == 0 {
			parts = append(parts, slug)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%d module(s))", slug, len(p.Modules)))
	}
	return strings.Join(parts, ", ")
}

// allInstructionModules lists every embedded instruction module slug, in the
// order the generator writes them.
func allInstructionModules() []string {
	names := templates.List("instructions")
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, strings.TrimSuffix(name, ".tmpl"))
	}
	return out
}

// agentProfileModules lists every module a selection writes, sorted, for the
// matrix test.
func agentProfileModules(profiles []string) []string {
	out := make([]string, 0, 64)
	for _, name := range allInstructionModules() {
		module := strings.TrimSuffix(name, ".md")
		if moduleSelected(module, profiles) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
