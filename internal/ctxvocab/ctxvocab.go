// Package ctxvocab reads the corpus controlled vocabularies: the category
// register (context/categories.yaml), the topic register (context/topics.yaml)
// and the objective groups (the context/objectives/ directory). It is the one
// place those three files are parsed, so the CLI lint, `context new` and the
// viewer filters cannot drift about which values the corpus accepts.
package ctxvocab

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// Register file paths and the objective directory, relative to the project root.
const (
	CategoriesFile = "context/categories.yaml"
	TopicsFile     = "context/topics.yaml"
	ObjectivesDir  = "context/objectives"
)

// slugReplacer and slugRegexp mirror the CLI's slug sanitation, so a value
// canonicalized here matches the one lint accepts.
var (
	slugReplacer = regexp.MustCompile(`[^a-z0-9-]+`)
	slugRegexp   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// Slug normalizes a register value to the kebab-case grammar.
func Slug(s string) string {
	s = slugReplacer.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	return strings.Trim(s, "-")
}

// SlugRegexp is the objective/topic grammar (kebab-case slug), for callers that
// validate a value rather than read a register.
func SlugRegexp() *regexp.Regexp { return slugRegexp }

// Register is a parsed controlled vocabulary: canonical slug -> aliases, plus
// the alias -> canonical index used to canonicalize a written value.
type Register struct {
	Values  map[string][]string // canonical slug -> aliases
	Aliases map[string]string   // alias and canonical -> canonical slug
}

// Slugs returns the canonical values in sorted order.
func (r *Register) Slugs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.Values))
	for slug := range r.Values {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

// Canonical returns the canonical value for a written one, and whether it is
// known (canonical or a registered alias).
func (r *Register) Canonical(v string) (string, bool) {
	if r == nil {
		return "", false
	}
	c, ok := r.Aliases[Slug(v)]
	return c, ok
}

func readRegister(path string, key string) (*Register, error) {
	reg := &Register{Values: map[string][]string{}, Aliases: map[string]string{}}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo-relative path
	if os.IsNotExist(err) {
		return reg, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]map[string][]string
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for canonical, aliases := range doc[key] {
		c := Slug(canonical)
		if c == "" {
			continue
		}
		reg.Values[c] = aliases
		reg.Aliases[c] = c
		for _, a := range aliases {
			if s := Slug(a); s != "" {
				reg.Aliases[s] = c
			}
		}
	}
	return reg, nil
}

// LoadCategories reads the category register. A missing file yields an empty
// register: the vocabulary is advisory, its absence never blocks a caller.
func LoadCategories(root string) (*Register, error) {
	return readRegister(filepath.Join(root, CategoriesFile), "categories")
}

// LoadTopics reads the topic register (same tolerance as the categories).
func LoadTopics(root string) (*Register, error) {
	return readRegister(filepath.Join(root, TopicsFile), "topics")
}

// Objectives lists the objective group ids: the `context/objectives/<id>.md`
// documents, sorted. A group is a document whose stem is already a kebab-case
// slug, so the value the frontmatter carries matches it verbatim. A missing
// directory yields an empty list.
func Objectives(root string) ([]string, error) {
	dir := filepath.Join(root, ObjectivesDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if slugRegexp.MatchString(stem) {
			out = append(out, stem)
		}
	}
	sort.Strings(out)
	return out, nil
}
