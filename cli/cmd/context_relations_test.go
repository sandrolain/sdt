package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func listHas(content, field, value string) bool {
	for _, v := range parseFrontmatterList(content, field) {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}

func TestContextNewPlanStampsAnalysisRelation(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	analysisPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil, "--type", "analysis", "--title", "source analysis", "--objective", "rel")))
	analysisUID := parseFrontmatterField(mustReadFile(t, analysisPath), "uid")

	planPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil, "--type", "plan", "--title", "rel plan", "--source", "analysis/"+filepath.Base(analysisPath))))
	planContent := mustReadFile(t, planPath)
	planUID := parseFrontmatterField(planContent, "uid")

	if got := parseFrontmatterField(planContent, "analysis_id"); got != analysisUID {
		t.Errorf("plan analysis_id = %q, want %q", got, analysisUID)
	}
	if !listHas(mustReadFile(t, analysisPath), "plans_ids", planUID) {
		t.Errorf("analysis plans_ids missing plan uid %s:\n%s", planUID, mustReadFile(t, analysisPath))
	}
}

func TestContextTaskStampsPlanRelation(t *testing.T) {
	setupContextProject(t)
	stubContextNow(t, time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC))

	planPath := strings.TrimSpace(string(execute(t, contextNewCmd, nil, "--type", "plan", "--title", "rel plan")))
	planUID := parseFrontmatterField(mustReadFile(t, planPath), "uid")

	execute(t, contextTaskAddCmd, nil, "--phase", "1", "--plan", filepath.Base(planPath), "step one")
	taskPath := filepath.Join("context", "tasks", "20260806-070000-rel-plan.md")
	taskContent := mustReadFile(t, taskPath)
	taskUID := parseFrontmatterField(taskContent, "uid")

	if got := parseFrontmatterField(taskContent, "plan_id"); got != planUID {
		t.Errorf("task plan_id = %q, want %q", got, planUID)
	}
	if !listHas(mustReadFile(t, planPath), "tasks_ids", taskUID) {
		t.Errorf("plan tasks_ids missing task uid %s:\n%s", taskUID, mustReadFile(t, planPath))
	}
}

func TestContextRelationsBackfill(t *testing.T) {
	runInTempDir(t)
	analysisUID, planUID, taskUID := testUIDv7(1), testUIDv7(2), testUIDv7(3)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: "+analysisUID+"\nsummary: a\nlinks: none\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: "+planUID+"\nsummary: p\nsources:\n  - analysis/a.md\n---\n")
	writeCtxDoc(t, "context/tasks/t.md", "---\nkind: tasks\nuid: "+taskUID+"\nsummary: t\nsources:\n  - plan/p.md\n---\n")

	if _, err := runRelationsBackfill(false); err == nil {
		t.Fatal("relation backfill must require the uid backfill first")
	}

	execute(t, contextUIDBackfillCmd, nil) // writes the marker

	out := string(execute(t, contextRelationsBackfillCmd, nil, "--dry-run"))
	if !strings.Contains(out, "would link 2") {
		t.Fatalf("dry-run report = %q, want 2 would-link", out)
	}
	if strings.Contains(mustReadFile(t, "context/plan/p.md"), "analysis_id:") {
		t.Fatal("dry-run must not write the typed relation")
	}

	execute(t, contextRelationsBackfillCmd, nil)
	if got := parseFrontmatterField(mustReadFile(t, "context/plan/p.md"), "analysis_id"); got != analysisUID {
		t.Errorf("plan analysis_id = %q, want %q", got, analysisUID)
	}
	if !listHas(mustReadFile(t, "context/analysis/a.md"), "plans_ids", planUID) {
		t.Error("analysis plans_ids missing plan uid")
	}
	if got := parseFrontmatterField(mustReadFile(t, "context/tasks/t.md"), "plan_id"); got != planUID {
		t.Errorf("task plan_id = %q, want %q", got, planUID)
	}
	if !listHas(mustReadFile(t, "context/plan/p.md"), "tasks_ids", taskUID) {
		t.Error("plan tasks_ids missing task uid")
	}

	second := string(execute(t, contextRelationsBackfillCmd, nil))
	if !strings.Contains(second, "linked 0") {
		t.Fatalf("relation backfill must be idempotent, got %q", second)
	}
}

