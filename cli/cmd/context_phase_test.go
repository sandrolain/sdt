package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPhaseHeadingIndexAcceptsEveryHeadingLevel covers the plan shape: plans
// write `### Phase <n>`, older documents `## Phase <n>`, and the selector must
// resolve both.
func TestPhaseHeadingIndexAcceptsEveryHeadingLevel(t *testing.T) {
	lines := strings.Split("## Phases\n\n### Phase 1 — One\n\n- [ ] a\n\n### Phase 2 — Two\n\n- [ ] b\n", "\n")
	if got := phaseHeadingIndex(lines, "2"); got < 0 || lines[got] != "### Phase 2 — Two" {
		t.Fatalf("### phase not found, index %d", got)
	}
	if got := phaseHeadingIndex([]string{"## Phase 2 — Two"}, "2"); got != 0 {
		t.Fatalf("## phase not found, index %d", got)
	}
}

// TestPhaseHeadingIndexIsTokenExact keeps `1` from matching `1a`.
func TestPhaseHeadingIndexIsTokenExact(t *testing.T) {
	lines := strings.Split("### Phase 1a — Sub\n", "\n")
	if got := phaseHeadingIndex(lines, "1"); got != -1 {
		t.Fatalf("phase 1 matched phase 1a at %d", got)
	}
	if got := phaseHeadingIndex(lines, "1a"); got != 0 {
		t.Fatalf("phase 1a not found, index %d", got)
	}
	if got := phaseHeadingIndex([]string{"### Phase 3 — Three"}, "2"); got != -1 {
		t.Fatalf("absent phase matched at %d", got)
	}
}

// TestContextCheckPhaseResolvesPlanPhases is the end-to-end contract: ticking a
// phase touches that phase's items and no other.
func TestContextCheckPhaseResolvesPlanPhases(t *testing.T) {
	dir := runInTempDir(t)
	stubContextNow(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	rel := "context/plan/20260101-000000-phased-plan.md"
	writeTestFile(t, filepath.Join(dir, rel), `---
kind: plan
uid: 01a0e43d-a000-7000-a000-0000000000bb
status: active
updated: 2026-01-01T00:00:00Z
---

## Phases

### Phase 1 — One

- [ ] first

### Phase 2 — Two

- [ ] second

### Phase 3 — Three

- [ ] third
`)
	execute(t, contextCheckCmd, nil, rel, "--phase", "2", "--state", "done")

	body := mustReadFile(t, filepath.Join(dir, rel))
	if !strings.Contains(body, "- [x] second") {
		t.Errorf("phase 2 item not ticked:\n%s", body)
	}
	if strings.Contains(body, "- [x] first") {
		t.Errorf("phase 1 item was ticked too:\n%s", body)
	}
	if strings.Contains(body, "- [x] third") {
		t.Errorf("phase 3 item was ticked too — the section boundary ran past the next ### heading:\n%s", body)
	}
}
