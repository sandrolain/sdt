package learn

import "time"

// ScheduleRung is one rung of the review ladder: a reason and the interval it
// earns. The ladder is fixed on purpose — wave 1 has no learned scheduler, so the
// same grade always yields the same date and the reason is stored beside it.
type ScheduleRung struct {
	Reason string `json:"reason" yaml:"reason"`
	Days   int    `json:"days" yaml:"days"`
}

// scheduleRungs is the ladder, shortest interval first. `failure` and
// `new_or_fragile` both come back tomorrow: a failed answer and a concept with no
// successful answer behind it are equally fragile.
var scheduleRungs = []ScheduleRung{
	{Reason: ReasonFailure, Days: 1},
	{Reason: ReasonNewOrFragile, Days: 1},
	{Reason: ReasonCorrectButHesitant, Days: 3},
	{Reason: ReasonIndependentSuccess, Days: 7},
	{Reason: ReasonTransferSuccess, Days: 14},
	{Reason: ReasonDurableSuccess, Days: 30},
}

// Grades returns the closed grade vocabulary, weakest claim last.
func Grades() []string {
	return []string{GradeCorrect, GradePartiallyCorrect, GradeIncorrect, GradeUngradable}
}

// KnownGrade reports whether a grade is in the vocabulary.
func KnownGrade(grade string) bool {
	for _, g := range Grades() {
		if g == grade {
			return true
		}
	}
	return false
}

// ScheduleReasons returns the closed reason vocabulary, shortest interval first.
func ScheduleReasons() []string {
	out := make([]string, 0, len(scheduleRungs))
	for _, r := range scheduleRungs {
		out = append(out, r.Reason)
	}
	return out
}

// KnownScheduleReason reports whether a reason is in the vocabulary.
func KnownScheduleReason(reason string) bool {
	for _, r := range scheduleRungs {
		if r.Reason == reason {
			return true
		}
	}
	return false
}

// ScheduleDays returns the interval a reason earns, or 0 for an unknown reason.
func ScheduleDays(reason string) int {
	for _, r := range scheduleRungs {
		if r.Reason == reason {
			return r.Days
		}
	}
	return 0
}

// Schedule is the review policy: from a grade, the hint it needed, the item type
// and the reason already stored, it returns the interval in days and the reason
// that explains it.
//
// The rules, in order:
//
//   - `incorrect` fails: one day, reason `failure`.
//   - `partially_correct` is hesitant: three days.
//   - `correct` with a hint is hesitant whatever the item type: the learner got
//     there with help, so the answer is not independent evidence.
//   - `correct` with no hint on a `transfer` item earns `transfer_success`.
//   - `correct` with no hint after an earlier success earns `durable_success`:
//     the success repeated, which is the only way mastery is reached.
//   - any other correct answer is `independent_success`.
//   - a success that follows a `failure` is downgraded one rung: re-earning a
//     concept takes longer than keeping it.
//
// An ungradable or unknown grade schedules nothing (0, "").
func Schedule(grade string, hint int, itemType, priorReason string) (days int, reason string) {
	switch grade {
	case GradeIncorrect:
		return ScheduleDays(ReasonFailure), ReasonFailure
	case GradePartiallyCorrect:
		return ScheduleDays(ReasonCorrectButHesitant), ReasonCorrectButHesitant
	case GradeCorrect:
	default:
		return 0, ""
	}
	var earned string
	switch {
	case hint > 0:
		earned = ReasonCorrectButHesitant
	case itemType == ItemTypeTransfer:
		earned = ReasonTransferSuccess
	case isSuccessReason(priorReason):
		earned = ReasonDurableSuccess
	default:
		earned = ReasonIndependentSuccess
	}
	if priorReason == ReasonFailure {
		earned = downgradeReason(earned)
	}
	return ScheduleDays(earned), earned
}

// isSuccessReason reports whether a reason records a correct answer.
func isSuccessReason(reason string) bool {
	switch reason {
	case ReasonIndependentSuccess, ReasonTransferSuccess, ReasonDurableSuccess:
		return true
	}
	return false
}

// downgradeReason drops one rung down the ladder, floored at `new_or_fragile`.
func downgradeReason(reason string) string {
	for i, r := range scheduleRungs {
		if r.Reason != reason || i == 0 {
			continue
		}
		next := scheduleRungs[i-1].Reason
		if next == ReasonFailure {
			next = ReasonNewOrFragile
		}
		return next
	}
	return ReasonNewOrFragile
}

// ScheduleDate is the calendar date `days` after now, in UTC.
func ScheduleDate(now time.Time, days int) string {
	return now.UTC().AddDate(0, 0, days).Format(DateLayout)
}
