package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// ── fixtures for the two levels ───────────────────────────────────────────────

// writeCourseConcept writes the three documents a teachable concept needs in a
// temp corpus: its page, its lesson and its item bank. The page carries the four
// claims the readiness rule asks for, so a level over it has something ready to
// count.
func writeCourseConcept(t *testing.T, id string, dependsOn ...string) {
	t.Helper()
	fm := "---\nkind: wiki\nid: " + id + "\nstatus: active\n" +
		"learning:\n  objectives:\n    - \"name the parts\"\n    - \"apply the rule\"\n" +
		"  difficulty: intermediate\n  minutes: 5\n"
	if len(dependsOn) > 0 {
		fm += "relations:\n  depends_on:\n"
		for _, dep := range dependsOn {
			fm += "    - \"[[" + dep + "]]\"\n"
		}
	}
	body := "## Claims\n\n1. {#claim-1} <!-- c-aaaaaaaa --> a\n2. {#claim-2} <!-- c-bbbbbbbb --> b\n" +
		"3. {#claim-3} <!-- c-cccccccc --> c\n4. {#claim-4} <!-- c-dddddddd --> d\n"
	writeLearningDoc(t, filepath.Join(sdtWikiDir, filepath.FromSlash(id)+sdtMarkdownExt), fm+"---\n\n"+body)

	lesson := "---\nkind: lesson\nid: " + id + "\nconcept: " + id +
		"\nsummary: s\nstatus: active\ncreated: 2026-11-01T00:00:00Z\nupdated: 2026-11-01T00:00:00Z\n---\n\n" +
		"## Prerequisites\n\n## Worked example\n\n`source_span`: refs/x.md@5772ca09:1-2\n\n## Transfer\n\nA new case.\n"
	writeLearningDoc(t, learnLessonPath(id), lesson)

	items := "| id | objective | type | prompt | expected | rubric | distractor | claim | hash |\n" +
		"|---|---|---|---|---|---|---|---|---|\n" +
		"| " + learnCourseLeaf(id) + "-1 | 1 | recall | name them | the parts | all | one | " + id + "#c-aaaaaaaa | x |\n" +
		"| " + learnCourseLeaf(id) + "-2 | 2 | transfer | apply | the answer | matches | wrong | " + id + "#c-bbbbbbbb | x |\n"
	writeAssessmentFor(t, id, items)
}

// learnCourseLeaf is the last segment of an id, for readable item ids.
func learnCourseLeaf(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// writeCourseLevel writes a level document with its membership in file order.
func writeCourseLevel(t *testing.T, kind, id string, members []string) string {
	t.Helper()
	section := "## Lessons"
	if kind == learnCourseSyllabus {
		section = "## Themes"
	}
	var b strings.Builder
	b.WriteString("---\nkind: " + kind + "\nid: " + id + "\nsummary: s\nstatus: active\n")
	if kind == learnCourseSyllabus {
		b.WriteString("audience: architects\nfrom_level: intermediate\nto_level: advanced\noutcome: decide boundaries\n")
	}
	b.WriteString("created: 2026-11-01T00:00:00Z\nupdated: 2026-11-01T00:00:00Z\n---\n\n")
	b.WriteString("## Thread\n\nwhy these belong together\n\n" + section + "\n\n")
	for _, m := range members {
		b.WriteString("- " + m + "\n")
	}
	return writeLearningDoc(t, learnCoursePath(kind, id), b.String())
}

// ── the verb ──────────────────────────────────────────────────────────────────

// TestLearnCourseReadsAThemeInOrder is the green case: two teachable lessons, one
// theme, membership in file order and a readiness rollup that is two facts.
func TestLearnCourseReadsAThemeInOrder(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")
	writeCourseConcept(t, "architecture/b")
	writeCourseLevel(t, learnCourseTheme, "themes/metrics", []string{"architecture/a", "architecture/b"})

	out := string(execute(t, learnCmd, nil, "course", "themes/metrics"))
	for _, want := range []string{"themes/metrics", "architecture/a", "architecture/b", "2/2"} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "refused") || strings.Contains(out, "conflicts") {
		t.Errorf("a clean theme reported a finding:\n%s", out)
	}
}

