package memo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func mustDay(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(DateLayout, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	reg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg == nil || len(reg.Rules) != 0 || len(reg.Memos) != 0 {
		t.Fatalf("want empty register, got %+v", reg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	disabled := false
	want := &Register{
		Rules: []Rule{{
			ID:        "deps-update",
			Summary:   "Update deps (patch/minor)",
			EveryDays: 14,
			LastDone:  "2026-09-20",
			Action:    "task deps:upgrade",
			Source:    "context/architecture/stack.md",
		}, {
			ID:        "toolchain",
			Summary:   "Check the toolchain",
			EveryDays: 60,
			Enabled:   &disabled,
		}},
		Memos: []Memo{{
			ID:      "rotate-token",
			Summary: "Rotate the deploy token",
			Due:     "2026-10-15",
		}},
	}
	if err := Save(root, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round-trip mismatch:\n want %+v\n  got %+v", want, got)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	err := Save(t.TempDir(), &Register{Rules: []Rule{{ID: "x"}}})
	if err == nil {
		t.Fatal("want a validation error for a rule with no every_days")
	}
}

func TestLoadMalformed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte("rules: [not-a-mapping\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("want an error for malformed YAML")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		reg  Register
	}{
		{"bad id", Register{Memos: []Memo{{ID: "Not Kebab", Summary: "s", Due: "2026-10-01"}}}},
		{"missing summary", Register{Rules: []Rule{{ID: "r", EveryDays: 1}}}},
		{"zero cadence", Register{Rules: []Rule{{ID: "r", Summary: "s", EveryDays: 0}}}},
		{"bad last_done", Register{Rules: []Rule{{ID: "r", Summary: "s", EveryDays: 1, LastDone: "yesterday"}}}},
		{"missing memo due", Register{Memos: []Memo{{ID: "m", Summary: "s"}}}},
		{"duplicate id across lists", Register{
			Rules: []Rule{{ID: "dup", Summary: "s", EveryDays: 1}},
			Memos: []Memo{{ID: "dup", Summary: "s", Due: "2026-10-01"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.reg.Validate(); err == nil {
				t.Fatalf("%s: want a validation error", tc.name)
			}
		})
	}
}

func TestDueOnRules(t *testing.T) {
	disabled := false
	today := mustDay(t, "2026-10-04")
	reg := &Register{Rules: []Rule{
		{ID: "never-done", Summary: "s", EveryDays: 7},                        // due immediately
		{ID: "due-today", Summary: "s", EveryDays: 7, LastDone: "2026-09-27"}, // boundary: +7 == today
		{ID: "overdue", Summary: "s", EveryDays: 7, LastDone: "2026-09-24"},   // 3 days overdue
		{ID: "not-yet", Summary: "s", EveryDays: 7, LastDone: "2026-10-01"},   // +7 in the future
		{ID: "disabled", Summary: "s", EveryDays: 1, LastDone: "2026-01-01", Enabled: &disabled},
	}}
	got, err := reg.DueOn(today)
	if err != nil {
		t.Fatalf("DueOn: %v", err)
	}
	byID := map[string]DueItem{}
	for _, d := range got {
		byID[d.ID] = d
	}
	for _, id := range []string{"never-done", "due-today", "overdue"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("want %q due", id)
		}
	}
	for _, id := range []string{"not-yet", "disabled"} {
		if _, ok := byID[id]; ok {
			t.Errorf("did not want %q due", id)
		}
	}
	if d := byID["due-today"]; d.OverdueDays != 0 || d.DueDate != "2026-10-04" {
		t.Errorf("due-today: want due date 2026-10-04, 0 overdue; got %+v", d)
	}
	if d := byID["overdue"]; d.OverdueDays != 3 {
		t.Errorf("overdue: want 3 days, got %d", d.OverdueDays)
	}
	if d := byID["never-done"]; d.DueDate != "" {
		t.Errorf("never-done: want an empty due date, got %q", d.DueDate)
	}
}

func TestDueOnMemos(t *testing.T) {
	today := mustDay(t, "2026-10-04")
	reg := &Register{Memos: []Memo{
		{ID: "due-today", Summary: "s", Due: "2026-10-04"},
		{ID: "overdue", Summary: "s", Due: "2026-10-01"},
		{ID: "future", Summary: "s", Due: "2026-11-01"},
		{ID: "done", Summary: "s", Due: "2026-09-01", Done: true},
	}}
	got, err := reg.DueOn(today)
	if err != nil {
		t.Fatalf("DueOn: %v", err)
	}
	ids := map[string]bool{}
	for _, d := range got {
		ids[d.ID] = true
	}
	if !ids["due-today"] || !ids["overdue"] {
		t.Errorf("want due-today and overdue due, got %+v", got)
	}
	if ids["future"] || ids["done"] {
		t.Errorf("did not want future or done due, got %+v", got)
	}
}

func TestRuleNextDue(t *testing.T) {
	if _, ok := (Rule{ID: "r", Summary: "s", EveryDays: 7}).NextDue(); ok {
		t.Error("a rule with no last_done must have no computed next-due day")
	}
	if next, ok := (Rule{ID: "r", Summary: "s", EveryDays: 7, LastDone: "2026-09-27"}).NextDue(); !ok || next != "2026-10-04" {
		t.Errorf("NextDue = %q,%v; want 2026-10-04,true", next, ok)
	}
}

func TestFindAndRemove(t *testing.T) {
	reg := &Register{
		Rules: []Rule{{ID: "r", Summary: "s", EveryDays: 1}},
		Memos: []Memo{{ID: "m", Summary: "s", Due: "2026-10-01"}},
	}
	if kind, i, ok := reg.Find("m"); !ok || kind != KindMemo || i != 0 {
		t.Fatalf("Find(m) = %q,%d,%v", kind, i, ok)
	}
	if !reg.Remove("r") || len(reg.Rules) != 0 {
		t.Fatalf("Remove(r) failed: %+v", reg.Rules)
	}
	if reg.Remove("r") {
		t.Fatal("Remove of a missing id should report false")
	}
}
