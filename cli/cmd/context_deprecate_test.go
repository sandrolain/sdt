//go:build !wasm

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestDeprecateRegistryDeclaresARetiredStatus is the guard on the mapping the
// verb reads: every declared value must be a member of the vocabulary it belongs
// to, and the table must not drift silently. A retired value outside its own
// vocabulary would be an out-of-vocabulary status the verb wrote itself.
func TestDeprecateRegistryDeclaresARetiredStatus(t *testing.T) {
	want := map[string]string{
		ctxTypePlan:         "abandoned",
		ctxTypeAnalysis:     statusArchived,
		ctxTypeProposal:     ctxDocStatusSuperseded,
		ctxTypePrompt:       statusArchived,
		ctxTypeResearch:     statusArchived,
		ctxTypeArchitecture: ctxDocStatusSuperseded,
		ctxTypeDecision:     "deprecated",
		ctxTypeTasks:        taskFileStatusArchived,
		ctxTypeWiki:         statusArchived,
		ctxTypeBriefing:     statusArchived,
	}
	for _, tp := range contextTypeList {
		got := tp.retiredStatus
		if expected, known := want[tp.kind]; known {
			if got != expected {
				t.Errorf("%s retiredStatus = %q, want %q", tp.kind, got, expected)
			}
		} else if got != "" {
			t.Errorf("%s declares %q as retired but is not in the table — update both", tp.kind, got)
		}
		if got == "" {
			continue
		}
		if !ctxStatusInVocab(tp, got) {
			t.Errorf("%s: retired status %q is not in its own vocabulary %s", tp.kind, got, ctxStatusVocab(tp))
		}
	}
}

// TestDeprecateSetsTheRetiredStatusWithoutMovingTheFile is the decision in one
// assertion: the state changes, the location does not, and the body is not
// touched.
func TestDeprecateSetsTheRetiredStatusWithoutMovingTheFile(t *testing.T) {
	runInTempDir(t)
	stubContextNow(t, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	const path = "context/analysis/20260920-130000-backend.md"
	fixtureDoc(t, path, "analysis", "completed", "")

	out := string(execute(t, contextDeprecateCmd, nil, path))
	if !strings.Contains(out, "completed → archived") {
		t.Errorf("output must report the transition: %q", out)
	}
	content := mustReadFile(t, path)
	if got := frontmatterField(content, ctxMapStatus); got != statusArchived {
		t.Errorf("status = %q, want %q", got, statusArchived)
	}
	if got := frontmatterField(content, "updated"); got != "2026-10-15T12:00:00Z" {
		t.Errorf("updated = %q, want the refreshed stamp", got)
	}
	if !strings.Contains(content, "## Body fixture") {
		t.Error("the body must be preserved")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the file must stay where it was: %v", err)
	}
}

// TestDeprecateRefusesATypeWithoutARetiredStatus pins both refusal sentences: a
// type whose vocabulary has no retirement is told what it does carry, and a type
// with no status field at all is told it has no retired state to set.
func TestDeprecateRefusesATypeWithoutARetiredStatus(t *testing.T) {
	runInTempDir(t)
	const questions = "context/questions/20260920-130000-q.md"
	const notes = "context/notes/20260920-130000-n.md"
	fixtureDoc(t, questions, "questions", "active", "")
	fixtureDoc(t, notes, "notes", "", "")

	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextDeprecateCmd, nil, questions))
	})
	if out := errBuf.String(); !strings.Contains(out, "carries no retired status") || !strings.Contains(out, "resolved") {
		t.Errorf("a type with statuses but no retirement must be told its vocabulary, got %q", out)
	}

	errBuf.Reset()
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextDeprecateCmd, nil, notes))
	})
	if out := errBuf.String(); !strings.Contains(out, "carries no status field") {
		t.Errorf("a type with no status field must be told so, got %q", out)
	}

	// Neither refusal may write.
	if got := frontmatterField(mustReadFile(t, questions), ctxMapStatus); got != "active" {
		t.Errorf("the refused document's status changed to %q", got)
	}
}

