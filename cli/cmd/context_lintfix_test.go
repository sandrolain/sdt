package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLintFixRemovesH1AndFillsObjective is the X3 gate: the mechanical findings
// stop being permanent debt, and the fix says what it rewrote.
func TestLintFixRemovesH1AndFillsObjective(t *testing.T) {
	dir := setupContextProject(t)
	analysis := "context/analysis/20260101-000000-source.md"
	writeCtxDoc(t, analysis, `---
kind: analysis
uid: `+cascadeUIDFor(analysis)+`
title: Source analysis
summary: the analysis a plan derives from
status: active
objective: closeout-gap
---

## Context

body
`)
	plan := "context/plan/20260101-000000-derived.md"
	writeCtxDoc(t, plan, `---
kind: plan
uid: `+cascadeUIDFor(plan)+`
summary: a plan with a body H1 and no objective
status: active
analysis_id: `+cascadeUIDFor(analysis)+`
---

# Derived plan

## Objective

do it

## Phases

- [ ] step
`)

	out := execute(t, contextLintCmd, nil, "--fix", plan)
	content := mustReadFile(t, filepath.Join(dir, plan))
	if strings.Contains(content, "\n# Derived plan") {
		t.Errorf("the body H1 survived --fix:\n%s", content)
	}
	if got := frontmatterField(content, "objective"); got != "closeout-gap" {
		t.Errorf("objective = %q, want the parent analysis's grouping", got)
	}
	if !strings.Contains(string(out), "fixed: "+plan) {
		t.Errorf("--fix did not name the file it rewrote:\n%s", out)
	}

	// Idempotent: a second run has nothing to repair.
	out = execute(t, contextLintCmd, nil, "--fix", plan)
	if !strings.Contains(string(out), "nothing mechanical to repair") {
		t.Errorf("second run was not a no-op:\n%s", out)
	}
}

// TestLintFixLeavesTerminalDocuments proves the default scope: a closed document
// is history, and `--fix-terminal` is the explicit act that rewrites it.
func TestLintFixLeavesTerminalDocuments(t *testing.T) {
	dir := setupContextProject(t)
	plan := "context/plan/20260101-000000-closed.md"
	writeCtxDoc(t, plan, `---
kind: plan
uid: `+cascadeUIDFor(plan)+`
summary: a plan already closed
status: completed
---

# Closed plan

## Phases

- [x] step
`)

	execute(t, contextLintCmd, nil, "--fix", plan)
	if content := mustReadFile(t, filepath.Join(dir, plan)); !strings.Contains(content, "# Closed plan") {
		t.Fatalf("the default fix rewrote a completed document:\n%s", content)
	}

	execute(t, contextLintCmd, nil, "--fix", "--fix-terminal", plan)
	if content := mustReadFile(t, filepath.Join(dir, plan)); strings.Contains(content, "# Closed plan") {
		t.Fatalf("--fix-terminal did not repair the document:\n%s", content)
	}
}

// TestLintFixLeavesMapRootAndDeckTitle: a map's single `#` is its root and a
// deck's `#` is a slide title. The lint exempts both from the no-H1 rule, so a
// fix that ignores the exemption destroys the document it was asked to repair.
func TestLintFixLeavesMapRootAndDeckTitle(t *testing.T) {
	dir := setupContextProject(t)
	for rel, body := range map[string]string{
		"context/notes/a.map.md":   "---\nkind: notes\nsummary: a map\n---\n# The root\n\n## Branch\n",
		"context/notes/a.slide.md": "---\nkind: notes\nsummary: a deck\n---\n# A slide title\n\nbody\n",
	} {
		writeCtxDoc(t, rel, body)
		execute(t, contextLintCmd, nil, "--fix", rel)
		if content := mustReadFile(t, filepath.Join(dir, rel)); !strings.Contains(content, "\n# ") {
			t.Errorf("%s lost its H1, which the lint exempts:\n%s", rel, content)
		}
	}
}

// TestLintFixFillsTaskObjectiveFromItsPlan covers the second derivable case: a
// task file groups under the plan it belongs to.
func TestLintFixFillsTaskObjectiveFromItsPlan(t *testing.T) {
	dir := setupContextProject(t)
	plan := "context/plan/20260101-000000-parent.md"
	writeCtxDoc(t, plan, `---
kind: plan
uid: `+cascadeUIDFor(plan)+`
summary: the parent plan
status: active
objective: parent-objective
---

## Phases

- [ ] step
`)
	task := "context/tasks/20260101-000000-parent-phase-1.md"
	writeCtxDoc(t, task, `---
kind: tasks
uid: `+cascadeUIDFor(task)+`
summary: the first stream
status: in-progress
links:
  - plan/20260101-000000-parent.md
sources:
  - plan/20260101-000000-parent.md
phase: "1"
---

- [ ] step
`)

	execute(t, contextLintCmd, nil, "--fix", task)
	if got := frontmatterField(mustReadFile(t, filepath.Join(dir, task)), "objective"); got != "parent-objective" {
		t.Errorf("task objective = %q, want the plan's grouping", got)
	}
}

// TestLintFixDoesNotInventAnObjective: an analysis has no parent, so the key
// stays a finding rather than a guess.
func TestLintFixDoesNotInventAnObjective(t *testing.T) {
	dir := setupContextProject(t)
	analysis := "context/analysis/20260101-000000-orphan.md"
	writeCtxDoc(t, analysis, `---
kind: analysis
uid: `+cascadeUIDFor(analysis)+`
title: Orphan analysis
summary: an analysis without an objective
status: active
---

## Context

body
`)

	execute(t, contextLintCmd, nil, "--fix", analysis)
	content := mustReadFile(t, filepath.Join(dir, analysis))
	if strings.Contains(content, "objective:") {
		t.Errorf("--fix invented an objective for a document with no parent:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(dir, analysis)); err != nil {
		t.Fatal(err)
	}
}
