// Package learn owns the project-scoped learner state
// (context/learning/learner.yaml): the learner profile, the goals, one record per
// learned concept, and the session log. It is the one place the learner-state
// schema is parsed and validated, so the CLI verbs cannot drift about what the
// state accepts or what a stable record id is.
//
// The state is a plain, versioned YAML file and is **learner evidence, not wiki
// knowledge**: nothing in it is authoritative for the corpus, and a rating is
// never promoted from inferred to observed by loading it. A missing file yields
// an empty state (its absence never blocks a caller); a malformed or invalid file
// returns an error the caller surfaces. Dates are UTC: calendar dates
// (YYYY-MM-DD) for review dates, RFC3339 for provenance stamps.
package learn

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// File is the learner-state path, relative to the project root.
const File = "context/learning/learner.yaml"

// Version is the learner-state schema version.
const Version = 1

// DateLayout is the calendar-date format the state stores.
const DateLayout = "2006-01-02"

// TimeLayout is the RFC3339 (UTC) format the state stores for provenance stamps.
const TimeLayout = time.RFC3339

// Assessment bases: whether a rating came from the learner or was inferred by the
// agent. An inferred rating stays inferred; one answer never implies mastery.
const (
	BasisObserved = "observed"
	BasisInferred = "inferred"
)

// Understanding levels, weakest first. Understanding is the agent's reading of the
// learner's last demonstration, never a claim about the corpus.
const (
	UnderstandingUnknown = "unknown"
	UnderstandingWeak    = "weak"
	UnderstandingPartial = "partial"
	UnderstandingSolid   = "solid"
)

// Profile is who is learning and since when.
type Profile struct {
	Name string `yaml:"name,omitempty"`
	// Since is a YYYY-MM-DD calendar date.
	Since string `yaml:"since,omitempty"`
}

// Goal is one declared learning outcome.
type Goal struct {
	ID      string `yaml:"id"`
	Concept string `yaml:"concept"`
	Outcome string `yaml:"outcome"`
	// Created is a YYYY-MM-DD calendar date.
	Created string `yaml:"created,omitempty"`
}

// Assessment is the last rating of one concept, with its provenance.
type Assessment struct {
	// At is an RFC3339 UTC stamp.
	At string `yaml:"at,omitempty"`
	// Basis is observed or inferred; empty means inferred (the weaker claim).
	Basis string `yaml:"basis,omitempty"`
	// Understanding is unknown, weak, partial or solid.
	Understanding string `yaml:"understanding,omitempty"`
}

// Concept is one concept record: the wiki page it maps to, its prerequisites,
// the evidence behind it, the last assessment, an open misconception and the day
// it is next due for review.
type Concept struct {
	// ID is the stable record id, always ConceptID(Wiki).
	ID            string      `yaml:"id"`
	Wiki          string      `yaml:"wiki"`
	Prerequisites []string    `yaml:"prerequisites,omitempty"`
	Evidence      []string    `yaml:"evidence,omitempty"`
	LastAssessed  *Assessment `yaml:"last_assessed,omitempty"`
	Misconception string      `yaml:"misconception,omitempty"`
	// NextReview is a YYYY-MM-DD calendar date.
	NextReview string `yaml:"next_review,omitempty"`
}

// Session is one session-log entry: what was covered, what stayed weak and what
// became due. Concept references are stable record ids.
type Session struct {
	// At is an RFC3339 UTC stamp.
	At      string   `yaml:"at"`
	Covered []string `yaml:"covered,omitempty"`
	Weak    []string `yaml:"weak,omitempty"`
	Due     []string `yaml:"due,omitempty"`
}

// State is the parsed context/learning/learner.yaml.
type State struct {
	Version  int       `yaml:"version"`
	Profile  Profile   `yaml:"profile,omitempty"`
	Goals    []Goal    `yaml:"goals,omitempty"`
	Concepts []Concept `yaml:"concepts,omitempty"`
	Sessions []Session `yaml:"sessions,omitempty"`
}

// nonID matches every run of characters a stable record id cannot carry; each run
// collapses into a single dash.
var nonID = regexp.MustCompile(`[^a-z0-9]+`)

// ConceptID derives the stable record id of a wiki page id, so the same concept
// always gets the same id and a second record cannot claim it. The mapping is
// lossy (a/b-c and a-b/c both map to a-b-c), which Validate catches by refusing a
// duplicate id, never by silently merging.
func ConceptID(wiki string) string {
	return strings.Trim(nonID.ReplaceAllString(strings.ToLower(strings.TrimSpace(wiki)), "-"), "-")
}

// Path returns the learner-state path for a project root.
func Path(root string) string {
	return filepath.Join(root, File)
}

// Load reads the learner state for a project root. A missing file yields an empty
// state; a malformed or invalid file returns an error.
func Load(root string) (*State, error) {
	path := Path(root)
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo-relative path
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var st State
	if err := yaml.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := st.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &st, nil
}

// Save validates and writes the state atomically (a temp file renamed into
// place). It creates the parent directory when absent.
func Save(root string, st *State) error {
	if st == nil {
		st = &State{}
	}
	if err := st.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301 -- project state dir
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //#nosec G306 -- learner-private state
		return err
	}
	return os.Rename(tmp, path)
}

