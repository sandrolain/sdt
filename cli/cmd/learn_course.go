package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── sdt learn course (the two levels above the lesson) ────────────────────────
//
// A course level is a corpus document: a theme lists the lessons whose concepts
// belong together, a syllabus lists the themes in the order they run. Neither
// owns a claim, an objective or an item — the page owns the knowledge and the
// lesson owns the item bank, so a level can only reference what exists.
//
// This verb reads one level, resolves every member in file order, refuses what
// cannot teach, and reports an order that contradicts a member's prerequisites.
// It never repairs and never reorders: the document is the author's, the report
// is the reader's.

// Sections that carry membership, per level. Position is order.
const (
	learnCourseLessonsSection = "Lessons"
	learnCourseThemesSection  = "Themes"
)

// The two level kinds, named once.
const (
	learnCourseTheme    = ctxTypeTheme
	learnCourseSyllabus = ctxTypeSyllabus
)

// learnCourseMember is one member of a level, resolved.
type learnCourseMember struct {
	Order   int      `json:"order" yaml:"order"`
	ID      string   `json:"id" yaml:"id"`
	Title   string   `json:"title,omitempty" yaml:"title,omitempty"`
	Status  string   `json:"status,omitempty" yaml:"status,omitempty"`
	Verdict string   `json:"verdict,omitempty" yaml:"verdict,omitempty"`
	Reasons []string `json:"reasons,omitempty" yaml:"reasons,omitempty"`
	// Members is a syllabus's expansion of the theme it names: the lessons the
	// theme carries, in the theme's own order. It is what makes the flattened
	// order checkable without a second read.
	Members []string `json:"members,omitempty" yaml:"members,omitempty"`
}

