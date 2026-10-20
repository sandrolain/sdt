package cmd

import (
	"strings"
	"testing"
	"time"
)

// TestExecuteTriggerResolves proves the >execute workflow trigger is registered
// with a resolvable contract and the plan subject, and that its resolved
// instruction file exists — the same lookup an agent performs when the trigger
// fires.
func TestExecuteTriggerResolves(t *testing.T) {
	var found *commandStub
	for i := range agentCommandStubs {
		if agentCommandStubs[i].id == "execute" {
			found = &agentCommandStubs[i]
			break
		}
	}
	if found == nil {
		t.Fatal(">execute is not registered in agentCommandStubs")
	}
	if found.kind != commandKindWorkflow {
		t.Errorf(">execute kind = %q, want %q", found.kind, commandKindWorkflow)
	}
	if found.subject != ctxTypePlan {
		t.Errorf(">execute subject = %q, want %q", found.subject, ctxTypePlan)
	}
	if found.contract != ctxTypePlan {
		t.Errorf(">execute contract = %q, want %q", found.contract, ctxTypePlan)
	}
	generated := map[string]bool{}
	for _, f := range instructionFiles("", "") {
		generated[f.name] = true
	}
	if !generated[found.contract+sdtMarkdownExt] {
		t.Errorf(">execute contract %q has no generated instruction file", found.contract)
	}
}

// TestCommandsIndexHasResolutionLadder asserts the working-context resolution
// ladder is present in the generated index (the single authoritative place the
// workflow stubs reference) with its four rungs and the ask rule.
func TestCommandsIndexHasResolutionLadder(t *testing.T) {
	idx := instrCommandsIndexTemplate([]commandIndexEntry{
		{Trigger: "plan", Contract: "plan", Kind: commandKindWorkflow, Subject: "analysis", Payload: "the analysis the plan is built from"},
	}, "p", time.Now())
	for _, want := range []string{
		"## Working-context resolution",
		"1. **Explicit**",
		"2. **Active / unique**",
		"3. **Under discussion**",
		"4. **Ask**",
		"tie-break to propose, never to decide",
	} {
		if !strings.Contains(idx, want) {
			t.Errorf("index missing %q", want)
		}
	}
}

// TestCommandsIndexTemplateGolden locks the exact bytes of the commands
// registry (agent init, `context commands new` and `context commands rm`
// regenerate it). The fixture is regenerable: UPDATE_GOLDEN=1 rewrites it
// deliberately instead of hand-editing a literal in this file.
func TestCommandsIndexTemplateGolden(t *testing.T) {
	now := time.Date(2026, 9, 20, 17, 31, 0, 0, time.UTC)
	got := instrCommandsIndexTemplate([]commandIndexEntry{
		{Trigger: "analysis", Contract: "analysis", Kind: commandKindDocument, Payload: "subject or scope of the analysis to create or extend", Examples: commandRowExamples([]string{">analysis: the failing test"})},
		{Trigger: "ingestion", Contract: "ingestion", Kind: commandKindDocument, Payload: "paths and/or URLs to ingest"},
	}, "tm", now)
	checkGolden(t, "commands-index.md", got)
}

// TestCommandsIndexShape checks the properties a byte-golden cannot explain: the
// registry names its contract paths, keeps the grammar and the gates once, and
// offers no per-trigger file to resolve through.
func TestCommandsIndexShape(t *testing.T) {
	now := time.Date(2026, 9, 20, 17, 31, 0, 0, time.UTC)
	idx := instrCommandsIndexTemplate(commandEntriesFor(agentProfileSlugs()), "tm", now)
	for _, want := range []string{
		"### Grammar",
		"| Trigger | Kind | Subject | Durable contract | Payload | Examples |",
		"## Working-context resolution",
		"## Invocation contract",
		"## When not to use",
		"`context/instructions/analysis.md`",
	} {
		if !strings.Contains(idx, want) {
			t.Errorf("registry missing %q", want)
		}
	}
	if strings.Contains(idx, "| Command file |") {
		t.Error("the registry must not route through a per-trigger file")
	}
	if got := strings.Count(idx, "### Grammar"); got != 1 {
		t.Errorf("the grammar is written %d time(s), want once", got)
	}
	if got := strings.Count(idx, "Approve-before-write"); got != 1 {
		t.Errorf("the approve gate is written %d time(s), want once", got)
	}
}

