package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/ctxrel"
)

// objectiveCorpus writes one analysis and one plan and returns the plan file
// list and typed relations the coverage check takes.
func objectiveCorpus(t *testing.T, analysisBody, planBody string) ([]string, *ctxrel.Edges) {
	t.Helper()
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md",
		"---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n\n"+analysisBody)
	writeCtxDoc(t, "context/plan/p.md",
		"---\nkind: plan\nuid: uid-p\nanalysis_id: uid-a\nsummary: p\nstatus: active\n---\n\n"+planBody)
	edges, err := ctxrel.Load(sdtWorkDir)
	if err != nil {
		t.Fatal(err)
	}
	planFiles, err := dirFiles(sdtPlanDir)
	if err != nil {
		t.Fatal(err)
	}
	return planFiles, edges
}

func TestParseAnalysisObjectives(t *testing.T) {
	t.Run("reads the checklist ids and their state", func(t *testing.T) {
		objectives, ok := parseAnalysisObjectives("## Objectives\n\n" +
			"- [x] B1 finish the reconciler <!-- c1 -->\n" +
			"- [ ] B2 add the resume view <!-- c2 -->\n")
		if !ok {
			t.Fatal("section not recognized")
		}
		if len(objectives) != 2 {
			t.Fatalf("objectives = %#v, want two", objectives)
		}
		if objectives[0].ID != "B1" || !objectives[0].Done || objectives[0].Text != "finish the reconciler" {
			t.Fatalf("objective 1 = %#v", objectives[0])
		}
		if objectives[1].ID != "B2" || objectives[1].Done {
			t.Fatalf("objective 2 = %#v", objectives[1])
		}
	})

	t.Run("an absent section is graceful", func(t *testing.T) {
		if _, ok := parseAnalysisObjectives("## Problem statement\n\n- [ ] B1 prose\n"); ok {
			t.Fatal("a document without `## Objectives` must not be read as declaring objectives")
		}
	})

	t.Run("a section holding prose only declares nothing", func(t *testing.T) {
		if _, ok := parseAnalysisObjectives("## Objectives\n\nThe reconciler exists.\n"); ok {
			t.Fatal("prose alone must not count as a declaration")
		}
	})

	t.Run("an id quoted later in the sentence is prose", func(t *testing.T) {
		// `**B1.**` sentences and `| B1 |` table cells are the shapes that made a
		// prose scrape report 87 false positives on this corpus.
		objectives, ok := parseAnalysisObjectives("## Objectives\n\n" +
			"- [ ] Finish the reconciler (see B1)\n")
		if ok || len(objectives) != 0 {
			t.Fatalf("objectives = %#v, want a non-leading id ignored", objectives)
		}
	})

	t.Run("only a level-2 section is the declaration", func(t *testing.T) {
		if _, ok := parseAnalysisObjectives("### Objectives\n\n- [ ] B1 x\n"); ok {
			t.Fatal("a level-3 `Objectives` must not be read as the declaration")
		}
	})

	t.Run("the heading match is case-insensitive", func(t *testing.T) {
		if _, ok := parseAnalysisObjectives("## objectives\n\n- [ ] B1 x\n"); !ok {
			t.Fatal("heading case must not change the contract")
		}
	})
}

func TestParsePlanPhaseCoverage(t *testing.T) {
	t.Run("reads the covers token of every phase", func(t *testing.T) {
		got := parsePlanPhaseCoverage("context/plan/p.md", "### Phase 1 — a\n\n**Covers:** analysis B1.\n\n"+
			"### Phase 2 — b\n\n**Depends on:** Phase 1. **Covers:** analysis B2, B3.\n\n"+
			"### Phase 3 — c\n\nno claim here\n")
		if len(got) != 2 {
			t.Fatalf("coverage = %#v, want only the two phases that claim", got)
		}
		if got[0].Phase != "1" || len(got[0].IDs) != 1 || got[0].IDs[0] != "B1" {
			t.Fatalf("phase 1 = %#v, want the identity token as the label", got[0])
		}
		if got[1].Phase != "2" || len(got[1].IDs) != 2 {
			t.Fatalf("phase 2 = %#v", got[1])
		}
	})

	t.Run("a duplicate claim is one objective", func(t *testing.T) {
		got := parsePlanPhaseCoverage("context/plan/p.md", "### Phase 1\n\n**Covers:** B1 and B1\n")
		if len(got) != 1 || len(got[0].IDs) != 1 {
			t.Fatalf("coverage = %#v, want the duplicate collapsed", got)
		}
	})

	t.Run("a non-phase section is not a claim", func(t *testing.T) {
		if got := parsePlanPhaseCoverage("context/plan/p.md", "## Objective\n\n**Covers:** B1\n"); len(got) != 0 {
			t.Fatalf("coverage = %#v, want none", got)
		}
	})
}

