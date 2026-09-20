package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// fixtureDoc writes a minimal frontmatter-bearing work file into the temp
// project. status="" skips the status line; kind analysis always gets updated.

func fixtureDoc(t *testing.T, path, kind, status, extra string) {
	t.Helper()
	content := "---\nkind: " + kind + "\nsummary: \"fixture\"\n"
	if status != "" {
		content += "status: " + status + "\n"
	}
	content += "created: \"2026-09-20T10:00:00Z\"\n"
	if kind == "analysis" {
		content += "updated: \"2026-09-20T10:00:00Z\"\n"
	}
	content += extra
	content += "---\n\n## Body fixture\n"
	writeTestFile(t, path, content)
}

func TestResolveContextDocPath(t *testing.T) {
	runInTempDir(t)
	fixtureDoc(t, "context/analysis/20260920-130000-backend.md", "analysis", "active", "")
	fixtureDoc(t, "context/decisions/0001-auth.md", "decision", "proposed", "")
	fixtureDoc(t, "context/tasks/20260920-100000-myplan-phase-1.md", "tasks", "pending", "")
	writeTestFile(t, "context/tmp/scratch.txt", "tmp\n")

	cases := []struct {
		name, ref string
		kind      string
		path      string
	}{
		{"relative", "context/analysis/20260920-130000-backend.md", "analysis", "context/analysis/20260920-130000-backend.md"},
		{"no extension", "context/analysis/20260920-130000-backend", "analysis", "context/analysis/20260920-130000-backend.md"},
		{"decision number file", "context/decisions/0001-auth.md", "decision", "context/decisions/0001-auth.md"},
		{"tasks file", "context/tasks/20260920-100000-myplan-phase-1.md", "tasks", "context/tasks/20260920-100000-myplan-phase-1.md"},
		{"tmp without extension", "context/tmp/scratch.txt", "tmp", "context/tmp/scratch.txt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc, err := resolveContextDocPath(c.ref)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Type.kind != c.kind {
				t.Errorf("type = %q, want %q", doc.Type.kind, c.kind)
			}
			if doc.Path != c.path {
				t.Errorf("path = %q, want %q", doc.Path, c.path)
			}
		})
	}

	t.Run("absolute path", func(t *testing.T) {
		abs, err := filepath.Abs("context/analysis/20260920-130000-backend.md")
		if err != nil {
			t.Fatal(err)
		}
		doc, err := resolveContextDocPath(abs)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Path != "context/analysis/20260920-130000-backend.md" {
			t.Errorf("path = %q", doc.Path)
		}
	})

	for _, ref := range []string{"context/plan/nope.md", "../outside.md", "context/nonexistent/x.md"} {
		if _, err := resolveContextDocPath(ref); err == nil {
			t.Errorf("resolveContextDocPath(%q) must error", ref)
		}
	}
}

