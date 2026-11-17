package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func validDelegationConfig() DelegationConfig {
	return DelegationConfig{
		MaxAttempts: 3,
		MaxDuration: "30m",
		Groups:      []DelegationGroupConfig{{Name: "provider-account", Concurrency: 2}},
		Agents: []DelegationAgentConfig{{
			Name: "reviewer", Command: []string{"agent-bin", "--acp"}, Model: "model-id",
			Roles: []string{"reviewer"}, Capabilities: []string{"read", "edit", "test"},
			AvailabilityGroup: "provider-account", Concurrency: 1, Cooldown: "10m",
		}},
	}
}

func TestValidateDelegationConfig(t *testing.T) {
	t.Run("accepts ordered candidates and defaults permissions to ask", func(t *testing.T) {
		if err := validateDelegationConfig(validDelegationConfig()); err != nil {
			t.Fatal(err)
		}
	})
	tests := []struct {
		name string
		edit func(*DelegationConfig)
		want string
	}{
		{"attempts are required", func(c *DelegationConfig) { c.MaxAttempts = 0 }, "max_attempts"},
		{"duration is required", func(c *DelegationConfig) { c.MaxDuration = "" }, "max_duration"},
		{"agent names are unique", func(c *DelegationConfig) { c.Agents = append(c.Agents, c.Agents[0]) }, "duplicate delegation agent"},
		{"unknown roles fail", func(c *DelegationConfig) { c.Agents[0].Roles = []string{"invented"} }, "unknown role"},
		{"capabilities are required", func(c *DelegationConfig) { c.Agents[0].Capabilities = nil }, "at least one capability"},
		{"agent concurrency is required", func(c *DelegationConfig) { c.Agents[0].Concurrency = 0 }, "concurrency must be positive"},
		{"group references resolve", func(c *DelegationConfig) { c.Agents[0].AvailabilityGroup = "missing" }, "unknown availability group"},
		{"permission decisions are closed", func(c *DelegationConfig) {
			c.PermissionPolicy = []DelegationPermissionRule{{Capability: "shell", Decision: "maybe"}}
		}, "invalid decision"},
		{"cooldowns are positive durations", func(c *DelegationConfig) { c.Agents[0].Cooldown = "0s" }, "cooldown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validDelegationConfig()
			tt.edit(&cfg)
			if err := validateDelegationConfig(cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateDelegationConfig() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestDelegationCandidatesFilterByRoleAndCapabilitiesInDeclaredOrder(t *testing.T) {
	cfg := validDelegationConfig()
	cfg.Agents = append(cfg.Agents,
		DelegationAgentConfig{Name: "backend", Roles: []string{"backend"}, Capabilities: []string{"read", "edit", "test"}},
		DelegationAgentConfig{Name: "reviewer-limited", Roles: []string{"reviewer"}, Capabilities: []string{"read"}},
		DelegationAgentConfig{Name: "reviewer-second", Roles: []string{"reviewer"}, Capabilities: []string{"read", "edit", "test"}},
	)
	got := delegationCandidates(cfg, "reviewer", []string{"edit", "test"})
	if len(got) != 2 {
		t.Fatalf("candidate count = %d, want 2: %+v", len(got), got)
	}
	if got[0].Name != "reviewer" || got[1].Name != "reviewer-second" {
		t.Fatalf("candidate order = %q then %q, want reviewer then reviewer-second", got[0].Name, got[1].Name)
	}
}

func TestProjectConfigDecodesDelegation(t *testing.T) {
	var cfg ProjectConfig
	if err := yaml.Unmarshal([]byte(`delegation:
  agents:
    - name: reviewer
      command: [agent-bin, --acp]
      model: model-id
      roles: [reviewer]
      capabilities: [read, edit, test]
      availability_group: provider-account
      concurrency: 1
      cooldown: 10m
  max_attempts: 3
  max_duration: 30m
  groups:
    - name: provider-account
      concurrency: 2
`), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := validateDelegationConfig(cfg.Delegation); err != nil {
		t.Fatal(err)
	}
	if cfg.Delegation.Agents[0].Name != "reviewer" || cfg.Delegation.Agents[0].Command[0] != "agent-bin" {
		t.Fatalf("delegation config did not decode: %+v", cfg.Delegation)
	}
}

func TestDelegationContractsRenderCompactSections(t *testing.T) {
	brief := renderDelegationBrief(delegationBrief{
		Goal: "Check the change", Scope: "Review the touched package",
		References: []string{"internal/example.go"}, Constraints: []string{"No network"},
		Acceptance: []string{"Report reproducible findings"}, Role: "reviewer",
		RoleProfile: "context/roles/reviewer.md", IndependentReview: true,
	})
	for _, want := range []string{"## Goal", "## Scope", "## References", "## Constraints", "## Acceptance", "## Role instruction", "context/roles/reviewer.md", "Do not rely on a principal verdict"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q:\n%s", want, brief)
		}
	}
	result := renderDelegationResult("No findings", "None", "go test passed", "None", "None", "Principal review")
	handoff := renderDelegationHandoff("Tests passed", "None", "result.md", "Principal review")
	for _, part := range []struct {
		rendered string
		sections []string
	}{
		{result, []string{"## Findings", "## Changes", "## Evidence", "## Blockers", "## Remaining work", "## Next action"}},
		{handoff, []string{"## Verified progress", "## Residual work", "## References", "## Next action"}},
	} {
		for _, section := range part.sections {
			if !strings.Contains(part.rendered, section) {
				t.Errorf("rendered contract missing %q:\n%s", section, part.rendered)
			}
		}
	}
}

func TestMaterializeDelegationRoleCopiesRoleAndSharedRules(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	roleDir := root + "/context/roles"
	if err := os.MkdirAll(roleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roleDir+"/shared.md", []byte("shared guidance"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roleDir+"/reviewer.md", []byte("reviewer profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := materializeDelegationRole(root, workspace, "reviewer"); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"shared.md": "shared guidance", "reviewer.md": "reviewer profile"} {
		data, err := os.ReadFile(workspace + "/context/roles/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("%s = %q, want %q", name, data, want)
		}
	}
}