func TestLintParentRelations(t *testing.T) {
	runInTempDir(t)
	analysisUID, planUID, orphanUID := testUIDv7(1), testUIDv7(2), testUIDv7(9)

	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: "+analysisUID+"\nsummary: a\nlinks: none\nplans_ids:\n  - "+planUID+"\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: "+planUID+"\nsummary: p\nanalysis_id: "+analysisUID+"\nsources:\n  - analysis/a.md\n---\n")
	writeCtxDoc(t, "context/plan/orphan.md", "---\nkind: plan\nuid: "+testUIDv7(3)+"\nsummary: orphan\nanalysis_id: "+orphanUID+"\nsources:\n  - analysis/a.md\n---\n")
	writeCtxDoc(t, "context/tasks/t.md", "---\nkind: tasks\nuid: "+testUIDv7(4)+"\nsummary: t\nsources:\n  - plan/p.md\n---\n")

	issues := lintParentRelations([]string{
		"context/analysis/a.md", "context/plan/p.md", "context/plan/orphan.md", "context/tasks/t.md",
	})
	var warns, suggests int
	for _, it := range issues {
		switch it.Priority {
		case ctxLintWarning:
			warns++
			if !strings.Contains(it.Message, "does not resolve") {
				t.Errorf("unexpected WARNING: %s", it.Message)
			}
		case ctxLintSuggestion:
			suggests++
			if !strings.Contains(it.Message, "has no `plan_id`") {
				t.Errorf("unexpected SUGGESTION: %s", it.Message)
			}
		}
	}
	if warns != 1 {
		t.Errorf("warnings = %d, want 1 (orphan analysis_id)", warns)
	}
	if suggests != 1 {
		t.Errorf("suggestions = %d, want 1 (task missing plan_id)", suggests)
	}
}

func TestLintPlanCitingASecondAnalysis(t *testing.T) {
	runInTempDir(t)
	analysisUID, otherUID, planUID := testUIDv7(1), testUIDv7(2), testUIDv7(3)

	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: "+analysisUID+"\nsummary: a\n---\n")
	writeCtxDoc(t, "context/analysis/b.md", "---\nkind: analysis\nuid: "+otherUID+"\nsummary: b\n---\n")
	// The split-sibling shape: the plan derives from a, and cites b in `sources`
	// as correlation.
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: "+planUID+"\nsummary: p\nanalysis_id: "+analysisUID+"\nsources:\n  - analysis/a.md\n  - analysis/b.md\n---\n")

	issues := lintParentRelations([]string{"context/analysis/a.md", "context/analysis/b.md", "context/plan/p.md"})
	var found []string
	for _, it := range issues {
		if it.Priority == ctxLintSuggestion {
			found = append(found, it.Message)
		}
	}
	if len(found) != 1 {
		t.Fatalf("suggestions = %#v, want exactly the second-analysis notice", found)
	}
	for _, want := range []string{"b.md", "exactly one analysis", "`links`"} {
		if !strings.Contains(found[0], want) {
			t.Errorf("notice %q does not mention %q", found[0], want)
		}
	}

	// moving the citation to `links` clears it: correlation is legal.
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: "+planUID+"\nsummary: p\nanalysis_id: "+analysisUID+"\nsources:\n  - analysis/a.md\nlinks:\n  - analysis/b.md\n---\n")
	issues = lintParentRelations([]string{"context/analysis/a.md", "context/analysis/b.md", "context/plan/p.md"})
	for _, it := range issues {
		if it.Priority == ctxLintSuggestion {
			t.Errorf("a `links` citation must not be reported: %s", it.Message)
		}
	}
}

