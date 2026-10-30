package learn

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Item types the lesson contract uses, weakest first. They live here so the
// scheduler and the grader name the same vocabulary the lesson table carries.
const (
	ItemTypeRecall   = "recall"
	ItemTypeDiagnose = "diagnose"
	ItemTypeApply    = "apply"
	ItemTypeTransfer = "transfer"
)

// Grading rules are the closed vocabulary an item may name. `auto` picks one from
// the item's shape; `manual` never grades.
const (
	RuleAuto     = "auto"
	RuleExact    = "exact"
	RuleNumeric  = "numeric"
	RuleKeywords = "keywords"
	RuleManual   = "manual"
)

// GradeThresholds are the keyword-coverage bands: at or above `correct` the answer
// counts as correct, at or above `partial` as partially correct, below as wrong.
// The bands are constants so the grader and its eval cases cannot drift.
const (
	KeywordCorrectThreshold = 0.60
	KeywordPartialThreshold = 0.30
)

// Answer is the graded verdict of one answer: the grade, the rule that produced
// it, the key it was checked against and — for keyword grading — the share of the
// key the answer covered.
type Answer struct {
	Grade    string  `json:"grade" yaml:"grade"`
	Rule     string  `json:"rule" yaml:"rule"`
	Expected string  `json:"expected" yaml:"expected"`
	Rubric   string  `json:"rubric,omitempty" yaml:"rubric,omitempty"`
	Coverage float64 `json:"coverage,omitempty" yaml:"coverage,omitempty"`
}

// GradingRules returns the closed grading vocabulary.
func GradingRules() []string {
	return []string{RuleAuto, RuleExact, RuleNumeric, RuleKeywords, RuleManual}
}

// KnownGradingRule reports whether a rule is in the vocabulary.
func KnownGradingRule(rule string) bool {
	for _, r := range GradingRules() {
		if r == rule {
			return true
		}
	}
	return false
}

// GradeAnswer grades one answer against an item's frozen key. It is deterministic:
// the same inputs always produce the same verdict, and a rule that cannot decide
// says `ungradable` rather than guessing.
func GradeAnswer(rule, expected, answer, itemType string) Answer {
	if strings.TrimSpace(rule) == "" {
		rule = RuleAuto
	}
	if rule == RuleAuto {
		rule = AutoRule(expected, answer, itemType)
	}
	out := Answer{Rule: rule, Expected: expected}
	switch rule {
	case RuleExact:
		out.Grade = gradeExact(expected, answer)
	case RuleNumeric:
		out.Grade = gradeNumeric(expected, answer)
	case RuleKeywords:
		out.Grade, out.Coverage = gradeKeywords(expected, answer)
	case RuleManual:
		out.Grade = GradeUngradable
	default:
		out.Grade = GradeUngradable
	}
	return out
}

// AutoRule picks the rule from the item's shape: a number on both sides is
// numeric; a recall or diagnose item with no number on either side is exact; every
// other item is graded by keyword coverage.
func AutoRule(expected, answer, itemType string) string {
	_, expectedNumber, _ := firstNumber(expected)
	_, answerNumber, _ := firstNumber(answer)
	if expectedNumber && answerNumber {
		return RuleNumeric
	}
	switch itemType {
	case ItemTypeRecall, ItemTypeDiagnose:
		return RuleExact
	}
	return RuleKeywords
}

// Normalize folds the differences a grader must ignore: case, surrounding and
// repeated whitespace, and trailing punctuation.
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimRight(s, ".,;:!?")
}

// gradeExact compares the normalised answer with the normalised key, or with any
// `;`-separated accepted alternative the key lists.
func gradeExact(expected, answer string) string {
	got := Normalize(answer)
	if got == "" {
		return GradeIncorrect
	}
	for _, alternative := range strings.Split(expected, ";") {
		if alt := Normalize(alternative); alt != "" && alt == got {
			return GradeCorrect
		}
	}
	return GradeIncorrect
}

// numberPattern finds the first number in a string, decimal comma accepted.
var numberPattern = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// firstNumber returns the first number in a string, whether one was found and
// whether the string expresses it as a percentage.
func firstNumber(s string) (float64, bool, bool) {
	m := numberPattern.FindString(s)
	if m == "" {
		return 0, false, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	if err != nil {
		return 0, false, false
	}
	return v, true, strings.Contains(s, "%")
}

// gradeNumeric compares the first number of the answer with the key's number. Two
// percentages are compared in percentage points (±0.1); otherwise the tolerance is
// 1% of the key.
func gradeNumeric(expected, answer string) string {
	want, wantOK, wantPercent := keyNumber(expected)
	if !wantOK {
		return GradeUngradable
	}
	got, gotOK, gotPercent := firstNumber(answer)
	if !gotOK {
		return GradeIncorrect
	}
	tolerance := math.Abs(want) * 0.01
	if wantPercent || gotPercent {
		tolerance = 0.1
	}
	if tolerance == 0 {
		tolerance = 0.01
	}
	if math.Abs(got-want) <= tolerance {
		return GradeCorrect
	}
	return GradeIncorrect
}

// keyNumber is the number an expected answer states as its result. A key that
// spells out a derivation (`2/36 = 5.56%`) states the result after the last `=`;
// without one the first number is the result.
func keyNumber(expected string) (float64, bool, bool) {
	if i := strings.LastIndex(expected, "="); i >= 0 {
		if v, ok, percent := firstNumber(expected[i+1:]); ok {
			return v, true, percent || strings.Contains(expected, "%")
		}
	}
	v, ok, percent := firstNumber(expected)
	return v, ok, percent
}

// gradeKeywords compares the share of the key's significant words the answer
// carries. A key with no significant word cannot be graded this way.
func gradeKeywords(expected, answer string) (string, float64) {
	words := keywordsOf(expected)
	if len(words) == 0 {
		return GradeUngradable, 0
	}
	tokens := strings.Fields(Normalize(answer))
	found := 0
	for _, word := range words {
		if tokenMatches(tokens, word) {
			found++
		}
	}
	coverage := float64(found) / float64(len(words))
	switch {
	case coverage >= KeywordCorrectThreshold:
		return GradeCorrect, coverage
	case coverage >= KeywordPartialThreshold:
		return GradePartiallyCorrect, coverage
	default:
		return GradeIncorrect, coverage
	}
}

// tokenMatches reports whether any answer token carries a key word. Prefixes match
// in both directions, so a plural and its singular are the same word.
func tokenMatches(tokens []string, word string) bool {
	for _, token := range tokens {
		if token == word || strings.HasPrefix(token, word) || strings.HasPrefix(word, token) {
			return true
		}
	}
	return false
}

// keywordsOf returns the distinct significant words of a key: longer than three
// runes, lower-cased, punctuation dropped.
func keywordsOf(expected string) []string {
	fields := strings.FieldsFunc(strings.ToLower(expected), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len([]rune(f)) <= 3 || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
