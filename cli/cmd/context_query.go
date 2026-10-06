package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/ctxquery"
)

// ctxQueryHelp documents the query surface shared by `context list` and
// `context search`; it is appended to both command Long strings so they cannot
// drift.
const ctxQueryHelp = `

Filters (all AND-ed; shared by list and search):
  --status <s>             frontmatter status
  --after | --before <when>  dated on/after | on/before
  --last <N><unit>         window: m=minutes, h=hours, d=days, w=weeks
  --date created|updated   reference timestamp (default created)
  --where <k=v>|<k!=v>     any frontmatter field (repeatable)
  --since | --until        aliases of --after | --before

<when> accepts YYYY-MM-DD, RFC3339, now and now-<N><unit>. --where matches a
scalar field exactly (case-sensitive), a list field by membership, accepts the
literals true/false, and matches an absent key only for k!=v.`

// buildQueryFilter assembles the shared ctxquery.Filter from the query flags
// common to `context list` and `context search` (--after/--before/--last,
// --since/--until aliases, --date, --where) plus the per-command facet and term
// inputs. "now" is read once so relative windows are consistent within a call.
func buildQueryFilter(cmd *cobra.Command, kinds, statuses, categories []string, terms []ctxquery.Term) (ctxquery.Filter, error) {
	now := contextNow().UTC()
	f := ctxquery.Filter{Kinds: kinds, Statuses: statuses, Categories: categories, Terms: terms}

	after, err := resolveDateFlag(cmd, "after", "since")
	if err != nil {
		return f, err
	}
	before, err := resolveDateFlag(cmd, "before", "until")
	if err != nil {
		return f, err
	}
	if last := getStringFlag(cmd, "last", false); last != "" {
		if after != "" {
			return f, fmt.Errorf("--last and --after/--since are mutually exclusive")
		}
		d, derr := ctxquery.ParseWindow(last)
		if derr != nil {
			return f, fmt.Errorf("--last: %w", derr)
		}
		after = now.Add(-d).Format(time.RFC3339)
	}
	if after != "" {
		t, terr := ctxquery.ParseWhen(after, now)
		if terr != nil {
			return f, fmt.Errorf("--after: %w", terr)
		}
		f.After = &t
	}
	if before != "" {
		t, terr := ctxquery.ParseWhen(before, now)
		if terr != nil {
			return f, fmt.Errorf("--before: %w", terr)
		}
		f.Before = &t
	}
	switch date := getStringFlag(cmd, "date", false); date {
	case "", statusCreated, statusUpdated:
		f.DateField = date
	default:
		return f, fmt.Errorf("--date must be %s or %s, got %q", statusCreated, statusUpdated, date)
	}
	for _, w := range getStringArrayFlag(cmd, "where", false) {
		term, perr := parseWhereTerm(w)
		if perr != nil {
			return f, perr
		}
		f.Terms = append(f.Terms, term)
	}
	return f, nil
}

// stringList returns a one-element slice, or nil for an empty string.
func stringList(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// searchAliasTerms compiles the legacy structured search flags into the shared
// engine's generic terms, so `--objective`/`--topic` keep working as aliases.
func searchAliasTerms(cmd *cobra.Command) []ctxquery.Term {
	var terms []ctxquery.Term
	if o := getStringFlag(cmd, "objective", false); o != "" {
		terms = append(terms, ctxquery.Term{Key: ctxFrontmatterObjective, Value: o})
	}
	if topic := getStringFlag(cmd, "topic", false); topic != "" {
		terms = append(terms, ctxquery.Term{Key: ctxFrontmatterTopics, Value: topic})
	}
	return terms
}

// resolveDateFlag returns the value of the canonical date flag or its alias,
// rejecting a conflicting pair.
func resolveDateFlag(cmd *cobra.Command, canonical, alias string) (string, error) {
	v := getStringFlag(cmd, canonical, false)
	a := getStringFlag(cmd, alias, false)
	if v != "" && a != "" && v != a {
		return "", fmt.Errorf("--%s and --%s are aliases; pass only one", canonical, alias)
	}
	if v != "" {
		return v, nil
	}
	return a, nil
}

// parseWhereTerm parses one `key=value` / `key!=value` --where value.
func parseWhereTerm(s string) (ctxquery.Term, error) {
	if k, v, ok := strings.Cut(s, "!="); ok {
		if strings.TrimSpace(k) == "" {
			return ctxquery.Term{}, fmt.Errorf("--where %q: empty key", s)
		}
		return ctxquery.Term{Key: strings.TrimSpace(k), Value: v, Negate: true}, nil
	}
	if k, v, ok := strings.Cut(s, "="); ok {
		if strings.TrimSpace(k) == "" {
			return ctxquery.Term{}, fmt.Errorf("--where %q: empty key", s)
		}
		return ctxquery.Term{Key: strings.TrimSpace(k), Value: v}, nil
	}
	return ctxquery.Term{}, fmt.Errorf("--where %q: want key=value or key!=value", s)
}

// filterFilesContext reads each file and keeps the ones whose facets satisfy the
// filter. It is the shared file-level filter behind `context list`.
func filterFilesContext(files []string, filter ctxquery.Filter) ([]string, error) {
	var out []string
	for _, f := range files {
		data, err := os.ReadFile(f) //#nosec G304 -- path from listContextFiles
		if err != nil {
			return nil, err
		}
		if filter.Match(ctxquery.FacetsFromContent(string(data))) {
			out = append(out, f)
		}
	}
	return out, nil
}
