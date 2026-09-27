package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func listHas(content, field, value string) bool {
	for _, v := range parseFrontmatterList(content, field) {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}

func TestContextNewPlanStampsAnalysisRelation(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	analysisPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil, "--type", "analysis", "--title", "source analysis", "--objective", "rel")))
	analysisUID := parseFrontmatterField(mustReadFile(t, analysisPath), "uid")

	planPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil, "--type", "plan", "--title", "rel plan", "--source", "analysis/"+filepath.Base(analysisPath))))
	planContent := mustReadFile(t, planPath)
	planUID := parseFrontmatterField(planContent, "uid")

	if got := parseFrontmatterField(planContent, "analysis_id"); got != analysisUID {
		t.Errorf("plan analysis_id = %q, want %q", got, analysisUID)
	}
	if !listHas(mustReadFile(t, analysisPath), "plans_ids", planUID) {
		t.Errorf("analysis plans_ids missing plan uid %s:\n%s", planUID, mustReadFile(t, analysisPath))
	}
}

func TestContextTaskStampsPlanRelation(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	planPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil, "--type", "plan", "--title", "rel plan")))
	planUID := parseFrontmatterField(mustReadFile(t, planPath), "uid")

	execute(t, contextTaskAddCmd, nil, "--phase", "1", "--plan", filepath.Base(planPath), "step one")
	taskPath := filepath.Join("context", "tasks", "20260806-070000-rel-plan-phase-1.md")
	taskContent := mustReadFile(t, taskPath)
	taskUID := parseFrontmatterField(taskContent, "uid")

	if got := parseFrontmatterField(taskContent, "plan_id"); got != planUID {
		t.Errorf("task plan_id = %q, want %q", got, planUID)
	}
	if !listHas(mustReadFile(t, planPath), "tasks_ids", taskUID) {
		t.Errorf("plan tasks_ids missing task uid %s:\n%s", taskUID, mustReadFile(t, planPath))
	}
}

func TestLintParentRelations(t *testing.T) {
	runInTempDir(t)
	analysisUID, planUID, orphanUID := testUIDv7(1), testUIDv7(2), testUIDv7(9)

	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: "+analysisUID+"\nsummary: a\nlinks: none\nplans_ids:\n  - "+planUID+"\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: "+planUID+"\nsummary: p\nanalysis_id: "+analysisUID+"\nsources:\n  - analysis/a.md\n---\n")
	writeCtxDoc(t, "context/plan/orphan.md", "---\nkind: plan\nuid: "+testUIDv7(3)+"\nsummary: orphan\nanalysis_id: "+orphanUID+"\nsources:\n  - analysis/a.md\n---\n")
	writeCtxDoc(t, "context/tasks/t.md", "---\nkind: tasks\nuid: "+testUIDv7(4)+"\nsummary: t\nsources:\n  - plan/p.md\n---\n")

	issues := lintParentRelations([]string{
		"context/analysis/a.md", "context/plan/p.md", "context/plan/orphan.md", "context/tasks/t.md",
	})
	var warns, suggests int
	for _, it := range issues {
		switch it.Priority {
		case ctxLintWarning:
			warns++
			if !strings.Contains(it.Message, "does not resolve") {
				t.Errorf("unexpected WARNING: %s", it.Message)
			}
		case ctxLintSuggestion:
			suggests++
			if !strings.Contains(it.Message, "has no `plan_id`") {
				t.Errorf("unexpected SUGGESTION: %s", it.Message)
			}
		}
	}
	if warns != 1 {
		t.Errorf("warnings = %d, want 1 (orphan analysis_id)", warns)
	}
	if suggests != 1 {
		t.Errorf("suggestions = %d, want 1 (task missing plan_id)", suggests)
	}
}
