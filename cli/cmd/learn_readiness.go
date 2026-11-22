package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

const (
	readinessStatusNotPlanned = "not_planned"
	readinessStatusAbsent     = "absent"
	readinessStatusUnreviewed = "unreviewed"
	readinessStatusReady      = "ready"
	readinessTargetMaterials  = "materials"
)

// learnReadinessStage keeps one specialist's verdict separate from the others.
type learnReadinessStage struct {
	Name     string   `json:"name" yaml:"name"`
	Status   string   `json:"status" yaml:"status"`
	Artifact string   `json:"artifact,omitempty" yaml:"artifact,omitempty"`
	Reasons  []string `json:"reasons,omitempty" yaml:"reasons,omitempty"`
}

type learnTopicReadiness struct {
	Topic      string                `json:"topic" yaml:"topic"`
	Concept    string                `json:"concept,omitempty" yaml:"concept,omitempty"`
	Title      string                `json:"title,omitempty" yaml:"title,omitempty"`
	Target     string                `json:"target" yaml:"target"`
	Stages     []learnReadinessStage `json:"stages" yaml:"stages"`
	NextAction string                `json:"next_action,omitempty" yaml:"next_action,omitempty"`
}

var learnReadinessCmd = &cobra.Command{
	Use:   "readiness <topic>",
	Short: "Derive topic readiness through an explicit target (read-only)",
	Long: `Derive source, wiki, learning and material readiness for one exact concept.

TARGET is required on every invocation. A partial topic with multiple candidates
is reported as an ambiguity; select one candidate by its exact ID. Unrequested
downstream stages are reported as ` + "`not_planned`" + `. Source custody is shown
separately from the learning verdict. Each specialist verdict remains separate.

EXAMPLE — sdt learn readiness architecture/business-domain --target learning`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := getStringFlag(cmd, "target", false)
		if target != ctxTypeWiki && target != ctxFrontmatterLearning && target != readinessTargetMaterials {
			exitWithError(cmd, fmt.Errorf("--target is required and must be wiki, learning or materials"))
			return
		}
		bld := loadLearnWiki(cmd)
		pages := readinessCandidates(bld, args[0])
		if len(pages) > 1 {
			ids := make([]string, 0, len(pages))
			for _, page := range pages {
				ids = append(ids, page.ID)
			}
			exitWithError(cmd, fmt.Errorf("topic %q is ambiguous; choose an exact concept ID: %s", args[0], strings.Join(ids, ", ")))
			return
		}
		if len(pages) == 0 {
			exitWithError(cmd, fmt.Errorf("no wiki concept matches %q", args[0]))
			return
		}
		page := pages[0]
		report := deriveTopicReadiness(args[0], target, page)
		learnEmit(cmd, report, func() {
			var out strings.Builder
			fmt.Fprintf(&out, "%s — %s (target: %s)\n", page.ID, page.Title, target)
			for _, stage := range report.Stages {
				fmt.Fprintf(&out, "%-10s %s", stage.Name, stage.Status)
				if stage.Artifact != "" {
					fmt.Fprintf(&out, " — %s", stage.Artifact)
				}
				out.WriteByte('\n')
				for _, reason := range stage.Reasons {
					fmt.Fprintf(&out, "           - %s\n", reason)
				}
			}
			if report.NextAction != "" {
				fmt.Fprintf(&out, "next: %s\n", report.NextAction)
			}
			outputString(cmd, out.String())
		})
	},
}

func readinessCandidates(bld *contextwiki.Builder, topic string) []*contextwiki.Page {
	topic = strings.TrimSpace(topic)
	if page := bld.ByID[topic]; page != nil {
		return []*contextwiki.Page{page}
	}
	// Preserve all title/partial matches: an exact title does not silently
	// discard a second concept that the same topic text also finds.
	return learnPagesFor(bld, topic)
}

func deriveTopicReadiness(topic, target string, page *contextwiki.Page) learnTopicReadiness {
	report := learnTopicReadiness{Topic: topic, Concept: page.ID, Title: page.Title, Target: target}
	stages := []learnReadinessStage{readinessSourcesStage(page), readinessWikiStage(page)}
	if target == ctxFrontmatterLearning || target == readinessTargetMaterials {
		stages = append(stages, readinessLearningStage(page))
	} else {
		stages = append(stages, learnReadinessStage{Name: ctxFrontmatterLearning, Status: readinessStatusNotPlanned})
	}
	if target == readinessTargetMaterials {
		stages = append(stages, readinessMaterialsStage(page))
	} else {
		stages = append(stages, learnReadinessStage{Name: readinessTargetMaterials, Status: readinessStatusNotPlanned})
	}
	for _, stage := range stages {
		report.Stages = append(report.Stages, stage)
		if report.NextAction == "" && stage.Status != readinessStatusReady && stage.Status != materialReviewRecorded && stage.Status != readinessStatusNotPlanned {
			report.NextAction = stage.Name + " readiness for " + stage.Artifact
		}
	}
	return report
}

func readinessWikiStage(page *contextwiki.Page) learnReadinessStage {
	wiki := learnReadinessStage{Name: ctxTypeWiki, Status: readinessStatusReady, Artifact: page.Path}
	if page.Status != "active" {
		wiki.Status = taskStatusBlocked
		wiki.Reasons = append(wiki.Reasons, fmt.Sprintf("page status is %q, expected active", page.Status))
	}
	if wiki.Artifact == "" {
		wiki.Status = learnProvenanceUnknown
	}
	return wiki
}

