package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/ctxrel"
)

// planTaskFixture writes a plan and its task files under a temp context project,
// wired by the typed relations (the model's single derivation edge): the plan
// declares its analysis, each task file declares `plan_id`.
func planTaskFixture(t *testing.T, planStatus string, taskStatuses ...string) (string, []string) {
	t.Helper()
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: "+planStatus+"\n---\n")
	var tasks []string
	for i, status := range taskStatuses {
		path := "context/tasks/t" + string(rune('0'+i)) + ".md"
		writeCtxDoc(t, path, "---\nkind: tasks\nuid: uid-t"+string(rune('0'+i))+"\nplan_id: uid-p\nsummary: task\nstatus: "+status+"\n---\n")
		tasks = append(tasks, path)
	}
	return "context/plan/p.md", tasks
}

func TestLintPlanTaskAgreement(t *testing.T) {
	t.Run("agreement yields nothing", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "completed", "completed", "archived")
		edges := testEdges(t)
		issues := lintPlanTaskAgreement([]string{planPath}, []string{"context/tasks/t0.md", "context/tasks/t1.md"}, edges)
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("disagreement names the unfinished tasks", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "completed", "completed", "pending")
		edges := testEdges(t)
		issues := lintPlanTaskAgreement([]string{planPath}, []string{"context/tasks/t0.md", "context/tasks/t1.md"}, edges)
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "1 task(s) are not done") || !strings.Contains(issues[0].Message, "t1.md") {
			t.Fatalf("issues = %#v, want one disagreement naming t1.md", issues)
		}
		if issues[0].Priority != ctxLintWarning {
			t.Fatalf("priority = %q, want WARNING", issues[0].Priority)
		}
	})

	t.Run("an unsplit plan is not a defect", func(t *testing.T) {
		// The plan is the execution ledger: a completed plan with no task file
		// carries its own checklist, so there is nothing to report.
		planPath, _ := planTaskFixture(t, "completed")
		edges := testEdges(t)
		issues := lintPlanTaskAgreement([]string{planPath}, nil, edges)
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for an unsplit plan", issues)
		}
	})

	t.Run("non-completed plan yields nothing", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "active", "pending")
		edges := testEdges(t)
		issues := lintPlanTaskAgreement([]string{planPath}, []string{"context/tasks/t0.md"}, edges)
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a non-completed plan", issues)
		}
	})

	t.Run("non-task files in the scan are ignored", func(t *testing.T) {
		runInTempDir(t)
		writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\n---\n")
		writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: completed\n---\n")
		edges := testEdges(t)
		issues := lintPlanTaskAgreement([]string{"context/plan/p.md"}, []string{"context/plan/p.md"}, edges)
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none: a plan is never its own task file", issues)
		}
	})

	t.Run("a task file whose sources cite another plan is not counted", func(t *testing.T) {
		// The model forbids deriving from a `sources` citation: the task file
		// declares `plan_id` for plan/p.md and only that plan may claim it.
		runInTempDir(t)
		writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: uid-a\nsummary: a\n---\n")
		writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: plan\nstatus: completed\n---\n")
		writeCtxDoc(t, "context/plan/q.md", "---\nkind: plan\nuid: uid-q\nanalysis_id: uid-a\nsummary: other plan\nstatus: completed\n---\n")
		writeCtxDoc(t, "context/tasks/t0.md", "---\nkind: tasks\nuid: uid-t0\nplan_id: uid-p\nsummary: task\nstatus: pending\nsources:\n  - plan/q.md\n---\n")
		edges := testEdges(t)
		issues := lintPlanTaskAgreement(
			[]string{"context/plan/p.md", "context/plan/q.md"},
			[]string{"context/tasks/t0.md"},
			edges,
		)
		// plan/p.md owns the task (unfinished → one warning); plan/q.md is cited
		// but owns nothing, and an unsplit plan is not a defect.
		if len(issues) != 1 {
			t.Fatalf("issues = %#v, want only the unfinished task under plan/p.md", issues)
		}
		if !strings.Contains(issues[0].Message, "1 task(s) are not done") || !strings.Contains(issues[0].Path, "plan/p.md") {
			t.Errorf("issue[0] = %#v, want the unfinished task under plan/p.md", issues[0])
		}
	})
}

// testEdges resolves the typed relations of the current (temp) project.
func testEdges(t *testing.T) *ctxrel.Edges {
	t.Helper()
	edges, err := ctxrel.Load(sdtWorkDir)
	if err != nil {
		t.Fatalf("ctxrel.Load: %v", err)
	}
	return edges
}
