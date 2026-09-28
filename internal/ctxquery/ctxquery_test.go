package ctxquery

import (
	"testing"
	"time"
)

const sampleDoc = `---
kind: analysis
status: active
title: "Demo doc"
created: "2026-09-01T10:00:00Z"
updated: "2026-09-10T12:00:00Z"
objective: context-query-filters
agent: opencode
note_type: dead-end
archived: "true"
topics:
  - cli
  - lifecycle
categories:
  - improvement
project: sdt_test
---

## Body
`

func TestFacetsFromContent(t *testing.T) {
	f := FacetsFromContent(sampleDoc)
	if f.Kind != "analysis" {
		t.Errorf("Kind = %q, want analysis", f.Kind)
	}
	if f.Status != "active" {
		t.Errorf("Status = %q, want active", f.Status)
	}
	if got, want := f.Created, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Created = %v, want %v", got, want)
	}
	if got, want := f.Updated, time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Updated = %v, want %v", got, want)
	}
	for key, want := range map[string][]string{
		"kind":       {"analysis"},
		"archived":   {"true"},
		"topics":     {"cli", "lifecycle"},
		"categories": {"improvement"},
		"objective":  {"context-query-filters"},
	} {
		got := f.Values[key]
		if len(got) != len(want) {
			t.Errorf("Values[%q] = %v, want %v", key, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Values[%q][%d] = %q, want %q", key, i, got[i], want[i])
			}
		}
	}
}

func TestFacetsFromContentNoFrontmatter(t *testing.T) {
	f := FacetsFromContent("# just a body\n")
	if f.Kind != "" || f.Values != nil {
		t.Fatalf("expected empty facets, got %+v", f)
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestFilterMatch(t *testing.T) {
	facets := FacetsFromContent(sampleDoc)
	tests := []struct {
		name string
		f    Filter
		want bool
	}{
		{"kind hit", Filter{Kinds: []string{"analysis"}}, true},
		{"kind miss", Filter{Kinds: []string{"plan"}}, false},
		{"kind list any", Filter{Kinds: []string{"plan", "analysis"}}, true},
		{"status hit", Filter{Statuses: []string{"active"}}, true},
		{"status miss", Filter{Statuses: []string{"draft"}}, false},
		{"categories any-match", Filter{Categories: []string{"bug", "improvement"}}, true},
		{"categories miss", Filter{Categories: []string{"bug", "refactor"}}, false},
		{"case sensitive", Filter{Kinds: []string{"Analysis"}}, false},
		{"term scalar", Filter{Terms: []Term{{Key: "objective", Value: "context-query-filters"}}}, true},
		{"term scalar miss", Filter{Terms: []Term{{Key: "objective", Value: "other"}}}, false},
		{"term list member", Filter{Terms: []Term{{Key: "topics", Value: "cli"}}}, true},
		{"term list non-member", Filter{Terms: []Term{{Key: "topics", Value: "web"}}}, false},
		{"term boolean literal", Filter{Terms: []Term{{Key: "archived", Value: "true"}}}, true},
		{"term boolean other", Filter{Terms: []Term{{Key: "archived", Value: "false"}}}, false},
		{"term absent key", Filter{Terms: []Term{{Key: "missing", Value: "x"}}}, false},
		{"term absent negated", Filter{Terms: []Term{{Key: "missing", Value: "x", Negate: true}}}, true},
		{"term present negated", Filter{Terms: []Term{{Key: "objective", Value: "context-query-filters", Negate: true}}}, false},
		{"conjunction all must hold", Filter{Kinds: []string{"analysis"}, Statuses: []string{"draft"}}, false},
		{"after inclusive boundary", Filter{After: ptr(time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))}, true},
		{"after later", Filter{After: ptr(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))}, false},
		{"before inclusive boundary", Filter{Before: ptr(time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))}, true},
		{"before earlier", Filter{Before: ptr(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC))}, false},
		{"updated reference hit", Filter{DateField: "updated", After: ptr(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))}, true},
		{"created reference miss", Filter{DateField: "created", After: ptr(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Match(facets); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterMatchZeroDate(t *testing.T) {
	facets := FacetsFromContent("---\nkind: notes\n---\nbody\n")
	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if (Filter{After: &after}).Match(facets) {
		t.Error("a date filter must not match a document with no date")
	}
}
