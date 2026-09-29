package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestContextListStatusAndWhere(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/a.md", "---\nkind: plan\nsummary: s\nstatus: active\nobjective: grp\ncreated: \"2026-08-01T00:00:00Z\"\n---\nbody\n")
	writeCtxDoc(t, "context/plan/b.md", "---\nkind: plan\nsummary: s\nstatus: completed\nobjective: other\ncreated: \"2026-08-02T00:00:00Z\"\n---\nbody\n")

	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--status", "active")); !strings.Contains(out, "a.md") || strings.Contains(out, "b.md") {
		t.Errorf("status filter wrong:\n%s", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--where", "objective=grp")); !strings.Contains(out, "a.md") || strings.Contains(out, "b.md") {
		t.Errorf("--where scalar filter wrong:\n%s", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--where", "objective!=grp")); strings.Contains(out, "a.md") || !strings.Contains(out, "b.md") {
		t.Errorf("--where negated filter wrong:\n%s", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--where", "kind=plan")); !strings.Contains(out, "a.md") || !strings.Contains(out, "b.md") {
		t.Errorf("--where generic key should match both:\n%s", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--where", "missing=x")); strings.TrimSpace(out) != "" {
		t.Errorf("absent key must match nothing:\n%s", out)
	}
}

func TestContextListDateFilters(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
	writeCtxDoc(t, "context/plan/recent.md", "---\nkind: plan\nsummary: s\nstatus: active\ncreated: \"2026-09-27T00:00:00Z\"\nupdated: \"2026-09-28T06:00:00Z\"\n---\nbody\n")
	writeCtxDoc(t, "context/plan/old.md", "---\nkind: plan\nsummary: s\nstatus: active\ncreated: \"2026-08-01T00:00:00Z\"\nupdated: \"2026-08-01T00:00:00Z\"\n---\nbody\n")

	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--last", "2d")); !strings.Contains(out, "recent.md") || strings.Contains(out, "old.md") {
		t.Errorf("--last window wrong:\n%s", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--after", "2026-09-01")); !strings.Contains(out, "recent.md") || strings.Contains(out, "old.md") {
		t.Errorf("--after wrong:\n%s", out)
	}
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--before", "2026-09-01")); strings.Contains(out, "recent.md") || !strings.Contains(out, "old.md") {
		t.Errorf("--before wrong:\n%s", out)
	}
	// --date updated selects the updated timestamp as reference.
	if out := string(execute(t, contextListCmd, nil, "--type", "plan", "--date", "updated", "--after", "2026-09-01")); !strings.Contains(out, "recent.md") || strings.Contains(out, "old.md") {
		t.Errorf("--date updated wrong:\n%s", out)
	}
}

func TestContextListQueryAliases(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/a.md", "---\nkind: plan\nsummary: s\nstatus: active\ncreated: \"2026-09-01T00:00:00Z\"\nagent: opencode\n---\nbody\n")
	writeCtxDoc(t, "context/plan/b.md", "---\nkind: plan\nsummary: s\nstatus: active\ncreated: \"2026-09-02T00:00:00Z\"\nagent: human\n---\nbody\n")

	after := string(execute(t, contextListCmd, nil, "--type", "plan", "--after", "2026-09-01T12:00:00Z"))
	since := string(execute(t, contextListCmd, nil, "--type", "plan", "--since", "2026-09-01T12:00:00Z"))
	if after != since {
		t.Errorf("--since must alias --after:\n%s\nvs\n%s", after, since)
	}
	agentFlag := string(execute(t, contextListCmd, nil, "--type", "plan", "--agent", "opencode"))
	agentWhere := string(execute(t, contextListCmd, nil, "--type", "plan", "--where", "agent=opencode"))
	if agentFlag != agentWhere {
		t.Errorf("--agent must alias --where agent=:\n%s\nvs\n%s", agentFlag, agentWhere)
	}
}

func TestContextSearchListParity(t *testing.T) {
	runInTempDir(t)
	ctxSearchIndex = nil
	writeCtxDoc(t, "context/analysis/active.md", "---\nkind: analysis\nsummary: alpha s\nstatus: active\ncreated: \"2026-09-10T00:00:00Z\"\n---\nalpha body\n")
	writeCtxDoc(t, "context/analysis/done.md", "---\nkind: analysis\nsummary: alpha s\nstatus: completed\ncreated: \"2026-09-10T00:00:00Z\"\n---\nalpha body\n")

	listOut := string(execute(t, contextListCmd, nil, "--type", "analysis", "--status", "active"))
	searchOut := string(execute(t, contextSearchCmd, nil, "alpha", "--type", "analysis", "--status", "active"))
	if !strings.Contains(listOut, "active.md") || strings.Contains(listOut, "done.md") {
		t.Errorf("list filter wrong:\n%s", listOut)
	}
	if !strings.Contains(searchOut, "active.md") || strings.Contains(searchOut, "done.md") {
		t.Errorf("search filter must agree with list:\n%s", searchOut)
	}
}

