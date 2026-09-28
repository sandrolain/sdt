// Package ctxquery is the shared conjunctive filter engine behind the context
// query commands (sdt context list and sdt context search). It projects one
// document's frontmatter into Facets and decides whether a Filter matches, so
// the two commands cannot drift. The engine is pure Go and reads no wall clock:
// callers inject "now" into the date parser.
package ctxquery

import (
	"slices"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// Facets is the filterable projection of one document's frontmatter.
type Facets struct {
	Kind    string
	Status  string
	Created time.Time
	Updated time.Time
	// Values holds every top-level frontmatter field; a scalar is stored as a
	// one-element slice and a list keeps its order.
	Values map[string][]string
}

// FacetsFromContent builds the facets from a full document (frontmatter plus
// body).
func FacetsFromContent(content string) Facets {
	return facetsFromValues(contextwiki.FrontmatterValues(content))
}

// FacetsFromFrontmatter builds the facets from a frontmatter block including
// its "---" delimiters.
func FacetsFromFrontmatter(fm string) Facets {
	return facetsFromValues(contextwiki.FrontmatterValues(fm))
}

func facetsFromValues(values map[string][]string) Facets {
	f := Facets{Values: values}
	f.Kind = firstValue(values, "kind")
	f.Status = firstValue(values, "status")
	f.Created = parseTimestamp(firstValue(values, "created"))
	f.Updated = parseTimestamp(firstValue(values, "updated"))
	return f
}

func firstValue(values map[string][]string, key string) string {
	if v := values[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Term is one `key=value` or `key!=value` constraint.
type Term struct {
	Key    string
	Value  string
	Negate bool
}

// Filter is a conjunctive filter: every set field and every term must match.
type Filter struct {
	Kinds    []string
	Statuses []string
	// Categories matches any of the document's `categories` list values
	// (any-match), mirroring the `context list/search --category` alias.
	Categories []string
	After      *time.Time
	Before     *time.Time
	// DateField selects the reference timestamp: "updated", otherwise created.
	DateField string
	Terms     []Term
}

// Match reports whether the facets satisfy every filter condition.
func (f Filter) Match(fc Facets) bool {
	if len(f.Kinds) > 0 && !slices.Contains(f.Kinds, fc.Kind) {
		return false
	}
	if len(f.Statuses) > 0 && !slices.Contains(f.Statuses, fc.Status) {
		return false
	}
	if len(f.Categories) > 0 && !anyContains(fc.Values["categories"], f.Categories) {
		return false
	}
	if f.After != nil || f.Before != nil {
		ref := fc.Created
		if f.DateField == "updated" {
			ref = fc.Updated
		}
		if ref.IsZero() {
			return false
		}
		if f.After != nil && ref.Before(*f.After) {
			return false
		}
		if f.Before != nil && ref.After(*f.Before) {
			return false
		}
	}
	for _, t := range f.Terms {
		if !t.matches(fc.Values) {
			return false
		}
	}
	return true
}

// matches reports whether the term holds for the value map. A scalar field
// matches on exact (case-sensitive) equality; a list field matches when the
// value is a member; an absent key matches `=` never and `!=` always.
func (t Term) matches(values map[string][]string) bool {
	matched := slices.Contains(values[t.Key], t.Value)
	if t.Negate {
		return !matched
	}
	return matched
}

// anyContains reports whether any of the wanted values is present in the list.
func anyContains(values, wanted []string) bool {
	for _, w := range wanted {
		if slices.Contains(values, w) {
			return true
		}
	}
	return false
}
