package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// deviationFixture writes a plan and a task file with one open item and returns
// the task path.
func deviationFixture(t *testing.T, status string) string {
	t.Helper()
	setupContextProject(t)
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")
	path := filepath.Join("context", "tasks", "20260806-070020-p-phase-1.md")
	writeCtxDoc(t, path, "---\nkind: tasks\nsummary: s\nstatus: "+status+"\ncreated: 2026-08-06T07:00:00Z\nupdated: 2026-08-06T07:00:00Z\n---\n\n## Phase 1\n\n- [ ] step one <!-- c1 -->\n\n## Review\n\nA first verify pass.\n")
	return path
}

func TestDeviationKindsAreClosed(t *testing.T) {
	for _, k := range ctxDeviationKinds {
		if !validDeviationKind(k) {
			t.Errorf("%q must be a valid deviation kind", k)
		}
	}
	if validDeviationKind("maybe") || validDeviationKind("") {
		t.Error("only the declared kinds are valid; an empty kind is not")
	}
	if ctxDeviationDefaultKind != "stop-and-ask" {
		t.Fatalf("default kind = %q, want stop-and-ask", ctxDeviationDefaultKind)
	}
}

func TestParseDeviationBody(t *testing.T) {
	kind, text, reason := parseDeviationBody("fix — the phase order was wrong (reason: measured at HEAD)")
	if kind != "fix" || text != "the phase order was wrong" || reason != "measured at HEAD" {
		t.Fatalf("got (%q, %q, %q)", kind, text, reason)
	}
	if kind, _, _ := parseDeviationBody("no separator here"); kind != "" {
		t.Fatalf("kind = %q, want empty for a body with no separator", kind)
	}
}

func TestContextTaskDeviationRoundTrip(t *testing.T) {
	path := deviationFixture(t, "in-progress")
	execute(t, contextDeviationAddCmd, nil, "--phase", "1", "--plan", "p.md",
		"--kind", "fix", "--reason", "the plan's phase order was wrong",
		"phase 3 check moved to phase 5")
	content := mustReadFile(t, path)
	if !strings.Contains(content, "## Deviations") {
		t.Fatalf("want a `## Deviations` section:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] fix — phase 3 check moved to phase 5 (reason: the plan's phase order was wrong)") {
		t.Fatalf("want the deviation item with its kind, text and reason:\n%s", content)
	}
	// The section sits before `## Review`, so the review stays the last word.
	if strings.Index(content, "## Deviations") > strings.Index(content, "## Review") {
		t.Fatalf("want `## Deviations` before `## Review`:\n%s", content)
	}
	out := string(execute(t, contextDeviationListCmd, nil, "--phase", "1", "--plan", "p.md"))
	if !strings.Contains(out, "fix: phase 3 check moved to phase 5") {
		t.Fatalf("list output = %q, want the recorded deviation", out)
	}
}

func TestContextTaskDeviationDefaultKindAsksTheUser(t *testing.T) {
	path := deviationFixture(t, "in-progress")
	execute(t, contextDeviationAddCmd, nil, "--phase", "1", "--plan", "p.md", "no kind was stated")
	if got := mustReadFile(t, path); !strings.Contains(got, "- [ ] "+ctxDeviationDefaultKind+" — no kind was stated") {
		t.Fatalf("want the default kind recorded:\n%s", got)
	}
}

func TestContextTaskDeviationRefusesUnknownKind(t *testing.T) {
	path := deviationFixture(t, "in-progress")
	before := mustReadFile(t, path)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextDeviationAddCmd, nil, "--phase", "1", "--plan", "p.md", "--kind", "maybe", "x"))
	})
	if after := mustReadFile(t, path); after != before {
		t.Fatalf("an unknown kind must not write anything:\n%s", after)
	}
}

