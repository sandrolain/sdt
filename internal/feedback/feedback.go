// Package feedback is the lane a portal's aggregated feedback enters by: a file
// a consumer produces, treated as a source like any other.
//
// Two artefacts, deliberately different. The **aggregate** is what a portal
// exports: counts over a window, with a sample floor so no entry can point back
// at one person. The **register** is the corpus's own record: one row per signal,
// with the state a person gives it, and never an automatic correction — feedback
// is evidence to triage, not an instruction to obey.
//
// The aggregate's free text is untrusted input. It is stored and shown, never
// interpreted: this package reads fields, it does not run anything, and
// `sdt context lint --security` scans the file before an agent reads it.
package feedback

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// File is the register path, relative to the project root.
const File = "context/feedback.yaml"

// Version is the aggregate schema this build reads.
const Version = 1

// MinSample is the smallest sample an aggregate may carry. A portal that exports
// fewer than this is refused, and a single signal below it is dropped: a count of
// one is a person, not a pattern.
const MinSample = 5

// Signal kinds, closed: a content report, an item analysis, or a question the
// wiki does not answer.
const (
	KindContent  = "content"
	KindItem     = "item"
	KindQuestion = "question"
)

// Entry states, closed: a signal is opened, then accepted (it becomes work) or
// rejected (it does not), with the reason kept either way.
const (
	StateOpen     = "open"
	StateAccepted = "accepted"
	StateRejected = "rejected"
)

// Kinds returns the closed signal vocabulary.
func Kinds() []string { return []string{KindContent, KindItem, KindQuestion} }

// States returns the closed state vocabulary.
func States() []string { return []string{StateOpen, StateAccepted, StateRejected} }

// Aggregate is one portal export.
type Aggregate struct {
	Schema  int      `yaml:"schema"`
	Window  string   `yaml:"window"`
	N       int      `yaml:"n"`
	Signals []Signal `yaml:"signals"`
}

// Signal is one aggregated observation. Target is `<page-id>#c-<id>`, a page id,
// or empty for a question the corpus does not answer.
type Signal struct {
	Target string `yaml:"target,omitempty"`
	Kind   string `yaml:"kind"`
	Detail string `yaml:"detail"`
	Count  int    `yaml:"count"`
}

// Register is the parsed context/feedback.yaml.
type Register struct {
	Items []Entry `yaml:"items,omitempty"`
}

// Entry is one signal the corpus is tracking.
type Entry struct {
	ID      string `yaml:"id"`
	Target  string `yaml:"target,omitempty"`
	Kind    string `yaml:"kind"`
	Detail  string `yaml:"detail"`
	Count   int    `yaml:"count"`
	Window  string `yaml:"window"`
	State   string `yaml:"state"`
	Reason  string `yaml:"reason,omitempty"`
	Source  string `yaml:"source,omitempty"`
	Created string `yaml:"created,omitempty"`
}

// ParseDocument reads a portal export as it enters the corpus: a markdown file
// carrying one fenced yaml block. The wrapper exists because the security scan
// reads documents — a bare .yaml under `ingestion/` would arrive unscanned.
func ParseDocument(data []byte) (Aggregate, error) {
	block, err := yamlFence(string(data))
	if err != nil {
		return Aggregate{}, err
	}
	return ParseAggregate([]byte(block))
}

// yamlFence extracts the first fenced yaml block, or the whole text when the
// document is already bare yaml.
func yamlFence(content string) (string, error) {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 {
			if trimmed == "```yaml" || trimmed == "```yml" {
				start = i + 1
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			return strings.Join(lines[start:i], "\n"), nil
		}
	}
	if start >= 0 {
		return "", errors.New("aggregate: unterminated yaml fence")
	}
	return content, nil
}

// ParseAggregate reads a portal export strictly: an unknown field, an unknown
// kind and a sample below the floor are refused rather than absorbed.
func ParseAggregate(data []byte) (Aggregate, error) {
	var agg Aggregate
	dec := yaml.NewDecoder(strings.NewReader(string(data)), yaml.DisallowUnknownField())
	if err := dec.Decode(&agg); err != nil {
		return agg, fmt.Errorf("aggregate: %w", err)
	}
	if agg.Schema != Version {
		return agg, fmt.Errorf("aggregate: schema %d is not %d", agg.Schema, Version)
	}
	if strings.TrimSpace(agg.Window) == "" {
		return agg, errors.New("aggregate: no window; a count without one says nothing")
	}
	if agg.N < MinSample {
		return agg, fmt.Errorf("aggregate: n=%d is below the floor of %d", agg.N, MinSample)
	}
	for i, s := range agg.Signals {
		if !contains(Kinds(), s.Kind) {
			return agg, fmt.Errorf("aggregate: signal %d kind %q is not one of %s", i+1, s.Kind, strings.Join(Kinds(), ", "))
		}
		if strings.TrimSpace(s.Detail) == "" {
			return agg, fmt.Errorf("aggregate: signal %d carries no detail", i+1)
		}
	}
	return agg, nil
}

