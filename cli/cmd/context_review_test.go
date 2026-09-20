package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendReviewBlockIdempotent(t *testing.T) {
	base := "---\nkind: tasks\nsummary: s\nstatus: completed\n---\n- [x] step\n"
	once := appendReviewBlock(base, "one finding — CONFIRMED (go test ok)")
	if !strings.Contains(once, "## Review") || !strings.Contains(once, "CONFIRMED") {
		t.Fatalf("expected a review block:\n%s", once)
	}
	twice := appendReviewBlock(once, "second body must not append")
	if twice != once {
		t.Errorf("appendReviewBlock must be idempotent:\n%s", twice)
	}
	if !hasReviewBlock(once) || hasReviewBlock(base) {
		t.Error("hasReviewBlock detection is wrong")
	}
}

func TestContextTaskReviewCompletesFile(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")
	path := filepath.Join("context", "tasks", "20260806-070001-p-phase-1.md")
	writeCtxDoc(t, path, "---\nkind: tasks\nsummary: s\nstatus: in-progress\ncreated: 2026-08-06T07:00:00Z\nupdated: 2026-08-06T07:00:00Z\n---\n\n- [x] step one\n")

	execute(t, contextTaskReviewCmd, nil, "--phase", "1", "--plan", "p.md", "--input", "gate green — CONFIRMED")
	content := mustReadFile(t, path)
	if !strings.Contains(content, "## Review") || !strings.Contains(content, "CONFIRMED") {
		t.Errorf("expected review block in task file:\n%s", content)
	}
	if !strings.Contains(content, "status: completed") {
		t.Errorf("expected the file completed after review with all items done:\n%s", content)
	}
}

func TestContextTaskReviewKeepsInProgressWhenUnfinished(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")
	path := filepath.Join("context", "tasks", "20260806-070002-p-phase-1.md")
	writeCtxDoc(t, path, "---\nkind: tasks\nsummary: s\nstatus: in-progress\n---\n\n- [ ] step one\n")

	execute(t, contextTaskReviewCmd, nil, "--phase", "1", "--plan", "p.md", "--input", "partial")
	content := mustReadFile(t, path)
	if strings.Contains(content, "status: completed") {
		t.Errorf("must not complete a file with unfinished items:\n%s", content)
	}
}

func TestLintCompletedTaskWithoutReview(t *testing.T) {
	setupContextProject(t)
	with := "---\nkind: tasks\nsummary: s\nstatus: completed\n---\n\n- [x] a\n\n## Review\n\nok — CONFIRMED\n"
	without := "---\nkind: tasks\nsummary: s\nstatus: completed\n---\n\n- [x] a\n"
	writeCtxDoc(t, "context/tasks/with.md", with)
	writeCtxDoc(t, "context/tasks/without.md", without)

	hasReviewIssue := func(path string) bool {
		for _, it := range lintDoc(path) {
			if strings.Contains(it.Message, "no `## Review`") {
				if it.Priority != ctxLintSuggestion {
					t.Errorf("expected SUGGESTION, got %s", it.Priority)
				}
				return true
			}
		}
		return false
	}
	if !hasReviewIssue("context/tasks/without.md") {
		t.Error("expected a review SUGGESTION for a completed task file without the block")
	}
	if hasReviewIssue("context/tasks/with.md") {
		t.Error("did not expect a review SUGGESTION when the block is present")
	}
	if ctxLintHint("completed task file has no `## Review` verify-step block (x)") == "" {
		t.Error("expected a curated hint for the review suggestion")
	}
}

func TestHasUnfinishedTaskItemIgnoresReviewBlock(t *testing.T) {
	content := "---\nkind: tasks\n---\n- [x] a\n\n## Review\n\n- not a checklist item\n"
	if hasUnfinishedTaskItem(content) {
		t.Error("the Review block must not count as an unfinished checklist item")
	}
	_ = os.Getenv("SDT") // keep os import used if trimmed later
}
