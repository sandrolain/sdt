package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// closedRoleSlugs is the expected closed set in canonical order (pm first,
// then execution/review roles). It is the frozen contract the register must
// match byte-for-byte so `role:` frontmatter validation and profile
// generation share one vocabulary.
var closedRoleSlugs = []string{"pm", "backend", "frontend", "architect", "reviewer", "qa", "devops", "docs"}

func TestRoleRegisterClosed(t *testing.T) {
	if len(roleRegister) != len(closedRoleSlugs) {
		t.Fatalf("expected %d roles, got %d", len(closedRoleSlugs), len(roleRegister))
	}

	coord, owned := 0, map[string]string{}
	for i, r := range roleRegister {
		if r.Slug != closedRoleSlugs[i] {
			t.Errorf("position %d: expected slug %q, got %q", i, closedRoleSlugs[i], r.Slug)
		}
		if r.Title == "" {
			t.Errorf("role %q has an empty title", r.Slug)
		}
		if r.Coordinator {
			coord++
		}
		for _, p := range r.Owned {
			if !strings.HasPrefix(p, "context/") {
				t.Errorf("role %q owns a non-context path %q", r.Slug, p)
			}
			if owner, dup := owned[p]; dup {
				t.Errorf("path %q owned by both %q and %q", p, owner, r.Slug)
			}
			owned[p] = r.Slug
		}
	}
	if coord != 1 {
		t.Errorf("expected exactly one coordinator, got %d", coord)
	}
	if roleRegister[0].Slug != "pm" || !roleRegister[0].Coordinator {
		t.Errorf("expected pm as the first coordinator role, got %q", roleRegister[0].Slug)
	}
	if coordinatorSlug() != "pm" {
		t.Errorf("expected coordinatorSlug() == pm, got %q", coordinatorSlug())
	}
}

func TestRoleSlugsConsistent(t *testing.T) {
	slugs := roleSlugs()
	if len(slugs) != len(closedRoleSlugs) {
		t.Fatalf("expected %d slugs, got %d", len(closedRoleSlugs), len(slugs))
	}
	for i, s := range slugs {
		if s != closedRoleSlugs[i] {
			t.Errorf("roleSlugs position %d: expected %q, got %q", i, closedRoleSlugs[i], s)
		}
		if r, ok := roleLookup(s); !ok || r.Slug != s {
			t.Errorf("roleLookup(%q) did not resolve to itself", s)
		}
	}
	if _, ok := roleLookup("sdet"); ok {
		t.Error("roleLookup accepted an unregistered slug")
	}
}

func TestAgentRolesShowText(t *testing.T) {
	runInTempDir(t)
	out := execute(t, agentRolesShowCmd, nil)
	for _, slug := range closedRoleSlugs {
		if !strings.Contains(string(out), slug) {
			t.Errorf("text output missing slug %q:\n%s", slug, out)
		}
	}
	if !strings.Contains(string(out), "* pm") {
		t.Errorf("expected pm marked as coordinator:\n%s", out)
	}
	if !strings.Contains(string(out), "8 role(s)") {
		t.Errorf("expected register count in footer:\n%s", out)
	}
}

func TestAgentRolesShowFormats(t *testing.T) {
	runInTempDir(t)
	jsonOut := execute(t, agentRolesShowCmd, nil, "--format", "json")
	var got []roleDescriptor
	if err := json.Unmarshal(jsonOut, &got); err != nil {
		t.Fatalf("json output is not valid role descriptors: %v\n%s", err, jsonOut)
	}
	if len(got) != len(roleRegister) || got[0].Slug != "pm" || !got[0].Coordinator {
		t.Errorf("json output mismatched register: %+v", got)
	}

	yamlOut := execute(t, agentRolesShowCmd, nil, "--format", "yaml")
	for _, slug := range closedRoleSlugs {
		if !strings.Contains(string(yamlOut), slug) {
			t.Errorf("yaml output missing slug %q:\n%s", slug, yamlOut)
		}
	}
}

func TestAgentRolesGroupRegistered(t *testing.T) {
	runInTempDir(t)
	// The group and its three subcommands are discoverable under `sdt agent roles`.
	found := false
	for _, c := range agentCmd.Commands() {
		if c.Name() == "roles" {
			found = true
			names := map[string]bool{}
			for _, sub := range c.Commands() {
				names[sub.Name()] = true
			}
			for _, want := range []string{"show", "init", "check"} {
				if !names[want] {
					t.Errorf("roles group missing subcommand %q", want)
				}
			}
		}
	}
	if !found {
		t.Error("roles group not registered under agentCmd")
	}
}
