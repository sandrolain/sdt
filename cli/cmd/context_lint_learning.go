package cmd

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/sandrolain/sdt/internal/contextwiki"
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
	"list", "map", "measure", "name", "predict", "prioritise", "prioritize",
	"propose", "recognise", "recognize", "relate", "select", "state", "trace",
}

const (
	ctxLearningMinObjectives = 2
	ctxLearningMaxObjectives = 4
	ctxLearningMinMinutes    = 5
	ctxLearningMaxMinutes    = 60
)

// learningBlockOf parses the optional `learning:` block out of a document's
// frontmatter (with its delimiters), returning the block when it is present and a
// map. A missing block and a malformed document both return false.
func learningBlockOf(frontmatter string) (map[string]any, bool) {
	fm, _ := contextwiki.SplitFrontmatter(frontmatter)
	if fm == "" {
		return nil, false
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return nil, false
	}
	raw, ok := doc[ctxFrontmatterLearning]
	if !ok {
		return nil, false
	}
	block, ok := raw.(map[string]any)
	return block, ok
}

// learningBlockPresent returns the raw `learning:` value when the frontmatter
// carries the key at all, so a malformed block is distinguishable from an absent
// one.
func learningBlockPresent(frontmatter string) (any, bool) {
	fm, _ := contextwiki.SplitFrontmatter(frontmatter)
	if fm == "" {
		return nil, false
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return nil, false
	}
	raw, ok := doc[ctxFrontmatterLearning]
	return raw, ok
}

// learningObjectivesOf returns a page's objectives, or nil when it carries none.
func learningObjectivesOf(frontmatter string) []string {
	block, ok := learningBlockOf(frontmatter)
	if !ok {
		return nil
	}
	return learningObjectives(block["objectives"])
}

// lintLearningBlock validates the optional `learning:` block on a wiki page: the
// objective count and shape, the difficulty vocabulary and the time bound.
func lintLearningBlock(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeWiki {
		return nil
	}
	block, ok := learningBlockOf(content)
	if !ok {
		if _, present := learningBlockPresent(content); present {
			return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: "`learning:` must be a map with `objectives`, `difficulty` and `minutes`"}}
		}
		return nil
	}
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}
	for key := range block {
		switch key {
		case "objectives", "difficulty", "minutes":
		default:
			add(ctxLintWarning, fmt.Sprintf("`learning.%s` is not part of the block (only objectives, difficulty, minutes)", key))
		}
	}

	issues = append(issues, lintLearningObjectives(path, block["objectives"])...)
	issues = append(issues, lintLearningDifficulty(path, block["difficulty"])...)
	return append(issues, lintLearningMinutes(path, block["minutes"])...)
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
		warn(fmt.Sprintf("`learning.objectives` must list %d-%d observable objectives", ctxLearningMinObjectives, ctxLearningMaxObjectives))
		return issues
	}
	if len(objectives) < ctxLearningMinObjectives || len(objectives) > ctxLearningMaxObjectives {
		warn(fmt.Sprintf("`learning.objectives` has %d entr(ies), want %d-%d", len(objectives), ctxLearningMinObjectives, ctxLearningMaxObjectives))
	}
	seen := map[string]bool{}
	for _, o := range objectives {
		norm := strings.ToLower(strings.TrimSpace(o))
		if norm == "" {
			warn("`learning.objectives` carries an empty entry")
			continue
		}
		if seen[norm] {
			warn(fmt.Sprintf("`learning.objectives` repeats %q", o))
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
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: "`learning.difficulty` is missing (one of " + strings.Join(ctxLearningDifficulties, ", ") + ")"}}
	}
	if !containsItem(ctxLearningDifficulties, d) {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("`learning.difficulty` %q is not one of %s", d, strings.Join(ctxLearningDifficulties, ", "))}}
	}
	return nil
}

// lintLearningMinutes checks the review-time bound.
func lintLearningMinutes(path string, raw any) []ctxLintIssue {
	warn := func(msg string) []ctxLintIssue {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: msg}}
	}
	outside := func(v any) []ctxLintIssue {
		return warn(fmt.Sprintf("`learning.minutes` %v is outside %d-%d", v, ctxLearningMinMinutes, ctxLearningMaxMinutes))
	}
	switch m := raw.(type) {
	case nil:
		return warn("`learning.minutes` is missing (the review time, in minutes)")
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
		return warn("`learning.minutes` must be a number")
	}
	return nil
}

// learningObjectives reads the objectives list from the block, tolerating the
// YAML shapes a hand-written file may use (a sequence of strings).
func learningObjectives(raw any) []string {
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
