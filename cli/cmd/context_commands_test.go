package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newProjectRegistry seeds a registry in the working directory, the way
// `agent init` leaves one, without asking the CLI to write the whole workspace.
func newProjectRegistry(t *testing.T) {
	t.Helper()
	stubContextNow(t, time.Date(2026, 9, 20, 17, 30, 0, 0, time.UTC))
	body := agentRenderGenerated(commandsRegistryMarker(), commandsIndexContentFor([]string{agentProfileCore}, "tm", contextNow()))
	writeTestFile(t, "context/commands/index.md", body)
}

// TestContextCommandsNew verifies that a trigger becomes a registry row: no
// per-trigger file is written, and the row sits outside the generated block so
// a refresh keeps it.
func TestContextCommandsNew(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)

	out := string(execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "research"))
	if !strings.Contains(out, "registered >triage") {
		t.Errorf("missing registration line in output: %q", out)
	}
	if _, err := os.Stat("context/commands/triage.md"); !os.IsNotExist(err) {
		t.Errorf("a trigger must not create a per-trigger file: %v", err)
	}

	idx := mustReadFile(t, "context/commands/index.md")
	if !strings.Contains(idx, "| `>triage` | document | — | `context/instructions/research.md` |") {
		t.Errorf("registry missing the trigger row:\n%s", idx)
	}
	if !strings.Contains(idx, "## Project triggers") {
		t.Errorf("registry missing the project section:\n%s", idx)
	}

	// The row is outside the generated block: a refresh of the built-in table
	// keeps it, while the generated table itself is rewritten.
	refreshed := agentRenderGenerated(commandsRegistryMarker(), commandsIndexContentFor([]string{agentProfileCore}, "tm", contextNow()))
	_, tail := commandsIndexSplit(idx)
	if !hasProjectRow(tail, "triage") {
		t.Errorf("the project row is inside the generated block:\n%s", idx)
	}
	if !strings.Contains(refreshed, "| `>analysis` |") {
		t.Error("the refreshed registry lost its built-in rows")
	}
}

// TestContextCommandsNewRejects guards the trigger grammar and the reserved
// index name.
func TestContextCommandsNewRejects(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "bad/slug"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "index"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "bad/contract"))
	})
	if idx := mustReadFile(t, "context/commands/index.md"); strings.Contains(idx, ">triage") {
		t.Errorf("a rejected trigger was registered:\n%s", idx)
	}
}

// TestContextCommandsNewPayload verifies the --payload flag: the phrase is the
// row's Payload cell, and a trigger created without one shows the explicit
// "not declared" placeholder rather than a blank cell.
func TestContextCommandsNewPayload(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)

	execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "research", "--payload", "the question the run answers")
	execute(t, contextCommandsNewCmd, nil, "scratch", "--contract", "research")

	idx := mustReadFile(t, "context/commands/index.md")
	if !strings.Contains(idx, "| the question the run answers |") {
		t.Errorf("registry Payload cell missing the declared phrase:\n%s", idx)
	}
	if !strings.Contains(idx, commandPayloadUndeclared) {
		t.Errorf("registry must show %q for an undeclared trigger:\n%s", commandPayloadUndeclared, idx)
	}
	rows := parseCommandRows(idx)
	for _, row := range rows {
		if row.Trigger == "triage" && row.Payload != "the question the run answers" {
			t.Errorf("parsed payload = %q", row.Payload)
		}
		if row.Trigger == "scratch" && row.Payload != commandPayloadUndeclared {
			t.Errorf("parsed undeclared payload = %q", row.Payload)
		}
	}
}

// TestContextCommandsNewPayloadRejects guards the phrase shape: it must be a
// non-empty single line, because it is stored as one table cell.
func TestContextCommandsNewPayloadRejects(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)
	before := mustReadFile(t, "context/commands/index.md")
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "triage", "--payload", "   "))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "triage", "--payload", "line one\nline two"))
	})
	if after := mustReadFile(t, "context/commands/index.md"); after != before {
		t.Errorf("a rejected --payload must not touch the registry:\n%s", after)
	}
}

// TestContextCommandsNewTaxonomy verifies a user trigger declares its kind and
// subject in the row, and that the default is a document command.
func TestContextCommandsNewTaxonomy(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)

	execute(t, contextCommandsNewCmd, nil, "advance", "--contract", "plan", "--kind", "workflow", "--subject", "analysis")
	execute(t, contextCommandsNewCmd, nil, "plain", "--contract", "research")

	idx := mustReadFile(t, "context/commands/index.md")
	if !strings.Contains(idx, "| `>advance` | workflow | `analysis` | `context/instructions/plan.md` |") {
		t.Errorf("registry row missing the workflow taxonomy:\n%s", idx)
	}
	if !strings.Contains(idx, "| `>plain` | document | — |") {
		t.Errorf("registry row missing the document default:\n%s", idx)
	}
	for _, row := range parseCommandRows(idx) {
		if row.Trigger == "advance" && (row.Kind != commandKindWorkflow || row.Subject != "analysis") {
			t.Errorf("parsed workflow row = %+v", row)
		}
	}
}

// TestContextCommandsNewTaxonomyRejects guards the taxonomy shape: an unknown
// kind, a non-slug subject, or a subject without the workflow kind is rejected
// and registers nothing.
func TestContextCommandsNewTaxonomyRejects(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "bad", "--kind", "sideways"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "bad", "--kind", "workflow", "--subject", "Not A Slug"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsNewCmd, nil, "bad", "--subject", "analysis"))
	})
	if idx := mustReadFile(t, "context/commands/index.md"); strings.Contains(idx, ">bad") {
		t.Errorf("a rejected taxonomy was registered:\n%s", idx)
	}
}

