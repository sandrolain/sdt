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

// Version is the learner-state schema version this build writes. Versions 0, 1 and
// 2 stay readable: every added field is optional, so an old file loads unchanged
// and is rewritten as the current version on the next Save. Version 3 carries the
// typed misconception (kind plus the concept it was confused with) and the goal's
// own record (dates, audience, status, session ids) in one bump.
const Version = 3

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

// Grades are the closed vocabulary of a graded answer. A grade is a fact about one
// attempt; understanding is the standing that attempt leaves behind.
const (
	GradeCorrect          = "correct"
	GradePartiallyCorrect = "partially_correct"
	GradeIncorrect        = "incorrect"
	GradeUngradable       = "ungradable"
)

// Misconception kinds are the closed vocabulary of a typed misconception: one
// failure shape each, never a topic. The observed sentence stays in
// Concept.Misconception; the kind is the half a reader can count and a lesson can
// target.
const (
	// MisconceptionOvergeneralized applies a rule past the precondition that makes
	// it true.
	MisconceptionOvergeneralized = "overgeneralized"
	// MisconceptionConflatedTerms merges two concepts the domain keeps apart.
	MisconceptionConflatedTerms = "conflated_terms"
	// MisconceptionWrongPrecondition names the right mechanism under the wrong
	// condition for using it.
	MisconceptionWrongPrecondition = "wrong_precondition"
	// MisconceptionMissingDistinction leaves out a distinction the domain requires.
	MisconceptionMissingDistinction = "missing_distinction"
	// MisconceptionFalseCausality reads a sequence or a correlation as a cause.
	MisconceptionFalseCausality = "false_causality"
	// MisconceptionStaleModel keeps a model the design has already left behind.
	MisconceptionStaleModel = "stale_model"
)

// Goal statuses are the closed vocabulary of a goal's lifecycle. A goal with no
// status is `open`: that is what a version-2 file means.
const (
	GoalOpen    = "open"
	GoalWorking = "working"
	GoalClosed  = "closed"
)

// Schedule reasons are the closed vocabulary of why a review date was chosen. The
// reason is stored beside the date so a reader can audit the interval.
const (
	ReasonNewOrFragile       = "new_or_fragile"
	ReasonCorrectButHesitant = "correct_but_hesitant"
	ReasonIndependentSuccess = "independent_success"
	ReasonTransferSuccess    = "transfer_success"
	ReasonDurableSuccess     = "durable_success"
	ReasonFailure            = "failure"
)

// Profile is who is learning and since when.
type Profile struct {
	Name string `yaml:"name,omitempty"`
	// Since is a YYYY-MM-DD calendar date.
	Since string `yaml:"since,omitempty"`
}

