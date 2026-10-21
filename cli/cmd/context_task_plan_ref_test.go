package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTaskAddResolvesABarePlanSlug is the C4 gate: a slug that names a real plan
// is that plan, so the task file is linked to its ledger instead of becoming an
// unlinked standalone checklist that looks like work on the plan.
func TestTaskAddResolvesABarePlanSlug(t *testing.T) {
	dir := setupContextProject(t)
	plan := "20260101-000000-real-plan"
	writeCtxDoc(t, "context/plan/"+plan+".md", `---
kind: plan
uid: `+cascadeUIDFor("context/plan/"+plan+".md")+`
summary: a real plan
status: active
objective: plan-resolution
---

## Phase 1

- [ ] first step
`)

	execute(t, contextTaskAddCmd, nil, "--phase", "1", "--plan", plan, "added by a bare slug")

	files, err := dirFiles(sdtTasksDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("task files = %v (%v), want one", files, err)
	}
	content := mustReadFile(t, filepath.Join(dir, files[0]))
	links := parseFrontmatterList(content, ctxFrontmatterLinks)
	if len(links) != 1 || links[0] != "plan/"+plan+".md" {
		t.Errorf("the task file is not linked to the plan it names: links = %v\n%s", links, content)
	}
	if !strings.Contains(content, "added by a bare slug") {
		t.Errorf("the item did not land in the plan's task file:\n%s", content)
	}
}

// TestTaskAddKeepsAStandaloneSlug proves the other half: a slug that names no
// plan still creates the standalone checklist it always did.
func TestTaskAddKeepsAStandaloneSlug(t *testing.T) {
	setupContextProject(t)

	execute(t, contextTaskAddCmd, nil, "--phase", "1", "--plan", "custom", "a standalone item")

	files, err := dirFiles(sdtTasksDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("task files = %v (%v), want one", files, err)
	}
	content := mustReadFile(t, files[0])
	if links := parseFrontmatterList(content, ctxFrontmatterLinks); len(links) != 0 {
		t.Errorf("a standalone checklist gained links to %v", links)
	}
}

// TestContextLongNamesEveryVisibleVerb: the parent's long text is the verb map an
// agent reads first, so a verb missing from it is one the agent has to discover
// by guessing.
func TestContextLongNamesEveryVisibleVerb(t *testing.T) {
	var missing []string
	for _, c := range contextCmd.Commands() {
		if c.Hidden || !c.IsAvailableCommand() {
			continue
		}
		if !strings.Contains(contextCmd.Long, c.Name()) {
			missing = append(missing, c.Name())
		}
	}
	if len(missing) > 0 {
		t.Errorf("the `context` long text does not name: %s", strings.Join(missing, ", "))
	}
	if !strings.Contains(contextCmd.Long, "close") {
		t.Error("the close verb is not advertised")
	}
}
