package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
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
// register (the vocabulary is advisory; absence never blocks lint).
func loadTopicRegister() (*ctxTopicRegister, error) {
	reg := &ctxTopicRegister{Topics: map[string][]string{}, Aliases: map[string]string{}}
	data, err := os.ReadFile(ctxTopicsFilePath) //#nosec G304 -- fixed repo path
	if os.IsNotExist(err) {
		return reg, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Topics map[string][]string `yaml:"topics"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", ctxTopicsFilePath, err)
	}
	for canonical, aliases := range doc.Topics {
		c := sanitizeSlug(canonical)
		if c == "" {
			continue
		}
		reg.Topics[c] = aliases
		reg.Aliases[c] = c
		for _, a := range aliases {
			reg.Aliases[sanitizeSlug(a)] = c
		}
	}
	return reg, nil
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
