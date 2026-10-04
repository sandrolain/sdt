// Package memo reads and writes the planned-operation register
// (context/memo.yaml): recurring rules with a cadence and one-off memos with a
// due date, together with the last-run state. It is the one place the register
// schema is parsed and the due computation lives, so the CLI and lint cannot
// drift about what the register accepts or when an operation is due.
//
// The register is a plain, versioned YAML file: `rules` and `memos` are lists
// of mapping entries. A missing file yields an empty register (its absence
// never blocks a caller); a malformed file returns an error the caller
// surfaces. Dates are UTC calendar dates (YYYY-MM-DD).
package memo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/sandrolain/sdt/internal/ctxvocab"
)

// File is the register path, relative to the project root.
const File = "context/memo.yaml"

// DateLayout is the calendar-date format the register stores.
const DateLayout = "2006-01-02"

// Entry kinds reported by a due listing.
const (
	KindRule = "rule"
	KindMemo = "memo"
)

// Rule is a recurring planned operation: a cadence plus the day it was last
// done. A rule with no recorded last_done is due immediately.
type Rule struct {
	ID        string `yaml:"id"`
	Summary   string `yaml:"summary"`
	EveryDays int    `yaml:"every_days"`
	LastDone  string `yaml:"last_done,omitempty"`
	Action    string `yaml:"action,omitempty"`
	Source    string `yaml:"source,omitempty"`
	Enabled   *bool  `yaml:"enabled,omitempty"`
}

// Memo is a one-off scheduled reminder with a due date. A memo is due once
// today reaches its due date and it is not yet done.
type Memo struct {
	ID      string `yaml:"id"`
	Summary string `yaml:"summary"`
	Due     string `yaml:"due"`
	Action  string `yaml:"action,omitempty"`
	Source  string `yaml:"source,omitempty"`
	Done    bool   `yaml:"done,omitempty"`
}

// Register is the parsed context/memo.yaml.
type Register struct {
	Rules []Rule `yaml:"rules,omitempty"`
	Memos []Memo `yaml:"memos,omitempty"`
}

// DueItem is one entry returned by a due listing.
type DueItem struct {
	Kind        string `json:"kind" yaml:"kind"` // KindRule | KindMemo
	ID          string `json:"id" yaml:"id"`
	Summary     string `json:"summary" yaml:"summary"`
	DueDate     string `json:"due_date,omitempty" yaml:"due_date,omitempty"` // the day it became due; empty for a rule with no last_done
	OverdueDays int    `json:"overdue_days" yaml:"overdue_days"`             // days past DueDate as of the query day (0 when due today)
	Action      string `json:"action,omitempty" yaml:"action,omitempty"`
	Source      string `json:"source,omitempty" yaml:"source,omitempty"`
}

// Path returns the register path for a project root.
func Path(root string) string {
	return filepath.Join(root, File)
}

// Load reads the register for a project root. A missing file yields an empty
// register; a malformed or invalid file returns an error.
func Load(root string) (*Register, error) {
	path := Path(root)
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo-relative path
	if os.IsNotExist(err) {
		return &Register{}, nil
	}
	if err != nil {
		return nil, err
	}
	var reg Register
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := reg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &reg, nil
}

// Save validates and writes the register atomically (a temp file renamed into
// place). It creates the parent directory when absent.
func Save(root string, reg *Register) error {
	if reg == nil {
		reg = &Register{}
	}
	if err := reg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301 -- project register dir
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { //#nosec G306 -- user-editable register
		return err
	}
	return os.Rename(tmp, path)
}

// Validate checks the register invariants: a non-empty kebab-case id unique
// across both lists, a summary, a positive cadence for a rule, a parseable
// date where one is required, and a required memo due date.
func (r *Register) Validate() error {
	seen := map[string]string{}
	for i := range r.Rules {
		rule := &r.Rules[i]
		if err := validateID(rule.ID, KindRule, seen); err != nil {
			return err
		}
		if strings.TrimSpace(rule.Summary) == "" {
			return fmt.Errorf("rule %q: summary is required", rule.ID)
		}
		if rule.EveryDays <= 0 {
			return fmt.Errorf("rule %q: every_days must be a positive integer", rule.ID)
		}
		if rule.LastDone != "" {
			if _, err := time.Parse(DateLayout, rule.LastDone); err != nil {
				return fmt.Errorf("rule %q: last_done %q is not a %s date", rule.ID, rule.LastDone, DateLayout)
			}
		}
	}
	for i := range r.Memos {
		memo := &r.Memos[i]
		if err := validateID(memo.ID, KindMemo, seen); err != nil {
			return err
		}
		if strings.TrimSpace(memo.Summary) == "" {
			return fmt.Errorf("memo %q: summary is required", memo.ID)
		}
		if _, err := time.Parse(DateLayout, memo.Due); err != nil {
			return fmt.Errorf("memo %q: due %q is not a %s date", memo.ID, memo.Due, DateLayout)
		}
	}
	return nil
}

