package cmd

import (
	"strings"
	"testing"
)

// TestContextSetUIDGuard: replacing a child's uid with a value its parent does
// not record is refused, and the error names the id the parent holds — the value
// the caller almost always meant. The legitimate repair (writing that very id)
// passes, and a document with no parent is not guarded at all.
func TestContextSetUIDGuard(t *testing.T) {
	runInTempDir(t)

	const (
		analysisUID = "01a19cb6-6d3d-7b77-8f43-1c0e2ac2f0af"
		planUID     = "01a19d02-ac85-7bd7-a415-719955bdc72f"
		strayUID    = "01a19d02-d596-73e3-8df1-ed80f6e4864d"
	)
	writeContextDoc(t, "analysis/a.md", "---\nkind: analysis\nuid: "+analysisUID+
		"\nstatus: active\nplans_ids: ["+planUID+"]\n---\n\n## Problem statement\n")
	writeContextDoc(t, "plan/p.md", "---\nkind: plan\nuid: "+planUID+
		"\nstatus: active\nanalysis_id: "+analysisUID+"\n---\n\n## Objective\n")

	plan, ok := ctxTypeLookup(ctxTypePlan)
	if !ok {
		t.Fatal("plan is not in the registry")
	}
	doc := ctxResolvedDoc{Path: "context/plan/p.md", Type: plan}

	if err := guardRelationUID(doc, planUID); err != nil {
		t.Errorf("the id the parent records must be accepted, got %v", err)
	}
	err := guardRelationUID(doc, strayUID)
	if err == nil {
		t.Fatal("a uid the parent does not record must be refused")
	}
	if !strings.Contains(err.Error(), planUID) {
		t.Errorf("the error must name the recorded id %q, got %v", planUID, err)
	}

	// A parent that records no children yet cannot disagree with anything.
	const looseAnalysisUID = "01a19d10-1111-7aaa-8bbb-ccccddddeeee"
	writeContextDoc(t, "analysis/b.md", "---\nkind: analysis\nuid: "+looseAnalysisUID+"\nstatus: active\n---\n\n## Problem statement\n")
	writeContextDoc(t, "plan/q.md", "---\nkind: plan\nuid: "+planUID+"\nstatus: active\nanalysis_id: "+looseAnalysisUID+"\n---\n\n## Objective\n")
	loose := ctxResolvedDoc{Path: "context/plan/q.md", Type: plan}
	if err := guardRelationUID(loose, strayUID); err != nil {
		t.Errorf("an empty parent list must not guard, got %v", err)
	}

	// A kind with no parent relation (an analysis) is never guarded.
	analysis, ok := ctxTypeLookup(ctxTypeAnalysis)
	if !ok {
		t.Fatal("analysis is not in the registry")
	}
	if err := guardRelationUID(ctxResolvedDoc{Path: "context/analysis/a.md", Type: analysis}, strayUID); err != nil {
		t.Errorf("an analysis has no parent and must not be guarded, got %v", err)
	}
}