// TestScriptsIndexTemplateGolden locks the seed of context/scripts/index.md.
func TestScriptsIndexTemplateGolden(t *testing.T) {
	want := `---
kind: scripts
summary: "Index of utility scripts in context/scripts/ (name · purpose · example). Keep the table in sync: add a row for every script, remove it when deleted."
---

# Scripts Index

Available utility scripts under ` + "`context/scripts/`" + `. Read this index to
find a script (name, purpose, example), then execute it on demand. General
guidance on when/how to add scripts: see ` + "`context/instructions/scripts.md`" + `.

## Available scripts

| Script | Purpose | Example |
|--------|---------|---------|
| _none yet_ | — | — |

## How to add a script

1. Write the script in ` + "`context/scripts/`" + ` (descriptive lowercase
   name, no timestamp; self-documenting first lines).
2. Register it here: add a row under Available scripts (name, purpose, example
   invocation).
3. Rename or delete rows when scripts change — the index must never lie.
`
	if got := scriptsIndexTemplate; got != want {
		t.Errorf("scripts-index golden mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestTopicsSeedGolden locks the seed of context/topics.yaml.
func TestTopicsSeedGolden(t *testing.T) {
	want := "# context/topics.yaml — controlled vocabulary for the `topics` frontmatter field.\n" +
		"#\n" +
		"# Canonical topics are kebab-case slugs. Aliases are accepted by `sdt context lint`\n" +
		"# and reported with the canonical form. Keep the list small and meaningful; a\n" +
		"# topic is a subject, not an initiative (that is `objective`).\n" +
		"#\n" +
		"# topics:\n" +
		"#   context-search:\n" +
		"#     - search\n" +
		"#     - retrieval\n" +
		"#   agent-harness:\n" +
		"#     - harness\n" +
		"#   knowledge-management:\n" +
		"#     - knowledge\n" +
		"#   document-management:\n" +
		"#     - docs\n" +
		"#   testing:\n" +
		"#     - tests\n" +
		"#   cli:\n" +
		"#     - commands\n"
	if got := topicsRegisterTemplate; got != want {
		t.Errorf("topics seed mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestCategoriesSeedGolden locks the seed of context/categories.yaml.
func TestCategoriesSeedGolden(t *testing.T) {
	want := "# context/categories.yaml — controlled vocabulary for the `categories` frontmatter field.\n" +
		"#\n" +
		"# Canonical categories are kebab-case slugs; a category classifies the kind of\n" +
		"# work a document is (bug, issue, new feature, ...), not its subject (that is\n" +
		"# `topics`). Aliases are accepted by `sdt context lint` and reported with the\n" +
		"# canonical form. Seeded with the agreed set; extend or trim to fit the project.\n" +
		"categories:\n" +
		"  bug:\n" +
		"    - bugfix\n" +
		"  issue: []\n" +
		"  new-feature:\n" +
		"    - feat\n" +
		"  feature-change: []\n" +
		"  refactor: []\n" +
		"  improvement:\n" +
		"    - enhancement\n" +
		"  research: []\n"
	if got := categoriesRegisterTemplate; got != want {
		t.Errorf("categories seed mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestWorkReadmeTemplateGolden locks the static shell of context/README.md and
// verifies the registry-driven Commands block is appended exactly once.
func TestWorkReadmeTemplateGolden(t *testing.T) {
	got := sdtWorkReadmeContent()
	if !strings.HasPrefix(got, "# context/ — Working Directory\n") {
		t.Errorf("README must start with the static title:\n%s", got[:80])
	}
	if !strings.Contains(got, "Store durable facts in `decisions/` (decisions) and `architecture/`; the rest of\n") {
		t.Errorf("README missing the closing static prose")
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("README must end with a trailing newline")
	}
	idx := strings.Index(got, "## Commands")
	if idx < 0 {
		t.Fatalf("README missing Commands section:\n%s", got)
	}
	// Exactly the static shell before the placeholder, then the registry block.
	shell := got[:idx]
	section := got[idx:]
	if strings.Count(section, "## Commands") != 1 {
		t.Errorf("Commands section duplicated")
	}
	if section != sdtWorkCommandsSection() {
		t.Errorf("Commands section mismatch\ngot:\n%s\nwant:\n%s", section, sdtWorkCommandsSection())
	}
	if !strings.Contains(section, ctxTypeHelpText(ctxNewTypes())) {
		t.Errorf("Commands section missing new-types help: %q", ctxTypeHelpText(ctxNewTypes()))
	}
	if !strings.Contains(section, ctxTypeHelpText(ctxPathTypes())) {
		t.Errorf("Commands section missing path-types help: %q", ctxTypeHelpText(ctxPathTypes()))
	}
	if !strings.Contains(section, ctxListHelpText()) {
		t.Errorf("Commands section missing list help: %q", ctxListHelpText())
	}
	if !strings.HasSuffix(shell, "`.env` git-ignored.\n\n") {
		t.Errorf("static shell must end with a blank line before Commands:\n%q", shell[len(shell)-20:])
	}
}