// TestLearnCourseReportsAnOrderConflict is the rule that makes the order
// checkable: a lesson that needs another first is reported when it comes before
// it, and nothing is reordered behind the author's back.
func TestLearnCourseReportsAnOrderConflict(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a", "architecture/b")
	writeCourseConcept(t, "architecture/b")
	writeCourseLevel(t, learnCourseTheme, "themes/metrics", []string{"architecture/a", "architecture/b"})

	out := string(execute(t, learnCmd, nil, "course", "themes/metrics"))
	if !strings.Contains(out, "order conflicts") {
		t.Fatalf("a reversed prerequisite went unreported:\n%s", out)
	}
	if !strings.Contains(out, "architecture/b is at position 2") {
		t.Errorf("the conflict does not name both positions:\n%s", out)
	}

	// The same membership in the right order reports nothing.
	writeCourseLevel(t, learnCourseTheme, "themes/metrics", []string{"architecture/b", "architecture/a"})
	if out := string(execute(t, learnCmd, nil, "course", "themes/metrics")); strings.Contains(out, "order conflicts") {
		t.Errorf("an ordered theme reported a conflict:\n%s", out)
	}
}

// TestLearnCourseRefusesBrokenMembership covers the four refusals the contract
// names: dangling, duplicate and wrong-kind (a level listed as a lesson), and an
// archived member. Each is a non-zero exit, because a membership that cannot
// teach is not a warning.
func TestLearnCourseRefusesBrokenMembership(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")
	writeCourseLevel(t, learnCourseTheme, "themes/other", []string{"architecture/a"})

	cases := map[string][]string{
		"dangling":   {"architecture/a", "architecture/missing"},
		"duplicate":  {"architecture/a", "architecture/a"},
		"wrong kind": {"architecture/a", "themes/other"},
	}
	for name, members := range cases {
		t.Run(name, func(t *testing.T) {
			writeCourseLevel(t, learnCourseTheme, "themes/metrics", members)
			var out string
			shouldExitWithCode(t, 1, func() string {
				out = string(execute(t, learnCmd, nil, "course", "themes/metrics"))
				return ""
			})
			if !strings.Contains(out, "refused") {
				t.Errorf("%s membership reported no refusal:\n%s", name, out)
			}
		})
	}

	t.Run("archived", func(t *testing.T) {
		writeCourseConcept(t, "architecture/gone")
		path := learnLessonPath("architecture/gone")
		doc := mustReadFile(t, path)
		writeLearningDoc(t, path, strings.Replace(doc, "status: active", "status: archived", 1))
		writeCourseLevel(t, learnCourseTheme, "themes/metrics", []string{"architecture/gone"})
		var out string
		shouldExitWithCode(t, 1, func() string {
			out = string(execute(t, learnCmd, nil, "course", "themes/metrics"))
			return ""
		})
		if !strings.Contains(out, "is archived") {
			t.Errorf("an archived lesson is not named as archived:\n%s", out)
		}
	})
}

// TestLearnCourseSyllabusExpandsThemes checks the level above: a syllabus lists
// themes, its flattened order reaches the lessons, and a dependency that crosses
// two themes is caught there as well.
func TestLearnCourseSyllabusExpandsThemes(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a", "architecture/b")
	writeCourseConcept(t, "architecture/b")
	writeCourseConcept(t, "architecture/c")
	writeCourseLevel(t, learnCourseTheme, "themes/first", []string{"architecture/a"})
	writeCourseLevel(t, learnCourseTheme, "themes/second", []string{"architecture/b"})
	writeCourseLevel(t, learnCourseSyllabus, "syllabi/course", []string{"themes/first", "themes/second"})

	out := string(execute(t, learnCmd, nil, "course", "syllabi/course"))
	if !strings.Contains(out, "themes/first") || !strings.Contains(out, "themes/second") {
		t.Fatalf("the syllabus did not resolve its themes:\n%s", out)
	}
	if !strings.Contains(out, "2/2") {
		t.Errorf("the rollup did not reach the lessons behind the themes:\n%s", out)
	}
	if !strings.Contains(out, "order conflicts") {
		t.Errorf("a dependency across two themes went unreported:\n%s", out)
	}
}

// TestLearnCourseValidateWalksBothLevels pins the whole-set form: one finding is
// enough to exit 1, and a missing level directory is not an error.
func TestLearnCourseValidateWalksBothLevels(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")
	writeCourseLevel(t, learnCourseTheme, "themes/good", []string{"architecture/a"})
	writeCourseLevel(t, learnCourseTheme, "themes/bad", []string{"architecture/missing"})

	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, learnCmd, nil, "course", "--validate"))
		return ""
	})
	if !strings.Contains(out, "course levels: 2") {
		t.Errorf("validate did not read both levels:\n%s", out)
	}
}

