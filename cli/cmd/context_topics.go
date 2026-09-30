package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/ctxvocab"
)

// Controlled subject vocabulary for `context/*.md` frontmatter: `topics` (what
// a document is about) and `entities` (named things it mentions). Distinct from
// `objective` (the initiative/group key). The canonical list lives in
// context/topics.yaml, emitted by `sdt agent init`; lint canonicalizes aliases
// and reports unknown values as advisory SUGGESTIONs.

// ctxTopicsFilePath is the on-disk controlled register.
const ctxTopicsFilePath = "context/topics.yaml"

// ctxTopicSlugRegexp is the kebab-case grammar shared with `objective`.
var ctxTopicSlugRegexp = ctxObjectiveRegexp

// ctxTopicRegister is the parsed context/topics.yaml: canonical topics with
// their accepted aliases.
type ctxTopicRegister struct {
	Topics  map[string][]string // canonical slug -> aliases
	Aliases map[string]string   // alias (and canonical) -> canonical slug
}

// loadTopicRegister reads context/topics.yaml. A missing file yields an empty
// register. The parsing lives in internal/ctxvocab, the one place the register
// files are read (the viewer serves the same vocabulary from there).
func loadTopicRegister() (*ctxTopicRegister, error) {
	reg, err := ctxvocab.LoadTopics(".")
	if err != nil {
		return nil, err
	}
	return &ctxTopicRegister{Topics: reg.Values, Aliases: reg.Aliases}, nil
}

// canonical returns the canonical topic for a value and whether the value is
// known (canonical or a registered alias).
func (r *ctxTopicRegister) canonical(v string) (string, bool) {
	if r == nil || len(r.Aliases) == 0 {
		return v, true
	}
	c, ok := r.Aliases[sanitizeSlug(v)]
	if !ok {
		return v, false
	}
	return c, true
}

// topicSlugs returns the canonical topics sorted, for help text and lint hints.
func (r *ctxTopicRegister) topicSlugs() []string {
	slugs := make([]string, 0, len(r.Topics))
	for s := range r.Topics {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	return slugs
}

// ctxTopicReg is the register loaded by the lint command run (nil in unit tests;
// a nil register accepts every topic). It is set per-run, never global state.
var ctxTopicReg *ctxTopicRegister

// lintTopicFields validates the optional `topics`/`entities` fields: a
// non-kebab-case slug is a WARNING (it can never resolve), an unknown topic is
// a SUGGESTION with the canonical topic list. Never fails lint.
func lintTopicFields(path, content string, reg *ctxTopicRegister) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, t := range parseFrontmatterList(content, "topics") {
		if !ctxTopicSlugRegexp.MatchString(t) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("topic %q must be a kebab-case slug (lowercase letters, digits and '-')", t)})
			continue
		}
		if _, ok := reg.canonical(t); !ok {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: fmt.Sprintf("unknown topic %q (known: %s)", t, strings.Join(reg.topicSlugs(), ", "))})
		}
	}
	for _, e := range parseFrontmatterList(content, "entities") {
		if !ctxTopicSlugRegexp.MatchString(e) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("entity %q must be a kebab-case slug (lowercase letters, digits and '-')", e)})
		}
	}
	return issues
}
