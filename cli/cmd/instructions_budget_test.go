package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/templates"
)

// Instruction-surface budgets.
//
// The always-loaded block is re-sent on every turn, so its size is paid on every
// turn; an on-demand module is paid once, when its action occurs. The budgets
// therefore differ, and each is a ceiling on growth, not a target:
//
//   - AGENTS instructions block ≤ 2,200 words — read on every turn;
//   - any instruction module ≤ 1,400 words — read once per session, when used;
//   - cli.md ≤ 400 words — the agent's own verb list, the catalog everything else
//     defers to `sdt schema` for.
//
// A module over the module ceiling is *ratcheted* rather than cut: it carries a
// long procedural contract (ingestion pipeline, plan phases) whose detail is the
// point. An exempt module's budget is its measured size at the moment of the
// ratchet, so it may only shrink from there; splitting it into a core contract
// plus a detail module is the exit from the exemption. `tasks` left the map that
// way — split into `tasks-handoff` and `tasks-review`, it now fits the common
// budget.

const (
	// 2,398 is the measured size, not a round ceiling: the audit
	// (analysis 20261025-154218, F2) found the block at 2,398/2,400, and a budget
	// with two words of room is a budget that says yes to the next sentence. The
	// block is a zero-growth surface — an addition displaces a line or moves detail
	// into the module that owns it (authoring.md, "Spend the reader's context").
	instructionBlockBudget   = 2398
	instructionModuleBudget  = 1400
	instructionCatalogBudget = 400
	// instructionFileBudget is the whole every-turn file — the generated block plus
	// the write-once project block, title and separators included — pinned at the
	// measured 4,380 words of 2026-10-27 (audit 20261025-154218, F1). The block
	// ceiling alone left the authored half unguarded: it was 1,977 words, so the
	// file a session actually pays for was 1.8x what the block budget named.
	// Raised to 4,390 on 2026-10-28: the lesson module's on-action row, five
	// words for a contract a session reads whenever it writes teaching content.
	instructionFileBudget = 4390
)

// instructionModuleRatchet is the exemption map: a module may not grow past the
// ceiling recorded here, and the entry is removed when it falls under the common
// budget or is split.
// instructionModuleCeilings are per-module ceilings below the common 1,400: a
// module whose contract is deliberately small, so growth is visible rather than
// absorbed by the common budget.
var instructionModuleCeilings = map[string]int{
	"sources": 800,
}

var instructionModuleRatchet = map[string]int{
	// ingestion rose to 3786 (2026-10-24): the receipt now carries the
	// concept-coverage line and the summary-check verdict, the two halves of a
	// book-scale run that the unit coverage alone could not report.
	// ingestion rose to 3790 (2026-10-28): the `type` cell points at `wiki.md` for
	// what each page type asserts, which lived by implication only.
	"ingestion": 3790,
	// plan rose to 2675 (2026-10-29): `close` is now stated as the only writer of a
	// plan's `completed`, because a checklist that reaches its last item no longer
	// flips the status — the closeout that belongs with it would be skipped.
	"plan":        2675,
	"analysis":    2496,
	"development": 2455,
	// wiki rose to 2443 (2026-10-24): the coverage record documents the
	// `concepts:` block beside the unit block, its identity and its findings.
	// Rose to 2495 (2026-10-28): the Verification section states that a stamp
	// informs and never gates — the rule the readiness gate reads, which had no
	// home outside a plan. Rose to 2590 the same day: the Page contract now says
	// what each page `type` asserts, which is where the type was documented by
	// implication only.
	"wiki":         2590,
	"distillation": 1763,
}

// countWords is the one definition the budgets are measured with.
func countWords(text string) int {
	return len(strings.Fields(text))
}

// instructionModuleBody renders a module's body without its frontmatter: the
// title and summary are metadata, not prose the agent reads.
func instructionModuleBody(tmpl string) string {
	body := templates.Must("instructions/"+tmpl, nil, nil)
	if i := strings.Index(body, "\n---\n"); i >= 0 && strings.HasPrefix(body, "---\n") {
		return body[i+len("\n---\n"):]
	}
	return body
}

// TestInstructionWordBudget is the gate: the always-loaded block and every
// module stay within their ceiling, and a failure names the module and its count
// so the fix is obvious rather than a bisect.
func TestInstructionWordBudget(t *testing.T) {
	block := countWords(agentBlockInstructions("p", "g"))
	if block > instructionBlockBudget {
		t.Errorf("AGENTS.md instructions block: %d words, budget %d — move detail into its module and leave a pointer", block, instructionBlockBudget)
	}

	for _, tmpl := range templates.List("instructions") {
		name := strings.TrimSuffix(tmpl, ".md.tmpl")
		got := countWords(instructionModuleBody(tmpl))
		budget := instructionModuleBudget
		if name == "cli" {
			budget = instructionCatalogBudget
		}
		if ceiling, ok := instructionModuleCeilings[name]; ok {
			budget = ceiling
		}
		if ratchet, ok := instructionModuleRatchet[name]; ok {
			budget = ratchet
			if got < instructionModuleBudget {
				t.Errorf("%s: %d words is under the common budget %d — remove it from instructionModuleRatchet", name, got, instructionModuleBudget)
			}
			delete(instructionModuleRatchet, name)
		}
		if got > budget {
			t.Errorf("%s: %d words, budget %d — split the module or move detail to the contract it belongs to", name, got, budget)
		}
	}
	for name := range instructionModuleRatchet {
		t.Errorf("%s: ratchet entry never checked (no such instruction module)", name)
	}
}

// TestAgentsFileWordBudget is the ceiling the block budget could not give: the
// whole every-turn file, project block included. The block ceiling measures the
// template; this one measures the file on disk, because the project block is
// authored in place. A checkout without the file skips with a sentence rather than
// failing: the ceiling applies where the file lives.
func TestAgentsFileWordBudget(t *testing.T) {
	path := filepath.Join("..", "..", agentTargetDefault)
	data, err := os.ReadFile(path) //#nosec G304 -- the project's own always-loaded file
	if err != nil {
		t.Skipf("%s is not in this checkout: the file ceiling applies where the file lives", agentTargetDefault)
	}
	words := countWords(string(data))
	if words > instructionFileBudget {
		t.Errorf("%s: %d words, budget %d — the project block is paid every turn too: move detail on demand or displace a line", agentTargetDefault, words, instructionFileBudget)
	}
	if words < instructionFileBudget/2 {
		t.Errorf("%s: %d words is far under the pinned ceiling %d — re-measure it rather than leaving a budget nobody can hit", agentTargetDefault, words, instructionFileBudget)
	}
}

// TestInstructionBudgetCoversEveryModule proves the budget loop is not silently
// skipping a member: every embedded module is measured, cli.md included.
func TestInstructionBudgetCoversEveryModule(t *testing.T) {
	names := templates.List("instructions")
	if len(names) == 0 {
		t.Fatal("no instruction templates found")
	}
	seen := map[string]bool{}
	for _, tmpl := range names {
		seen[strings.TrimSuffix(tmpl, ".md.tmpl")] = true
	}
	for _, want := range []string{"cli", "plan", "tasks", "development"} {
		if !seen[want] {
			t.Errorf("module %q missing from the budgeted set", want)
		}
	}
	if fmt.Sprintf("%d", instructionBlockBudget) != "2398" {
		t.Error("the always-loaded budget changed without updating authoring.md")
	}
}