// Goal is one declared learning outcome. It owns its record: when it started, when
// it closed, for whom it is declared, where it stands, and which sessions worked
// it. A goal with no status is open.
type Goal struct {
	ID      string `yaml:"id"`
	Concept string `yaml:"concept"`
	Outcome string `yaml:"outcome"`
	// Created is a YYYY-MM-DD calendar date.
	Created string `yaml:"created,omitempty"`
	// Started is a YYYY-MM-DD calendar date, stamped when the goal is declared.
	Started string `yaml:"started,omitempty"`
	// Closed is a YYYY-MM-DD calendar date, stamped when the status becomes closed.
	Closed string `yaml:"closed,omitempty"`
	// Audience is who the outcome is declared for, empty when unspecified.
	Audience string `yaml:"audience,omitempty"`
	// Status is open, working or closed.
	Status string `yaml:"status,omitempty"`
	// Sessions are the ids of the session entries that worked this goal.
	Sessions []string `yaml:"sessions,omitempty"`
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

// Attempt is the last graded answer on a concept: which item, what grade, how much
// hint it needed and where the grade came from. It is the evidence behind the
// standing, kept beside it rather than folded into it.
type Attempt struct {
	// At is an RFC3339 UTC stamp.
	At    string `yaml:"at,omitempty"`
	Item  string `yaml:"item,omitempty"`
	Grade string `yaml:"grade,omitempty"`
	Hint  int    `yaml:"hint,omitempty"`
	// Basis is observed or inferred; empty means inferred (the weaker claim).
	Basis string `yaml:"basis,omitempty"`
}

// Concept is one concept record: the wiki page it maps to, its prerequisites,
// the evidence behind it, the last assessment, an open misconception — observed as
// a sentence and, once it is typed, as its kind — the concepts it was confused
// with, and the day it is next due for review.
type Concept struct {
	// ID is the stable record id, always ConceptID(Wiki).
	ID            string      `yaml:"id"`
	Wiki          string      `yaml:"wiki"`
	Prerequisites []string    `yaml:"prerequisites,omitempty"`
	Evidence      []string    `yaml:"evidence,omitempty"`
	LastAssessed  *Assessment `yaml:"last_assessed,omitempty"`
	// Misconception is the observation as the learner stated it, verbatim.
	Misconception string `yaml:"misconception,omitempty"`
	// MisconceptionKind is the typed half: one of the closed kind vocabulary.
	MisconceptionKind string `yaml:"misconception_kind,omitempty"`
	// ConfusedWith names the concept record ids this concept was mistaken for.
	ConfusedWith []string `yaml:"confused_with,omitempty"`
	// NextReview is a YYYY-MM-DD calendar date.
	NextReview string `yaml:"next_review,omitempty"`
	// HintLevel is the hint rung the last answer needed, 0 for none.
	HintLevel int `yaml:"hint_level,omitempty"`
	// ScheduleReason is why NextReview sits where it does.
	ScheduleReason string `yaml:"schedule_reason,omitempty"`
	// LastAttempt is the last graded answer, when one was recorded.
	LastAttempt *Attempt `yaml:"last_attempt,omitempty"`
}

// Session is one session-log entry: when it happened, what it covered, what stayed
// weak and what became due. Concept and goal references are stable record ids.
type Session struct {
	// ID is the stable session id, derived from At by SessionID.
	ID string `yaml:"id,omitempty"`
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

// SessionID derives the stable id of a session entry from its own stamp. It is
// deterministic on purpose: the log stays readable in a diff, and two entries in
// the same second are refused by Validate rather than silently merged.
func SessionID(at time.Time) string {
	return "s" + at.UTC().Format("20060102T150405")
}

// normalize fills what a version-2 file leaves implicit, on the write path only:
// a goal with no status is open, and a session with no id gets the one derived
// from its own stamp. Load never promotes anything, so an old file reads exactly
// as it was written until the next Save upgrades it.
func (st *State) normalize() {
	for i := range st.Goals {
		if st.Goals[i].Status == "" {
			st.Goals[i].Status = GoalOpen
		}
	}
	for i := range st.Sessions {
		if st.Sessions[i].ID != "" {
			continue
		}
		if at, err := time.Parse(TimeLayout, st.Sessions[i].At); err == nil {
			st.Sessions[i].ID = SessionID(at)
		}
	}
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
// place). It creates the parent directory when absent, and always writes the
// current schema version: an old file is upgraded, never downgraded.
func Save(root string, st *State) error {
	if st == nil {
		st = &State{}
	}
	if st.Version > Version {
		return fmt.Errorf("unsupported learner-state version %d (want %d or lower)", st.Version, Version)
	}
	st.Version = Version
	st.normalize()
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
	if s.Version < 0 || s.Version > Version {
		return fmt.Errorf("unsupported learner-state version %d (want 0 to %d, both inclusive)", s.Version, Version)
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
		for _, date := range []struct{ name, value string }{
			{"created", g.Created}, {"started", g.Started}, {"closed", g.Closed},
		} {
			if date.value == "" {
				continue
			}
			if _, err := time.Parse(DateLayout, date.value); err != nil {
				return fmt.Errorf("goal %q: %s %q is not a %s date", g.ID, date.name, date.value, DateLayout)
			}
		}
		if g.Status != "" && !KnownGoalStatus(g.Status) {
			return fmt.Errorf("goal %q: status %q is not a known status (%s)",
				g.ID, g.Status, strings.Join(GoalStatuses(), ", "))
		}
		sessions := map[string]bool{}
		for j, id := range g.Sessions {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("goal %q: sessions[%d] is empty", g.ID, j)
			}
			if sessions[id] {
				return fmt.Errorf("goal %q: sessions[%d] %q repeats", g.ID, j, id)
			}
			sessions[id] = true
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
	if c.HintLevel < 0 {
		return fmt.Errorf("concept %q: hint_level %d cannot be negative", c.ID, c.HintLevel)
	}
	if c.ScheduleReason != "" && !KnownScheduleReason(c.ScheduleReason) {
		return fmt.Errorf("concept %q: schedule_reason %q is not a known reason", c.ID, c.ScheduleReason)
	}
	if c.LastAttempt != nil {
		if err := c.LastAttempt.validate(c.ID); err != nil {
			return err
		}
	}
	if c.MisconceptionKind != "" && !KnownMisconceptionKind(c.MisconceptionKind) {
		return fmt.Errorf("concept %q: misconception_kind %q is not a known kind (%s)",
			c.ID, c.MisconceptionKind, strings.Join(MisconceptionKinds(), ", "))
	}
	confused := map[string]bool{}
	for j, other := range c.ConfusedWith {
		if !idPattern.MatchString(other) {
			return fmt.Errorf("concept %q: confused_with[%d] %q is not a concept record id", c.ID, j, other)
		}
		if confused[other] {
			return fmt.Errorf("concept %q: confused_with[%d] %q repeats", c.ID, j, other)
		}
		confused[other] = true
	}
	return nil
}

// validate checks one attempt: a parseable stamp, a known grade and basis, and a
// non-negative hint rung.
func (a *Attempt) validate(id string) error {
	if a.At != "" {
		if _, err := time.Parse(TimeLayout, a.At); err != nil {
			return fmt.Errorf("concept %q: last_attempt.at %q is not an %s stamp", id, a.At, TimeLayout)
		}
	}
	if a.Grade != "" && !KnownGrade(a.Grade) {
		return fmt.Errorf("concept %q: last_attempt.grade %q is not a known grade", id, a.Grade)
	}
	switch a.Basis {
	case "", BasisObserved, BasisInferred:
	default:
		return fmt.Errorf("concept %q: last_attempt.basis %q is not observed or inferred", id, a.Basis)
	}
	if a.Hint < 0 {
		return fmt.Errorf("concept %q: last_attempt.hint %d cannot be negative", id, a.Hint)
	}
	return nil
}

// validateSessions checks that every session-log entry carries a stamp and a
// unique id, and that its `covered` list has no repeated concept.
func (s *State) validateSessions() error {
	seen := map[string]bool{}
	for i := range s.Sessions {
		entry := &s.Sessions[i]
		if _, err := time.Parse(TimeLayout, entry.At); err != nil {
			return fmt.Errorf("session %d: at %q is not an %s stamp", i, entry.At, TimeLayout)
		}
		if entry.ID != "" {
			if seen[entry.ID] {
				return fmt.Errorf("session %d: id %q repeats", i, entry.ID)
			}
			seen[entry.ID] = true
		}
		covered := map[string]bool{}
		for j, id := range entry.Covered {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("session %d: covered[%d] is empty", i, j)
			}
			if covered[id] {
				return fmt.Errorf("session %d: covered[%d] %q repeats", i, j, id)
			}
			covered[id] = true
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