func TestRelationsBackfillPrunesADanglingReverseEntry(t *testing.T) {
	runInTempDir(t)
	analysisUID, planUID, taskUID := testUIDv7(1), testUIDv7(2), testUIDv7(3)
	dangling := testUIDv7(8)

	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: "+analysisUID+"\nsummary: a\nplans_ids:\n  - "+dangling+"\n  - "+planUID+"\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: "+planUID+"\nsummary: p\nanalysis_id: "+analysisUID+"\n---\n")
	writeCtxDoc(t, "context/tasks/t.md", "---\nkind: tasks\nuid: "+taskUID+"\nsummary: t\nplan_id: "+planUID+"\n---\n")

	if !listHas(mustReadFile(t, "context/analysis/a.md"), "plans_ids", dangling) {
		t.Fatal("fixture must start with a dangling entry")
	}
	execute(t, contextUIDBackfillCmd, nil) // writes the marker
	out := string(execute(t, contextRelationsBackfillCmd, nil, "--dry-run"))
	if !strings.Contains(out, "context/analysis/a.md") {
		t.Errorf("dry-run must list the document it would prune: %q", out)
	}
	execute(t, contextRelationsBackfillCmd, nil)

	after := mustReadFile(t, "context/analysis/a.md")
	if listHas(after, "plans_ids", dangling) {
		t.Errorf("backfill must prune a uid that resolves to no document:\n%s", after)
	}
	if !listHas(after, "plans_ids", planUID) {
		t.Errorf("backfill must keep the real child:\n%s", after)
	}
	if second := string(execute(t, contextRelationsBackfillCmd, nil)); !strings.Contains(second, "linked 0") {
		t.Errorf("the reconcile must be idempotent, got %q", second)
	}
}

// TestRewriteFrontmatterListKeepsTheShapeItFinds is the gate for the shape this
// writer used to mix. It assumed a block list: given a flow value it appended `- `
// items under the key's own line, and the document stopped parsing. The table
// crosses the shape the key carries with the operations the two callers perform,
// and asserts the shape, the exact values, the untouched remainder and that the
// result still parses.
func TestRewriteFrontmatterListKeepsTheShapeItFinds(t *testing.T) {
	const head = "---\nkind: analysis\nsummary: \"s\"\n"
	const rest = "agent: opencode\n---\nbody\n"
	flow := "plans_ids: ['uid-a']"
	block := "plans_ids:\n  - uid-a"
	mixed := "plans_ids: ['uid-a']\n  - uid-a"

	cases := []struct {
		name   string
		key    string // "" means the key is absent
		call   func(string) (string, bool)
		expect string
	}{
		{"append to a flow key keeps it flow", flow,
			func(c string) (string, bool) { return appendFrontmatterListValue(c, "plans_ids", "uid-b") },
			"plans_ids: ['uid-a','uid-b']"},
		{"append to a block key keeps it block", block,
			func(c string) (string, bool) { return appendFrontmatterListValue(c, "plans_ids", "uid-b") },
			"plans_ids:\n  - uid-a\n  - uid-b"},
		{"append of an existing value is a no-op in either shape (flow)", flow,
			func(c string) (string, bool) { return appendFrontmatterListValue(c, "plans_ids", "uid-a") },
			flow},
		{"append of an existing value is a no-op in either shape (block)", block,
			func(c string) (string, bool) { return appendFrontmatterListValue(c, "plans_ids", "uid-a") },
			block},
		{"an absent key is created in the block form", "",
			func(c string) (string, bool) { return appendFrontmatterListValue(c, "plans_ids", "uid-b") },
			"plans_ids:\n  - uid-b"},
		{"reconcile prunes and stays flow", flow,
			func(c string) (string, bool) { return reconcileFrontmatterList(c, "plans_ids", []string{"uid-b"}) },
			"plans_ids: ['uid-b']"},
		{"reconcile prunes and stays block", block,
			func(c string) (string, bool) { return reconcileFrontmatterList(c, "plans_ids", []string{"uid-b"}) },
			"plans_ids:\n  - uid-b"},
		{"reconcile to nothing empties the flow key", flow,
			func(c string) (string, bool) { return reconcileFrontmatterList(c, "plans_ids", nil) },
			"plans_ids: []"},
		{"a mixed key is repaired to the shape on its line", mixed,
			func(c string) (string, bool) { return reconcileFrontmatterList(c, "plans_ids", []string{"uid-a"}) },
			"plans_ids: ['uid-a']"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := head + tc.key + "\n" + rest
			if tc.key == "" {
				before = head + rest
			}
			after, _ := tc.call(before)
			if !strings.Contains(after, tc.expect) {
				t.Errorf("key lines = %q, want %q\nafter:\n%s", after, tc.expect, after)
			}
			// The key and its value lines are the only thing that may move: the
			// head, the body and every other frontmatter line stay as they were.
			if !strings.HasPrefix(after, head) || !strings.HasSuffix(after, "---\nbody\n") {
				t.Errorf("the head or the body changed:\n%s", after)
			}
			if n := strings.Count(after, "agent: opencode"); n != 1 {
				t.Errorf("another frontmatter line was lost or duplicated (%d occurrences):\n%s", n, after)
			}
			if !frontmatterIsWellFormed(after) {
				t.Errorf("the result does not parse:\n%s", after)
			}
		})
	}
}