// Merge folds an aggregate into a register: a signal at or above the floor
// becomes an open entry with a stable id derived from its target, kind and
// window, so re-importing the same export does not duplicate it. A count below
// the floor is dropped and reported, never stored.
//
// The free text is copied as data. Nothing here interprets a detail: the entry
// exists so a person can decide what the signal means.
func (r *Register) Merge(agg Aggregate, source, created string) (added int, dropped int) {
	seen := map[string]bool{}
	for _, it := range r.Items {
		seen[it.ID] = true
	}
	for _, s := range agg.Signals {
		if s.Count < MinSample {
			dropped++
			continue
		}
		id := entryID(s, agg.Window)
		if seen[id] {
			continue
		}
		seen[id] = true
		r.Items = append(r.Items, Entry{
			ID:      id,
			Target:  s.Target,
			Kind:    s.Kind,
			Detail:  strings.TrimSpace(s.Detail),
			Count:   s.Count,
			Window:  agg.Window,
			State:   StateOpen,
			Source:  source,
			Created: created,
		})
		added++
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].ID < r.Items[j].ID })
	return added, dropped
}

// entryID is stable across re-imports of the same export: the same target, kind
// and window address the same entry.
func entryID(s Signal, window string) string {
	key := strings.Join([]string{s.Target, s.Kind, window}, "|")
	var b strings.Builder
	for _, r := range key {
		if r < 128 {
			b.WriteRune(r)
		}
	}
	return "fb-" + shortHash(b.String())
}

// shortHash is a small deterministic digest over the entry key. Collisions are
// not a concern at register scale, and the id is an address, not a security
// boundary.
func shortHash(s string) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	out := make([]byte, 8)
	for i := range out {
		out[i] = alphabet[h%uint64(len(alphabet))]
		h /= uint64(len(alphabet))
	}
	return string(out)
}

// Validate checks a register: closed vocabularies, the sample floor, an accepted
// or rejected entry with a reason, and a target that names where it points.
func (r *Register) Validate() error {
	seen := map[string]bool{}
	for i, it := range r.Items {
		where := fmt.Sprintf("entry %d", i+1)
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("%s: no id", where)
		}
		if seen[it.ID] {
			return fmt.Errorf("%s: id %q appears twice", where, it.ID)
		}
		seen[it.ID] = true
		if !contains(Kinds(), it.Kind) {
			return fmt.Errorf("%s: kind %q is not one of %s", where, it.Kind, strings.Join(Kinds(), ", "))
		}
		if !contains(States(), it.State) {
			return fmt.Errorf("%s: state %q is not one of %s", where, it.State, strings.Join(States(), ", "))
		}
		if it.Count < MinSample {
			return fmt.Errorf("%s: count %d is below the floor of %d", where, it.Count, MinSample)
		}
		if strings.TrimSpace(it.Window) == "" {
			return fmt.Errorf("%s: no window", where)
		}
		if strings.TrimSpace(it.Detail) == "" {
			return fmt.Errorf("%s: no detail", where)
		}
		if it.Kind != KindQuestion && strings.TrimSpace(it.Target) == "" {
			return fmt.Errorf("%s: a %s signal must name a target", where, it.Kind)
		}
		if it.Kind == KindQuestion && it.Target != "" && !strings.Contains(it.Target, "/") {
			return fmt.Errorf("%s: target %q is not a page id or <page>#c-<id>", where, it.Target)
		}
		if it.State != StateOpen && strings.TrimSpace(it.Reason) == "" {
			return fmt.Errorf("%s: %s without a reason", where, it.State)
		}
	}
	return nil
}

// Path returns the register path for a project root.
func Path(root string) string { return filepath.Join(root, File) }

// Load reads the register for a project root. A missing file is an empty
// register, never an error.
func Load(root string) (*Register, error) {
	data, err := os.ReadFile(Path(root)) //#nosec G304 -- fixed repo-relative path
	if os.IsNotExist(err) {
		return &Register{}, nil
	}
	if err != nil {
		return nil, err
	}
	var reg Register
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	if err := reg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	return &reg, nil
}

// Render is the markdown projection: one line per entry, grouped by state so the
// open ones read first. It carries the id and the count, never the detail's
// prose — the register holds that, and a projection that duplicated it would
// drift.
func (r *Register) Render() string {
	var b strings.Builder
	b.WriteString("<!-- generated from context/feedback.yaml; edit the register, not this file -->\n\n")
	b.WriteString("# Feedback register\n\n")
	if len(r.Items) == 0 {
		b.WriteString("_No signals recorded._\n")
		return b.String()
	}
	order := []string{StateOpen, StateAccepted, StateRejected}
	for _, state := range order {
		var rows []Entry
		for _, it := range r.Items {
			if it.State == state {
				rows = append(rows, it)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", state)
		for _, it := range rows {
			target := it.Target
			if target == "" {
				target = "—"
			}
			fmt.Fprintf(&b, "- `%s` · %s · %s · %d · %s\n", it.ID, it.Kind, target, it.Count, it.Window)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
