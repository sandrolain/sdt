package learn

import (
	"testing"
	"time"
)

// TestScheduleLadder pins the review policy: every reason, the interval it earns
// and the two rules that make it more than a lookup — a hint never buys an
// independent success, and a success after a failure is downgraded one rung.
func TestScheduleLadder(t *testing.T) {
	cases := []struct {
		name       string
		grade      string
		hint       int
		itemType   string
		prior      string
		wantDays   int
		wantReason string
	}{
		{"incorrect fails", GradeIncorrect, 0, ItemTypeRecall, "", 1, ReasonFailure},
		{"incorrect after durable still fails", GradeIncorrect, 0, ItemTypeRecall, ReasonDurableSuccess, 1, ReasonFailure},
		{"partial is hesitant", GradePartiallyCorrect, 0, ItemTypeRecall, "", 3, ReasonCorrectButHesitant},
		{"correct with a hint is hesitant", GradeCorrect, 2, ItemTypeRecall, "", 3, ReasonCorrectButHesitant},
		{"a hint beats a transfer item", GradeCorrect, 1, ItemTypeTransfer, "", 3, ReasonCorrectButHesitant},
		{"independent success", GradeCorrect, 0, ItemTypeRecall, "", 7, ReasonIndependentSuccess},
		{"transfer success", GradeCorrect, 0, ItemTypeTransfer, "", 14, ReasonTransferSuccess},
		{"durable success repeats an independent one", GradeCorrect, 0, ItemTypeRecall, ReasonIndependentSuccess, 30, ReasonDurableSuccess},
		{"success after a failure is downgraded", GradeCorrect, 0, ItemTypeRecall, ReasonFailure, 3, ReasonCorrectButHesitant},
		{"a transfer after a failure loses one rung", GradeCorrect, 0, ItemTypeTransfer, ReasonFailure, 7, ReasonIndependentSuccess},
		{"ungradable schedules nothing", GradeUngradable, 0, ItemTypeRecall, "", 0, ""},
		{"an unknown grade schedules nothing", "mastered", 0, ItemTypeRecall, "", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			days, reason := Schedule(tc.grade, tc.hint, tc.itemType, tc.prior)
			if days != tc.wantDays || reason != tc.wantReason {
				t.Errorf("Schedule(%q, %d, %q, %q) = (%d, %q), want (%d, %q)",
					tc.grade, tc.hint, tc.itemType, tc.prior, days, reason, tc.wantDays, tc.wantReason)
			}
		})
	}
}

// TestScheduleVocabulary proves the closed vocabularies stay closed and that every
// reason has an interval.
func TestScheduleVocabulary(t *testing.T) {
	for _, g := range Grades() {
		if !KnownGrade(g) {
			t.Errorf("Grades() lists %q but KnownGrade refuses it", g)
		}
	}
	if KnownGrade("mastered") {
		t.Error("an invented grade must not be known")
	}
	for _, r := range ScheduleReasons() {
		if !KnownScheduleReason(r) {
			t.Errorf("ScheduleReasons() lists %q but KnownScheduleReason refuses it", r)
		}
		if ScheduleDays(r) <= 0 {
			t.Errorf("reason %q has no interval", r)
		}
	}
	if KnownScheduleReason("crammed") {
		t.Error("an invented reason must not be known")
	}
}

// TestScheduleDate pins the calendar arithmetic: the date is the day `days` after
// now, in UTC, and a failure comes back tomorrow.
func TestScheduleDate(t *testing.T) {
	now := time.Date(2026, 10, 30, 23, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	if got := ScheduleDate(now, 1); got != "2026-10-31" {
		t.Errorf("ScheduleDate(+1) = %q, want 2026-10-31", got)
	}
	if got := ScheduleDate(now, 0); got != "2026-10-30" {
		t.Errorf("ScheduleDate(0) = %q, want 2026-10-30", got)
	}
	if got := ScheduleDate(now, 30); got != "2026-11-29" {
		t.Errorf("ScheduleDate(+30) = %q, want 2026-11-29", got)
	}
}
