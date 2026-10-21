package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestEssentialsIsBoundedAndNamesWhatMatters is the P7 gate: the session-start
// digest is capped by a constant, it carries the architecture summaries and the
// decisions in force, and it leaves out the decisions that no longer apply.
func TestEssentialsIsBoundedAndNamesWhatMatters(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/architecture/stack.md",
		"---\nkind: architecture\ntitle: Stack\nsummary: the declared stack and its constraints\nstatus: current\n---\nbody\n")
	writeCtxDoc(t, "context/decisions/0001-accepted.md",
		"---\nkind: decision\ntitle: One verb per job\nsummary: merge the overlapping commands\nstatus: accepted\n---\nbody\n")
	writeCtxDoc(t, "context/decisions/0002-gone.md",
		"---\nkind: decision\ntitle: Old choice\nsummary: superseded long ago\nstatus: deprecated\n---\nbody\n")

	execute(t, contextReindexCmd, nil)

	data, err := os.ReadFile(sdtEssentialsFile)
	if err != nil {
		t.Fatalf("reindex did not write %s: %v", sdtEssentialsFile, err)
	}
	content := string(data)
	body := content[strings.Index(content, "\n---\n")+5:]

	if got := countWords(body); got > ctxEssentialsWordBudget {
		t.Errorf("essentials is %d words, budget %d — the session-start read is unbounded again", got, ctxEssentialsWordBudget)
	}
	for _, want := range []string{"architecture/stack.md", "merge the overlapping commands", "the declared stack"} {
		if !strings.Contains(content, want) {
			t.Errorf("essentials does not name %q", want)
		}
	}
	if strings.Contains(content, "superseded long ago") {
		t.Error("essentials carries a decision that is no longer in force")
	}
}

// TestEssentialsKeepsTheNewestDecisions is the ordering gate: the digest carries
// what is in force, so when the budget cannot hold every decision it must spend
// from the recent end and drop the oldest, saying so.
func TestEssentialsKeepsTheNewestDecisions(t *testing.T) {
	setupContextProject(t)
	long := strings.TrimSpace(strings.Repeat("an evidence led decision about the corpus and its writers ", 12))
	for i := 1; i <= 12; i++ {
		writeCtxDoc(t, fmt.Sprintf("context/decisions/%04d-d%02d.md", i, i),
			fmt.Sprintf("---\nkind: decision\ntitle: D%02d\nsummary: %s\nstatus: accepted\n---\nbody\n", i, long))
	}

	execute(t, contextReindexCmd, nil)

	data, err := os.ReadFile(sdtEssentialsFile)
	if err != nil {
		t.Fatalf("reindex did not write %s: %v", sdtEssentialsFile, err)
	}
	content := string(data)
	if !strings.Contains(content, "0012-d12.md") {
		t.Error("the newest decision is missing from the digest")
	}
	if strings.Contains(content, "0001-d01.md") {
		t.Error("the oldest decision survived the cut: the digest is spending its budget from the wrong end")
	}
	if !strings.Contains(content, "oldest are dropped first") {
		t.Errorf("the omission line does not say which end was cut:\n%s", content)
	}
}

// TestEssentialsIsIdempotent proves a second reindex does not churn the file: a
// timestamped rewrite would make every session look like a corpus change.
func TestEssentialsIsIdempotent(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/architecture/stack.md",
		"---\nkind: architecture\ntitle: Stack\nsummary: the declared stack\nstatus: current\n---\nbody\n")
	execute(t, contextReindexCmd, nil)
	first, err := os.ReadFile(sdtEssentialsFile)
	if err != nil {
		t.Fatal(err)
	}
	execute(t, contextReindexCmd, nil)
	second, err := os.ReadFile(sdtEssentialsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("reindex rewrote an unchanged essentials digest")
	}
}