func TestContextSearchWhereAndWindow(t *testing.T) {
	runInTempDir(t)
	ctxSearchIndex = nil
	stubContextNow(t, time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
	writeCtxDoc(t, "context/analysis/recent.md", "---\nkind: analysis\nsummary: alpha s\nstatus: active\ncreated: \"2026-09-28T06:00:00Z\"\nnote_type: dead-end\narchived: \"true\"\ntopics:\n  - cli\ncategories:\n  - improvement\n---\nalpha body\n")
	writeCtxDoc(t, "context/analysis/old.md", "---\nkind: analysis\nsummary: alpha s\nstatus: active\ncreated: \"2026-09-20T00:00:00Z\"\n---\nalpha body\n")

	cases := []struct {
		args []string
		want string
		gone string
	}{
		{[]string{"--where", "topics=cli"}, "recent.md", "old.md"},
		{[]string{"--where", "note_type=dead-end"}, "recent.md", "old.md"},
		{[]string{"--where", "categories=improvement"}, "recent.md", "old.md"},
		{[]string{"--where", "archived=true"}, "recent.md", "old.md"},
		{[]string{"--where", "note_type=other"}, "", "recent.md"},
		{[]string{"--last", "12h"}, "recent.md", "old.md"},
	}
	for _, c := range cases {
		args := append([]string{"alpha", "--type", "analysis"}, c.args...)
		out := string(execute(t, contextSearchCmd, nil, args...))
		if c.want != "" && !strings.Contains(out, c.want) {
			t.Errorf("%v: expected %s in:\n%s", c.args, c.want, out)
		}
		if c.gone != "" && strings.Contains(out, c.gone) {
			t.Errorf("%v: did not expect %s in:\n%s", c.args, c.gone, out)
		}
	}
}

func TestContextListStatusVocabularyValidation(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: s\nstatus: active\n---\nbody\n")
	writeCtxDoc(t, "context/analysis/b.md", "---\nkind: analysis\nsummary: s\nstatus: postponed\n---\nbody\n")

	// Every value of the kind's closed vocabulary is accepted, and the filter
	// really selects on it (analysis 20260925-195434 risks: a postponed option
	// must be one command away, and validation must not be a silent empty list).
	for _, value := range []string{"active", "draft", "completed", "postponed", "archived"} {
		out := string(execute(t, contextListCmd, nil, "--type", "analysis", "--status", value))
		switch value {
		case "active":
			if !strings.Contains(out, "a.md") || strings.Contains(out, "b.md") {
				t.Errorf("--status active wrong:\n%s", out)
			}
		case "postponed":
			if !strings.Contains(out, "b.md") || strings.Contains(out, "a.md") {
				t.Errorf("--status postponed wrong:\n%s", out)
			}
		default:
			if strings.TrimSpace(out) != "" {
				t.Errorf("--status %s should match no document, got:\n%s", value, out)
			}
		}
	}

	// Out-of-vocabulary value for the resolved kind is an error naming the
	// vocabulary, not an empty result. exitWithError logs through the root
	// command's err writer, installed by the PersistentPreRun of execute().
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextListCmd, nil, "--type", "analysis", "--status", "bogus"))
	})
	if out := errBuf.String(); !strings.Contains(out, "invalid status") || !strings.Contains(out, "postponed") {
		t.Errorf("out-of-vocabulary error should name the vocabulary, got:\n%s", out)
	}

	// A kind with no status field is an explicit error too.
	for _, typ := range []string{"notes", "worklog"} {
		errBuf.Reset()
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, contextListCmd, nil, "--type", typ, "--status", "active"))
		})
		if out := errBuf.String(); !strings.Contains(out, "no status vocabulary") {
			t.Errorf("--type %s --status should report no vocabulary, got:\n%s", typ, out)
		}
	}
}

func TestContextListQueryErrors(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/plan/a.md", "---\nkind: plan\nsummary: s\nstatus: active\n---\nbody\n")
	for _, args := range [][]string{
		{"--where", "nope"},
		{"--last", "1d", "--after", "2026-01-01"},
		{"--date", "foo"},
		{"--last", "1mo"},
		{"--after", "not-a-date"},
	} {
		args := append([]string{"--type", "plan"}, args...)
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, contextListCmd, nil, args...))
		})
	}
}
