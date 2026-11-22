package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

type learningMetadataMove struct {
	Concept   string
	Wiki      string
	Lesson    string
	OldWiki   []byte
	OldLesson []byte
	NewWiki   string
	NewLesson string
	Metadata  learnMetadata
}

var learnMigrateMetadataCmd = &cobra.Command{
	Use:   "migrate-metadata",
	Short: "Move wiki learning metadata to lessons",
	Long: `Move objectives, difficulty and minutes from wiki frontmatter to the
matching lesson frontmatter. Dry-run is the default. The wiki objective order is
preserved for assessment indexes. Conflicts stop the whole migration before any
write. Apply is resumable; originals are backed up under .sdt/learning-migration.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if restore := getStringFlag(cmd, "restore", false); restore != "" {
			return restoreLearningMetadataBackup(restore)
		}
		moves, conflicts, err := planLearningMetadataMoves()
		if err != nil {
			return err
		}
		for _, conflict := range conflicts {
			outputString(cmd, "conflict: "+conflict+"\n")
		}
		if len(conflicts) > 0 {
			return fmt.Errorf("migration stopped: %d conflict(s)", len(conflicts))
		}
		apply := getBoolFlag(cmd, "apply", false)
		if !apply {
			for _, move := range moves {
				meta := move.Metadata
				outputString(cmd, fmt.Sprintf("would move %s (%d objectives, difficulty %s, %d min): %s → %s\n", move.Concept, len(meta.Objectives), meta.Difficulty, meta.Minutes, corpusRelative(move.Wiki), corpusRelative(move.Lesson)))
			}
			outputString(cmd, fmt.Sprintf("dry-run: %d concept(s); pass --apply to write\n", len(moves)))
			return nil
		}
		root := sdtProjectRoot()
		backup := filepath.Join(root, ".sdt", "learning-migration", time.Now().UTC().Format("20060102T150405Z"))
		for _, move := range moves {
			if bytes.Equal(move.OldWiki, []byte(move.NewWiki)) && bytes.Equal(move.OldLesson, []byte(move.NewLesson)) {
				continue
			}
			for _, item := range []struct {
				path string
				old  []byte
			}{{move.Wiki, move.OldWiki}, {move.Lesson, move.OldLesson}} {
				rel, err := filepath.Rel(root, item.path)
				if err != nil {
					return fmt.Errorf("relative backup path %s: %w", item.path, err)
				}
				if err := createContextDocFileMode(filepath.Join(backup, rel), item.old, 0o600); err != nil {
					return fmt.Errorf("backup %s: %w", item.path, err)
				}
			}
			// Lesson first makes an interrupted migration safe to resume.
			if err := writeContextDocFile(move.Lesson, move.OldLesson, move.NewLesson); err != nil {
				return fmt.Errorf("write lesson %s: %w", move.Lesson, err)
			}
			if err := writeContextDocFile(move.Wiki, move.OldWiki, move.NewWiki); err != nil {
				return fmt.Errorf("write wiki %s: %w", move.Wiki, err)
			}
			outputString(cmd, fmt.Sprintf("migrated %s\n", move.Concept))
		}
		outputString(cmd, "rollback evidence: "+backup+"\n")
		return nil
	},
}

func init() {
	learnMigrateMetadataCmd.Flags().Bool("apply", false, "write the migration; without this flag, print a dry-run")
	learnMigrateMetadataCmd.Flags().String("restore", "", "restore a migration backup directory")
	learnCmd.AddCommand(learnMigrateMetadataCmd)
}

func restoreLearningMetadataBackup(backup string) error {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	backupRoot, err := os.OpenRoot(backup)
	if err != nil {
		return err
	}
	defer closeReadRoot(backupRoot)
	destinationRoot, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer closeReadRoot(destinationRoot)
	return filepath.WalkDir(backup, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(backup, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		ctx := -1
		for i, part := range parts {
			if part == gitIgnoreModeContext {
				ctx = i
				break
			}
		}
		if ctx < 0 {
			return nil
		}
		targetRel := filepath.FromSlash(strings.Join(parts[ctx:], "/"))
		target := filepath.Join(root, targetRel)
		backupRel, err := filepath.Rel(backup, path)
		if err != nil {
			return err
		}
		old, err := backupRoot.ReadFile(backupRel)
		if err != nil {
			return err
		}
		current, err := destinationRoot.ReadFile(targetRel)
		if err != nil {
			return err
		}
		if err := writeContextDocFile(target, current, string(old)); err != nil {
			return fmt.Errorf("restore %s: %w", target, err)
		}
		return nil
	})
}

func planLearningMetadataMoves() ([]learningMetadataMove, []string, error) {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	defer closeReadRoot(rootFS)
	wikiRoot := filepath.Join(root, sdtWikiDir)
	var moves []learningMetadataMove
	var conflicts []string
	err = filepath.WalkDir(wikiRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), sdtMarkdownExt) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		wikiBytes, err := rootFS.ReadFile(rel)
		if err != nil {
			return err
		}
		move, issues, err := planLearningMetadataMove(rootFS, root, path, wikiBytes)
		if err != nil {
			return err
		}
		conflicts = append(conflicts, issues...)
		if move != nil {
			moves = append(moves, *move)
		}
		return nil
	})
	return moves, conflicts, err
}

// closeReadRoot releases a read-only traversal root; closing it cannot commit
// data, so a close error has no recoverable migration state to report.
func closeReadRoot(root *os.Root) {
	root.Close() //nolint:errcheck // closing read-only roots only releases their handles
}

func planLearningMetadataMove(rootFS *os.Root, root, wikiPath string, wikiBytes []byte) (*learningMetadataMove, []string, error) {
	fields, err := parseLearningMigrationFrontmatter(string(wikiBytes))
	if err != nil {
		return nil, nil, nil
	}
	raw, present := fields[ctxFrontmatterLearning]
	if !present {
		return nil, nil, nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return nil, []string{wikiPath + ": learning must be a map"}, nil
	}
	concept, ok := fields["id"].(string)
	if !ok || concept == "" {
		return nil, []string{wikiPath + ": missing id"}, nil
	}
	lesson := filepath.Join(root, learnLessonPath(concept))
	lessonRel, err := filepath.Rel(root, lesson)
	if err != nil {
		return nil, nil, err
	}
	lessonBytes, err := rootFS.ReadFile(lessonRel)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: matching lesson is missing (%s)", concept, lesson)}, nil
	}
	lessonFields, err := parseLearningMigrationFrontmatter(string(lessonBytes))
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: lesson frontmatter is invalid", concept)}, nil
	}
	issues := learningMetadataConflicts(wikiPath, concept, block, lessonFields)
	newWiki, err := removeTopLevelLearningBlock(string(wikiBytes))
	if err != nil {
		return nil, nil, err
	}
	missing := map[string]any{}
	for _, key := range []string{ctxLearningObjectives, ctxLearningDifficulty, ctxLearningMinutes} {
		if _, exists := lessonFields[key]; !exists {
			missing[key] = block[key]
		}
	}
	newLesson, err := addLearningMetadata(string(lessonBytes), missing)
	if err != nil {
		return nil, nil, err
	}
	meta := learnMetadata{Objectives: learningObjectives(block[ctxLearningObjectives])}
	if difficulty, ok := block[ctxLearningDifficulty].(string); ok {
		meta.Difficulty = difficulty
	}
	if minutes, ok := learningMinutes(block[ctxLearningMinutes]); ok {
		meta.Minutes = minutes
	}
	return &learningMetadataMove{Concept: concept, Wiki: wikiPath, Lesson: lesson, OldWiki: wikiBytes, OldLesson: lessonBytes, NewWiki: newWiki, NewLesson: newLesson, Metadata: meta}, issues, nil
}

func learningMetadataConflicts(wikiPath, concept string, block, lesson map[string]any) []string {
	var conflicts []string
	for key := range block {
		if key != ctxLearningObjectives && key != ctxLearningDifficulty && key != ctxLearningMinutes {
			conflicts = append(conflicts, fmt.Sprintf("%s: learning.%s is unknown and needs explicit mapping", wikiPath, key))
		}
	}
	for _, key := range []string{ctxLearningObjectives, ctxLearningDifficulty, ctxLearningMinutes} {
		value, exists := block[key]
		if !exists {
			conflicts = append(conflicts, fmt.Sprintf("%s: learning.%s is missing", concept, key))
			continue
		}
		if current, exists := lesson[key]; exists && !reflect.DeepEqual(normalizeLearningValue(current), normalizeLearningValue(value)) {
			conflicts = append(conflicts, fmt.Sprintf("%s: lesson %s conflicts with wiki learning.%s", concept, key, key))
		}
	}
	return conflicts
}

func parseLearningMigrationFrontmatter(content string) (map[string]any, error) {
	fm, _ := contextwiki.SplitFrontmatter(content)
	var fields map[string]any
	if fm == "" {
		return nil, fmt.Errorf("missing frontmatter")
	}
	fm = strings.TrimPrefix(fm, "---\n")
	fm = strings.TrimSuffix(fm, "\n---\n")
	fm = strings.TrimSuffix(fm, "\n---")
	if err := yaml.Unmarshal([]byte(fm), &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func removeTopLevelLearningBlock(content string) (string, error) {
	fm, body := contextwiki.SplitFrontmatter(content)
	lines := frontmatterLines(fm)
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "learning:") {
			start = i
			break
		}
	}
	if start < 0 {
		return content, nil
	}
	end := start + 1
	for end < len(lines) {
		line := lines[end]
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			end++
			continue
		}
		break
	}
	clean := append(append([]string(nil), lines[:start]...), lines[end:]...)
	return wrapLearningFrontmatter(clean, body), nil
}

func addLearningMetadata(content string, fields map[string]any) (string, error) {
	if len(fields) == 0 {
		return content, nil
	}
	fm, body := contextwiki.SplitFrontmatter(content)
	lines := frontmatterLines(fm)
	data, err := yaml.Marshal(fields)
	if err != nil {
		return "", err
	}
	insert := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	return wrapLearningFrontmatter(append(append(lines, insert...), ""), body), nil
}

func frontmatterLines(fm string) []string {
	fm = strings.TrimPrefix(fm, "---\n")
	fm = strings.TrimSuffix(fm, "---\n")
	fm = strings.TrimSuffix(fm, "\n---\n")
	fm = strings.TrimSuffix(fm, "\n---")
	if fm == "" {
		return nil
	}
	return strings.Split(fm, "\n")
}

func wrapLearningFrontmatter(lines []string, body string) string {
	return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body
}

func normalizeLearningValue(v any) any {
	if list, ok := v.([]any); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			out = append(out, fmt.Sprint(item))
		}
		return out
	}
	return fmt.Sprint(v)
}
