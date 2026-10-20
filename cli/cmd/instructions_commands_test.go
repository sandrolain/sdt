package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

// TestTriggerDeclarationsAreConsistent checks the trigger table against itself
// and against the registry it renders: the durable contract it names must exist
// as an instruction module, the declared kind must be one of the two the
// registry knows, a workflow must name a registered subject kind, and the
// rendered row must carry exactly the declaration — trigger, kind, subject,
// contract and payload.
//
// The `>tasks` row was declared a document command while it advances plan →
// tasks, so its row had no Subject and the agent had to infer one. A
// declaration that disagrees with its own rendering is the defect this guards;
// whether a given row is classified correctly is a taxonomy question, settled
// where the row is written.
func TestTriggerDeclarationsAreConsistent(t *testing.T) {
	known := map[string]bool{}
	for _, ty := range contextTypeList {
		known[ty.kind] = true
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seen := map[string]bool{}
	for _, s := range agentCommandStubs {
		if seen[s.id] {
			t.Errorf("trigger %q declared twice", s.id)
		}
		seen[s.id] = true

		if s.contract == "" {
			t.Errorf("trigger %q declares no contract", s.id)
			continue
		}
		if !templates.Exists("instructions/" + s.contract + ".md.tmpl") {
			t.Errorf("trigger %q declares contract %q, which has no instruction module", s.id, s.contract)
		}
		if len(s.examples) == 0 {
			t.Errorf("trigger %q declares no example", s.id)
		}

		if got := declaredCommandKind(s.id); got != s.kind {
			t.Errorf("trigger %q: lookup returns kind %q, the table declares %q", s.id, got, s.kind)
		}
		if got := declaredCommandSubject(s.id); got != s.subject {
			t.Errorf("trigger %q: lookup returns subject %q, the table declares %q", s.id, got, s.subject)
		}

		rows := parseCommandRows(instrCommandsIndexTemplate(commandEntriesFor(agentProfileSlugs()), "p", now))
		var row *commandRow
		for i := range rows {
			if rows[i].Trigger == s.id {
				row = &rows[i]
				break
			}
		}
		if row == nil {
			t.Errorf("trigger %q has no registry row", s.id)
			continue
		}
		if row.Kind != s.kind {
			t.Errorf("trigger %q: rendered kind %q, declared %q", s.id, row.Kind, s.kind)
		}
		if row.Contract != commandContractPath(s.contract) {
			t.Errorf("trigger %q: rendered contract %q, declared %q", s.id, row.Contract, commandContractPath(s.contract))
		}
		if row.Payload != s.payload {
			t.Errorf("trigger %q: rendered payload %q, declared %q", s.id, row.Payload, s.payload)
		}
		for _, ex := range s.examples {
			if !strings.Contains(row.Examples, ex) {
				t.Errorf("trigger %q: rendered examples %q do not carry %q", s.id, row.Examples, ex)
			}
		}

		switch s.kind {
		case commandKindDocument:
			if s.subject != "" {
				t.Errorf("trigger %q is a document command but declares subject %q", s.id, s.subject)
			}
			if row.Subject != "—" {
				t.Errorf("trigger %q is a document command but renders subject %q", s.id, row.Subject)
			}
		case commandKindWorkflow:
			if !known[s.subject] {
				t.Errorf("trigger %q is a workflow but subject %q is not a registered kind", s.id, s.subject)
			}
			if row.Subject != s.subject {
				t.Errorf("trigger %q: rendered subject %q, declared %q", s.id, row.Subject, s.subject)
			}
		default:
			t.Errorf("trigger %q declares unknown kind %q", s.id, s.kind)
		}
	}
}

// TestRegistryRoundTripsItsRows proves a rendered row parses back into the same
// declaration: the table is the only place a trigger is declared, so a row the
// agent reads and a row the CLI writes must be the same thing.
func TestRegistryRoundTripsItsRows(t *testing.T) {
	line := commandRowLine(commandRow{
		Trigger:  "triage",
		Kind:     commandKindWorkflow,
		Subject:  "analysis",
		Contract: "context/instructions/research.md",
		Payload:  "the symptom to triage",
		Examples: "`>triage: failing test`",
	})
	rows := parseCommandRows("| Trigger | Kind | Subject | Durable contract | Payload | Examples |\n|---|---|---|---|---|---|\n" + line)
	if len(rows) != 1 {
		t.Fatalf("parsed %d row(s), want 1", len(rows))
	}
	got := rows[0]
	if got.Trigger != "triage" || got.Kind != commandKindWorkflow || got.Subject != "analysis" ||
		got.Contract != "context/instructions/research.md" || got.Payload != "the symptom to triage" ||
		got.Examples != "`>triage: failing test`" {
		t.Fatalf("row did not round-trip: %+v", got)
	}
	if again := commandRowLine(got); again != line {
		t.Errorf("row re-renders differently:\ngot  %q\nwant %q", again, line)
	}
}

// TestVerifyResolvesEveryRegistryRow proves the verify check has teeth: a row
// whose contract was removed is a CRITICAL, and a detail file without
// frontmatter is a WARNING. The registry is the lookup surface, so a row that
// resolves to nothing is worse than a missing file: the agent would follow it.
func TestVerifyResolvesEveryRegistryRow(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")

	if issues := agentVerifyChecks(); len(issues) != 0 {
		t.Fatalf("a fresh workspace is not clean: %+v", issues)
	}

	execute(t, contextCommandsNewCmd, nil, "triage", "--contract", "analysis", "--detail")
	if issues := agentVerifyChecks(); len(issues) != 0 {
		t.Fatalf("a registered trigger with an existing contract is not clean: %+v", issues)
	}

	if err := os.Remove(filepath.Join(sdtInstrDir, "analysis.md")); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range agentVerifyChecks() {
		if issue.Priority == ctxLintCritical && strings.Contains(issue.Message, ">triage") {
			found = true
		}
	}
	if !found {
		t.Error("verify did not report the trigger whose contract is gone")
	}

	writeTestFile(t, filepath.Join(sdtCommandsDir, "triage.md"), "no frontmatter here\n")
	warned := false
	for _, issue := range agentVerifyChecks() {
		if issue.Priority == ctxLintWarning && strings.Contains(issue.Message, "no frontmatter") {
			warned = true
		}
	}
	if !warned {
		t.Error("verify did not report a detail file without frontmatter")
	}
}
