package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// TestContextEditMintsClaimIDsAcrossKinds pins the write-path rule on the three
// document kinds that carry claims, plus a body that carries none.
func TestContextEditMintsClaimIDsAcrossKinds(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		ref     string
		front   string
		section string
		body    string
	}{
		{
			name:    "wiki",
			path:    "context/wiki/architecture/page.md",
			ref:     "wiki/architecture/page",
			front:   "---\nkind: wiki\ntitle: page\nsummary: a page\nstatus: draft\n---\n",
			section: "Claims",
			body:    "## Claims\n\n1. {#claim-1} a fact (refs/converted/x.md@abcdef12:1)\n",
		},
		{
			name:    "lesson",
			path:    "context/learning/architecture/page.md",
			ref:     "learning/architecture/page",
			front:   "---\nkind: lesson\nid: architecture/page\nconcept: architecture/page\ntitle: page\nsummary: a lesson\nstatus: draft\n---\n",
			section: "Items",
			body:    "## Items\n\n1. {#claim-1} an item (refs/converted/x.md@abcdef12:1)\n",
		},
		{
			name:    "plan",
			path:    "context/plan/page.md",
			ref:     "plan/page",
			front:   "---\nkind: plan\nsummary: a plan\nstatus: active\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n",
			section: "Phases",
			body:    "## Phases\n\n1. {#claim-1} a step (refs/converted/x.md@abcdef12:1)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupContextProject(t)
			writeCtxDoc(t, tc.path, tc.front+"\n"+tc.body)
			prev := newClaimID
			newClaimID = func() string { return "c-kindtest" }
			t.Cleanup(func() { newClaimID = prev })

			execute(t, contextEditCmd, nil, tc.ref, "--section", tc.section, "--input", tc.body)
			got := readDoc(t, strings.TrimPrefix(tc.path, "context/"))
			if !strings.Contains(got, "{#claim-1} <!-- c-kindtest -->") {
				t.Fatalf("claim anchor was not stamped:\n%s", got)
			}
			if !contextwiki.ClaimIDRegexp.MatchString("<!-- c-kindtest -->") {
				t.Fatalf("the stamped id does not match the corpus claim-id shape")
			}
		})
	}
}
