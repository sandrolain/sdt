package cmd

import (
	"fmt"
	"strings"
)

// ── course-level body checks ──────────────────────────────────────────────────
//
// A theme and a syllabus are documents that reference what exists: the theme
// lists lessons, the syllabus lists themes, and neither carries a claim, an
// objective or an item (`course.md`). Their shape is mechanical — one prose
// section and one membership section — so it is checked here. The membership is
// resolved through the same function `sdt learn course` uses, so a member the
// verb refuses is never a lint pass.

// ctxCourseSections is the ordered section list each level carries, and no other.
func ctxCourseSections(kind string) []string {
	return []string{learnCourseThreadSection, learnCourseMembershipSection(kind)}
}

// learnCourseThreadSection is the prose section a level owns: why these concepts
// belong together (a theme) or for whom and in what order they run (a syllabus).
const learnCourseThreadSection = "Thread"

// learnCourseMembershipSection is the section a level's membership lives under.
func learnCourseMembershipSection(kind string) string {
	if kind == learnCourseSyllabus {
		return learnCourseThemesSection
	}
	return learnCourseLessonsSection
}

// lintCourse validates one level's shape and its membership.
func lintCourse(path, content, kind string) []ctxLintIssue {
	if kind != learnCourseTheme && kind != learnCourseSyllabus {
		return nil
	}
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}

	body := documentBody(content)
	headings, _ := markdownSections(body)
	issues = append(issues, lintSections(path, headings, ctxCourseSections(kind), kind)...)

	members := learnCourseMembership(content, kind)
	if len(members) == 0 {
		add(ctxLintWarning, fmt.Sprintf("`## %s` lists no member: a level that groups nothing teaches nothing",
			learnCourseMembershipSection(kind)))
	}
	seen := map[string]bool{}
	for i, member := range members {
		if seen[member] {
			add(ctxLintWarning, fmt.Sprintf("member %q appears twice; one concept occupies one position", member))
			continue
		}
		seen[member] = true
		if _, reason := learnCourseMemberFile(kind, member); reason != "" {
			add(ctxLintWarning, fmt.Sprintf("member %d %q %s", i+1, member, reason))
		}
	}
	issues = append(issues, lintCourseRestatement(path, content, kind)...)
	issues = append(issues, lintCourseIntake(path, content, kind)...)
	return issues
}

// lintCourseRestatement guards the rule that a level references and never
// restates: a level that carries a claim table or an item table is copying what
// belongs to the page and the lesson.
func lintCourseRestatement(path, content, kind string) []ctxLintIssue {
	body := documentBody(content)
	if strings.Contains(body, "{#claim-") || strings.Contains(body, "<!-- c-") {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning,
			Message: "carries a claim anchor: a " + kind + " references concepts, never their claims"}}
	}
	if strings.Contains(body, "| expected |") && strings.Contains(body, "| rubric |") {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning,
			Message: "carries an item table: the items belong to the lesson's assessment, not to a level"}}
	}
	return nil
}

// lintCourseIntake binds the intake to its level: only a syllabus states one, and
// a syllabus that states none cannot be addressed or checked.
func lintCourseIntake(path, content, kind string) []ctxLintIssue {
	intake := learnCourseIntakeOf(content)
	if kind == learnCourseTheme {
		if !intake.empty() {
			return []ctxLintIssue{{Path: path, Priority: ctxLintWarning,
				Message: "a theme carries the syllabus intake keys: audience, levels and outcome belong to the course, not to a group of lessons"}}
		}
		return nil
	}
	var issues []ctxLintIssue
	for _, issue := range learnCourseIntakeIssues(intake) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: issue})
	}
	return issues
}
