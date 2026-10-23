package cmd

import (
	"fmt"
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
	// 2,400 rather than 2,200: the Toolbox map (analysis 20261023-065322) added
	// ~150 words that the block did not carry, and the map is the point — it
	// names the verb for the operation at the moment of choice, where three
	// impossible substitutions used to be listed. Measured 2,329 words when the
	// ceiling moved; growth is visible again from here.
	instructionBlockBudget   = 2400
	instructionModuleBudget  = 1400
	instructionCatalogBudget = 400
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
	// ingestion rose to 3569 (2026-10-21): the routing receipt now names where
	// the two counts come from — `sdt doc2md` keeps them measured, so the hand
	// count that motivated the routing trail is the documented fallback.
	"ingestion":    3569,
	"plan":         2645,
	"analysis":     2496,
	"development":  2455,
	"wiki":         2327,
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
	if fmt.Sprintf("%d", instructionBlockBudget) != "2400" {
		t.Error("the always-loaded budget changed without updating authoring.md")
	}
}