func readinessLearningStage(page *contextwiki.Page) learnReadinessStage {
	check := learnReadinessOf(page)
	stage := learnReadinessStage{Name: ctxFrontmatterLearning, Status: readinessStatusReady, Artifact: check.Lesson, Reasons: append([]string(nil), check.Reasons...)}
	if check.Verdict != learnVerdictTeachable {
		stage.Status = taskStatusBlocked
	}
	if check.Lesson == "" {
		stage.Artifact = learnLessonPath(page.ID)
	}
	return stage
}

func readinessMaterialsStage(page *contextwiki.Page) learnReadinessStage {
	rows, err := materialStatusRows(page.ID, "", "")
	stage := learnReadinessStage{Name: readinessTargetMaterials, Status: readinessStatusReady, Artifact: page.ID}
	if err != nil {
		stage.Status = learnProvenanceUnknown
		if strings.Contains(err.Error(), "no material for") {
			stage.Status = readinessStatusAbsent
			stage.Artifact = page.ID
		}
		stage.Reasons = []string{err.Error()}
		return stage
	}
	if len(rows) == 0 {
		stage.Status = readinessStatusAbsent
		stage.Artifact = page.ID
		stage.Reasons = []string{"no material guide is linked to this concept"}
		return stage
	}
	paths := readinessMaterialPaths(page.ID)
	var artifacts []string
	for _, row := range rows {
		if path := paths[row.Lang+"\x00"+row.Syllabus]; path != "" {
			artifacts = append(artifacts, path)
		}
		status := readinessMaterialStatus(row)
		if status == renderStale || (status == "unreviewed" && stage.Status != renderStale) || (status == taskStatusBlocked && stage.Status == readinessStatusReady) {
			stage.Status = status
		}
		if row.Freshness != renderFresh || row.Review != materialReviewRecorded || row.Technical != materialTechnicalSourceCheck || row.Deck != renderFresh {
			stage.Reasons = append(stage.Reasons, fmt.Sprintf("%s: currency=%s review=%s technical=%s deck=%s", row.Lang, row.Freshness, row.Review, row.Technical, row.Deck))
		}
	}
	stage.Artifact = strings.Join(artifacts, ", ")
	return stage
}

func readinessMaterialPaths(conceptID string) map[string]string {
	matches, err := matchMaterialFiles(conceptID, "", "")
	if err != nil {
		return nil
	}
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	paths := map[string]string{}
	for _, match := range matches {
		path := match.Path
		if rel, err := filepath.Rel(root, path); err == nil {
			path = rel
		}
		paths[match.Header.Lang+"\x00"+strings.TrimPrefix(match.Header.Syllabus, "syllabi/")] = filepath.ToSlash(path)
	}
	return paths
}

func readinessMaterialStatus(row materialStatusRow) string {
	if row.Freshness != renderFresh || row.Deck != renderFresh {
		return renderStale
	}
	if row.Review != materialReviewRecorded {
		return "unreviewed"
	}
	if row.Technical != materialTechnicalSourceCheck {
		return taskStatusBlocked
	}
	return readinessStatusReady
}

func readinessSourcesStage(page *contextwiki.Page) learnReadinessStage {
	stage := learnReadinessStage{Name: ctxFrontmatterSources, Status: materialReviewRecorded}
	if len(page.Sources) == 0 {
		stage.Status = readinessStatusAbsent
		stage.Artifact = page.Path
		stage.Reasons = []string{"the concept has no source references recorded"}
		return stage
	}

	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	paths, err := walkMarkdownFiles(sdtSourcesDir)
	if err != nil {
		stage.Status = learnProvenanceUnknown
		stage.Artifact = strings.Join(page.Sources, ", ")
		stage.Reasons = []string{"source sheet custody could not be read: " + err.Error()}
		return stage
	}
	statusByExtract := map[string]string{}
	pathByExtract := map[string]string{}
	for _, path := range paths {
		data, readErr := os.ReadFile(path) //#nosec G304 -- paths are returned by the bounded source-lane walk
		if readErr != nil {
			continue
		}
		content := string(data)
		extract := filepath.ToSlash(parseFrontmatterField(content, "extract"))
		if extract == "" {
			continue
		}
		statusByExtract[extract] = parseFrontmatterField(content, ctxMapStatus)
		if rel, relErr := filepath.Rel(root, path); relErr == nil {
			pathByExtract[extract] = filepath.ToSlash(rel)
		} else {
			pathByExtract[extract] = filepath.ToSlash(path)
		}
	}

	var details []string
	for _, ref := range page.Sources {
		ref = filepath.ToSlash(strings.TrimSpace(ref))
		status, found := statusByExtract[ref]
		if !found {
			stage.Status = learnProvenanceUnknown
			details = append(details, ref+" (no source sheet matched)")
			continue
		}
		details = append(details, pathByExtract[ref]+" (custody: "+status+")")
		switch status {
		case ctxSourceStatusCandidate:
			if stage.Status != learnProvenanceUnknown {
				stage.Status = readinessStatusUnreviewed
			}
		case ctxSourceStatusCurated, ctxSourceStatusUsed, ctxSourceStatusRetired, ctxSourceStatusGone:
		case "":
			stage.Status = learnProvenanceUnknown
		default:
			stage.Status = learnProvenanceUnknown
		}
		if status == ctxSourceStatusRetired || status == ctxSourceStatusGone {
			stage.Reasons = append(stage.Reasons, fmt.Sprintf("%s is %s; custody is reported separately from learning readiness", ref, status))
		}
	}
	stage.Artifact = strings.Join(details, ", ")
	return stage
}

func init() {
	learnReadinessCmd.Flags().String("target", "", "Required readiness target: wiki|learning|materials")
	learnCmd.AddCommand(learnReadinessCmd)
}
