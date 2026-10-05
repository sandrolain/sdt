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

// reviewTaskFixture writes a plan and its task file and returns the task path.
func reviewTaskFixture(t *testing.T, status string) string {
	t.Helper()
	setupContextProject(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")
	path := filepath.Join("context", "tasks", "20260806-070010-p-phase-1.md")
	writeCtxDoc(t, path, "---\nkind: tasks\nsummary: s\nstatus: "+status+"\ncreated: 2026-08-06T07:00:00Z\nupdated: 2026-08-06T07:00:00Z\n---\n\n- [x] step one\n")
	return path
}

func TestReviewVerdictVocabularyIsOneSource(t *testing.T) {
	// ctxReviewVerdictHelp is derived from the closed set, so the help text and
	// the validation can never disagree.
	if got, want := ctxReviewVerdictHelp, strings.Join(ctxReviewVerdicts, " | "); got != want {
		t.Fatalf("help = %q, want %q", got, want)
	}
	for _, v := range ctxReviewVerdicts {
		if !validReviewVerdict(v) {
			t.Errorf("%q must be a valid verdict", v)
		}
	}
	if validReviewVerdict("MAYBE") {
		t.Error("MAYBE must not be a valid verdict")
	}
}

func TestContextTaskReviewRecordsPerFindingVerdicts(t *testing.T) {
	path := reviewTaskFixture(t, "in-progress")
	execute(t, contextTaskReviewCmd, nil, "--phase", "1", "--plan", "p.md",
		"--finding", "vet is clean", "--verdict", "CONFIRMED",
		"--finding", "coverage target met", "--verdict", "UNVERIFIED")
	content := mustReadFile(t, path)
	if !strings.Contains(content, "### Findings") {
		t.Fatalf("want a `### Findings` subsection:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] vet is clean — CONFIRMED <!-- c2 -->") ||
		!strings.Contains(content, "- [ ] coverage target met — UNVERIFIED <!-- c3 -->") {
		t.Fatalf("want one anchored item per finding with its trailing verdict:\n%s", content)
	}
}

func TestAppendFindingItemsIdentityIsStable(t *testing.T) {
	base := appendReviewBlock("- [x] a\n", "")
	finding := reviewFinding{Claim: "vet is clean", Verdict: "CONFIRMED"}
	once := appendFindingItems(base, []reviewFinding{finding})
	if strings.Count(once, "vet is clean") != 1 {
		t.Fatalf("want the finding recorded once:\n%s", once)
	}
	if again := appendFindingItems(once, []reviewFinding{finding}); again != once {
		t.Fatalf("re-running the same finding must be a no-op:\n%s", again)
	}
	// The same claim under a different verdict is a new finding with a new id:
	// a changed verdict is never silently overwritten.
	changed := appendFindingItems(once, []reviewFinding{{Claim: "vet is clean", Verdict: "DISPROVED"}})
	if strings.Count(changed, "vet is clean") != 2 {
		t.Fatalf("want the re-stated claim recorded again:\n%s", changed)
	}
	if !strings.Contains(changed, "vet is clean — CONFIRMED <!-- c2 -->") ||
		!strings.Contains(changed, "vet is clean — DISPROVED <!-- c3 -->") {
		t.Fatalf("want distinct ids for the two verdicts:\n%s", changed)
	}
}

func TestAppendFindingItemsLandAboveTheGateRecord(t *testing.T) {
	content := appendGateRecord("- [x] a\n", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), "",
		[]gateStepResult{{Step: "build", Status: "pass"}})
	got := appendFindingItems(content, []reviewFinding{{Claim: "vet is clean", Verdict: "CONFIRMED"}})
	if strings.Index(got, "### Findings") > strings.Index(got, "### Gate") {
		t.Fatalf("want the findings above the gate record:\n%s", got)
	}
}

func TestReviewFindingsRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"verdict without a finding", []string{"--verdict", "CONFIRMED"}, "--verdict given without --finding"},
		{"count mismatch", []string{"--finding", "a", "--finding", "b", "--verdict", "CONFIRMED"}, "pair by position"},
		{"invalid verdict", []string{"--finding", "a", "--verdict", "MAYBE"}, "is not a verify-step verdict"},
		{"empty finding", []string{"--finding", "   ", "--verdict", "CONFIRMED"}, "is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := contextTaskReviewCmd
			resetCmdFlags(cmd.Root())
			if err := cmd.Flags().Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			_, err := reviewFindings(cmd)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestLintFindingVerdict(t *testing.T) {
	t.Run("an item without a verdict token is a suggestion", func(t *testing.T) {
		content := "---\nkind: tasks\nstatus: completed\n---\n\n## Review\n\n### Findings\n\n- [ ] vet is clean <!-- c1 -->\n"
		issues := lintFindingVerdict("context/tasks/t.md", content)
		if len(issues) != 1 {
			t.Fatalf("issues = %#v, want one missing-verdict suggestion", issues)
		}
		if issues[0].Priority != ctxLintSuggestion || !strings.Contains(issues[0].Message, "no verdict token") {
			t.Fatalf("issue = %#v", issues[0])
		}
		if !strings.Contains(issues[0].Message, "vet is clean") {
			t.Fatalf("message = %q, want it to name the finding", issues[0].Message)
		}
		if ctxLintHint(issues[0].Message) == "" {
			t.Fatalf("no curated hint for %q", issues[0].Message)
		}
	})

	t.Run("a trailing verdict token is silent", func(t *testing.T) {
		content := "---\nkind: tasks\nstatus: completed\n---\n\n## Review\n\n### Findings\n\n- [ ] vet is clean — CONFIRMED <!-- c1 -->\n"
		if issues := lintFindingVerdict("context/tasks/t.md", content); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("a verdict word mid-sentence does not count", func(t *testing.T) {
		// The verdict is the trailing token by construction, so prose that
		// mentions one is still missing a verdict.
		content := "---\nkind: tasks\nstatus: completed\n---\n\n## Review\n\n### Findings\n\n- [ ] UNVERIFIED claims remain <!-- c1 -->\n"
		if issues := lintFindingVerdict("context/tasks/t.md", content); len(issues) != 1 {
			t.Fatalf("issues = %#v, want one: the verdict must be the trailing token", issues)
		}
	})

	t.Run("an open file is not asked for verdicts", func(t *testing.T) {
		content := "---\nkind: tasks\nstatus: in-progress\n---\n\n## Review\n\n### Findings\n\n- [ ] vet is clean <!-- c1 -->\n"
		if issues := lintFindingVerdict("context/tasks/t.md", content); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none for a file still open", issues)
		}
	})
}

func TestHasUnfinishedTaskItemIgnoresRecordSections(t *testing.T) {
	// `## Deviations` items are open by design until a reviewer closes them and
	// `### Findings` items carry verdicts; neither is a work item, so neither may
	// keep a task file from completing.
	content := "---\nkind: tasks\n---\n\n## Phase 1\n\n- [x] a\n\n## Deviations\n\n- [ ] fix — moved\n\n## Review\n\n### Findings\n\n- [ ] claim — UNVERIFIED\n"
	if hasUnfinishedTaskItem(content) {
		t.Fatal("record sections must not count as unfinished work")
	}
	// A genuinely open phase item still counts.
	if !hasUnfinishedTaskItem(strings.Replace(content, "- [x] a", "- [ ] a", 1)) {
		t.Fatal("an open phase item must still count")
	}
}