// learnCourseReport is one level, read.
type learnCourseReport struct {
	ID        string              `json:"id" yaml:"id"`
	Path      string              `json:"path" yaml:"path"`
	Kind      string              `json:"kind" yaml:"kind"`
	Title     string              `json:"title,omitempty" yaml:"title,omitempty"`
	Status    string              `json:"status,omitempty" yaml:"status,omitempty"`
	Intake    learnCourseIntake   `json:"intake,omitempty" yaml:"intake,omitempty"`
	Members   []learnCourseMember `json:"members" yaml:"members"`
	Refusals  []string            `json:"refusals,omitempty" yaml:"refusals,omitempty"`
	Conflicts []string            `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
	// Ready and Total are the readiness rollup over the lessons the level
	// reaches. Two facts, never a percentage.
	Ready int `json:"lessons_ready" yaml:"lessons_ready"`
	Total int `json:"lessons_total" yaml:"lessons_total"`
	// Materials names the study artefact beside each concept that has one, so a
	// course view can link it without building a path of its own (decision 0044).
	Materials map[string]*learnLessonMaterial `json:"materials,omitempty" yaml:"materials,omitempty"`
}

// text renders one level for a human reader.
func (r learnCourseReport) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s (%d %s, %d/%d lesson(s) ready)\n",
		r.ID, r.Kind, len(r.Members), learnCourseMemberWord(r.Kind), r.Ready, r.Total)
	if !r.Intake.empty() {
		b.WriteString(r.Intake.line())
	}
	for _, m := range r.Members {
		verdict := m.Verdict
		if verdict == "" {
			verdict = m.Status
		}
		fmt.Fprintf(&b, "  %d. %-44s %s\n", m.Order, m.ID, verdict)
		for _, reason := range m.Reasons {
			fmt.Fprintf(&b, "       - %s\n", reason)
		}
	}
	if len(r.Refusals) > 0 {
		fmt.Fprintf(&b, "refused (%d):\n", len(r.Refusals))
		for _, refusal := range r.Refusals {
			fmt.Fprintf(&b, "  - %s\n", refusal)
		}
	}
	if len(r.Conflicts) > 0 {
		fmt.Fprintf(&b, "order conflicts (%d):\n", len(r.Conflicts))
		for _, conflict := range r.Conflicts {
			fmt.Fprintf(&b, "  - %s\n", conflict)
		}
	}
	return b.String()
}

// learnCourseMemberWord names what a level's membership holds.
func learnCourseMemberWord(kind string) string {
	if kind == learnCourseSyllabus {
		return "theme(s)"
	}
	return "lesson(s)"
}

// learnCourseKindOf derives the level from the id's directory, which is the
// convention the scaffold writes (`themes/<slug>`, `syllabi/<slug>`). The
// document's own `kind` is checked against it, so a misfiled level is a refusal
// rather than a guess.
func learnCourseKindOf(id string) (string, bool) {
	switch {
	case strings.HasPrefix(id, "themes/"):
		return learnCourseTheme, true
	case strings.HasPrefix(id, "syllabi/"):
		return learnCourseSyllabus, true
	}
	return "", false
}

// learnCoursePath is where a level lives: the id's path plus the kind suffix,
// the same shape `context new` writes.
func learnCoursePath(kind, id string) string {
	suffix := ctxThemeSuffix
	if kind == learnCourseSyllabus {
		suffix = ctxSyllabusSuffix
	}
	return filepath.Join(sdtLearningDir, filepath.FromSlash(id)+suffix+sdtMarkdownExt)
}

// learnCourseMembership reads one level's membership section as an ordered list.
// A bullet carries one bare id (`- architecture/four-key-metrics`); a wiki link
// is accepted as the same value, because an author reaching for the graph writes
// one. Prose under the section is not membership, so it is ignored.
func learnCourseMembership(content, kind string) []string {
	section := learnCourseLessonsSection
	if kind == learnCourseSyllabus {
		section = learnCourseThemesSection
	}
	var ids []string
	for _, line := range strings.Split(sectionOf(content, section), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
			continue
		}
		value := strings.TrimSpace(trimmed[2:])
		if inner, ok := strings.CutPrefix(value, "[["); ok {
			value, _, _ = strings.Cut(inner, "]]")
		}
		value = strings.TrimSpace(strings.TrimSuffix(value, sdtMarkdownExt))
		if value != "" {
			ids = append(ids, value)
		}
	}
	return ids
}

// learnCourseIntake is the block a syllabus must state about itself: who it is
// for, the level pair it moves the learner between, and the outcome it promises.
// Objectives stay on the page; an outcome is a sentence about the learner's work.
type learnCourseIntake struct {
	Audience  string `json:"audience,omitempty" yaml:"audience,omitempty"`
	FromLevel string `json:"from_level,omitempty" yaml:"from_level,omitempty"`
	ToLevel   string `json:"to_level,omitempty" yaml:"to_level,omitempty"`
	Outcome   string `json:"outcome,omitempty" yaml:"outcome,omitempty"`
}

// empty reports whether a syllabus states none of its intake.
func (i learnCourseIntake) empty() bool {
	return i.Audience == "" && i.FromLevel == "" && i.ToLevel == "" && i.Outcome == ""
}

// line renders the intake for a human reader.
func (i learnCourseIntake) line() string {
	from, to := i.FromLevel, i.ToLevel
	if from == "" {
		from = "?"
	}
	if to == "" {
		to = "?"
	}
	return fmt.Sprintf("  audience: %s · level %s → %s\n  outcome: %s\n", i.Audience, from, to, i.Outcome)
}

// learnCourseIntakeOf reads the four keys. A value still carrying its scaffold
// placeholder counts as absent: the author has not answered yet.
func learnCourseIntakeOf(content string) learnCourseIntake {
	value := func(key string) string {
		raw := strings.TrimSpace(parseFrontmatterField(content, key))
		if strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">") {
			return ""
		}
		return raw
	}
	return learnCourseIntake{
		Audience:  value("audience"),
		FromLevel: value("from_level"),
		ToLevel:   value("to_level"),
		Outcome:   value("outcome"),
	}
}

// learnCourseIntakeIssues is the intake policy, stated once: a syllabus without an
// audience cannot be addressed, an unknown level is not in the vocabulary, and a
// level pair that runs backwards promises to end below where it starts.
func learnCourseIntakeIssues(intake learnCourseIntake) []string {
	var issues []string
	if intake.Audience == "" {
		issues = append(issues, "states no audience: a course nobody addresses has no reader")
	}
	if intake.Outcome == "" {
		issues = append(issues, "states no outcome: what the learner can do at the end is the reason the course exists")
	}
	for _, level := range []struct{ key, value string }{
		{"from_level", intake.FromLevel},
		{"to_level", intake.ToLevel},
	} {
		if level.value == "" {
			continue
		}
		if !containsItem(ctxLearningDifficulties, level.value) {
			issues = append(issues, fmt.Sprintf("has %s %q, want one of %s",
				level.key, level.value, strings.Join(ctxLearningDifficulties, ", ")))
		}
	}
	if from, to := learnCourseLevelIndex(intake.FromLevel), learnCourseLevelIndex(intake.ToLevel); from >= 0 && to >= 0 && from > to {
		issues = append(issues, fmt.Sprintf("starts at %s and ends at %s: the pair runs backwards",
			intake.FromLevel, intake.ToLevel))
	}
	return issues
}

// learnCourseLevelIndex is a level's position in the closed vocabulary, or -1.
func learnCourseLevelIndex(level string) int {
	for i, known := range ctxLearningDifficulties {
		if known == level {
			return i
		}
	}
	return -1
}

// learnCourseCoverageRow is one objective of one lesson in the level's order,
// with the item that covers it and the claim that item's answer rests on.
type learnCourseCoverageRow struct {
	Objective int    `json:"objective" yaml:"objective"`
	Lesson    string `json:"lesson" yaml:"lesson"`
	Item      string `json:"item,omitempty" yaml:"item,omitempty"`
	Claim     string `json:"claim,omitempty" yaml:"claim,omitempty"`
}

// covered reports whether an item answers for the objective.
func (r learnCourseCoverageRow) covered() bool { return r.Item != "" }

// learnCourseCoverage walks the level's lessons in order and, for each objective
// the page declares, names the item that covers it and the claim the item cites.
// A row with no item is an objective the course cannot assess — the gap the
// matrix exists to show.
func learnCourseCoverage(bld *contextwiki.Builder, kind string, members []learnCourseMember) []learnCourseCoverageRow {
	var rows []learnCourseCoverageRow
	for _, slot := range learnCourseSequence(kind, members) {
		page := bld.Resolve(slot.Lesson)
		if page == nil {
			continue
		}
		objectives := learningObjectivesOf(page.Frontmatter)
		items, _ := learnAssessmentItems(slot.Lesson)
		if len(items) == 0 {
			items, _ = learnLessonItemsForCoverage(slot.Lesson)
		}
		for i := 1; i <= len(objectives); i++ {
			row := learnCourseCoverageRow{Objective: i, Lesson: slot.Lesson}
			for _, it := range items {
				if it.Objective == i {
					row.Item = it.ID
					row.Claim = it.Claim
					break
				}
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// learnLessonItemsForCoverage falls back to the lesson's own item table for a
// concept written before the bank existed, so a level is never reported as
// uncovered only because it predates the split.
func learnLessonItemsForCoverage(lessonID string) ([]learnLessonItem, bool) {
	content := learnDocText(learnLessonPath(lessonID))
	if content == "" {
		return nil, false
	}
	return learnLessonItemsOf(sectionOf(content, "Assess")), true
}

// learnCourseCoverageText renders the matrix for a human reader, then the two
// facts of readiness. No percentage and no composite number appears.
func learnCourseCoverageText(rows []learnCourseCoverageRow, report learnCourseReport) string {
	var b strings.Builder
	for _, r := range rows {
		item, claim := r.Item, r.Claim
		if item == "" {
			item, claim = "uncovered", "-"
		}
		fmt.Fprintf(&b, "  %-44s objective %d  item %-20s %s\n", r.Lesson, r.Objective, item, claim)
	}
	covered := 0
	for _, r := range rows {
		if r.covered() {
			covered++
		}
	}
	fmt.Fprintf(&b, "lessons ready %d/%d · objectives covered %d/%d\n", report.Ready, report.Total, covered, len(rows))
	return b.String()
}

var learnCourseCmd = &cobra.Command{
	Use:   "course <id>",
	Short: "Read a theme or a syllabus: membership, readiness and order conflicts (read-only)",
	Long: `Read one course level: a theme (an id starting with ` + "`themes/`" + `) lists lessons, a
syllabus (an id starting with ` + "`syllabi/`" + `) lists themes. The verb resolves every
member in file order — position is order — and reports what a reader resolves.

WHEN — before teaching a course, after authoring a level, or when a membership
must be checked before it is published.
WHEN NOT — you want one concept's lesson: ` + "`sdt learn lesson`" + `. You want to know
whether a concept is teachable: ` + "`sdt learn check`" + `.

RETURNS — the level, its members in order, each lesson's readiness verdict, the
refusals and the order conflicts. A dangling, archived, duplicate or wrong-kind
member is a refusal and exits 1. An order that contradicts a member's
` + "`relations.depends_on`" + ` or ` + "`## Prerequisites`" + ` is reported and never resolved.
` + "`--validate`" + ` reads every level and counts both as findings. Reads only.

EXAMPLE — sdt learn course themes/delivery-metrics
          sdt learn course syllabi/software-architect --format json
          sdt learn course --validate`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld := loadLearnWiki(cmd)
		if getBoolFlag(cmd, "validate", false) {
			reports := learnCourseValidateAll(bld)
			findings := 0
			for _, r := range reports {
				findings += len(r.Refusals) + len(r.Conflicts)
			}
			learnEmit(cmd, reports, func() {
				if len(reports) == 0 {
					outputString(cmd, "no course level exists under context/learning/themes or context/learning/syllabi\n")
					return
				}
				for _, r := range reports {
					outputString(cmd, r.text())
				}
				outputString(cmd, fmt.Sprintf("course levels: %d, findings: %d\n", len(reports), findings))
			})
			if findings > 0 {
				exitWithError(cmd, fmt.Errorf("%d course finding(s)", findings))
			}
			return
		}
		if len(args) != 1 {
			exitWithError(cmd, fmt.Errorf("one level is required, use sdt learn course <id> (e.g. themes/delivery-metrics)"))
			return
		}
		report, err := learnCourseRead(bld, args[0])
		exitWithError(cmd, err)
		if getBoolFlag(cmd, "coverage", false) {
			rows := learnCourseCoverage(bld, report.Kind, report.Members)
			learnEmit(cmd, rows, func() {
				outputString(cmd, report.text())
				outputString(cmd, learnCourseCoverageText(rows, report))
			})
			// An empty level covers nothing: it is incomplete for the same reason a
			// lesson without items is — a course that lists nothing cannot teach.
			incomplete := len(rows) == 0 || report.Ready != report.Total
			for _, r := range rows {
				if !r.covered() {
					incomplete = true
				}
			}
			if len(report.Refusals) > 0 || incomplete {
				exitWithError(cmd, fmt.Errorf("the course does not cover what it lists"))
			}
			return
		}
		learnEmit(cmd, report, func() { outputString(cmd, report.text()) })
		if len(report.Refusals) > 0 {
			exitWithError(cmd, fmt.Errorf("%d membership refusal(s)", len(report.Refusals)))
		}
	},
}

// learnCourseRead reads one level and resolves its membership.
func learnCourseRead(bld *contextwiki.Builder, id string) (learnCourseReport, error) {
	kind, ok := learnCourseKindOf(id)
	if !ok {
		return learnCourseReport{}, fmt.Errorf("%q is neither a theme nor a syllabus: an id starts with themes/ or syllabi/", id)
	}
	path := learnCoursePath(kind, id)
	data, err := os.ReadFile(path) //#nosec G304 -- path built from the level id
	if err != nil {
		return learnCourseReport{}, fmt.Errorf("no %s at %s", kind, path)
	}
	content := string(data)
	if got := parseFrontmatterField(content, "kind"); got != "" && got != kind {
		return learnCourseReport{}, fmt.Errorf("%s: id says %s, frontmatter says kind %q", path, kind, got)
	}
	return learnCourseReportOf(bld, id, kind, path, content), nil
}

// learnCourseValidateAll reads every level under the two directories.
func learnCourseValidateAll(bld *contextwiki.Builder) []learnCourseReport {
	var out []learnCourseReport
	for _, dir := range []string{"themes", "syllabi"} {
		files, err := walkMarkdownFiles(filepath.Join(sdtLearningDir, dir))
		if err != nil {
			continue
		}
		sort.Strings(files)
		for _, path := range files {
			rel, relErr := filepath.Rel(sdtLearningDir, path)
			if relErr != nil {
				continue
			}
			slash := filepath.ToSlash(rel)
			for _, suffix := range []string{ctxThemeSuffix, ctxSyllabusSuffix} {
				if !strings.HasSuffix(slash, suffix+sdtMarkdownExt) {
					continue
				}
				id := strings.TrimSuffix(slash, suffix+sdtMarkdownExt)
				report, readErr := learnCourseRead(bld, id)
				if readErr != nil {
					out = append(out, learnCourseReport{ID: id, Path: path, Refusals: []string{readErr.Error()}})
					break
				}
				out = append(out, report)
				break
			}
		}
	}
	return out
}

// learnCourseReportOf resolves a level already read from disk.
func learnCourseReportOf(bld *contextwiki.Builder, id, kind, path, content string) learnCourseReport {
	r := learnCourseReport{
		ID:     id,
		Path:   path,
		Kind:   kind,
		Title:  parseFrontmatterField(content, "title"),
		Status: parseFrontmatterField(content, ctxMapStatus),
	}
	if kind == learnCourseSyllabus {
		r.Intake = learnCourseIntakeOf(content)
		for _, issue := range learnCourseIntakeIssues(r.Intake) {
			r.Refusals = append(r.Refusals, "intake: "+issue)
		}
	}
	seen := map[string]bool{}
	for i, member := range learnCourseMembership(content, kind) {
		m := learnCourseMem(bld, kind, member, i+1)
		if seen[member] {
			m.Reasons = append(m.Reasons, "appears twice; one concept occupies one position")
		}
		seen[member] = true
		for _, reason := range m.Reasons {
			r.Refusals = append(r.Refusals, fmt.Sprintf("member %d %q: %s", i+1, member, reason))
		}
		r.Members = append(r.Members, m)
	}
	r.Ready, r.Total = learnCourseReadiness(bld, kind, r.Members)
	r.Conflicts = learnCourseOrderConflicts(bld, kind, r.Members)
	r.Materials = learnCourseMaterials(r.Members)
	return r
}

// learnCourseMaterials resolves the material beside every concept the level
// reaches. A member that is itself a concept is tried too, so a theme report
// carries its lessons' materials as well as a syllabus carries its themes'.
func learnCourseMaterials(members []learnCourseMember) map[string]*learnLessonMaterial {
	out := map[string]*learnLessonMaterial{}
	for _, m := range members {
		concepts := m.Members
		if len(concepts) == 0 {
			concepts = []string{m.ID}
		}
		for _, concept := range concepts {
			if mat := learnMaterialOf(concept); mat != nil {
				out[concept] = mat
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// learnIsLevelPath reports whether a corpus path is a course level rather than a
// lesson. The levels share the learning tree with the lessons, so every walk that
// treats a document as a concept has to skip them by their suffix.
func learnIsLevelPath(path string) bool {
	return strings.HasSuffix(path, ctxThemeSuffix+sdtMarkdownExt) ||
		strings.HasSuffix(path, ctxSyllabusSuffix+sdtMarkdownExt)
}

// learnCourseMemberFile resolves one member to the document that holds it, or a
// reason it cannot be a member at all. It is the resolver the verb and the
// corpus lint share, so a membership the verb refuses is never a lint pass.
func learnCourseMemberFile(levelKind, member string) (string, string) {
	wantKind := ctxTypeLesson
	if levelKind == learnCourseSyllabus {
		wantKind = learnCourseTheme
	}
	memberKind, isLevel := learnCourseKindOf(member)
	if wantKind == ctxTypeLesson && isLevel {
		return "", fmt.Sprintf("is a %s, not a lesson", memberKind)
	}
	if wantKind == learnCourseTheme && (!isLevel || memberKind != learnCourseTheme) {
		return "", "is not a theme; a syllabus lists themes"
	}
	path := learnLessonPath(member)
	if wantKind == learnCourseTheme {
		path = learnCoursePath(learnCourseTheme, member)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Sprintf("no document at %s", path)
	}
	return path, ""
}

// learnCourseMem resolves one member: what it is, whether it exists, and — for a
// lesson — whether it can be taught.
func learnCourseMem(bld *contextwiki.Builder, kind, member string, order int) learnCourseMember {
	m := learnCourseMember{Order: order, ID: member}
	wantKind := ctxTypeLesson
	if kind == learnCourseSyllabus {
		wantKind = learnCourseTheme
	}
	path, reason := learnCourseMemberFile(kind, member)
	if reason != "" {
		m.Reasons = append(m.Reasons, reason)
		return m
	}
	data, err := os.ReadFile(path) //#nosec G304 -- path built from the member id
	if err != nil {
		m.Reasons = append(m.Reasons, fmt.Sprintf("no document at %s", path))
		return m
	}
	content := string(data)
	m.Status = parseFrontmatterField(content, ctxMapStatus)
	m.Title = parseFrontmatterField(content, "title")
	if m.Status == statusArchived {
		m.Reasons = append(m.Reasons, "is archived")
	}
	if wantKind == learnCourseTheme {
		// A theme is not a concept: it has no page and no readiness of its own.
		m.Members = learnCourseMembership(content, learnCourseTheme)
		return m
	}
	page := bld.Resolve(member)
	if page == nil {
		m.Reasons = append(m.Reasons, "no wiki page carries this concept")
		return m
	}
	m.Verdict = learnReadinessOf(page).Verdict
	return m
}

// learnCourseReadiness counts the lessons a level reaches and how many are ready:
// two facts, never a percentage (`learning.md`, *No score*).
func learnCourseReadiness(bld *contextwiki.Builder, kind string, members []learnCourseMember) (ready, total int) {
	for _, m := range members {
		if kind == learnCourseSyllabus {
			for _, lessonID := range m.Members {
				total++
				if page := bld.Resolve(lessonID); page != nil && learnReadinessOf(page).Verdict == learnVerdictTeachable {
					ready++
				}
			}
			continue
		}
		total++
		if m.Verdict == learnVerdictTeachable {
			ready++
		}
	}
	return ready, total
}

// learnCourseSlot is one lesson in the level's flattened order.
type learnCourseSlot struct {
	Position int    // 1-based, across the whole level
	Lesson   string // the concept id
}

// learnCourseSequence flattens a level into the order a learner meets it: a
// theme is its own membership, a syllabus is its themes expanded in order.
func learnCourseSequence(kind string, members []learnCourseMember) []learnCourseSlot {
	var slots []learnCourseSlot
	for _, m := range members {
		if kind == learnCourseSyllabus {
			for _, lessonID := range m.Members {
				slots = append(slots, learnCourseSlot{Position: len(slots) + 1, Lesson: lessonID})
			}
			continue
		}
		slots = append(slots, learnCourseSlot{Position: len(slots) + 1, Lesson: m.ID})
	}
	return slots
}

// learnCourseOrderConflicts compares the flattened order with what each lesson
// declares it needs first: the typed relations of its page and the ids its
// `## Prerequisites` section names. A prerequisite that appears later in the
// course is a conflict — reported with both positions, never reordered, because
// the document is the author's and the graph is the corpus's.
func learnCourseOrderConflicts(bld *contextwiki.Builder, kind string, members []learnCourseMember) []string {
	slots := learnCourseSequence(kind, members)
	position := map[string]int{}
	for _, s := range slots {
		if _, ok := position[s.Lesson]; !ok {
			position[s.Lesson] = s.Position
		}
	}
	var conflicts []string
	seen := map[string]bool{}
	for _, s := range slots {
		for _, prereq := range learnCoursePrerequisitesOf(bld, s.Lesson) {
			theirPos, member := position[prereq]
			if !member || theirPos <= s.Position {
				continue
			}
			key := s.Lesson + "|" + prereq
			if seen[key] {
				continue
			}
			seen[key] = true
			conflicts = append(conflicts, fmt.Sprintf(
				"%s is at position %d but %s, which it needs first, is at %d",
				prereq, theirPos, s.Lesson, s.Position))
		}
	}
	return conflicts
}

// learnCoursePrerequisitesOf is the ids one lesson declares before itself: the
// typed relations of its page, plus the ids its `## Prerequisites` section names
// in backticks or as a wiki link (the two forms the lesson contract uses).
func learnCoursePrerequisitesOf(bld *contextwiki.Builder, lessonID string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(strings.TrimSuffix(id, sdtMarkdownExt))
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	if page := bld.Resolve(lessonID); page != nil {
		for _, dep := range learnPrerequisites(bld, page) {
			add(dep)
		}
	}
	section := sectionOf(learnDocText(learnLessonPath(lessonID)), "Prerequisites")
	for _, ref := range learnCourseLinkRefs(section) {
		add(ref)
	}
	return out
}

// learnCourseLinkRefs returns the wiki links and the backticked spans of a prose
// section: the two forms a lesson uses to name a concept it builds on.
func learnCourseLinkRefs(section string) []string {
	var refs []string
	rest := section
	for {
		start := strings.Index(rest, "[[")
		if start < 0 {
			break
		}
		rest = rest[start+2:]
		end := strings.Index(rest, "]]")
		if end < 0 {
			break
		}
		refs = append(refs, rest[:end])
		rest = rest[end+2:]
	}
	for _, part := range strings.Split(section, "`") {
		if strings.ContainsAny(part, " \n\t") || part == "" {
			continue
		}
		refs = append(refs, part)
	}
	return refs
}

func init() {
	learnCourseCmd.Flags().Bool("validate", false, "Read every theme and syllabus and report findings (read-only)")
	learnCourseCmd.Flags().Bool("coverage", false, "Print the objective to item matrix across the membership (read-only)")
	learnCmd.AddCommand(learnCourseCmd)
}
