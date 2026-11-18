//go:build unix

package cmd

import (
	"github.com/sandrolain/sdt/internal/delegation"
	"testing"
)

func TestPermissionDecisionAuthority(t *testing.T) {
	for _, tc := range []struct {
		source, rationale, mandate string
		valid                      bool
	}{
		{"principal", "bounded edit", "user request", true},
		{"user", "explicit response in conversation", "", true},
		{"principal", "bounded edit", "", false},
		{"principal", " ", "user request", false},
		{"policy", "rule", "rule", false},
		{"user", "", "", false},
	} {
		if err := validatePermissionAuthority(tc.source, tc.rationale, tc.mandate); (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

func TestPermissionHistoryFilters(t *testing.T) {
	records := []delegation.PermissionRecord{{RequestID: "a", Source: "principal", Disposition: "resolved"}, {RequestID: "b", Source: "user", Disposition: "pending", Escalation: "needs user"}}
	got := filterPermissionHistory(records, "user", "pending", true)
	if len(got) != 1 || got[0].RequestID != "b" {
		t.Fatalf("%+v", got)
	}
	if len(filterPermissionHistory(records, "principal", "pending", false)) != 0 {
		t.Fatal("filter combined as OR")
	}
	if len(filterPermissionHistory(records, "", "", false)) != 2 {
		t.Fatal("history lost")
	}
}
