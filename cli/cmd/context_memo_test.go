package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/memo"
)

func TestContextMemoLifecycle(t *testing.T) {
	runInTempDir(t)

	execute(t, contextMemoAddRuleCmd, nil, "--id", "deps-update", "--summary", "Update deps", "--every-days", "14", "--last-done", "2026-09-20")
	execute(t, contextMemoAddMemoCmd, nil, "--id", "rotate-token", "--summary", "Rotate the token", "--due", "2026-10-15")

	out := execute(t, contextMemoListCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "deps-update") || !strings.Contains(string(out), "rotate-token") {
		t.Fatalf("list must show both entries, got:\n%s", out)
	}

	// 2026-10-04 is exactly last_done + 14 for the rule; the memo is still ahead.
	out = execute(t, contextMemoDueCmd, nil, "--today", "2026-10-04")
	if !strings.Contains(string(out), "deps-update") || strings.Contains(string(out), "rotate-token") {
		t.Fatalf("due 2026-10-04: want only deps-update, got:\n%s", out)
	}

	// By 2026-10-16 both are due.
	out = execute(t, contextMemoDueCmd, nil, "--today", "2026-10-16")
	if !strings.Contains(string(out), "deps-update") || !strings.Contains(string(out), "rotate-token") {
		t.Fatalf("due 2026-10-16: want both, got:\n%s", out)
	}

	// done on the rule resets the cadence from the recorded day.
	execute(t, contextMemoDoneCmd, nil, "deps-update", "--today", "2026-10-04")
	reg, err := memo.Load(".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, i, ok := reg.Find("deps-update"); !ok || reg.Rules[i].LastDone != "2026-10-04" {
		t.Fatalf("done must record last_done, got %+v", reg.Rules)
	}
	out = execute(t, contextMemoDueCmd, nil, "--today", "2026-10-04")
	if strings.Contains(string(out), "deps-update") {
		t.Fatalf("a just-done rule must not be due, got:\n%s", out)
	}

	execute(t, contextMemoRemoveCmd, nil, "rotate-token")
	out = execute(t, contextMemoListCmd, nil, "--format", "json")
	if strings.Contains(string(out), "rotate-token") {
		t.Fatalf("remove must drop the memo, got:\n%s", out)
	}
}

func TestContextMemoDueSilentWhenClean(t *testing.T) {
	runInTempDir(t)
	execute(t, contextMemoAddRuleCmd, nil, "--id", "fresh", "--summary", "s", "--every-days", "30", "--last-done", "2026-10-04")

	out := execute(t, contextMemoDueCmd, nil, "--today", "2026-10-04")
	if len(out) != 0 {
		t.Fatalf("text output must be empty when nothing is due, got %q", out)
	}
	out = execute(t, contextMemoDueCmd, nil, "--today", "2026-10-04", "--format", "json")
	if strings.TrimSpace(string(out)) != "[]" {
		t.Fatalf("json output must be an empty list, got %q", out)
	}
}

func TestContextMemoDueCheckExitCode(t *testing.T) {
	runInTempDir(t)
	execute(t, contextMemoAddMemoCmd, nil, "--id", "past", "--summary", "s", "--due", "2026-10-01")

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextMemoDueCmd, nil, "--today", "2026-10-04", "--check"))
	})
	execute(t, contextMemoDoneCmd, nil, "past")
	shouldExitWithCode(t, -1, func() string {
		return string(execute(t, contextMemoDueCmd, nil, "--today", "2026-10-04", "--check"))
	})
}

func TestContextMemoRejectsDuplicateAndInvalid(t *testing.T) {
	runInTempDir(t)
	execute(t, contextMemoAddMemoCmd, nil, "--id", "x", "--summary", "s", "--due", "2026-10-01")

	// A rule cannot reuse an existing id.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextMemoAddRuleCmd, nil, "--id", "x", "--summary", "s", "--every-days", "1"))
	})
	// A non-positive cadence is refused by the register validation.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextMemoAddRuleCmd, nil, "--id", "zero", "--summary", "s", "--every-days", "0"))
	})
	// done/remove of an unknown id is an error.
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextMemoRemoveCmd, nil, "nope")) })
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextMemoDoneCmd, nil, "nope")) })
	// A bad --today is an error.
	shouldExitWithCode(t, 1, func() string { return string(execute(t, contextMemoDueCmd, nil, "--today", "not-a-date")) })
}
