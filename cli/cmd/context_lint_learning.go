package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

const (
	ctxLearningObjectives = "objectives"
	ctxLearningDifficulty = "difficulty"
	ctxLearningMinutes    = "minutes"
)

// ── learning block (concept pages) ────────────────────────────────────────────
//
// A concept page may carry a small `learning:` block stating what a learner gets
// out of it. The block is deliberately tiny and the lesson document owns
// everything else: the objectives here, the example and the items there
// (instructions/lesson.md). Absence is the normal case — adoption is gradual and
// a page without objectives is not a defect — so only a present block is
// checked.

// ctxLearningDifficulties is the closed difficulty vocabulary, in order.
var ctxLearningDifficulties = []string{"introductory", "intermediate", "advanced"}

// ctxLearningVerbPrefixes is the advisory verb set: an objective should start
// with something a learner can be seen to do. It is a SUGGESTION, not a gate — a
// verb added tomorrow should be visible in a finding, not refused.
var ctxLearningVerbPrefixes = []string{
	"analyse", "analyze", "apply", "argue", "calculate", "choose", "classify",
	"compare", "compute", "construct", "critique", "decide", "define",
	"demonstrate", "derive", "design", "diagnose", "distinguish", "estimate",
	"evaluate", "explain", "identify", "implement", "interpret", "justify",
	useList, "map", "measure", "name", "predict", "prioritise", "prioritize",
	"propose", "recognise", "recognize", "relate", "select", "state", "trace",
}

const (
	ctxLearningMinObjectives = 2
	ctxLearningMaxObjectives = 4
	ctxLearningMinMinutes    = 5
	ctxLearningMaxMinutes    = 60
)

type learnMetadata struct {
	Objectives []string
	Difficulty string
	Minutes    int
}

func learningMetadataFields(frontmatter string) map[string]any {
	fm, _ := contextwiki.SplitFrontmatter(frontmatter)
	if fm == "" {
		return nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return nil
	}
	return doc
}

func learningMetadataForConcept(concept string) learnMetadata {
	data, err := os.ReadFile(learnLessonPath(concept)) //#nosec G304 -- path comes from a resolved concept id
	if err != nil {
		return learnMetadata{}
	}
	return learningMetadataFrom(string(data))
}

func learningMetadataFrom(content string) learnMetadata {
	fields := learningMetadataFields(content)
	meta := learnMetadata{Objectives: learningObjectives(fields[ctxLearningObjectives])}
	if difficulty, ok := fields[ctxLearningDifficulty].(string); ok {
		meta.Difficulty = difficulty
	}
	if n, ok := learningMinutes(fields[ctxLearningMinutes]); ok {
		meta.Minutes = n
	}
	return meta
}

// lintLearningBlock rejects the retired wiki field and validates lesson-owned
// objective metadata. No wiki fallback is supported while the project evolves.
func lintLearningBlock(path, content, kind string) []ctxLintIssue {
	fields := learningMetadataFields(content)
	if kind == ctxTypeWiki {
		if _, present := fields[ctxFrontmatterLearning]; present {
			return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: "wiki `learning:` is obsolete; move objectives, difficulty and minutes to the matching lesson"}}
		}
		return nil
	}
	if kind != ctxTypeLesson {
		return nil
	}
	var issues []ctxLintIssue
	issues = append(issues, lintLearningObjectives(path, fields[ctxLearningObjectives])...)
	issues = append(issues, lintLearningDifficulty(path, fields[ctxLearningDifficulty])...)
	return append(issues, lintLearningMinutes(path, fields[ctxLearningMinutes])...)
}

func learningMinutes(raw any) (int, bool) {
	switch n := raw.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		if n > uint64(^uint(0)>>1) {
			return 0, false
		}
		return int(n), true
	case float64:
		return int(n), n == float64(int(n))
	default:
		return 0, false
	}
}

// lintLearningObjectives checks the objectives list: its size, empties,
// duplicates, and the advisory verb opening.
func lintLearningObjectives(path string, raw any) []ctxLintIssue {
	var issues []ctxLintIssue
	warn := func(msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: msg})
	}
	objectives := learningObjectives(raw)
	if len(objectives) == 0 {
		warn(fmt.Sprintf("`objectives` must list %d-%d observable objectives", ctxLearningMinObjectives, ctxLearningMaxObjectives))
		return issues
	}
	if len(objectives) < ctxLearningMinObjectives || len(objectives) > ctxLearningMaxObjectives {
		warn(fmt.Sprintf("`objectives` has %d entr(ies), want %d-%d", len(objectives), ctxLearningMinObjectives, ctxLearningMaxObjectives))
	}
	seen := map[string]bool{}
	for _, o := range objectives {
		norm := strings.ToLower(strings.TrimSpace(o))
		if norm == "" {
			warn("`objectives` carries an empty entry")
			continue
		}
		if seen[norm] {
			warn(fmt.Sprintf("`objectives` repeats %q", o))
		}
		seen[norm] = true
		if first := strings.Fields(norm)[0]; !containsItem(ctxLearningVerbPrefixes, first) {
			issues = append(issues, ctxLintIssue{
				Path: path, Priority: ctxLintSuggestion,
				Message: fmt.Sprintf("objective %q does not begin with a verb a learner can be seen to do (%s…)", o, strings.Join(ctxLearningVerbPrefixes[:4], ", ")),
			})
		}
	}
	return issues
}

// lintLearningDifficulty checks the closed difficulty vocabulary.
func lintLearningDifficulty(path string, raw any) []ctxLintIssue {
	d, isString := raw.(string)
	if !isString || d == "" {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: "`difficulty` is missing (one of " + strings.Join(ctxLearningDifficulties, ", ") + ")"}}
	}
	if !containsItem(ctxLearningDifficulties, d) {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("`difficulty` %q is not one of %s", d, strings.Join(ctxLearningDifficulties, ", "))}}
	}
	return nil
}

// lintLearningMinutes checks the review-time bound.
func lintLearningMinutes(path string, raw any) []ctxLintIssue {
	warn := func(msg string) []ctxLintIssue {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: msg}}
	}
	outside := func(v any) []ctxLintIssue {
		return warn(fmt.Sprintf("`minutes` %v is outside %d-%d", v, ctxLearningMinMinutes, ctxLearningMaxMinutes))
	}
	switch m := raw.(type) {
	case nil:
		return warn("`minutes` is missing (the review time, in minutes)")
	case int:
		if m < ctxLearningMinMinutes || m > ctxLearningMaxMinutes {
			return outside(m)
		}
	case int64:
		if m < int64(ctxLearningMinMinutes) || m > int64(ctxLearningMaxMinutes) {
			return outside(m)
		}
	case uint64:
		if m < uint64(ctxLearningMinMinutes) || m > uint64(ctxLearningMaxMinutes) {
			return outside(m)
		}
	case float64:
		if m < ctxLearningMinMinutes || m > ctxLearningMaxMinutes {
			return outside(m)
		}
	default:
		return warn("`minutes` must be a number")
	}
	return nil
}

// learningObjectives reads the objectives list from the block, tolerating the
// YAML shapes a hand-written file may use (a sequence of strings).
func learningObjectives(raw any) []string {
	if list, ok := raw.([]string); ok {
		return append([]string(nil), list...)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			continue
		}
		out = append(out, s)
	}
	return out
}