// MakeRule returns a new enabled rule with the given id/summary/cadence.
func MakeRule(id, summary string, everyDays int) Rule {
	enabled := true
	return Rule{ID: ctxvocab.Slug(id), Summary: summary, EveryDays: everyDays, Enabled: &enabled}
}

// Find locates an entry by id: its kind and index, and whether it exists.
func (r *Register) Find(id string) (kind string, index int, ok bool) {
	id = ctxvocab.Slug(id)
	for i := range r.Rules {
		if r.Rules[i].ID == id {
			return KindRule, i, true
		}
	}
	for i := range r.Memos {
		if r.Memos[i].ID == id {
			return KindMemo, i, true
		}
	}
	return "", -1, false
}

// Remove deletes the entry with the given id, reporting whether it existed.
func (r *Register) Remove(id string) bool {
	kind, i, ok := r.Find(id)
	if !ok {
		return false
	}
	switch kind {
	case KindRule:
		r.Rules = append(r.Rules[:i], r.Rules[i+1:]...)
	case KindMemo:
		r.Memos = append(r.Memos[:i], r.Memos[i+1:]...)
	}
	return true
}

// DueOn returns the entries due on the given day: enabled rules whose cadence
// has elapsed (or that were never done) and unfinished memos whose due date has
// arrived. Rules and memos are merged and sorted by due date then id; a rule
// with no last_done sorts first (empty due date).
func (r *Register) DueOn(today time.Time) ([]DueItem, error) {
	today = DateOnly(today)
	out := []DueItem{}
	for _, rule := range r.Rules {
		if !rule.EnabledNow() {
			continue
		}
		if rule.LastDone == "" {
			out = append(out, DueItem{Kind: KindRule, ID: rule.ID, Summary: rule.Summary, Action: rule.Action, Source: rule.Source})
			continue
		}
		last, err := time.Parse(DateLayout, rule.LastDone)
		if err != nil {
			return nil, fmt.Errorf("rule %q: last_done %q: %w", rule.ID, rule.LastDone, err)
		}
		next := last.AddDate(0, 0, rule.EveryDays)
		if !today.Before(next) {
			out = append(out, DueItem{Kind: KindRule, ID: rule.ID, Summary: rule.Summary, DueDate: next.Format(DateLayout), OverdueDays: daysBetween(next, today), Action: rule.Action, Source: rule.Source})
		}
	}
	for _, memo := range r.Memos {
		if memo.Done {
			continue
		}
		due, err := time.Parse(DateLayout, memo.Due)
		if err != nil {
			return nil, fmt.Errorf("memo %q: due %q: %w", memo.ID, memo.Due, err)
		}
		if !today.Before(due) {
			out = append(out, DueItem{Kind: KindMemo, ID: memo.ID, Summary: memo.Summary, DueDate: due.Format(DateLayout), OverdueDays: daysBetween(due, today), Action: memo.Action, Source: memo.Source})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DueDate != out[j].DueDate {
			return out[i].DueDate < out[j].DueDate
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// EnabledNow reports whether the rule participates in the due computation
// (default true when `enabled` is omitted).
func (rule Rule) EnabledNow() bool {
	return rule.Enabled == nil || *rule.Enabled
}

// DateOnly truncates a time to its UTC calendar day.
func DateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Today returns the current UTC calendar day.
func Today() time.Time {
	return DateOnly(time.Now())
}

func validateID(id, kind string, seen map[string]string) error {
	if !ctxvocab.SlugRegexp().MatchString(id) {
		return fmt.Errorf("%s id %q must be a kebab-case slug (lowercase letters, digits and '-')", kind, id)
	}
	if prior, dup := seen[id]; dup {
		return fmt.Errorf("id %q is already used by a %s entry (ids must be unique across rules and memos)", id, prior)
	}
	seen[id] = kind
	return nil
}

// daysBetween returns whole days from a to b (both UTC calendar days).
func daysBetween(a, b time.Time) int {
	return int(b.Sub(a).Hours() / 24)
}