// TestContextCommandsNewDetail verifies --detail writes the companion document,
// that the row name survives a regeneration, and that the detail file is never
// a generated one (so --force cannot wipe its prose).
func TestContextCommandsNewDetail(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)

	execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "research", "--payload", "the symptom to triage", "--detail")
	detail := mustReadFile(t, "context/commands/triage.md")
	if got := frontmatterField(detail, "kind"); got != "commands" {
		t.Errorf("detail kind = %q", got)
	}
	if !strings.Contains(detail, "the symptom to triage") {
		t.Errorf("detail does not carry the payload phrase:\n%s", detail)
	}
	if strings.Contains(detail, "sdt:begin:") {
		t.Errorf("the detail file must be a project document, not a generated one:\n%s", detail)
	}
	if !hasProjectRow(mustReadFile(t, "context/commands/index.md"), "triage") {
		t.Error("the row was not registered")
	}

	out := string(execute(t, contextCommandsRmCmd, nil, "triage"))
	if !strings.Contains(out, "removed context/commands/triage.md") {
		t.Errorf("rm did not remove the detail file: %q", out)
	}
}

// TestContextCommandsRm verifies row removal: the row disappears, the detail
// file goes with it, a surviving row keeps its payload and `rm` never archives.
func TestContextCommandsRm(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)
	execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "research", "--payload", "the removed trigger's payload", "--detail")
	execute(t, contextCommandsNewCmd, nil, "other", "--contract", "research", "--payload", "subject of the other command")

	out := string(execute(t, contextCommandsRmCmd, nil, "triage"))
	if !strings.Contains(out, "removed >triage") {
		t.Errorf("missing removal line in output: %q", out)
	}
	if _, err := os.Stat("context/commands/triage.md"); !os.IsNotExist(err) {
		t.Errorf("detail file still present: %v", err)
	}
	if _, err := os.Stat(sdtTrashDir); !os.IsNotExist(err) {
		t.Errorf("commands rm must not write a deprecated copy: %v", err)
	}
	idx := mustReadFile(t, "context/commands/index.md")
	if strings.Contains(idx, ">triage") {
		t.Errorf("registry still lists the removed trigger:\n%s", idx)
	}
	if strings.Contains(idx, "the removed trigger's payload") {
		t.Errorf("registry still carries the removed trigger's payload:\n%s", idx)
	}
	if !strings.Contains(idx, "subject of the other command") || !strings.Contains(idx, ">other") {
		t.Errorf("registry dropped a surviving trigger:\n%s", idx)
	}
}

func TestContextCommandsRmRejects(t *testing.T) {
	runInTempDir(t)
	newProjectRegistry(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsRmCmd, nil, "index"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsRmCmd, nil, "missing"))
	})
	// A built-in trigger comes from the installed set, not from the project.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextCommandsRmCmd, nil, "analysis"))
	})
}

// TestContextNewWiki verifies the wiki scaffold: subpath path creation,
// frontmatter contract (kind/id/title/summary/status/created/updated) and the
// default Summary/Claims/Notes body.
func TestContextNewWiki(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC))

	out := string(execute(t, contextNewCmd, nil, "--type", "wiki", "--slug", "backend/auth", "--title", "Auth", "--summary", "auth design"))
	if !strings.Contains(out, filepath.Join(sdtWikiDir, "backend", "auth.md")) {
		t.Fatalf("expected wiki path in output: %q", out)
	}
	page := mustReadFile(t, filepath.Join(sdtWikiDir, "backend", "auth.md"))
	for key, want := range map[string]string{
		"kind":    "wiki",
		"id":      "backend/auth",
		"title":   "Auth",
		"summary": "auth design",
		"status":  "draft",
		"created": "2026-09-20T18:00:00Z",
		"updated": "2026-09-20T18:00:00Z",
	} {
		if got := frontmatterField(page, key); got != want {
			t.Errorf("wiki %s = %q, want %q", key, got, want)
		}
	}
	for _, want := range []string{"## Summary", "## Claims", "## Notes"} {
		if !strings.Contains(page, want) {
			t.Errorf("wiki body missing %q:\n%s", want, page)
		}
	}
}

// TestContextPathWiki verifies subpath support in `context path`.
func TestContextPathWiki(t *testing.T) {
	runInTempDir(t)
	out := string(execute(t, contextPathCmd, nil, "--type", "wiki", "--slug", "backend/session"))
	if !strings.Contains(out, filepath.Join(sdtWikiDir, "backend", "session.md")) {
		t.Errorf("wiki path = %q", out)
	}
}

// TestCommandsShowPrintsOneRow is the R5 gate: a known trigger resolves from one
// row without the whole registry, and an unknown one names what exists.
func TestCommandsShowPrintsOneRow(t *testing.T) {
	setupContextProject(t)

	out := string(execute(t, contextCommandsShowCmd, nil, "plan"))
	for _, want := range []string{">plan", "workflow", "context/instructions/plan.md", "analysis/"} {
		if !strings.Contains(out, want) {
			t.Errorf("commands show plan missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, ">wiki") {
		t.Errorf("commands show plan printed another row:\n%s", out)
	}

	js := string(execute(t, contextCommandsShowCmd, nil, "plan", "--format", "json"))
	for _, want := range []string{`"trigger"`, `"kind"`, `"examples"`} {
		if !strings.Contains(js, want) {
			t.Errorf("commands show --format json missing %q:\n%s", want, js)
		}
	}
}
