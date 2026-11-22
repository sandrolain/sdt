package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLearningMigrationFixture(t *testing.T, lessonObjectives string) (string, string) {
	t.Helper()
	runInTempDir(t)
	wiki := filepath.Join(sdtWikiDir, "architecture", "x.md")
	lesson := learnLessonPath("architecture/x")
	writeLearningDoc(t, wiki, "---\nkind: wiki\nid: architecture/x\ntitle: X\nlearning:\n  objectives:\n    - \"name the parts\"\n    - \"apply the rule\"\n  difficulty: intermediate\n  minutes: 10\n---\n\n## Summary\n\nX.\n")
	writeLearningDoc(t, lesson, "---\nkind: lesson\nid: architecture/x\n"+lessonObjectives+"summary: X lesson\n---\n\n## Worked example\n\nX.\n")
	return wiki, lesson
}

func TestLearnMigrateMetadataDryRunDoesNotWrite(t *testing.T) {
	wiki, lesson := writeLearningMigrationFixture(t, "")
	oldWiki, _ := os.ReadFile(wiki)
	oldLesson, _ := os.ReadFile(lesson)
	moves, conflicts, err := planLearningMetadataMoves()
	if err != nil || len(conflicts) != 0 || len(moves) != 1 {
		t.Fatalf("plan = %d moves, %v conflicts, %v", len(moves), conflicts, err)
	}
	newWiki, _ := os.ReadFile(wiki)
	newLesson, _ := os.ReadFile(lesson)
	if string(oldWiki) != string(newWiki) || string(oldLesson) != string(newLesson) {
		t.Fatal("planning wrote corpus files")
	}
}

func TestLearnMigrateMetadataRejectsConflictBeforeAnyWrite(t *testing.T) {
	wiki, lesson := writeLearningMigrationFixture(t, "objectives:\n  - \"different\"\n")
	oldWiki, _ := os.ReadFile(wiki)
	oldLesson, _ := os.ReadFile(lesson)
	_, conflicts, err := planLearningMetadataMoves()
	if err != nil || len(conflicts) == 0 {
		t.Fatalf("conflicts = %v, err = %v", conflicts, err)
	}
	newWiki, _ := os.ReadFile(wiki)
	newLesson, _ := os.ReadFile(lesson)
	if string(oldWiki) != string(newWiki) || string(oldLesson) != string(newLesson) {
		t.Fatal("a conflict changed corpus files")
	}
}

func TestLearnMigrateMetadataRejectsReorderedObjectives(t *testing.T) {
	wiki, lesson := writeLearningMigrationFixture(t, "objectives:\n  - \"apply the rule\"\n  - \"name the parts\"\n")
	oldWiki, _ := os.ReadFile(wiki)
	oldLesson, _ := os.ReadFile(lesson)
	_, conflicts, err := planLearningMetadataMoves()
	if err != nil || len(conflicts) == 0 || !strings.Contains(strings.Join(conflicts, "\n"), "conflicts with wiki learning.objectives") {
		t.Fatalf("reordered objectives were not rejected: %v, err=%v", conflicts, err)
	}
	newWiki, _ := os.ReadFile(wiki)
	newLesson, _ := os.ReadFile(lesson)
	if string(oldWiki) != string(newWiki) || string(oldLesson) != string(newLesson) {
		t.Fatal("rejected objective order changed corpus files")
	}
}

func TestLearnMigrateMetadataResumesAfterLessonWrite(t *testing.T) {
	wiki, lesson := writeLearningMigrationFixture(t, "")
	lessonBytes, err := os.ReadFile(lesson)
	if err != nil {
		t.Fatal(err)
	}
	partial := strings.Replace(string(lessonBytes), "id: architecture/x\n", "id: architecture/x\nobjectives:\n  - \"name the parts\"\n  - \"apply the rule\"\ndifficulty: intermediate\nminutes: 10\n", 1)
	if err := os.WriteFile(lesson, []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := string(execute(t, learnMigrateMetadataCmd, nil, "--apply")); strings.Contains(out, "ERR") {
		t.Fatalf("resume migration: %s", out)
	}
	wikiBytes, err := os.ReadFile(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wikiBytes), "learning:") {
		t.Fatal("resume left learning metadata in the wiki")
	}
}

func TestLearnMigrateMetadataAppliesAndCanResume(t *testing.T) {
	wiki, lesson := writeLearningMigrationFixture(t, "")
	out := string(execute(t, learnMigrateMetadataCmd, nil, "--apply"))
	if strings.Contains(out, "ERR") {
		t.Fatalf("apply migration: %s", out)
	}
	wikiData, _ := os.ReadFile(wiki)
	lessonData, _ := os.ReadFile(lesson)
	if strings.Contains(string(wikiData), "learning:") {
		t.Fatal("wiki still carries learning metadata")
	}
	if !strings.Contains(string(lessonData), "objectives:") || !strings.Contains(string(lessonData), "difficulty: intermediate") || !strings.Contains(string(lessonData), "minutes: 10") {
		t.Fatalf("lesson lacks migrated values:\n%s", lessonData)
	}
	moves, conflicts, err := planLearningMetadataMoves()
	if err != nil || len(conflicts) != 0 || len(moves) != 0 {
		t.Fatalf("second plan = %d moves, %v conflicts, %v", len(moves), conflicts, err)
	}
	backups, err := filepath.Glob(filepath.Join(".sdt", "learning-migration", "*", "context", "*", "architecture", "x.md"))
	if err != nil || len(backups) == 0 {
		t.Fatalf("rollback evidence missing: %v, %v", backups, err)
	}
}

func TestLearnMigrateMetadataRestoresExactOriginals(t *testing.T) {
	wiki, lesson := writeLearningMigrationFixture(t, "")
	oldWiki, _ := os.ReadFile(wiki)
	oldLesson, _ := os.ReadFile(lesson)
	if out := string(execute(t, learnMigrateMetadataCmd, nil, "--apply")); strings.Contains(out, "ERR") {
		t.Fatalf("apply migration: %s", out)
	}
	backups, err := filepath.Glob(filepath.Join(".sdt", "learning-migration", "*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup dirs = %v, err=%v", backups, err)
	}
	writeLearningDoc(t, wiki, "damaged wiki")
	writeLearningDoc(t, lesson, "damaged lesson")
	if err := restoreLearningMetadataBackup(backups[0]); err != nil {
		t.Fatalf("restore: %v", err)
	}
	gotWiki, _ := os.ReadFile(wiki)
	gotLesson, _ := os.ReadFile(lesson)
	if string(gotWiki) != string(oldWiki) || string(gotLesson) != string(oldLesson) {
		t.Fatal("restore did not recover the original bytes")
	}
}
