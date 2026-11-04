//go:build !wasm

package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCurriculumKindIdentity binds the id each learning-level kind derives to the
// path it lands on. The theme and the syllabus keep their suffix on the file name
// only — their directory already names the level — while the assessment, which
// sits beside its lesson, carries the suffix in the id because the suffix is the
// only thing telling the two apart. One registry field is the only switch.
func TestCurriculumKindIdentity(t *testing.T) {
	cases := []struct {
		kind     string
		slug     string
		wantID   string
		wantPath string
	}{
		{
			ctxTypeLesson, "architecture/four-key-metrics",
			"architecture/four-key-metrics",
			"context/learning/architecture/four-key-metrics.md",
		},
		{
			ctxTypeAssessment, "architecture/four-key-metrics",
			"architecture/four-key-metrics.assessment",
			"context/learning/architecture/four-key-metrics.assessment.md",
		},
		{
			ctxTypeTheme, "themes/delivery-metrics",
			"themes/delivery-metrics",
			"context/learning/themes/delivery-metrics.theme.md",
		},
		{
			ctxTypeSyllabus, "syllabi/software-architect",
			"syllabi/software-architect",
			"context/learning/syllabi/software-architect.syllabus.md",
		},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			tp := ctxTypeLookupOrFatal(t, c.kind)
			if got := ctxTypeID(tp, c.slug); got != c.wantID {
				t.Errorf("id = %q, want %q", got, c.wantID)
			}
			path, err := contextPath(c.kind, c.slug, "", "", "")
			if err != nil {
				t.Fatalf("contextPath(%s): %v", c.kind, err)
			}
			if got := filepath.ToSlash(path); got != c.wantPath {
				t.Errorf("path = %q, want %q", got, c.wantPath)
			}
		})
	}
}

// TestNewThemeAndSyllabusScaffold writes both levels and reads back what a reader
// resolves: the path, the kind, the bare id and the one section membership lives
// under. A level that cannot be created by the CLI is a level nobody authors
// consistently.
func TestNewThemeAndSyllabusScaffold(t *testing.T) {
	runInTempDir(t)

	themePath := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
		"--type", ctxTypeTheme, "--slug", "themes/delivery-metrics",
		"--title", "Delivery metrics — theme", "--summary", "s")))
	if got := filepath.ToSlash(themePath); got != "context/learning/themes/delivery-metrics.theme.md" {
		t.Fatalf("theme path = %q", got)
	}
	theme := mustReadFile(t, themePath)
	if got := frontmatterField(theme, "kind"); got != ctxTypeTheme {
		t.Errorf("theme kind = %q, want %q", got, ctxTypeTheme)
	}
	if got := frontmatterField(theme, "id"); got != "themes/delivery-metrics" {
		t.Errorf("theme id = %q, want %q", got, "themes/delivery-metrics")
	}
	if !strings.Contains(theme, "## Lessons") {
		t.Error("the theme scaffold carries no `## Lessons` section: membership has no home")
	}

	syllabusPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil,
		"--type", ctxTypeSyllabus, "--slug", "syllabi/software-architect",
		"--title", "Software architect — syllabus", "--summary", "s")))
	if got := filepath.ToSlash(syllabusPath); got != "context/learning/syllabi/software-architect.syllabus.md" {
		t.Fatalf("syllabus path = %q", got)
	}
	syllabus := mustReadFile(t, syllabusPath)
	if got := frontmatterField(syllabus, "kind"); got != ctxTypeSyllabus {
		t.Errorf("syllabus kind = %q, want %q", got, ctxTypeSyllabus)
	}
	if got := frontmatterField(syllabus, "id"); got != "syllabi/software-architect" {
		t.Errorf("syllabus id = %q, want %q", got, "syllabi/software-architect")
	}
	if !strings.Contains(syllabus, "## Themes") {
		t.Error("the syllabus scaffold carries no `## Themes` section: order has no home")
	}

	// The two levels are addressable like every other type: a listing finds each
	// one, and neither hides inside the other's kind.
	if out := string(execute(t, contextListCmd, nil, "--type", ctxTypeTheme)); !strings.Contains(out, "delivery-metrics.theme.md") {
		t.Errorf("context list --type theme did not find the theme: %q", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", ctxTypeSyllabus)); !strings.Contains(out, "software-architect.syllabus.md") {
		t.Errorf("context list --type syllabus did not find the syllabus: %q", out)
	}
}

// TestCurriculumSuffixDoesNotLeakIntoTheID guards the inverse: the provisional
// convention this registry replaced put the level suffix in the id as well, which
// reads as `themes/delivery-metrics.theme`. A regression would be invisible in a
// listing and visible only to a resolver, so it is asserted here.
func TestCurriculumSuffixDoesNotLeakIntoTheID(t *testing.T) {
	for _, kind := range []string{ctxTypeTheme, ctxTypeSyllabus} {
		tp := ctxTypeLookupOrFatal(t, kind)
		if tp.suffix == "" {
			t.Errorf("%s carries no file-name suffix, so a listing cannot tell the levels apart", kind)
		}
		if tp.suffixInID {
			t.Errorf("%s puts %q in its id; the directory already names the level", kind, tp.suffix)
		}
	}
	if tp := ctxTypeLookupOrFatal(t, ctxTypeAssessment); !tp.suffixInID {
		t.Error("assessment lost the suffix in its id: it sits beside its lesson, so the suffix is the only discriminator")
	}
}

// TestLintCourseChecksShapeAndMembership binds the corpus check to the same
// resolver the verb uses: a dangling member is a finding in lint exactly when it
// is a refusal in `learn course`, so the two surfaces cannot disagree.
func TestLintCourseChecksShapeAndMembership(t *testing.T) {
	runInTempDir(t)
	writeCourseConcept(t, "architecture/a")

	clean := "---\nkind: theme\nid: themes/t\nsummary: s\nstatus: active\n---\n\n## Thread\n\nwhy\n\n## Lessons\n\n- architecture/a\n"
	if issues := lintCourse("context/learning/themes/t.theme.md", clean, ctxTypeTheme); len(issues) != 0 {
		t.Fatalf("a clean theme reported %d finding(s): %v", len(issues), issues)
	}

	dangling := strings.Replace(clean, "- architecture/a", "- architecture/a\n- architecture/missing", 1)
	issues := lintCourse("context/learning/themes/t.theme.md", dangling, ctxTypeTheme)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "architecture/missing") {
		t.Fatalf("a dangling member produced %v", issues)
	}

	empty := "---\nkind: syllabus\nid: syllabi/s\nsummary: s\nstatus: active\n---\n\n## Thread\n\nwhy\n\n## Themes\n\n"
	if issues := lintCourse("context/learning/syllabi/s.syllabus.md", empty, ctxTypeSyllabus); len(issues) == 0 {
		t.Fatal("an empty syllabus produced no finding: a level that groups nothing teaches nothing")
	}

	restating := clean + "\n| id | expected | rubric |\n|---|---|---|\n| a-1 | x | y |\n"
	issues = lintCourse("context/learning/themes/t.theme.md", restating, ctxTypeTheme)
	found := false
	for _, is := range issues {
		if strings.Contains(is.Message, "item table") {
			found = true
		}
	}
	if !found {
		t.Errorf("a level carrying an item table produced no finding: %v", issues)
	}
}
