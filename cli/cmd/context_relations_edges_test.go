package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// relationsEdgesFixture writes one document per family plus a wiki pair, and a
// note whose only reference leaves the corpus.
func relationsEdgesFixture(t *testing.T) {
	t.Helper()
	runInTempDir(t)
	writeCtxDoc(t, "context/analysis/a.md", "---\nkind: analysis\nuid: a-uid\nsummary: parent\n---\n")
	writeCtxDoc(t, "context/plan/p.md", "---\nkind: plan\nuid: p-uid\nsummary: child\nanalysis_id: a-uid\nsources:\n  - analysis/a.md\nlinks:\n  - notes/n.md\n---\n")
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: outside\nlinks:\n  - refs/converted/x.md\n---\n")
	writeCtxDoc(t, "context/wiki/topic/alpha.md", "---\nkind: wiki\nid: topic/alpha\ntype: concept\ntitle: Alpha\nstatus: active\nsummary: source\nrelations:\n  part_of:\n    - \"[[topic/beta|Beta]]\"\n---\n[[refines::topic/beta]]\n")
	writeCtxDoc(t, "context/wiki/topic/beta.md", "---\nkind: wiki\nid: topic/beta\ntype: concept\ntitle: Beta\nstatus: active\nsummary: target\n---\n## Body\n")
}

func TestContextRelationsEdgesOutbound(t *testing.T) {
	relationsEdgesFixture(t)

	out := execute(t, contextRelationsEdgesCmd, nil, "context/plan/p.md", "--format", "json")
	var res ctxRelationsEdgesResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if res.Direction != "outbound" || res.Depth != 1 {
		t.Errorf("direction/depth = %q/%d, want outbound/1", res.Direction, res.Depth)
	}
	if res.Total != 3 || len(res.Edges) != 3 {
		t.Fatalf("edges = %d, want 3 (correlation, derivation, lifecycle)", res.Total)
	}
	want := map[string]bool{"correlation": false, "derivation": false, "lifecycle": false}
	for _, e := range res.Edges {
		want[string(e.Family)] = true
	}
	for family, seen := range want {
		if !seen {
			t.Errorf("plan outbound is missing the %s family: %v", family, res.Edges)
		}
	}
}

func TestContextRelationsEdgesInbound(t *testing.T) {
	relationsEdgesFixture(t)

	out := execute(t, contextRelationsEdgesCmd, nil, "context/analysis/a.md", "--inbound", "--format", "json")
	var res ctxRelationsEdgesResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if res.Direction != "inbound" {
		t.Errorf("direction = %q, want inbound", res.Direction)
	}
	if res.Total != 2 {
		t.Fatalf("inbound edges = %d, want 2 (derivation, lifecycle): %v", res.Total, res.Edges)
	}
	for _, e := range res.Edges {
		if e.From != "context/plan/p.md" {
			t.Errorf("inbound edge from %q, want the plan", e.From)
		}
	}

	text := string(execute(t, contextRelationsEdgesCmd, nil, "context/analysis/a.md", "--inbound"))
	for _, family := range []string{"derivation", "lifecycle"} {
		if !strings.Contains(text, family) {
			t.Errorf("text output is missing %q:\n%s", family, text)
		}
	}
}

func TestContextRelationsEdgesVerbFilter(t *testing.T) {
	relationsEdgesFixture(t)

	out := execute(t, contextRelationsEdgesCmd, nil, "context/wiki/topic/alpha.md", "--verb", "part_of", "--format", "json")
	var res ctxRelationsEdgesResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if res.Total != 1 || res.Edges[0].Verb != "part_of" {
		t.Fatalf("verb filter = %v, want one part_of edge", res.Edges)
	}
	if res.Edges[0].To != "context/wiki/topic/beta.md" || res.Edges[0].Label != "Beta" {
		t.Errorf("edge = %+v, want the beta page labelled Beta", res.Edges[0])
	}

	none := string(execute(t, contextRelationsEdgesCmd, nil, "context/wiki/topic/alpha.md", "--verb", "conflicts_with"))
	if !strings.Contains(none, "no outbound edge") {
		t.Errorf("an empty filter must say so, got %q", none)
	}
}

func TestContextRelationsEdgesDropsTargetsOutsideTheCorpus(t *testing.T) {
	relationsEdgesFixture(t)

	out := string(execute(t, contextRelationsEdgesCmd, nil, "context/notes/n.md"))
	if !strings.Contains(out, "no outbound edge") {
		t.Errorf("a refs/ target must not be an edge, got %q", out)
	}
}

func TestContextRelationsEdgesRefusesDepthAboveOne(t *testing.T) {
	relationsEdgesFixture(t)

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, contextRelationsEdgesCmd, nil, "context/plan/p.md", "--depth", "2"))
	})
}
