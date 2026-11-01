package learn

import (
	"os"
	"strings"
	"testing"
	"time"
)

// v2File is a version-2 learner state: a misconception as a bare sentence, a goal
// with its four original fields, and a session with no id. It is what every file
// written before wave 2 looks like.
const v2File = `version: 2
goals:
  - id: g1
    concept: backend/auth
    outcome: explain the resolution order
    created: 2026-10-01
concepts:
  - id: backend-auth
    wiki: backend/auth
    misconception: conflates resolution with lookup
sessions:
  - at: 2026-10-12T10:05:00Z
    covered:
      - backend-auth
`

// TestMisconceptionKindVocabulary binds the closed kind vocabulary and its
// predicate: a seventh kind is a deliberate act, and a topic is not a kind.
func TestMisconceptionKindVocabulary(t *testing.T) {
	want := []string{
		"overgeneralized", "conflated_terms", "wrong_precondition",
		"missing_distinction", "false_causality", "stale_model",
	}
	got := MisconceptionKinds()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	for _, kind := range got {
		if !KnownMisconceptionKind(kind) {
			t.Errorf("%q must be known", kind)
		}
	}
	for _, unknown := range []string{"", "topics", "Overgeneralized", "misunderstood"} {
		if KnownMisconceptionKind(unknown) {
			t.Errorf("%q must not be a known kind", unknown)
		}
	}
}

// TestGoalStatusVocabulary binds the lifecycle: three states, in order, and an
// empty value is not one of them (it means open, normalized on save).
func TestGoalStatusVocabulary(t *testing.T) {
	want := []string{"open", "working", "closed"}
	if got := GoalStatuses(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	for _, status := range want {
		if !KnownGoalStatus(status) {
			t.Errorf("%q must be known", status)
		}
	}
	if KnownGoalStatus("") || KnownGoalStatus("done") {
		t.Error("an empty or unknown status must be refused")
	}
}

// TestSessionIDIsDerivedAndStable proves the id comes from the stamp alone, so a
// re-run produces the same entry instead of a duplicate.
func TestSessionIDIsDerivedAndStable(t *testing.T) {
	at := time.Date(2026, 10, 12, 10, 5, 0, 0, time.UTC)
	if got := SessionID(at); got != "s20261012T100500" {
		t.Fatalf("SessionID = %q", got)
	}
	if SessionID(at) != SessionID(at.UTC()) {
		t.Error("the id must not depend on the time's location")
	}
	if SessionID(at) == SessionID(at.Add(time.Second)) {
		t.Error("two stamps a second apart must not collide")
	}
}

// TestLoadDoesNotNormalizeV2 proves the read path promotes nothing: an old file
// reads exactly as written, and the upgrade happens on the next Save.
func TestLoadDoesNotNormalizeV2(t *testing.T) {
	st, err := Load(writeState(t, v2File))
	if err != nil {
		t.Fatalf("a version-2 file must load: %v", err)
	}
	if st.Goals[0].Status != "" || st.Sessions[0].ID != "" {
		t.Errorf("Load must not fill defaults, got %+v / %+v", st.Goals[0], st.Sessions[0])
	}
}

// TestSaveUpgradesV2ToV3 proves the migration carries every old field and fills
// what version 3 makes explicit.
func TestSaveUpgradesV2ToV3(t *testing.T) {
	root := writeState(t, v2File)
	st, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := Save(root, st); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "version: 3") {
		t.Errorf("the upgraded file must carry version 3:\n%s", raw)
	}

	again, err := Load(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	g := again.Goals[0]
	if g.Status != GoalOpen || g.Created != "2026-10-01" || g.Outcome != "explain the resolution order" {
		t.Errorf("the upgrade lost or mistyped the goal: %+v", g)
	}
	if again.Sessions[0].ID != "s20261012T100500" {
		t.Errorf("the upgrade must stamp the session id, got %q", again.Sessions[0].ID)
	}
	if c := again.Concepts[0]; c.Misconception != "conflates resolution with lookup" || c.MisconceptionKind != "" {
		t.Errorf("the sentence must survive and the kind stay empty: %+v", c)
	}
}

// TestV3FieldsRoundTrip covers the new surface both ways.
func TestV3FieldsRoundTrip(t *testing.T) {
	root := writeState(t, `version: 3
goals:
  - id: g1
    concept: backend/auth
    outcome: explain the resolution order
    created: 2026-10-01
    started: 2026-10-01
    closed: 2026-10-20
    audience: a learner
    status: closed
    sessions:
      - s20261012T100500
concepts:
  - id: backend-auth
    wiki: backend/auth
    misconception: applied the rule everywhere
    misconception_kind: overgeneralized
    confused_with:
      - backend-registry
`)
	st, err := Load(root)
	if err != nil {
		t.Fatalf("a version-3 file must load: %v", err)
	}
	g := st.Goals[0]
	if g.Status != GoalClosed || g.Started != "2026-10-01" || g.Closed != "2026-10-20" ||
		g.Audience != "a learner" || len(g.Sessions) != 1 {
		t.Errorf("goal = %+v", g)
	}
	c := st.Concepts[0]
	if c.MisconceptionKind != MisconceptionOvergeneralized ||
		len(c.ConfusedWith) != 1 || c.ConfusedWith[0] != "backend-registry" {
		t.Errorf("concept = %+v", c)
	}
	if err := Save(root, st); err != nil {
		t.Fatalf("save: %v", err)
	}
}

// TestValidateRejectsV3Violations covers the refusals the new fields add.
func TestValidateRejectsV3Violations(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{
			"unknown kind",
			"version: 3\nconcepts:\n  - id: a\n    wiki: a\n    misconception_kind: misunderstood\n",
			"misconception_kind \"misunderstood\" is not a known kind",
		},
		{
			"confused_with shape",
			"version: 3\nconcepts:\n  - id: a\n    wiki: a\n    confused_with:\n      - Not An Id\n",
			"is not a concept record id",
		},
		{
			"confused_with repeat",
			"version: 3\nconcepts:\n  - id: a\n    wiki: a\n    confused_with:\n      - b\n      - b\n",
			"repeats",
		},
		{
			"unknown status",
			"version: 3\ngoals:\n  - id: g1\n    concept: a\n    outcome: b\n    status: done\n",
			"status \"done\" is not a known status",
		},
		{
			"goal date shape",
			"version: 3\ngoals:\n  - id: g1\n    concept: a\n    outcome: b\n    started: 01/10/2026\n",
			"started \"01/10/2026\" is not a 2006-01-02 date",
		},
		{
			"goal session repeat",
			"version: 3\ngoals:\n  - id: g1\n    concept: a\n    outcome: b\n    sessions:\n      - s1\n      - s1\n",
			"sessions[1] \"s1\" repeats",
		},
		{
			"duplicate session id",
			"version: 3\nsessions:\n  - id: s1\n    at: 2026-10-12T10:05:00Z\n  - id: s1\n    at: 2026-10-12T10:06:00Z\n",
			"id \"s1\" repeats",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeState(t, tc.body))
			if err == nil {
				t.Fatalf("invalid state accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}
