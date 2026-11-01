package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/learn"
)

// TestLearnRecordTypedMisconception covers the typed half and the relation: a
// known kind is stored beside the sentence, and a confusion resolves to the
// record id of the page it was mistaken for.
func TestLearnRecordTypedMisconception(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)

	out := string(execute(t, learnRecordCmd, nil,
		"--concept", "backend/auth",
		"--misconception", "applied the rule everywhere",
		"--misconception-kind", "overgeneralized",
		"--confused-with", "backend/registry", "--confused-with", "backend/registry"))
	if !strings.Contains(out, "misconception: overgeneralized") ||
		!strings.Contains(out, "confused with: backend-registry") {
		t.Errorf("the report must name the kind and the relation, got:\n%s", out)
	}
	st := loadRecorded(t)
	c := st.Concepts[0]
	if c.MisconceptionKind != learn.MisconceptionOvergeneralized {
		t.Errorf("kind = %q", c.MisconceptionKind)
	}
	if c.Misconception != "applied the rule everywhere" {
		t.Errorf("the sentence must survive beside the kind, got %q", c.Misconception)
	}
	if len(c.ConfusedWith) != 1 || c.ConfusedWith[0] != "backend-registry" {
		t.Errorf("a repeated target must dedup, got %+v", c.ConfusedWith)
	}
}

// TestLearnRecordTypedRefusals covers the refusals the typed misconception adds.
func TestLearnRecordTypedRefusals(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	cases := []struct {
		name string
		args []string
	}{
		{"kind without concept", []string{"--goal", "backend/auth=x", "--misconception-kind", "overgeneralized"}},
		{"relation without concept", []string{"--goal", "backend/auth=x", "--confused-with", "backend/registry"}},
		{"unknown kind", []string{"--concept", "backend/auth", "--misconception-kind", "misunderstood"}},
		{"unknown target", []string{"--concept", "backend/auth", "--confused-with", "backend/invented"}},
		{"self confusion", []string{"--concept", "backend/auth", "--confused-with", "backend/auth"}},
		{"status without goal", []string{"--concept", "backend/auth", "--goal-status", "open"}},
		{"audience without goal", []string{"--concept", "backend/auth", "--goal-audience", "a learner"}},
		{"unknown status", []string{"--goal", "backend/auth=x", "--goal-status", "done"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shouldExitWithCode(t, 1, func() string {
				return string(execute(t, learnRecordCmd, nil, tc.args...))
			})
		})
	}
}

// TestLearnRecordGoalLifecycle covers the goal's own record: it starts open and
// started today, moves to working, and closing stamps the date.
func TestLearnRecordGoalLifecycle(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)

	out := string(execute(t, learnRecordCmd, nil,
		"--goal", "backend/auth=explain the resolution order", "--goal-audience", "a learner"))
	if !strings.Contains(out, "goal: recorded") {
		t.Errorf("a goal must be reported, got:\n%s", out)
	}
	g := loadRecorded(t).Goals[0]
	if g.Status != learn.GoalOpen || g.Started != "2026-10-13" || g.Audience != "a learner" {
		t.Errorf("a new goal is open and started today: %+v", g)
	}

	execute(t, learnRecordCmd, nil, "--goal", "backend/auth=explain the resolution order",
		"--goal-status", "working")
	if got := loadRecorded(t).Goals[0].Status; got != learn.GoalWorking {
		t.Errorf("status = %q, want working", got)
	}

	execute(t, learnRecordCmd, nil, "--goal", "backend/auth=explain the resolution order",
		"--goal-status", "closed")
	g = loadRecorded(t).Goals[0]
	if g.Status != learn.GoalClosed || g.Closed != "2026-10-13" {
		t.Errorf("closing must stamp the date: %+v", g)
	}
	if g.Audience != "a learner" || g.Started != "2026-10-13" {
		t.Errorf("a status move must not lose the record: %+v", g)
	}
}

// TestLearnRecordSessionOwnsItsGoal is G10 end to end: the session derives its id,
// links itself to the goal it covered, and a repeated call does not duplicate it.
func TestLearnRecordSessionOwnsItsGoal(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	stubContextNow(t, now)

	execute(t, learnRecordCmd, nil, "--goal", "backend/auth=explain the resolution order")
	execute(t, learnRecordCmd, nil, "--session", "--covered", "backend-auth")

	st := loadRecorded(t)
	want := learn.SessionID(now)
	if len(st.Sessions) != 1 || st.Sessions[0].ID != want {
		t.Fatalf("the entry must carry the derived id %q, got %+v", want, st.Sessions)
	}
	if len(st.Goals[0].Sessions) != 1 || st.Goals[0].Sessions[0] != want {
		t.Errorf("the goal must record the session that worked it, got %+v", st.Goals[0])
	}

	// The same stamp is the same session: no twin, no duplicate link.
	execute(t, learnRecordCmd, nil, "--session", "--covered", "backend-auth")
	st = loadRecorded(t)
	if len(st.Sessions) != 1 {
		t.Errorf("a repeated stamp must merge, got %d entries", len(st.Sessions))
	}
	if len(st.Goals[0].Sessions) != 1 {
		t.Errorf("the goal must keep one reference, got %+v", st.Goals[0].Sessions)
	}
}

// TestLearnStatusShowsGoalLifecycle proves the goal's record is reported, not just
// counted.
func TestLearnStatusShowsGoalLifecycle(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	execute(t, learnRecordCmd, nil, "--goal", "backend/auth=explain the resolution order",
		"--goal-audience", "a learner")
	execute(t, learnRecordCmd, nil, "--goal", "backend/registry=map the registry",
		"--goal-status", "closed")

	out := string(execute(t, learnStatusCmd, nil))
	for _, want := range []string{
		"goals: 2",
		"backend/auth (open, 0 session(s)), for a learner",
		"backend/registry (closed, 0 session(s)), closed 2026-10-13",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q, got:\n%s", want, out)
		}
	}

	out = string(execute(t, learnResumeCmd, nil))
	if !strings.Contains(out, "goals: 2 (1 open, 1 closed)") {
		t.Errorf("resume must separate open from closed, got:\n%s", out)
	}
}

// TestLearnResumeNamesTheMisconceptionKind proves a typed misconception is what the
// next action says, instead of the whole sentence.
func TestLearnResumeNamesTheMisconceptionKind(t *testing.T) {
	runInTempDir(t)
	learnV2Fixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	execute(t, learnRecordCmd, nil,
		"--concept", "architecture/v2", "--item", "v-1", "--grade", "incorrect",
		"--misconception", "applied the rule everywhere",
		"--misconception-kind", "overgeneralized")

	out := string(execute(t, learnResumeCmd, nil))
	if !strings.Contains(out, "misconception: overgeneralized") {
		t.Errorf("resume must name the kind, got:\n%s", out)
	}
	if strings.Contains(out, "applied the rule everywhere") {
		t.Errorf("resume must not print the whole sentence, got:\n%s", out)
	}
}

// TestLearnRecordSessionLeavesUncoveredGoalsAlone proves the link is derived from
// what the session declares, not from every goal in the file.
func TestLearnRecordSessionLeavesUncoveredGoalsAlone(t *testing.T) {
	runInTempDir(t)
	learnWikiFixture(t)
	stubContextNow(t, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))

	execute(t, learnRecordCmd, nil, "--goal", "backend/registry=map the registry")
	execute(t, learnRecordCmd, nil, "--session", "--covered", "backend-auth")

	if got := loadRecorded(t).Goals[0].Sessions; len(got) != 0 {
		t.Errorf("an uncovered goal must stay unlinked, got %+v", got)
	}
}