// idPattern is the accepted shape of a hand-written record id.
var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Validate checks the state invariants: a supported schema version, kebab-case
// ids unique within their list, a goal with a concept and an outcome, a concept
// whose id matches its page and whose stamps and dates parse, and a session with
// a timestamp.
func (s *State) Validate() error {
	if s == nil {
		return fmt.Errorf("nil state")
	}
	if s.Version != 0 && s.Version != Version {
		return fmt.Errorf("unsupported learner-state version %d (want %d)", s.Version, Version)
	}
	if err := s.validateProfile(); err != nil {
		return err
	}
	if err := s.validateGoals(); err != nil {
		return err
	}
	if err := s.validateConcepts(); err != nil {
		return err
	}
	return s.validateSessions()
}

// validateProfile checks the optional profile date.
func (s *State) validateProfile() error {
	if s.Profile.Since == "" {
		return nil
	}
	if _, err := time.Parse(DateLayout, s.Profile.Since); err != nil {
		return fmt.Errorf("profile: since %q is not a %s date", s.Profile.Since, DateLayout)
	}
	return nil
}

// validateGoals checks every goal and keeps ids unique across the list.
func (s *State) validateGoals() error {
	seen := map[string]bool{}
	for i := range s.Goals {
		g := &s.Goals[i]
		if err := validateID("goal", g.ID, seen); err != nil {
			return err
		}
		if strings.TrimSpace(g.Concept) == "" {
			return fmt.Errorf("goal %q: concept is required", g.ID)
		}
		if strings.TrimSpace(g.Outcome) == "" {
			return fmt.Errorf("goal %q: outcome is required", g.ID)
		}
		if g.Created != "" {
			if _, err := time.Parse(DateLayout, g.Created); err != nil {
				return fmt.Errorf("goal %q: created %q is not a %s date", g.ID, g.Created, DateLayout)
			}
		}
	}
	return nil
}

// validateConcepts checks every concept record, including that its id is the
// stable id of the page it maps to.
func (s *State) validateConcepts() error {
	seen := map[string]bool{}
	for i := range s.Concepts {
		c := &s.Concepts[i]
		if err := validateID("concept", c.ID, seen); err != nil {
			return err
		}
		if err := c.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate checks one concept record's own invariants.
func (c *Concept) validate() error {
	if strings.TrimSpace(c.Wiki) == "" {
		return fmt.Errorf("concept %q: wiki is required", c.ID)
	}
	if want := ConceptID(c.Wiki); c.ID != want {
		return fmt.Errorf("concept %q: id must be the stable id of its page (%q)", c.ID, want)
	}
	for j, p := range c.Prerequisites {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("concept %q: prerequisites[%d] is empty", c.ID, j)
		}
	}
	for j, e := range c.Evidence {
		if strings.TrimSpace(e) == "" {
			return fmt.Errorf("concept %q: evidence[%d] is empty", c.ID, j)
		}
	}
	if c.LastAssessed != nil {
		if err := c.LastAssessed.validate(c.ID); err != nil {
			return err
		}
	}
	if c.NextReview != "" {
		if _, err := time.Parse(DateLayout, c.NextReview); err != nil {
			return fmt.Errorf("concept %q: next_review %q is not a %s date", c.ID, c.NextReview, DateLayout)
		}
	}
	return nil
}

// validateSessions checks that every session-log entry carries a stamp.
func (s *State) validateSessions() error {
	for i := range s.Sessions {
		if _, err := time.Parse(TimeLayout, s.Sessions[i].At); err != nil {
			return fmt.Errorf("session %d: at %q is not an %s stamp", i, s.Sessions[i].At, TimeLayout)
		}
	}
	return nil
}

// validate checks one assessment: a parseable stamp, a known basis and a known
// understanding level.
func (a *Assessment) validate(id string) error {
	if a.At != "" {
		if _, err := time.Parse(TimeLayout, a.At); err != nil {
			return fmt.Errorf("concept %q: last_assessed.at %q is not an %s stamp", id, a.At, TimeLayout)
		}
	}
	switch a.Basis {
	case "", BasisObserved, BasisInferred:
	default:
		return fmt.Errorf("concept %q: last_assessed.basis %q is not observed or inferred", id, a.Basis)
	}
	switch a.Understanding {
	case "", UnderstandingUnknown, UnderstandingWeak, UnderstandingPartial, UnderstandingSolid:
	default:
		return fmt.Errorf("concept %q: last_assessed.understanding %q is not a known level", id, a.Understanding)
	}
	return nil
}

// validateID checks a hand-written record id and its uniqueness within its list.
func validateID(kind, id string, seen map[string]bool) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("%s: id is required", kind)
	}
	if !idPattern.MatchString(trimmed) {
		return fmt.Errorf("%s %q: id must be kebab-case", kind, id)
	}
	if seen[trimmed] {
		return fmt.Errorf("%s %q: duplicate id", kind, id)
	}
	seen[trimmed] = true
	return nil
}
