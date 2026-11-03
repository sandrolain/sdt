package cmd

import (
	"strings"
	"testing"
)

const claimPagePath = "context/wiki/architecture/page.md"
const claimPageRef = "wiki/architecture/page.md"

func TestContextEditMintsClaimIDs(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, claimPagePath, "---\nkind: wiki\ntitle: page\nsummary: a page\nstatus: draft\n---\n\n## Claims\n\n1. {#claim-1} a fact (refs/converted/x.md@abcdef12:1)\n")

	prev := newClaimID
	newClaimID = func() string { return "c-testid01" }
	t.Cleanup(func() { newClaimID = prev })

	execute(t, contextEditCmd, nil, claimPageRef, "--section", "Claims", "--input", "## Claims\n\n1. {#claim-1} a fact (refs/converted/x.md@abcdef12:1)\n")
	got := readDoc(t, claimPageRef)
	if !strings.Contains(got, "{#claim-1} <!-- c-testid01 -->") {
		t.Fatalf("claim anchor was not stamped:\n%s", got)
	}
	if strings.Count(got, "c-testid01") != 1 {
		t.Fatalf("id stamped more than once:\n%s", got)
	}
}

func TestContextEditKeepsAnExistingClaimID(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, claimPagePath, "---\nkind: wiki\ntitle: page\nsummary: a page\nstatus: draft\n---\n\n## Claims\n\n1. {#claim-1} <!-- c-existin1 --> a fact (refs/converted/x.md@abcdef12:1)\n")

	prev := newClaimID
	newClaimID = func() string { return "c-shouldnt" }
	t.Cleanup(func() { newClaimID = prev })

	execute(t, contextEditCmd, nil, claimPageRef, "--section", "Claims", "--input", "## Claims\n\n1. {#claim-1} <!-- c-existin1 --> a rewritten fact (refs/converted/x.md@abcdef12:1)\n")
	got := readDoc(t, claimPageRef)
	if !strings.Contains(got, "<!-- c-existin1 -->") {
		t.Fatalf("existing id was lost:\n%s", got)
	}
	if strings.Contains(got, "c-shouldnt") {
		t.Fatalf("an existing id was regenerated:\n%s", got)
	}
}

func TestContextEditMintsOnlyValidClaimAnchors(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, claimPagePath, "---\nkind: wiki\ntitle: page\nsummary: a page\nstatus: draft\n---\n\n## Notes\n\ntext\n")

	prev := newClaimID
	newClaimID = func() string { return "c-never000" }
	t.Cleanup(func() { newClaimID = prev })

	execute(t, contextEditCmd, nil, claimPageRef, "--section", "Notes", "--input", "## Notes\n\ntext without a claim\n")
	got := readDoc(t, claimPageRef)
	if strings.Contains(got, "c-never000") {
		t.Fatalf("an id was minted without a claim anchor:\n%s", got)
	}
}
