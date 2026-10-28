package cmd

import (
	"fmt"
	"strings"
)

// Optional frontmatter families.
//
// The base keys (kind, id, title, summary, status, created, updated) are
// mandatory for every document. Everything beyond them is an *axis* a project
// may or may not use — maturity, a warrant for a promotion, an evidence class,
// a verified stamp, a rigor dial, the objective grouping key and its checklist.
// Running those checks unasked is the worst of both worlds: a team that never
// records a maturity sees a warning it cannot act on, and "0 findings" stops
// meaning anything.
//
// So the axis is declared: `instructions.frontmatter` in `.sdt.yaml` names the
// families this project uses, and lint stays silent about the rest. A project
// with no `.sdt.yaml` at all resolves to *every* family on — there is no
// declaration to narrow the check, and `sdt context lint <path>` on an arbitrary
// directory stays informative.

type frontmatterFamily struct {
	Slug    string
	Title   string
	Summary string
	Keys    []string
}

var frontmatterFamilyRegister = []frontmatterFamily{
	{Slug: "maturity", Title: "Maturity", Summary: "the trust axis and its successor",
		Keys: []string{ctxFrontmatterMaturity, ctxFrontmatterSupersededBy}},
	{Slug: "warrant", Title: "Warrant", Summary: "the evidence that earned a promotion",
		Keys: []string{ctxFrontmatterWarrant}},
	{Slug: "evidence", Title: "Evidence class", Summary: "how a claim is known",
		Keys: []string{ctxFrontmatterEvidenceClass}},
	{Slug: "verified", Title: "Verified stamp", Summary: "when and against what a codebase-adjacent claim was last checked",
		Keys: []string{ctxFrontmatterVerifiedAt, ctxFrontmatterVerifiedAgnst}},
	{Slug: "rigor", Title: "Rigor", Summary: "the review dial a plan is held to",
		Keys: []string{ctxFrontmatterRigor}},
	{Slug: "objectives", Title: "Objectives", Summary: "the grouping key and the objective checklist",
		Keys: []string{ctxFrontmatterObjective}},
	{Slug: "learning", Title: "Learning block", Summary: "the objectives, difficulty and review time a concept page states",
		Keys: []string{ctxFrontmatterLearning}},
}

// frontmatterFamilyBySlug resolves a slug against the register.
func frontmatterFamilyBySlug(slug string) (frontmatterFamily, bool) {
	for _, f := range frontmatterFamilyRegister {
		if f.Slug == slug {
			return f, true
		}
	}
	return frontmatterFamily{}, false
}

// frontmatterFamilySlugs lists the register's slugs, for help and error text.
func frontmatterFamilySlugs() []string {
	out := make([]string, 0, len(frontmatterFamilyRegister))
	for _, f := range frontmatterFamilyRegister {
		out = append(out, f.Slug)
	}
	return out
}

// validateFrontmatterFamilies refuses a family the register does not know: a
// typo would otherwise silently enable nothing.
func validateFrontmatterFamilies(families []string) error {
	for _, raw := range families {
		for _, slug := range strings.Split(raw, ",") {
			slug = strings.TrimSpace(slug)
			if slug == "" {
				continue
			}
			if _, ok := frontmatterFamilyBySlug(slug); !ok {
				return fmt.Errorf("unknown frontmatter family %q (known: %s)", slug, strings.Join(frontmatterFamilySlugs(), ", "))
			}
		}
	}
	return nil
}

// currentFrontmatterFamilies resolves the enabled families as a set:
//
//   - `.sdt.yaml` absent — every family is on (no declaration narrows it);
//   - `.sdt.yaml` present with no `instructions.frontmatter` — base keys only;
//   - `.sdt.yaml` present with the key — exactly the declared families.
//
// An unknown name is ignored here rather than failing every command; the
// explicit refusals live where the key is written (`agent init`/`refresh`), and
// `context lint` reports it once as a CRITICAL config finding.
func currentFrontmatterFamilies() map[string]bool {
	set := map[string]bool{}
	cfg := loadProjectConfig(sdtConfigFile)
	if cfg == nil {
		for _, f := range frontmatterFamilyRegister {
			set[f.Slug] = true
		}
		return set
	}
	for _, raw := range cfg.Instructions.Frontmatter {
		for _, slug := range strings.Split(raw, ",") {
			slug = strings.TrimSpace(slug)
			if _, ok := frontmatterFamilyBySlug(slug); ok {
				set[slug] = true
			}
		}
	}
	return set
}

// frontmatterFamilyNotes lists the enabled families with the keys each one
// covers, for `sdt context vocab`.
func frontmatterFamilyNotes() []string {
	set := currentFrontmatterFamilies()
	var out []string
	for _, f := range frontmatterFamilyRegister {
		mark := "off"
		if set[f.Slug] {
			mark = "on"
		}
		out = append(out, fmt.Sprintf("%s [%s] — %s (`%s`)", f.Slug, mark, f.Summary, strings.Join(f.Keys, "`, `")))
	}
	return out
}

// lintFrontmatterFamilyConfig reports an unknown family name in `.sdt.yaml`
// once, as a CRITICAL configuration finding: the key is hand-written, so the
// typo is the only way to get here.
func lintFrontmatterFamilyConfig() []ctxLintIssue {
	cfg := loadProjectConfig(sdtConfigFile)
	if cfg == nil || len(cfg.Instructions.Frontmatter) == 0 {
		return nil
	}
	var issues []ctxLintIssue
	for _, raw := range cfg.Instructions.Frontmatter {
		for _, slug := range strings.Split(raw, ",") {
			slug = strings.TrimSpace(slug)
			if slug == "" {
				continue
			}
			if _, ok := frontmatterFamilyBySlug(slug); !ok {
				issues = append(issues, ctxLintIssue{
					Path:     sdtConfigFile,
					Priority: ctxLintCritical,
					Message:  fmt.Sprintf("unknown frontmatter family %q in `instructions.frontmatter` (known: %s)", slug, strings.Join(frontmatterFamilySlugs(), ", ")),
				})
			}
		}
	}
	return issues
}
