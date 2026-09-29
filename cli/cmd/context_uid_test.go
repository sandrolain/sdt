package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uuid"
)

// assertUIDv7 fails unless value is a canonical RFC 9562 UUIDv7 string.
func assertUIDv7(t *testing.T, value string) {
	t.Helper()
	u, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		t.Fatalf("uid %q does not parse as a UUID: %v", value, err)
	}
	if v := u[6] >> 4; v != 7 {
		t.Fatalf("uid %q is not version 7 (version nibble %d)", value, v)
	}
}

func TestNewUIDIsVersion7AndUnique(t *testing.T) {
	a, b := newUID(), newUID()
	assertUIDv7(t, a)
	if a == b {
		t.Errorf("two newUID calls collided: %s", a)
	}
}

func TestContextNewEmitsUIDPerKind(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	for _, dt := range ctxNewTypes() {
		out := execute(t, contextNewCmd, nil, "--type", dt.kind, "--title", "uid probe")
		path := strings.TrimSpace(string(out))
		content := mustReadFile(t, path)
		uid := parseFrontmatterField(content, "uid")
		if uid == "" {
			t.Errorf("kind %s: no uid emitted:\n%s", dt.kind, content)
			continue
		}
		assertUIDv7(t, uid)
	}
}

func TestContextTaskEmitsUID(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nsummary: p\nstatus: active\n---\nbody\n")

	execute(t, contextTaskAddCmd, nil, "--phase", "1", "--plan", "p.md", "step one")
	path := filepath.Join("context", "tasks", "20260806-070000-p.md")
	content := mustReadFile(t, path)
	uid := parseFrontmatterField(content, "uid")
	if uid == "" {
		t.Fatalf("task file has no uid:\n%s", content)
	}
	assertUIDv7(t, uid)
}

func TestValidUIDv7(t *testing.T) {
	if !validUIDv7(newUID()) {
		t.Fatal("a freshly generated uid must validate")
	}
	for _, bad := range []string{"", "not-a-uuid", newUID() + "x", "01890f0c-0000-4000-8000-000000000000", strings.ToUpper(newUID())} {
		if validUIDv7(bad) {
			t.Errorf("validUIDv7(%q) = true, want false", bad)
		}
	}
}

func TestLintUIDField(t *testing.T) {
	runInTempDir(t)
	prio := func(sev string) string { return sev }

	missing := lintUIDField("context/analysis/a.md", "---\nkind: analysis\nsummary: a\n---\n", ctxTypeAnalysis, prio)
	if len(missing) != 1 || missing[0].Priority != ctxLintSuggestion {
		t.Fatalf("missing uid without marker = %#v, want one SUGGESTION", missing)
	}

	if err := os.MkdirAll(filepath.Dir(ctxUIDBackfillMarker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctxUIDBackfillMarker, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := lintUIDField("context/analysis/a.md", "---\nkind: analysis\nsummary: a\n---\n", ctxTypeAnalysis, prio)
	if len(after) != 1 || after[0].Priority != ctxLintWarning {
		t.Fatalf("missing uid after backfill = %#v, want one WARNING", after)
	}

	malformed := lintUIDField("context/analysis/a.md", "---\nkind: analysis\nsummary: a\nuid: nope\n---\n", ctxTypeAnalysis, prio)
	if len(malformed) != 1 || malformed[0].Priority != ctxLintWarning {
		t.Fatalf("malformed uid = %#v, want one WARNING", malformed)
	}

	valid := lintUIDField("context/analysis/a.md", "---\nkind: analysis\nsummary: a\nuid: "+newUID()+"\n---\n", ctxTypeAnalysis, prio)
	if valid != nil {
		t.Fatalf("valid uid = %#v, want nil", valid)
	}

	if got := lintUIDField("context/commands/x.md", "---\nkind: commands\nsummary: x\n---\n", ctxTypeCommands, prio); got != nil {
		t.Fatalf("excluded kind = %#v, want nil", got)
	}
}

func TestContextUIDBackfill(t *testing.T) {
	runInTempDir(t)
	const analysis = "context/analysis/a.md"
	writeCtxDoc(t, analysis, "---\nkind: analysis\nsummary: a\n---\nbody\n")
	writeCtxDoc(t, "context/commands/c.md", "---\nkind: commands\nid: commands/c\nsummary: c\n---\nbody\n")

	out := string(execute(t, contextUIDBackfillCmd, nil, "--dry-run"))
	if !strings.Contains(out, "would stamp 1") {
		t.Fatalf("dry-run report = %q, want 1 would-stamp", out)
	}
	if content := mustReadFile(t, analysis); strings.Contains(content, "uid:") {
		t.Fatalf("dry-run must not write a uid:\n%s", content)
	}
	if uidBackfillDone() {
		t.Fatal("dry-run must not write the backfill marker")
	}

	execute(t, contextUIDBackfillCmd, nil)
	content := mustReadFile(t, analysis)
	uid := parseFrontmatterField(content, "uid")
	assertUIDv7(t, uid)
	if !strings.Contains(content, "kind: analysis\nuid: ") {
		t.Errorf("uid must be emitted right after kind:\n%s", content)
	}
	if c := mustReadFile(t, "context/commands/c.md"); strings.Contains(c, "uid:") {
		t.Errorf("ineligible command file must not be stamped:\n%s", c)
	}
	if !uidBackfillDone() {
		t.Fatal("real run must write the backfill marker")
	}

	second := string(execute(t, contextUIDBackfillCmd, nil))
	if !strings.Contains(second, "stamped 0") {
		t.Fatalf("backfill must be idempotent, got %q", second)
	}
	if mustReadFile(t, analysis) != content {
		t.Fatal("idempotent run changed the document")
	}
}

func TestContextStatusSetStampsUID(t *testing.T) {
	runInTempDir(t)
	const doc = "context/analysis/20260920-130000-a.md"
	writeCtxDoc(t, doc, "---\nkind: analysis\nsummary: a\nstatus: active\n---\nbody\n")

	execute(t, contextStatusSetCmd, nil, doc, "--status", "archived")
	assertUIDv7(t, parseFrontmatterField(mustReadFile(t, doc), "uid"))
}

func TestContextRenameStampsUID(t *testing.T) {
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/20260920-130000-a.md", "---\nkind: analysis\nsummary: a\nstatus: active\n---\nbody\n")

	execute(t, contextRenameCmd, nil, "context/analysis/20260920-130000-a.md", "--slug", "backend")
	assertUIDv7(t, parseFrontmatterField(mustReadFile(t, "context/analysis/20260920-130000-backend.md"), "uid"))
}

func TestLintUIDDuplicates(t *testing.T) {
	runInTempDir(t)
	uid := newUID()
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nsummary: a\nuid: "+uid+"\n---\n")
	writeCtxDoc(t, "context/analysis/b.md", "---\nkind: analysis\nsummary: b\nuid: "+uid+"\n---\n")
	writeCtxDoc(t, "context/analysis/c.md", "---\nkind: analysis\nsummary: c\nuid: "+newUID()+"\n---\n")

	issues := lintUIDDuplicates([]string{"context/analysis/a.md", "context/analysis/b.md", "context/analysis/c.md"})
	if len(issues) != 1 || issues[0].Priority != ctxLintWarning || !strings.Contains(issues[0].Message, "duplicate `uid`") {
		t.Fatalf("duplicate issues = %#v, want one WARNING on the second file", issues)
	}
	if issues[0].Path != "context/analysis/b.md" {
		t.Errorf("flagged %s, want the later file b.md", issues[0].Path)
	}
}
