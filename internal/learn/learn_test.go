package learn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeState writes a raw learner-state file under a fresh project root.
func writeState(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

const VALID = `version: 1
profile:
  name: a learner
  since: 2026-10-01
goals:
  - id: g1
    concept: backend/auth
    outcome: explain the resolution order
    created: 2026-10-01
concepts:
  - id: backend-auth
    wiki: backend/auth
    prerequisites:
      - backend/registry
    evidence:
      - refs/auth.md@abcd1234:10-20
    last_assessed:
      at: 2026-10-12T10:00:00Z
      basis: observed
      understanding: partial
    misconception: conflates resolution with lookup
    next_review: 2026-10-19
sessions:
  - at: 2026-10-12T10:05:00Z
    covered:
      - backend-auth
    weak:
      - backend-auth
    due:
      - backend-auth
`

// TestLoadMissingIsEmpty covers the non-fatal absence: a project that never
// recorded learner state still yields a usable, empty state.
func TestLoadMissingIsEmpty(t *testing.T) {
	st, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("missing state must not error: %v", err)
	}
	if st == nil || len(st.Goals) != 0 || len(st.Concepts) != 0 || len(st.Sessions) != 0 {
		t.Fatalf("missing state must be empty, got %+v", st)
	}
}

// TestLoadValidState proves the whole schema round-trips: profile, goals, concept
// records with their assessment provenance, and the session log.
func TestLoadValidState(t *testing.T) {
	st, err := Load(writeState(t, VALID))
	if err != nil {
		t.Fatalf("valid state rejected: %v", err)
	}
	if st.Profile.Name != "a learner" || st.Profile.Since != "2026-10-01" {
		t.Errorf("profile = %+v", st.Profile)
	}
	if len(st.Goals) != 1 || st.Goals[0].Outcome != "explain the resolution order" {
		t.Errorf("goals = %+v", st.Goals)
	}
	if len(st.Concepts) != 1 {
		t.Fatalf("concepts = %+v", st.Concepts)
	}
	c := st.Concepts[0]
	if c.ID != ConceptID(c.Wiki) {
		t.Errorf("concept id %q is not the stable id of %q", c.ID, c.Wiki)
	}
	if c.LastAssessed == nil || c.LastAssessed.Basis != BasisObserved ||
		c.LastAssessed.Understanding != UnderstandingPartial {
		t.Errorf("assessment lost its provenance: %+v", c.LastAssessed)
	}
	if c.NextReview != "2026-10-19" || len(st.Sessions) != 1 {
		t.Errorf("review/session not parsed: %+v %+v", c, st.Sessions)
	}
}

// TestLoadRejectsInvalidStates covers the validator: every refusal names what is
// wrong, so a malformed learner state is never half-loaded.
func TestLoadRejectsInvalidStates(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"version", "version: 2\n", "unsupported learner-state version"},
		{"profile date", "profile:\n  since: 13/10/2026\n", "is not a 2006-01-02 date"},
		{"goal id missing", "goals:\n  - concept: a\n    outcome: b\n", "goal: id is required"},
		{"goal id shape", "goals:\n  - id: Goal 1\n    concept: a\n    outcome: b\n", "id must be kebab-case"},
		{"goal concept", "goals:\n  - id: g1\n    outcome: b\n", "concept is required"},
		{"goal outcome", "goals:\n  - id: g1\n    concept: a\n", "outcome is required"},
		{
			"goal duplicate", "goals:\n  - id: g1\n    concept: a\n    outcome: b\n  - id: g1\n    concept: c\n    outcome: d\n",
			"duplicate id",
		},
		{"concept wiki", "concepts:\n  - id: backend-auth\n", "wiki is required"},
		{
			"concept id drift", "concepts:\n  - id: auth\n    wiki: backend/auth\n",
			"id must be the stable id of its page",
		},
		{
			"concept duplicate", "concepts:\n  - id: backend-auth\n    wiki: backend/auth\n  - id: backend-auth\n    wiki: backend/auth\n",
			"duplicate id",
		},
		{"prerequisite empty", "concepts:\n  - id: a\n    wiki: a\n    prerequisites:\n      - \"  \"\n", "prerequisites[0] is empty"},
		{"review date", "concepts:\n  - id: a\n    wiki: a\n    next_review: soon\n", "next_review \"soon\" is not"},
		{
			"assessment stamp", "concepts:\n  - id: a\n    wiki: a\n    last_assessed:\n      at: yesterday\n",
			"last_assessed.at \"yesterday\" is not",
		},
		{
			"assessment basis", "concepts:\n  - id: a\n    wiki: a\n    last_assessed:\n      basis: assumed\n",
			"basis \"assumed\" is not observed or inferred",
		},
		{
			"assessment level", "concepts:\n  - id: a\n    wiki: a\n    last_assessed:\n      understanding: mastered\n",
			"understanding \"mastered\" is not a known level",
		},
		{"session stamp", "sessions:\n  - covered:\n      - a\n", "session 0: at \"\" is not"},
		{"malformed", "goals: [\n", "yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := Load(writeState(t, tc.body))
			if err == nil {
				t.Fatalf("invalid state accepted: %+v", st)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			if st != nil {
				t.Errorf("a rejected state must not be returned: %+v", st)
			}
		})
	}
}

// TestConceptIDIsStable pins the id derivation: the same page always maps to the
// same record id, a subpath flattens to one kebab token, and surrounding slashes
// do not change it.
func TestConceptIDIsStable(t *testing.T) {
	for _, tc := range []struct{ wiki, want string }{
		{"backend/auth", "backend-auth"},
		{"/backend/auth/", "backend-auth"},
		{"auth", "auth"},
		{" a b ", "a-b"},
	} {
		if got := ConceptID(tc.wiki); got != tc.want {
			t.Errorf("ConceptID(%q) = %q, want %q", tc.wiki, got, tc.want)
		}
		if again := ConceptID(tc.wiki); again != ConceptID(tc.wiki) {
			t.Errorf("ConceptID(%q) unstable: %q then %q", tc.wiki, tc.want, again)
		}
	}
}

// TestSaveRoundTrip proves the state is written atomically and reads back equal,
// and that Save refuses to write an invalid state.
func TestSaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := &State{
		Version: Version,
		Profile: Profile{Name: "a learner", Since: "2026-10-01"},
		Goals:   []Goal{{ID: "g1", Concept: "backend/auth", Outcome: "explain", Created: "2026-10-01"}},
		Concepts: []Concept{{
			ID: "backend-auth", Wiki: "backend/auth",
			LastAssessed: &Assessment{At: "2026-10-12T10:00:00Z", Basis: BasisInferred, Understanding: UnderstandingWeak},
			NextReview:   "2026-10-19",
		}},
	}
	if err := Save(root, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Concepts) != 1 || got.Concepts[0].LastAssessed == nil ||
		got.Concepts[0].LastAssessed.Basis != BasisInferred {
		t.Fatalf("round trip lost state: %+v", got)
	}
	if got.Profile.Name != want.Profile.Name || got.Sessions == nil && len(want.Sessions) != 0 {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if err := Save(root, &State{Version: 2}); err == nil {
		t.Error("Save must refuse an unsupported version")
	}
}
