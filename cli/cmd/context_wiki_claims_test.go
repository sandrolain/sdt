package cmd

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/distill"
)

const claimsArtifact = `---
source: test
---

# Book

# Chapter 1

## Alpha

one
two
three
four
five
six
seven
eight
nine
ten

## Beta

eleven
twelve
thirteen
fourteen
`

const claimsPage = `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: Claim fixture.
sources:
  - refs/converted/book.md
---

## Claims

1. {#claim-1} Broad (refs/converted/book.md@abcdef12:1-26)
2. {#claim-2} Narrow (refs/converted/book.md@abcdef12:9-14)
`

func writeClaimsFixture(t *testing.T) {
	t.Helper()
	writeCtxDoc(t, "context/refs/converted/book.md", claimsArtifact)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", claimsPage)
}

func TestWikiClaimsFlagsBroadClaim(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeClaimsFixture(t)
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/alpha"))
	if !strings.Contains(out, "claim-1") || !strings.Contains(out, "units=3") {
		t.Fatalf("broad claim was not reported:\n%s", out)
	}
	if strings.Contains(out, "claim-2") {
		t.Fatalf("narrow claim was reported:\n%s", out)
	}
}

func TestWikiClaimsThresholds(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeClaimsFixture(t)
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/alpha", "--max-lines", "100", "--max-units", "10"))
	if out != "" {
		t.Fatalf("thresholds should clear the claims:\n%s", out)
	}
}

func TestWikiClaimsJSON(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeClaimsFixture(t)
	out := execute(t, contextWikiClaimsCmd, nil, "architecture/alpha", "--format", "json")
	var findings []claimFinding
	if err := json.Unmarshal(out, &findings); err != nil {
		t.Fatalf("claims JSON is not readable: %v\n%s", err, out)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), out)
	}
	if findings[0].Claim != "claim-1" || findings[0].Units < 2 || findings[0].Lines < 1 {
		t.Errorf("unexpected finding: %+v", findings[0])
	}
}

func TestWikiClaimsLineThreshold(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeClaimsFixture(t)
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/alpha", "--max-lines", "5", "--max-units", "10"))
	if !strings.Contains(out, "claim-2") {
		t.Fatalf("short citation over a tight line threshold was not reported:\n%s", out)
	}
}

