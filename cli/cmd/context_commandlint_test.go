package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContextLintCommandsClean(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/commands/clean.md", `---
kind: commands
id: commands/clean
summary: A self-contained trigger prompt
links:
  - instructions/plan.md
sources:
  - instructions/plan.md
---
body

## When not to use
- informational only
`)
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), `"clean.md"`) {
		t.Errorf("expected no findings for the clean command file:\n%s", out)
	}
}

func TestContextLintCommandZeroRefsAllowed(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/commands/standalone.md", `---
kind: commands
id: commands/standalone
summary: Self-contained prompt with no instruction reference
---
body

## When not to use
- informational only
`)
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if strings.Contains(string(out), `"standalone.md"`) {
		t.Errorf("expected no findings for a zero-reference (self-contained) command:\n%s", out)
	}
}

func TestContextLintCommandsKindIDSummaryViolations(t *testing.T) {
	setupContextProject(t)
	summary := strings.Repeat("x", 1025)
	writeCtxDoc(t, "context/commands/bad.md", "---\nkind: plan\nid: commands/nope\nsummary: "+summary+"\n---\nbody\n")
	out := execute(t, contextLintCmd, nil, "--format", "json")
	var issues []ctxLintIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("invalid lint JSON: %v\n%s", err, out)
	}
	find := func(msg string) bool {
		for _, it := range issues {
			if strings.Contains(it.Message, msg) {
				return true
			}
		}
		return false
	}
	if !find("`kind` must be `commands`") {
		t.Errorf("expected kind mismatch warning:\n%s", out)
	}
	if !find("`id` must be `commands/bad`") {
		t.Errorf("expected id mismatch warning:\n%s", out)
	}
	if !find("`summary` is 1025 chars (max 1024)") {
		t.Errorf("expected summary cap warning:\n%s", out)
	}
}

func TestContextLintCommandUnresolvedInstructionRef(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/commands/broken.md", `---
kind: commands
id: commands/broken
summary: Refers to a missing contract
sources:
  - instructions/does-not-exist.md
---
body

## When not to use
- informational only
`)
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), "unresolved instruction reference: instructions/does-not-exist.md") {
		t.Errorf("expected unresolved instruction reference warning:\n%s", out)
	}
	if !strings.Contains(string(out), "fix the target path in the command frontmatter") {
		t.Errorf("expected remediation hint:\n%s", out)
	}
	if strings.Contains(string(out), "broken source reference: instructions/does-not-exist.md") {
		t.Errorf("generic broken-source warning must be suppressed for command instruction refs (no duplicate):\n%s", out)
	}
}

func TestContextLintCommandWhenNotToUseSuggestion(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/commands/nosection.md", `---
kind: commands
id: commands/nosection
summary: Trigger without a when-not-to-use section
---
body
`)
	out := execute(t, contextLintCmd, nil, "--format", "json")
	if !strings.Contains(string(out), ctxLintSuggestion) || !strings.Contains(string(out), "When not to use") {
		t.Errorf("expected a SUGGESTION for the missing when-not-to-use section:\n%s", out)
	}
}

func TestContextReindexIncludesCommands(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/commands/clean.md", "---\nkind: commands\nid: commands/clean\nsummary: Command summary\n---\nbody\n")
	execute(t, contextReindexCmd, nil)
	idx := mustReadFile(t, "context/index.md")
	if !strings.Contains(idx, "[[commands/clean.md]]") {
		t.Errorf("expected commands doc in index:\n%s", idx)
	}
	if !strings.Contains(idx, "Command summary") {
		t.Errorf("expected command summary in index:\n%s", idx)
	}
}