func TestResolveContextDocIdentity(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC))
	fixtureDoc(t, "context/analysis/20260920-130000-backend.md", "analysis", "active", "")
	fixtureDoc(t, "context/questions/20260920-100000-open-api.md", "questions", "active", "")
	fixtureDoc(t, "context/architecture/config.md", "architecture", "draft", "")
	fixtureDoc(t, "context/decisions/0001-auth.md", "decision", "proposed", "")
	task := taskFileFor("1", "myplan")
	fixtureDoc(t, task, "tasks", "pending", "")

	t.Run("dated unique slug", func(t *testing.T) {
		doc, err := resolveContextDocIdentity(newTestCmd("--type", "analysis", "--slug", "backend"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Type.kind != ctxTypeAnalysis || doc.Path != "context/analysis/20260920-130000-backend.md" {
			t.Errorf("doc = %+v", doc)
		}
	})

	t.Run("dated slug collision", func(t *testing.T) {
		// a second file matching *-backend.md makes the resolution ambiguous
		fixtureDoc(t, "context/analysis/20260920-120000-backend.md", "analysis", "active", "")
		if _, err := resolveContextDocIdentity(newTestCmd("--type", "analysis", "--slug", "backend")); err == nil {
			t.Error("ambiguous slug must error")
		}
	})

	t.Run("no match", func(t *testing.T) {
		if _, err := resolveContextDocIdentity(newTestCmd("--type", "analysis", "--slug", "zzz")); err == nil {
			t.Error("missing slug must error")
		}
	})

	t.Run("bare", func(t *testing.T) {
		doc, err := resolveContextDocIdentity(newTestCmd("--type", "architecture", "--slug", "config"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Path != "context/architecture/config.md" {
			t.Errorf("path = %q", doc.Path)
		}
	})

	t.Run("decision number", func(t *testing.T) {
		doc, err := resolveContextDocIdentity(newTestCmd("--type", "decision", "--number", "0001", "--slug", "auth"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Path != "context/decisions/0001-auth.md" {
			t.Errorf("path = %q", doc.Path)
		}
		if _, err := resolveContextDocIdentity(newTestCmd("--type", "decision", "--slug", "auth")); err == nil {
			t.Error("decision without --number must error")
		}
	})

	t.Run("tasks phase plan", func(t *testing.T) {
		doc, err := resolveContextDocIdentity(newTestCmd("--type", "tasks", "--phase", "1", "--plan", "myplan"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Path != task {
			t.Errorf("path = %q, want %q", doc.Path, task)
		}
	})
}

// newTestCmd builds a cobra command with identity flags parsed from args, so
// resolveContextDocIdentity can be exercised without the status subcommands.
func newTestCmd(args ...string) *cobra.Command {
	c := &cobra.Command{Use: "probe"}
	c.Flags().String("type", "", "")
	c.Flags().String("slug", "", "")
	c.Flags().String("number", "", "")
	c.Flags().String("phase", "", "")
	c.Flags().String("plan", "", "")
	if err := c.Flags().Parse(args); err != nil {
		panic(err)
	}
	return c
}

func TestStatusGet(t *testing.T) {
	runInTempDir(t)
	fixtureDoc(t, "context/analysis/20260920-130000-backend.md", "analysis", "active", "")

	out := strings.TrimSpace(string(execute(t, contextStatusGetCmd, nil, "context/analysis/20260920-130000-backend.md")))
	if out != "active" {
		t.Errorf("text status = %q, want active", out)
	}

	out = string(execute(t, contextStatusGetCmd, nil, "--type", "analysis", "--slug", "backend", "--format", "json"))
	var res ctxStatusResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if res.Status != "active" || res.Type != "analysis" || res.Path != "context/analysis/20260920-130000-backend.md" {
		t.Errorf("json result = %+v", res)
	}

	out = string(execute(t, contextStatusGetCmd, nil, "--type", "analysis", "--slug", "backend", "--format", "yaml"))
	if !strings.Contains(out, "status: active") {
		t.Errorf("yaml result missing status: %s", out)
	}
}

func TestStatusGetWorklogRejected(t *testing.T) {
	runInTempDir(t)
	fixtureDoc(t, "context/worklog/20260920-110000-x.md", "worklog", "", "")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextStatusGetCmd, nil, "context/worklog/20260920-110000-x.md"))
	})
}

func TestStatusSet(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 16, 0, 0, 0, time.UTC))
	fixtureDoc(t, "context/analysis/20260920-130000-backend.md", "analysis", "active", "")

	out := string(execute(t, contextStatusSetCmd, nil, "context/analysis/20260920-130000-backend.md", "--status", "archived"))
	if strings.TrimSpace(out) != "ok" {
		t.Errorf("set output = %q", out)
	}
	data, err := os.ReadFile("context/analysis/20260920-130000-backend.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if got := frontmatterField(content, "status"); got != "archived" {
		t.Errorf("status = %q, want archived", got)
	}
	if got := frontmatterField(content, "updated"); got != "2026-09-20T16:00:00Z" {
		t.Errorf("updated = %q, want refreshed timestamp", got)
	}
	if !strings.Contains(content, "## Body fixture") {
		t.Error("body must be preserved")
	}
}

func TestStatusSetAddsMissingFields(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 17, 0, 0, 0, time.UTC))
	writeTestFile(t, "context/analysis/20260920-130000-legacy.md", "---\nkind: analysis\nsummary: \"legacy\"\ncreated: \"2026-09-20T10:00:00Z\"\n---\n\nLegacy body\n")

	execute(t, contextStatusSetCmd, nil, "context/analysis/20260920-130000-legacy.md", "--status", "draft")
	data, err := os.ReadFile("context/analysis/20260920-130000-legacy.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if got := frontmatterField(content, "status"); got != "draft" {
		t.Errorf("status = %q, want draft", got)
	}
	if got := frontmatterField(content, "updated"); got != "2026-09-20T17:00:00Z" {
		t.Errorf("updated = %q, want added timestamp", got)
	}
	if !strings.Contains(content, "Legacy body") {
		t.Error("body must be preserved")
	}
}

func TestStatusSetVocabRejection(t *testing.T) {
	runInTempDir(t)
	fixtureDoc(t, "context/analysis/20260920-130000-backend.md", "analysis", "active", "")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextStatusSetCmd, nil, "context/analysis/20260920-130000-backend.md", "--status", "bogus"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextStatusSetCmd, nil, "--type", "analysis", "--slug", "nope", "--status", "archived"))
	})
	fixtureDoc(t, "context/questions/20260920-100000-open-api.md", "questions", "active", "")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextStatusSetCmd, nil, "--type", "questions", "--slug", "open-api", "--status", "review"))
	})
}

func TestStatusSetWorklogRejected(t *testing.T) {
	runInTempDir(t)
	fixtureDoc(t, "context/worklog/20260920-110000-x.md", "worklog", "", "")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextStatusSetCmd, nil, "context/worklog/20260920-110000-x.md", "--status", "archived"))
	})
}

func TestStatusSetDecisionContentGuard(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC))
	fixtureDoc(t, "context/decisions/0001-auth.md", "decision", "proposed", "")

	execute(t, contextStatusSetCmd, nil, "context/decisions/0001-auth.md", "--status", "accepted")

	data, err := os.ReadFile("context/decisions/0001-auth.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if got := frontmatterField(content, "status"); got != "accepted" {
		t.Errorf("decision status = %q, want accepted", got)
	}
	if got := frontmatterField(content, "updated"); got != "" {
		t.Errorf("decision must not gain an updated field, got %q", got)
	}
	if !strings.Contains(content, "## Body fixture") {
		t.Error("decision content must be unchanged apart from status")
	}
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextStatusSetCmd, nil, "context/decisions/0001-auth.md", "--status", "bogus"))
	})
}