func TestLintObjectivePhaseCoverage(t *testing.T) {
	t.Run("a claim the analysis does not declare is a warning on the plan", func(t *testing.T) {
		files, edges := objectiveCorpus(t,
			"## Objectives\n\n- [ ] B1 reconciler <!-- c1 -->\n",
			"### Phase 1\n\n**Covers:** analysis B1.\n\n### Phase 2\n\n**Covers:** analysis B7.\n")
		issues := lintObjectivePhaseCoverage(files, edges)
		if len(issues) != 1 {
			t.Fatalf("issues = %#v, want only the undeclared B7 claim", issues)
		}
		if issues[0].Priority != ctxLintWarning || issues[0].Path != "context/plan/p.md" {
			t.Fatalf("issue = %#v, want a WARNING addressed at the plan", issues[0])
		}
		if !strings.Contains(issues[0].Message, "B7") || !strings.Contains(issues[0].Message, "context/analysis/a.md") {
			t.Fatalf("message = %q, want it to name the undeclared id and the analysis", issues[0].Message)
		}
	})

	t.Run("a declared objective covered by a phase is silent", func(t *testing.T) {
		files, edges := objectiveCorpus(t,
			"## Objectives\n\n- [ ] B1 reconciler <!-- c1 -->\n- [ ] B2 resume view <!-- c2 -->\n",
			"### Phase 1\n\n**Covers:** analysis B1.\n\n### Phase 2\n\n**Covers:** analysis B2.\n")
		if issues := lintObjectivePhaseCoverage(files, edges); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none when both directions reconcile", issues)
		}
	})

	t.Run("an objective no phase covers is a suggestion on the analysis", func(t *testing.T) {
		files, edges := objectiveCorpus(t,
			"## Objectives\n\n- [ ] B1 reconciler <!-- c1 -->\n- [ ] B2 resume view <!-- c2 -->\n",
			"### Phase 1\n\n**Covers:** analysis B1.\n")
		issues := lintObjectivePhaseCoverage(files, edges)
		if len(issues) != 1 {
			t.Fatalf("issues = %#v, want only the uncovered B2", issues)
		}
		if issues[0].Priority != ctxLintSuggestion || issues[0].Path != "context/analysis/a.md" {
			t.Fatalf("issue = %#v, want a SUGGESTION addressed at the analysis", issues[0])
		}
		if !strings.Contains(issues[0].Message, "B2") {
			t.Fatalf("message = %q, want it to name the uncovered objective", issues[0].Message)
		}
	})

	t.Run("an analysis without the section is skipped in both directions", func(t *testing.T) {
		files, edges := objectiveCorpus(t,
			"## Problem statement\n\n- [ ] B1 reconciler <!-- c1 -->\n",
			"### Phase 1\n\n**Covers:** analysis B1.\n\n### Phase 2\n\n**Covers:** analysis Z9.\n")
		if issues := lintObjectivePhaseCoverage(files, edges); len(issues) != 0 {
			t.Fatalf("issues = %#v, want none: an undeclared section is a valid state", issues)
		}
	})

	t.Run("a plan that only cites the analysis is not a derived plan", func(t *testing.T) {
		runInTempDir(t)
		writeCtxDoc(t, "context/analysis/a.md",
			"---\nkind: analysis\nuid: uid-a\nsummary: a\nstatus: active\n---\n\n## Objectives\n\n- [ ] B1 reconciler <!-- c1 -->\n")
		// No `analysis_id`: the plan mentions the analysis in `sources`, which is
		// traceability prose, not the typed derivation.
		writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: uid-p\nsummary: p\nstatus: active\n"+
			"sources:\n  - analysis/a.md\n---\n\n### Phase 1\n\n**Covers:** analysis B1.\n")
		edges, err := ctxrel.Load(sdtWorkDir)
		if err != nil {
			t.Fatal(err)
		}
		files, err := dirFiles(sdtPlanDir)
		if err != nil {
			t.Fatal(err)
		}
		issues := lintObjectivePhaseCoverage(files, edges)
		if len(issues) != 1 || issues[0].Path != "context/analysis/a.md" {
			t.Fatalf("issues = %#v, want B1 reported uncovered and no warning on the citing plan", issues)
		}
	})

	t.Run("both messages carry a curated hint", func(t *testing.T) {
		files, edges := objectiveCorpus(t,
			"## Objectives\n\n- [ ] B1 reconciler <!-- c1 -->\n",
			"### Phase 1\n\n**Covers:** analysis B7.\n")
		issues := lintObjectivePhaseCoverage(files, edges)
		if len(issues) != 2 {
			t.Fatalf("issues = %#v, want the undeclared claim and the uncovered objective", issues)
		}
		for _, issue := range issues {
			if ctxLintHint(issue.Message) == "" {
				t.Fatalf("no curated hint for %q", issue.Message)
			}
		}
	})
}
