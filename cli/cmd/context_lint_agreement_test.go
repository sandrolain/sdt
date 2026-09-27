package cmd

import (
	"strings"
	"testing"
)

func TestTaskPlanRefMirrorsViewer(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "plain plan source",
			content: "---\nkind: tasks\nsummary: t\nsources:\n  - plan/p.md\n---\n",
			want:    "context/plan/p.md",
		},
		{
			name:    "context-prefixed source",
			content: "---\nkind: tasks\nsummary: t\nsources:\n  - context/plan/p.md\n---\n",
			want:    "context/plan/p.md",
		},
		{
			name:    "first plan ref wins",
			content: "---\nkind: tasks\nsummary: t\nsources:\n  - analysis/a.md\n  - plan/p.md\n---\n",
			want:    "context/plan/p.md",
		},
		{
			name:    "non-plan ref is not matched",
			content: "---\nkind: tasks\nsummary: t\nsources:\n  - notes/n.md\n---\n",
			want:    "",
		},
		{
			name:    "no sources",
			content: "---\nkind: tasks\nsummary: t\n---\n",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskPlanRef(tt.content); got != tt.want {
				t.Fatalf("taskPlanRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

// planTaskFixture writes a plan and its task files under a temp context project.
func planTaskFixture(t *testing.T, planStatus string, taskStatuses ...string) (string, []string) {
	t.Helper()
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: plan\nstatus: "+planStatus+"\n---\n")
	var tasks []string
	for i, status := range taskStatuses {
		path := "context/tasks/t" + string(rune('0'+i)) + ".md"
		writeCtxDoc(t, path, "---\nkind: tasks\nsummary: task\nstatus: "+status+"\nsources:\n  - plan/p.md\n---\n")
		tasks = append(tasks, path)
	}
	return "context/plan/p.md", tasks
}

func TestLintPlanTaskAgreement(t *testing.T) {
	t.Run("agreement yields nothing", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "completed", "completed", "archived")
		issues := lintPlanTaskAgreement([]string{planPath}, []string{"context/tasks/t0.md", "context/tasks/t1.md"})
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("disagreement names the unfinished tasks", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "completed", "completed", "pending")
		issues := lintPlanTaskAgreement([]string{planPath}, []string{"context/tasks/t0.md", "context/tasks/t1.md"})
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "1 task(s) are not done") || !strings.Contains(issues[0].Message, "t1.md") {
			t.Fatalf("issues = %#v, want one disagreement naming t1.md", issues)
		}
		if issues[0].Priority != ctxLintWarning {
			t.Fatalf("priority = %q, want WARNING", issues[0].Priority)
		}
	})

	t.Run("unresolved reference is distinct from unfinished", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "completed")
		issues := lintPlanTaskAgreement([]string{planPath}, nil)
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "no task file references it") {
			t.Fatalf("issues = %#v, want one unresolved-reference warning", issues)
		}
	})

	t.Run("non-completed plan yields nothing", func(t *testing.T) {
		planPath, _ := planTaskFixture(t, "active", "pending")
		issues := lintPlanTaskAgreement([]string{planPath}, []string{"context/tasks/t0.md"})
		if len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a non-completed plan", issues)
		}
	})

	t.Run("non-task files in the scan are ignored", func(t *testing.T) {
		runInTempDir(t)
		writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: plan\nstatus: completed\nsources:\n  - plan/other.md\n---\n")
		issues := lintPlanTaskAgreement([]string{"context/plan/p.md"}, []string{"context/plan/p.md"})
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "no task file references it") {
			t.Fatalf("issues = %#v, want the unresolved warning, not the plan counted as its own task", issues)
		}
	})
}