// TestRelationsBackfillKeepsAFlowKeyParsable runs the trigger that corrupted two
// documents on this corpus: an analysis whose `plans_ids` is a flow list, and a
// plan whose `sources` name it and which carries no `analysis_id`. The writer
// appended block items under the flow value; the assertion is the lint's own
// verdict plus one value plus idempotence, because "one link reported" was true of
// the broken run too — the reader handled the shape and the writer did not.
func TestRelationsBackfillKeepsAFlowKeyParsable(t *testing.T) {
	setupContextProject(t)
	writeTestFile(t, ctxUIDBackfillMarker, "{}\n")

	const analysisUID = "01a13e15-e485-762d-a20b-8aaaff76c72e"
	const planUID = "01a13e87-d5d7-7641-bfca-7c5eee179390"
	analysisPath := "context/analysis/20260806-070000-src.md"
	writeTestFile(t, analysisPath,
		"---\nkind: analysis\nuid: "+analysisUID+"\nsummary: \"s\"\nstatus: active\nplans_ids: ['"+planUID+"']\n---\n\nbody\n")
	writeTestFile(t, "context/plan/20260806-070000-p.md",
		"---\nkind: plan\nuid: "+planUID+"\nsummary: \"p\"\nstatus: active\nsources: ['analysis/20260806-070000-src.md']\n---\n\nbody\n")

	if _, err := runRelationsBackfill(false); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	after := mustReadFile(t, analysisPath)
	if !frontmatterIsWellFormed(after) {
		t.Fatalf("the backfill left an unparseable frontmatter:\n%s", after)
	}
	if got := parseFrontmatterList(after, "plans_ids"); len(got) != 1 || strings.TrimSpace(got[0]) != planUID {
		t.Errorf("plans_ids = %v, want exactly the plan uid", got)
	}
	if !strings.Contains(after, "plans_ids: ['"+planUID+"']") {
		t.Errorf("the flow shape was not kept:\n%s", after)
	}
	out := string(execute(t, contextLintCmd, nil, analysisPath))
	if strings.Contains(out, "[CRITICAL]") || !strings.Contains(out, "0 CRITICAL") {
		t.Errorf("the lint reports a critical finding:\n%s", out)
	}

	// A second run must change nothing.
	second := mustReadFile(t, analysisPath)
	if _, err := runRelationsBackfill(false); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if got := mustReadFile(t, analysisPath); got != second {
		t.Errorf("a second run changed the document:\nbefore:\n%s\nafter:\n%s", second, got)
	}
}