// TestLearnCourseCoverageIsTwoFacts pins the shape the contract asks for: one
// row per objective naming the item and the claim, then readiness and coverage as
// two facts. A percentage here would be the composite `learning.md` forbids, so
// the absence of a percent sign is part of the assertion.
func TestLearnCourseCoverageIsTwoFacts(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")
	writeCourseLevel(t, learnCourseTheme, "themes/metrics", []string{"architecture/a"})

	out := string(execute(t, learnCmd, nil, "course", "themes/metrics", "--coverage"))
	if strings.Contains(out, "%") {
		t.Errorf("the coverage report carries a percentage:\n%s", out)
	}
	for _, want := range []string{"objectives covered 2/2", "lessons ready 1/1", "item a-1", "architecture/a#c-aaaaaaaa"} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage report does not carry %q:\n%s", want, out)
		}
	}
}

// TestLearnCourseCoverageNamesAnUncoveredObjective is the red case: an objective
// no item answers for is printed as uncovered and the command exits 1, because a
// course that lists what it cannot assess is the defect the matrix exists for.
func TestLearnCourseCoverageNamesAnUncoveredObjective(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")
	page := filepath.Join(sdtWikiDir, "architecture", "a.md")
	doc := mustReadFile(t, page)
	doc = strings.Replace(doc, "    - \"apply the rule\"", "    - \"apply the rule\"\n    - \"judge a case\"", 1)
	writeLearningDoc(t, page, doc)
	writeCourseLevel(t, learnCourseTheme, "themes/metrics", []string{"architecture/a"})

	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, learnCmd, nil, "course", "themes/metrics", "--coverage"))
		return ""
	})
	if !strings.Contains(out, "uncovered") {
		t.Errorf("the third objective was not reported as uncovered:\n%s", out)
	}
	if !strings.Contains(out, "objectives covered 2/3") {
		t.Errorf("the coverage fact is wrong:\n%s", out)
	}
}

// TestLearnCourseCoverageOfAnEmptyLevel is the degenerate case: a level that
// lists nothing is not a clean course, it is an unfinished one.
func TestLearnCourseCoverageOfAnEmptyLevel(t *testing.T) {
	runInTempDir(t)
	writeCourseLevel(t, learnCourseSyllabus, "syllabi/empty", nil)
	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, learnCmd, nil, "course", "syllabi/empty", "--coverage"))
		return ""
	})
	if !strings.Contains(out, "objectives covered 0/0") {
		t.Errorf("the empty level reported something else:\n%s", out)
	}
}

// TestLearnCourseIntakeIsRefusedWhenItCannotAddress pins the four intake rules:
// a syllabus without an audience, without an outcome, with an unknown level, or
// with a level pair that runs backwards is refused rather than taught.
func TestLearnCourseIntakeIsRefusedWhenItCannotAddress(t *testing.T) {
	cases := map[string]string{
		"no audience":    "from_level: intermediate\nto_level: advanced\noutcome: decide boundaries\n",
		"no outcome":     "audience: architects\nfrom_level: intermediate\nto_level: advanced\n",
		"unknown level":  "audience: architects\nfrom_level: senior\nto_level: advanced\noutcome: decide boundaries\n",
		"backwards pair": "audience: architects\nfrom_level: advanced\nto_level: introductory\noutcome: decide boundaries\n",
	}
	for name, intake := range cases {
		t.Run(name, func(t *testing.T) {
			runInTempDir(t)
			writeCourseConcept(t, "architecture/a")
			writeCourseLevel(t, learnCourseTheme, "themes/t", []string{"architecture/a"})
			doc := "---\nkind: syllabus\nid: syllabi/s\nsummary: s\nstatus: active\n" + intake +
				"created: 2026-11-01T00:00:00Z\nupdated: 2026-11-01T00:00:00Z\n---\n\n## Thread\n\nwhy\n\n## Themes\n\n- themes/t\n"
			writeLearningDoc(t, learnCoursePath(learnCourseSyllabus, "syllabi/s"), doc)
			shouldExitWithCode(t, 1, func() string {
				execute(t, learnCmd, nil, "course", "syllabi/s")
				return ""
			})
		})
	}
}

// TestLearnCourseIntakeIsPrinted is the readable half: a valid syllabus states its
// audience, level pair and outcome before it lists a theme.
func TestLearnCourseIntakeIsPrinted(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")
	writeCourseLevel(t, learnCourseTheme, "themes/t", []string{"architecture/a"})
	writeCourseLevel(t, learnCourseSyllabus, "syllabi/s", []string{"themes/t"})

	out := string(execute(t, learnCmd, nil, "course", "syllabi/s"))
	for _, want := range []string{"audience: architects", "intermediate → advanced", "outcome: decide boundaries"} {
		if !strings.Contains(out, want) {
			t.Errorf("the intake block does not carry %q:\n%s", want, out)
		}
	}
}