// TestDeprecateRefusesAnAlreadyRetiredDocument keeps the transition honest: the
// verb reports a change, so it does not make a no-op one.
func TestDeprecateRefusesAnAlreadyRetiredDocument(t *testing.T) {
	runInTempDir(t)
	const path = "context/analysis/20260920-130000-done.md"
	fixtureDoc(t, path, "analysis", statusArchived, "")

	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextDeprecateCmd, nil, path))
	})
	if out := errBuf.String(); !strings.Contains(out, "already") {
		t.Errorf("expected an already-retired refusal, got %q", out)
	}
}

// TestDeprecateRecordsTheSuccessor covers the half of the decision that makes the
// state change checkable: a named successor is recorded, and one that does not
// resolve is refused rather than written.
func TestDeprecateRecordsTheSuccessor(t *testing.T) {
	runInTempDir(t)
	const old = "context/analysis/20260920-130000-old.md"
	const successor = "context/analysis/20260921-130000-new.md"
	fixtureDoc(t, old, "analysis", "completed", "")
	fixtureDoc(t, successor, "analysis", "active", "")

	execute(t, contextDeprecateCmd, nil, old, "--by", successor)
	content := mustReadFile(t, old)
	if got := frontmatterField(content, ctxFrontmatterSupersededBy); got != "analysis/20260921-130000-new.md" {
		t.Errorf("superseded_by = %q, want the corpus-relative successor", got)
	}
	if got := frontmatterField(content, ctxMapStatus); got != statusArchived {
		t.Errorf("status = %q, want the retired value", got)
	}

	// A successor that does not resolve must refuse, and must not write.
	const dangling = "context/analysis/20260922-130000-dangling.md"
	fixtureDoc(t, dangling, "analysis", "completed", "")
	var errBuf bytes.Buffer
	rootCmd.SetErr(&errBuf)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextDeprecateCmd, nil, dangling, "--by", "analysis/does-not-exist.md"))
	})
	if out := errBuf.String(); !strings.Contains(out, "does not resolve") {
		t.Errorf("expected a dangling-successor refusal, got %q", out)
	}
	if got := frontmatterField(mustReadFile(t, dangling), ctxMapStatus); got != "completed" {
		t.Errorf("a refused deprecate wrote status %q", got)
	}

	// A document may not supersede itself.
	errBuf.Reset()
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextDeprecateCmd, nil, successor, "--by", successor))
	})
	if out := errBuf.String(); !strings.Contains(out, "the document being retired") {
		t.Errorf("expected a self-supersession refusal, got %q", out)
	}
}

// TestDeprecateDryRunWritesNothing pins the flag against the writer: a preview
// that stamped `updated` or a uid would not be a preview.
func TestDeprecateDryRunWritesNothing(t *testing.T) {
	runInTempDir(t)
	const path = "context/analysis/20260920-130000-preview.md"
	fixtureDoc(t, path, "analysis", "completed", "")
	before := mustReadFile(t, path)

	out := string(execute(t, contextDeprecateCmd, nil, path, "--dry-run"))
	if !strings.Contains(out, "would retire") || !strings.Contains(out, "archived") {
		t.Errorf("dry-run must report the would-be transition: %q", out)
	}
	if after := mustReadFile(t, path); after != before {
		t.Errorf("dry-run wrote to the file:\n--- before\n%s\n--- after\n%s", before, after)
	}
}

// TestDeprecateFormatsReportTheTransition keeps the three formats from
// disagreeing about what happened.
func TestDeprecateFormatsReportTheTransition(t *testing.T) {
	runInTempDir(t)
	const path = "context/decisions/0001-x.md"
	fixtureDoc(t, path, "decision", "accepted", "")
	out := execute(t, contextDeprecateCmd, nil, path, "--format", "json")
	var res ctxDeprecateResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out)
	}
	if res.From != "accepted" || res.To != "deprecated" {
		t.Errorf("json transition = %s → %s, want accepted → deprecated", res.From, res.To)
	}
	if res.Kind != ctxTypeDecision {
		t.Errorf("json kind = %q", res.Kind)
	}
	// The yaml check needs its own fixture: the JSON call already retired this
	// one, and re-running it would be refused as a repeat rather than reported.
	const second = "context/decisions/0002-y.md"
	fixtureDoc(t, second, "decision", "proposed", "")
	if y := string(execute(t, contextDeprecateCmd, nil, second, "--format", "yaml")); !strings.Contains(y, "to: deprecated") {
		t.Errorf("yaml transition missing:\n%s", y)
	}
}
