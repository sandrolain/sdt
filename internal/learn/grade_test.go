package learn

import "testing"

// TestNormalizeFoldsWhatAGraderMustIgnore pins the normalisation: case, repeated
// whitespace and trailing punctuation are not differences in an answer.
func TestNormalizeFoldsWhatAGraderMustIgnore(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"  Deployment   Frequency. ", "deployment frequency"},
		{"two failures!", "two failures"},
		{"A\tB\nC", "a b c"},
		{"", ""},
	} {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestGradeExact covers the exact rule: normalised equality, accepted
// alternatives, and an empty answer as wrong rather than ungradable.
func TestGradeExact(t *testing.T) {
	cases := []struct{ name, expected, answer, want string }{
		{"equal", "deployment frequency", "Deployment Frequency.", GradeCorrect},
		{"alternative", "deployment frequency; lead time", "lead time", GradeCorrect},
		{"no alternative", "deployment frequency; lead time", "change failure rate", GradeIncorrect},
		{"empty answer", "deployment frequency", "   ", GradeIncorrect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeAnswer(RuleExact, tc.expected, tc.answer, ItemTypeRecall).Grade; got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGradeNumeric covers the numeric rule: percentage points, relative
// tolerance, the result after a derivation, and a key with no number.
func TestGradeNumeric(t *testing.T) {
	cases := []struct{ name, expected, answer, want string }{
		{"percentage points", "2/36 = 5.56% (±0.1 percentage points)", "5.6%", GradeCorrect},
		{"percentage too far", "5.56%", "5.8%", GradeIncorrect},
		{"relative tolerance", "36", "36.3", GradeCorrect},
		{"relative too far", "36", "40", GradeIncorrect},
		{"comma decimal", "5,56%", "5.6%", GradeCorrect},
		{"no number in the answer", "36", "many", GradeIncorrect},
		{"no number in the key", "several", "3", GradeUngradable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeAnswer(RuleNumeric, tc.expected, tc.answer, ItemTypeApply).Grade; got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGradeKeywords covers the keyword bands: coverage at or above 60% is correct,
// 30–60% partial, below 30% wrong, and a key with no significant word is
// ungradable.
func TestGradeKeywords(t *testing.T) {
	const expected = "names the window and the zero days"
	cases := []struct{ name, answer, want string }{
		{"full", "names the window and the zero days", GradeCorrect},
		{"half", "names the window", GradePartiallyCorrect},
		{"one word", "window", GradeIncorrect},
		{"nothing", "no idea", GradeIncorrect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := GradeAnswer(RuleKeywords, expected, tc.answer, ItemTypeDiagnose)
			if answer.Grade != tc.want {
				t.Errorf("grade = %q, want %q (coverage %.2f)", answer.Grade, tc.want, answer.Coverage)
			}
			if answer.Rule != RuleKeywords {
				t.Errorf("rule = %q, want %q", answer.Rule, RuleKeywords)
			}
		})
	}
	if got := GradeAnswer(RuleKeywords, "a b", "a b", ItemTypeDiagnose).Grade; got != GradeUngradable {
		t.Errorf("a key with no significant word must be ungradable, got %q", got)
	}
}

// TestGradeManualIsUngradable proves the one rule that never decides.
func TestGradeManualIsUngradable(t *testing.T) {
	if got := GradeAnswer(RuleManual, "any", "any", ItemTypeTransfer).Grade; got != GradeUngradable {
		t.Errorf("manual grading = %q, want %q", got, GradeUngradable)
	}
}

// TestAutoRulePicksFromTheItem pins the automatic choice: a number on both sides is
// numeric, a recall or diagnose item with no number is exact, everything else is
// graded by coverage.
func TestAutoRulePicksFromTheItem(t *testing.T) {
	cases := []struct{ name, expected, answer, itemType, want string }{
		{"numbers", "5.56%", "5.6%", ItemTypeApply, RuleNumeric},
		{"recall prose", "deployment frequency", "deployment frequency", ItemTypeRecall, RuleExact},
		{"diagnose prose", "names the window", "the window", ItemTypeDiagnose, RuleExact},
		{"apply prose", "names the window and the zero days", "names the window and the zero days", ItemTypeApply, RuleKeywords},
		{"transfer prose", "names ownership", "ownership", ItemTypeTransfer, RuleKeywords},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AutoRule(tc.expected, tc.answer, tc.itemType); got != tc.want {
				t.Errorf("AutoRule = %q, want %q", got, tc.want)
			}
			if got := GradeAnswer(RuleAuto, tc.expected, tc.answer, tc.itemType).Rule; got != tc.want {
				t.Errorf("GradeAnswer(auto).Rule = %q, want %q", got, tc.want)
			}
		})
	}
	if got := GradeAnswer("invented", "x", "x", ItemTypeRecall).Grade; got != GradeUngradable {
		t.Errorf("an unknown rule must be ungradable, got %q", got)
	}
}

// TestGradingRuleVocabulary proves the rule list and the predicate agree.
func TestGradingRuleVocabulary(t *testing.T) {
	for _, r := range GradingRules() {
		if !KnownGradingRule(r) {
			t.Errorf("GradingRules() lists %q but KnownGradingRule refuses it", r)
		}
	}
	if KnownGradingRule("fuzzy") {
		t.Error("an invented rule must not be known")
	}
}
