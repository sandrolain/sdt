package cmd

import (
	"testing"

	"github.com/sandrolain/sdt/internal/ctxrel"
)

func TestContextTraceChain(t *testing.T) {
	dir := runInTempDir(t)
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	plan := writeCascadePlan(t, dir, "20260101-000000-a-plan", ctxWikiStatusActive, analysis, true)
	task := writeCascadeTask(t, dir, "20260101-000000-a-plan-phase-1", taskFileStatusInProgress, plan, "x")

	edges, err := ctxrel.Load("context")
	if err != nil {
		t.Fatal(err)
	}
	chain := ctxTraceChain(task, edges)
	if len(chain) != 3 {
		t.Fatalf("chain = %d steps, want 3: %+v", len(chain), chain)
	}
	if chain[0].Kind != "tasks" || chain[1].Kind != "plan" || chain[2].Kind != "analysis" {
		t.Errorf("chain kinds = %q,%q,%q", chain[0].Kind, chain[1].Kind, chain[2].Kind)
	}
	if chain[0].Ref != task || chain[1].Ref != plan || chain[2].Ref != analysis {
		t.Errorf("chain refs = %q,%q,%q", chain[0].Ref, chain[1].Ref, chain[2].Ref)
	}
}

func TestContextTraceRootIsItsOwnChain(t *testing.T) {
	dir := runInTempDir(t)
	analysis := writeCascadeAnalysis(t, dir, "20260101-000000-a", ctxWikiStatusActive)
	edges, err := ctxrel.Load("context")
	if err != nil {
		t.Fatal(err)
	}
	chain := ctxTraceChain(analysis, edges)
	if len(chain) != 1 || chain[0].Kind != "analysis" {
		t.Fatalf("root chain = %+v, want one analysis step", chain)
	}
}