func TestWikiClaimsUnresolvedCitation(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/alpha.md", `---
kind: wiki
id: architecture/alpha
title: Alpha
type: concept
status: active
summary: unresolved
sources:
  - refs/converted/book.md
---

## Claims

1. {#claim-1} Missing (refs/converted/missing.md@abcdef12:1-2)
`)
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/alpha", "--format", "json"))
	for _, want := range []string{`"unresolved": true`, `"ref": "refs/converted/missing.md"`} {
		if !strings.Contains(out, want) {
			t.Errorf("unresolved claim is missing %q:\n%s", want, out)
		}
	}
}

func TestWikiClaimsWholeCitationUsesArtifactLines(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeClaimsFixture(t)
	writeCtxDoc(t, "context/wiki/architecture/whole.md", `---
kind: wiki
id: architecture/whole
title: Whole
type: concept
status: active
summary: whole
sources:
  - refs/converted/book.md
---

## Claims

1. {#claim-1} Whole (refs/converted/book.md@abcdef12)
`)
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/whole", "--max-lines", "10", "--max-units", "100"))
	want := "lines=" + strconv.Itoa(distill.BodyLines(claimsArtifact))
	if !strings.Contains(out, want) {
		t.Fatalf("whole citation did not use the artifact lines (%s):\n%s", want, out)
	}
}

// budgetPage renders a page carrying n claim anchors.
func budgetPage(id string, n int) string {
	var b strings.Builder
	b.WriteString("---\nkind: wiki\nid: " + id + "\ntitle: " + id + "\ntype: concept\nstatus: active\nsummary: Budget fixture.\ntags: [test]\nsources:\n  - refs/converted/book.md\n---\n\n## Claims\n\n")
	for i := 1; i <= n; i++ {
		b.WriteString(strconv.Itoa(i) + ". {#claim-" + strconv.Itoa(i) + "} claim (refs/converted/book.md@abcdef12:9)\n")
	}
	return b.String()
}

func TestWikiClaimsBudgetDefaultsToTheLintConstant(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/at-limit.md", budgetPage("architecture/at-limit", contextwiki.BudgetClaims))
	writeCtxDoc(t, "context/wiki/architecture/over.md", budgetPage("architecture/over", contextwiki.BudgetClaims+1))
	out := string(execute(t, contextWikiClaimsCmd, nil, "--budget"))
	if !strings.Contains(out, "architecture/over.md") {
		t.Errorf("a page over the lint constant is not reported:\n%s", out)
	}
	if strings.Contains(out, "architecture/at-limit.md") {
		t.Errorf("a page at the lint constant must not be reported:\n%s", out)
	}
	if contextwiki.BudgetClaims != 20 {
		t.Fatalf("the lint constant moved to %d; the test binds the default to it", contextwiki.BudgetClaims)
	}
}

func TestWikiClaimsBudgetSortsWorstFirst(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/twenty-two.md", budgetPage("architecture/twenty-two", 22))
	writeCtxDoc(t, "context/wiki/architecture/twenty-five.md", budgetPage("architecture/twenty-five", 25))
	out := string(execute(t, contextWikiClaimsCmd, nil, "--budget"))
	if strings.Index(out, "twenty-five.md") > strings.Index(out, "twenty-two.md") {
		t.Errorf("the worst page is not first:\n%s", out)
	}
}

func TestWikiClaimsBudgetAtLowersThreshold(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/at-limit.md", budgetPage("architecture/at-limit", contextwiki.BudgetClaims))
	if out := string(execute(t, contextWikiClaimsCmd, nil, "--budget")); strings.Contains(out, "at-limit.md") {
		t.Errorf("the page is at the default threshold and must not be reported:\n%s", out)
	}
	out := string(execute(t, contextWikiClaimsCmd, nil, "--budget", "--at", "15"))
	if !strings.Contains(out, "at-limit.md") || !strings.Contains(out, "over budget by 5") {
		t.Errorf("--at 15 did not lower the threshold:\n%s", out)
	}
}

func TestWikiClaimsBudgetJSON(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/wiki/architecture/over.md", budgetPage("architecture/over", 23))
	out := execute(t, contextWikiClaimsCmd, nil, "--budget", "--format", "json")
	var findings []claimBudgetFinding
	if err := json.Unmarshal(out, &findings); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(findings) != 1 || findings[0].Claims != 23 || findings[0].Over != 3 {
		t.Errorf("unexpected findings: %+v", findings)
	}
}

func TestWikiClaimsBudgetAtRequiresBudget(t *testing.T) {
	setupContextProject(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextWikiClaimsCmd, nil, "--at", "5"))
	})
}

// validatePage renders a page whose claim prose is given, with extra frontmatter
// fields spliced in above `sources`.
func validatePage(id, extraFM string, claims ...string) string {
	var b strings.Builder
	b.WriteString("---\nkind: wiki\nid: " + id + "\ntitle: " + id + "\ntype: concept\nstatus: active\nsummary: Validate fixture.\n")
	b.WriteString(extraFM)
	b.WriteString("sources:\n  - refs/converted/book.md\n---\n\n## Claims\n\n")
	for i, claim := range claims {
		b.WriteString(strconv.Itoa(i+1) + ". {#claim-" + strconv.Itoa(i+1) + "} " + claim +
			" (refs/converted/book.md@abcdef12:9)\n")
	}
	return b.String()
}

func TestWikiClaimsValidateSelectsAnUncorroboratedPage(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/medium.md",
		validatePage("architecture/medium", "confidence: medium\n", "one", "two"))
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/medium", "--validate"))
	for _, want := range []string{"[VALIDATE]", "page claims=2 signals=confidence", "confidence medium"} {
		if !strings.Contains(out, want) {
			t.Errorf("page shortlist row is missing %q:\n%s", want, out)
		}
	}
}

func TestWikiClaimsValidateSelectsAMarkedClaim(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/inferred.md",
		validatePage("architecture/inferred", "confidence: high\nmaturity: established\n",
			"a plain claim", "b is inferred from the sample"))
	out := execute(t, contextWikiClaimsCmd, nil, "architecture/inferred", "--validate", "--format", "json")
	var rows []validateRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1:\n%s", len(rows), out)
	}
	if rows[0].Target != "claim" || rows[0].Claim != "claim-2" || rows[0].Signals[0] != "inferred" {
		t.Errorf("unexpected row: %+v", rows[0])
	}
}

func TestWikiClaimsValidateSelectsADueEvolvingClaim(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/evolving.md",
		validatePage("architecture/evolving", "confidence: high\nmaturity: established\nvalidity: evolving\nvalid_until: 2020-01-01\n", "one"))
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/evolving", "--validate"))
	if !strings.Contains(out, "signals=validity") || !strings.Contains(out, "2020-01-01") {
		t.Fatalf("a dated claim was not selected:\n%s", out)
	}
}

func TestWikiClaimsValidateHorizonKeepsAFutureClaimOut(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	future := time.Now().AddDate(10, 0, 0).Format("2006-01-02")
	writeCtxDoc(t, "context/wiki/architecture/future.md",
		validatePage("architecture/future", "confidence: high\nmaturity: established\nvalidity: evolving\nvalid_until: "+future+"\n", "one"))
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/future", "--validate", "--horizon", "30"))
	if out != "" {
		t.Fatalf("a claim beyond the horizon was selected:\n%s", out)
	}
}

func TestWikiClaimsValidateSelectsAProvisionalPage(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/draftish.md",
		validatePage("architecture/draftish", "confidence: high\nmaturity: provisional\n", "one"))
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/draftish", "--validate"))
	if !strings.Contains(out, "signals=maturity") || !strings.Contains(out, "maturity provisional") {
		t.Fatalf("a provisional page was not selected:\n%s", out)
	}
}

func TestWikiClaimsValidateLeavesACorroboratedPageOut(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/settled.md",
		validatePage("architecture/settled", "confidence: high\nmaturity: established\n", "one", "two"))
	out := string(execute(t, contextWikiClaimsCmd, nil, "architecture/settled", "--validate"))
	if out != "" {
		t.Fatalf("a corroborated page was selected:\n%s", out)
	}
}

func TestWikiClaimsValidateNamesMarkedClaimsInsideAPageRow(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/both.md",
		validatePage("architecture/both", "confidence: medium\n", "one", "two is inferred"))
	out := execute(t, contextWikiClaimsCmd, nil, "architecture/both", "--validate", "--format", "json")
	var rows []validateRow
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("a page-level signal must not repeat its claims: got %d rows\n%s", len(rows), out)
	}
	if rows[0].Target != "page" || !strings.Contains(rows[0].Reason, "#claim-2") {
		t.Errorf("page row does not name its marked claim: %+v", rows[0])
	}
}

func TestWikiClaimsValidateRejectsBudget(t *testing.T) {
	setupContextProject(t)
	stubWikiExit(t)
	writeCtxDoc(t, "context/wiki/architecture/medium.md",
		validatePage("architecture/medium", "confidence: medium\n", "one"))
	out := string(execute(t, contextWikiClaimsCmd, nil, "--validate", "--budget"))
	if out != "" {
		t.Fatalf("two reports were answered at once:\n%s", out)
	}
}
