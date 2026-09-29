package ctxrel

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDoc(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func analysis(uid, sources string) string {
	return "---\nkind: analysis\nuid: " + uid + "\n" + sources + "summary: s\n---\n\nbody\n"
}

func plan(uid, analysisID, sources string) string {
	return "---\nkind: plan\nuid: " + uid + "\nanalysis_id: " + analysisID + "\n" + sources + "summary: s\n---\n\nbody\n"
}

func tasks(uid, planID, sources string) string {
	return "---\nkind: tasks\nuid: " + uid + "\nplan_id: " + planID + "\n" + sources + "summary: s\n---\n\nbody\n"
}

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeDoc(t, dir, "analysis/a.md", analysis("uid-a", ""))
	writeDoc(t, dir, "analysis/b.md", analysis("uid-b", ""))
	writeDoc(t, dir, "plan/p.md", plan("uid-p", "uid-a", "sources:\n  - analysis/a.md\n  - analysis/b.md\n"))
	writeDoc(t, dir, "plan/q.md", plan("uid-q", "uid-missing", "sources:\n  - analysis/a.md\n"))
	writeDoc(t, dir, "tasks/t1.md", tasks("uid-t1", "uid-p", "sources:\n  - plan/p.md\n  - plan/q.md\n"))
	writeDoc(t, dir, "tasks/t2.md", tasks("uid-t2", "", "sources:\n  - plan/p.md\n"))
	writeDoc(t, dir, "notes/n.md", "---\nkind: notes\nuid: uid-n\nsummary: s\n---\n\nbody\n")
	return dir
}

func TestLoadResolvesTypedParents(t *testing.T) {
	edges, err := Load(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"context/plan/p.md":       "context/analysis/a.md",
		"context/plan/q.md":       "", // unresolvable uid
		"context/tasks/t1.md":     "context/plan/p.md",
		"context/tasks/t2.md":     "", // no plan_id
		"context/analysis/a.md":   "",
		"context/notes/n.md":      "", // outside the lifecycle chain
		"context/plan/unknown.md": "",
	}
	for ref, want := range cases {
		if got := edges.ParentOf(ref); got != want {
			t.Errorf("ParentOf(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestParentIsSingularAndIgnoresSources(t *testing.T) {
	edges, err := Load(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	// plan/p.md names analysis/b.md in `sources`; only `analysis_id` derives.
	if got := edges.ParentOf("context/plan/p.md"); got != "context/analysis/a.md" {
		t.Errorf("ParentOf(plan/p.md) = %q, want the analysis_id target only", got)
	}
	if children := edges.ChildrenOf("context/analysis/b.md"); len(children) != 0 {
		t.Errorf("ChildrenOf(analysis/b.md) = %v, want none", children)
	}
	// tasks/t1.md names plan/q.md in `sources`; only `plan_id` derives.
	if got := edges.ParentOf("context/tasks/t1.md"); got != "context/plan/p.md" {
		t.Errorf("ParentOf(tasks/t1.md) = %q, want the plan_id target only", got)
	}
	if children := edges.ChildrenOf("context/plan/q.md"); len(children) != 0 {
		t.Errorf("ChildrenOf(plan/q.md) = %v, want none", children)
	}
}

func TestChildrenAreSortedAndComplete(t *testing.T) {
	edges, err := Load(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := edges.ChildrenOf("context/plan/p.md"); len(got) != 1 || got[0] != "context/tasks/t1.md" {
		t.Errorf("ChildrenOf(plan/p.md) = %v, want [context/tasks/t1.md]", got)
	}
	if got := edges.ChildrenOf("context/analysis/a.md"); len(got) != 1 || got[0] != "context/plan/p.md" {
		t.Errorf("ChildrenOf(analysis/a.md) = %v, want [context/plan/p.md]", got)
	}
	if got := edges.ChildrenOf("context/plan/missing.md"); got != nil {
		t.Errorf("ChildrenOf(unknown) = %v, want nil", got)
	}
}

func TestLoadToleratesAMissingLifecycleDirectory(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "analysis/a.md", analysis("uid-a", ""))
	// no plan/ and no tasks/ directory at all
	edges, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := edges.ChildrenOf("context/analysis/a.md"); got != nil {
		t.Errorf("ChildrenOf = %v, want nil", got)
	}
	if got := edges.ParentOf("context/plan/p.md"); got != "" {
		t.Errorf("ParentOf = %q, want empty", got)
	}
}

func TestNilEdgesAreSafe(t *testing.T) {
	var edges *Edges
	if got := edges.ParentOf("context/plan/p.md"); got != "" {
		t.Errorf("ParentOf on nil = %q, want empty", got)
	}
	if got := edges.ChildrenOf("context/plan/p.md"); got != nil {
		t.Errorf("ChildrenOf on nil = %v, want nil", got)
	}
}