func TestContextTaskDeviationMarksTheLinkedItem(t *testing.T) {
	t.Run("a stop-and-ask deviation blocks the item", func(t *testing.T) {
		path := deviationFixture(t, "in-progress")
		execute(t, contextDeviationAddCmd, nil, "--phase", "1", "--plan", "p.md",
			"--id", "c1", "--reason", "waiting on the answer", "blocked on the user")
		got := mustReadFile(t, path)
		if !strings.Contains(got, "- [!] step one (blocked: waiting on the answer) <!-- c1 -->") {
			t.Fatalf("want the linked item blocked and its text kept:\n%s", got)
		}
	})

	t.Run("another kind marks the item in progress", func(t *testing.T) {
		path := deviationFixture(t, "in-progress")
		execute(t, contextDeviationAddCmd, nil, "--phase", "1", "--plan", "p.md",
			"--id", "c1", "--kind", "fix", "the order changed")
		got := mustReadFile(t, path)
		if !strings.Contains(got, "- [~] step one <!-- c1 -->") {
			t.Fatalf("want the linked item marked in progress with its text kept:\n%s", got)
		}
	})

	t.Run("an unknown id is refused before writing", func(t *testing.T) {
		path := deviationFixture(t, "in-progress")
		before := mustReadFile(t, path)
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, contextDeviationAddCmd, nil, "--phase", "1", "--plan", "p.md", "--id", "c99", "x"))
		})
		if after := mustReadFile(t, path); after != before {
			t.Fatalf("an unknown id must not write anything:\n%s", after)
		}
	})
}

func TestLintDeviation(t *testing.T) {
	open := "---\nkind: tasks\nstatus: completed\n---\n\n## Phase 1\n\n- [x] a <!-- c1 -->\n\n## Deviations\n\n- [ ] fix — moved <!-- c2 -->\n"

	t.Run("an open deviation in a completed file is a warning", func(t *testing.T) {
		issues := lintDeviation("context/tasks/t.md", open)
		if len(issues) != 1 || issues[0].Priority != ctxLintWarning {
			t.Fatalf("issues = %#v, want one WARNING", issues)
		}
		if !strings.Contains(issues[0].Message, "open deviation (fix)") || !strings.Contains(issues[0].Message, "moved") {
			t.Fatalf("message = %q, want the kind and the text", issues[0].Message)
		}
		if ctxLintHint(issues[0].Message) == "" {
			t.Fatalf("no curated hint for %q", issues[0].Message)
		}
	})

	t.Run("a closed deviation is silent", func(t *testing.T) {
		closed := strings.Replace(open, "- [ ] fix —", "- [x] fix —", 1)
		if issues := lintDeviation("context/tasks/t.md", closed); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})

	t.Run("an open deviation in a live file is not reported", func(t *testing.T) {
		live := strings.Replace(open, "status: completed", "status: in-progress", 1)
		if issues := lintDeviation("context/tasks/t.md", live); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none while the work is still open", issues)
		}
	})

	t.Run("an unknown kind is a warning in any file", func(t *testing.T) {
		for _, status := range []string{"completed", "in-progress"} {
			content := strings.Replace(strings.Replace(open, "- [ ] fix —", "- [ ] maybe —", 1), "status: completed", "status: "+status, 1)
			issues := lintDeviation("context/tasks/t.md", content)
			if len(issues) != 1 || !strings.Contains(issues[0].Message, "unknown kind maybe") {
				t.Fatalf("status %s: issues = %#v, want the unknown-kind warning", status, issues)
			}
		}
	})

	t.Run("an item without a separator is an unknown kind", func(t *testing.T) {
		content := strings.Replace(open, "- [ ] fix — moved", "- [ ] no separator", 1)
		issues := lintDeviation("context/tasks/t.md", content)
		if len(issues) != 1 || !strings.Contains(issues[0].Message, "unknown kind") {
			t.Fatalf("issues = %#v, want the malformed line reported", issues)
		}
	})

	t.Run("a file without the section declares nothing", func(t *testing.T) {
		if issues := lintDeviation("context/tasks/t.md", "---\nkind: tasks\nstatus: completed\n---\n\n- [x] a\n"); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none", issues)
		}
	})
}

func TestAppendDeviationItemPreservesAnExistingSection(t *testing.T) {
	content := "---\nkind: tasks\n---\n\n## Deviations\n\n- [x] fix — an older one <!-- c1 -->\n\n## Review\n\nok\n"
	got := appendDeviationItem(content, "add", "a second one", "")
	if strings.Count(got, "## Deviations") != 1 {
		t.Fatalf("want one section, got:\n%s", got)
	}
	if !strings.Contains(got, "an older one") || !strings.Contains(got, "add — a second one") {
		t.Fatalf("want both items kept:\n%s", got)
	}
	if strings.Index(got, "add — a second one") > strings.Index(got, "## Review") {
		t.Fatalf("want the new item inside the section, before `## Review`:\n%s", got)
	}
}
