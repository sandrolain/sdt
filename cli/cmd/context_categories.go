package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/ctxvocab"
)

// Controlled vocabulary for the `categories` frontmatter field: the kind of
// work an analysis is (bug, issue, new feature, ...), distinct from `topics`
// (its subject) and `objective` (the initiative). The canonical list lives in
// context/categories.yaml, emitted by `sdt agent init`; lint canonicalizes
// aliases and reports unknown values as advisory SUGGESTIONs.

// ctxCategoriesFilePath is the on-disk controlled register.
const ctxCategoriesFilePath = "context/categories.yaml"

// ctxCategoryRegister is the parsed context/categories.yaml: canonical
// categories with their accepted aliases.
type ctxCategoryRegister struct {
	Categories map[string][]string // canonical slug -> aliases
	Aliases    map[string]string   // alias (and canonical) -> canonical slug
}

// loadCategoryRegister reads context/categories.yaml. A missing file yields an
// empty register (the vocabulary is advisory; absence never blocks lint). The
// parsing lives in internal/ctxvocab, the one place the register files are read
// (the viewer serves the same vocabulary from there).
func loadCategoryRegister() (*ctxCategoryRegister, error) {
	reg, err := ctxvocab.LoadCategories(".")
	if err != nil {
		return nil, err
	}
	return &ctxCategoryRegister{Categories: reg.Values, Aliases: reg.Aliases}, nil
}

// canonical returns the canonical category for a value and whether the value is
// known (canonical or a registered alias).
func (r *ctxCategoryRegister) canonical(v string) (string, bool) {
	if r == nil || len(r.Aliases) == 0 {
		return v, true
	}
	c, ok := r.Aliases[sanitizeSlug(v)]
	if !ok {
		return v, false
	}
	return c, true
}

// categorySlugs returns the canonical categories sorted, for lint hints.
func (r *ctxCategoryRegister) categorySlugs() []string {
	slugs := make([]string, 0, len(r.Categories))
	for s := range r.Categories {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	return slugs
}

// ctxCategoryReg is the register loaded by the lint command run (nil in unit
// tests; a nil register accepts every category). Set per-run, never global.
var ctxCategoryReg *ctxCategoryRegister

// lintCategoryFields validates the optional `categories` field: a non-kebab-case
// slug is a WARNING (it can never resolve), an unknown category is a SUGGESTION
// with the canonical category list. Never fails lint.
func lintCategoryFields(path, content string, reg *ctxCategoryRegister) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, c := range parseFrontmatterList(content, "categories") {
		if !ctxTopicSlugRegexp.MatchString(c) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("category %q must be a kebab-case slug (lowercase letters, digits and '-')", c)})
			continue
		}
		if _, ok := reg.canonical(c); !ok {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: fmt.Sprintf("unknown category %q (known: %s)", c, strings.Join(reg.categorySlugs(), ", "))})
		}
	}
	return issues
}
