package cmd

import (
	"fmt"
	"strings"
	"time"
)

// validateDelegationConfig rejects declarations that could otherwise produce
// ambiguous routing or silently remove an aggregate task bound.
//
//nolint:gocognit // This function preserves a multi-step state transition as one auditable unit.
//nolint:gocognit,gocyclo // This function preserves a multi-step state transition as one auditable unit.
//nolint:gocyclo
func validateDelegationConfig(cfg DelegationConfig) error { //nolint:gocyclo
	if len(cfg.Agents) == 0 {
		if cfg.MaxAttempts != 0 || cfg.MaxDuration != "" || len(cfg.Groups) != 0 || len(cfg.PermissionPolicy) != 0 {
			return fmt.Errorf("delegation limits, groups, and permission policy require at least one agent")
		}
		return nil
	}
	if cfg.MaxAttempts < 1 {
		return fmt.Errorf("delegation.max_attempts must be positive")
	}
	duration, err := time.ParseDuration(cfg.MaxDuration)
	if err != nil || duration <= 0 {
		return fmt.Errorf("delegation.max_duration must be a positive duration")
	}

	groups := make(map[string]struct{}, len(cfg.Groups))
	for _, group := range cfg.Groups {
		if strings.TrimSpace(group.Name) == "" {
			return fmt.Errorf("delegation group name is required")
		}
		if group.Concurrency < 1 {
			return fmt.Errorf("delegation group %q concurrency must be positive", group.Name)
		}
		if _, exists := groups[group.Name]; exists {
			return fmt.Errorf("duplicate delegation group %q", group.Name)
		}
		groups[group.Name] = struct{}{}
	}

	names := make(map[string]struct{}, len(cfg.Agents))
	for i, agent := range cfg.Agents {
		prefix := fmt.Sprintf("delegation.agents[%d]", i)
		if strings.TrimSpace(agent.Name) == "" {
			return fmt.Errorf("%s name is required", prefix)
		}
		if _, exists := names[agent.Name]; exists {
			return fmt.Errorf("duplicate delegation agent %q", agent.Name)
		}
		names[agent.Name] = struct{}{}
		if len(agent.Command) == 0 || strings.TrimSpace(agent.Command[0]) == "" {
			return fmt.Errorf("%s command must contain an executable", prefix)
		}
		if strings.TrimSpace(agent.Model) == "" {
			return fmt.Errorf("%s model is required", prefix)
		}
		if len(agent.Roles) == 0 {
			return fmt.Errorf("%s must declare at least one role", prefix)
		}
		if len(agent.Capabilities) == 0 {
			return fmt.Errorf("%s must declare at least one capability", prefix)
		}
		seenRoles := map[string]struct{}{}
		for _, role := range agent.Roles {
			if _, ok := roleLookup(role); !ok {
				return fmt.Errorf("%s declares unknown role %q", prefix, role)
			}
			if _, exists := seenRoles[role]; exists {
				return fmt.Errorf("%s repeats role %q", prefix, role)
			}
			seenRoles[role] = struct{}{}
		}
		if agent.Concurrency < 1 {
			return fmt.Errorf("%s concurrency must be positive", prefix)
		}
		cooldown, err := time.ParseDuration(agent.Cooldown)
		if err != nil || cooldown <= 0 {
			return fmt.Errorf("%s cooldown must be a positive duration for indeterminate recovery probes", prefix)
		}
		if agent.AvailabilityGroup != "" {
			if _, ok := groups[agent.AvailabilityGroup]; !ok {
				return fmt.Errorf("%s references unknown availability group %q", prefix, agent.AvailabilityGroup)
			}
		}
		seenCapabilities := map[string]struct{}{}
		for _, capability := range agent.Capabilities {
			if strings.TrimSpace(capability) == "" {
				return fmt.Errorf("%s capabilities cannot be empty", prefix)
			}
			if _, exists := seenCapabilities[capability]; exists {
				return fmt.Errorf("%s repeats capability %q", prefix, capability)
			}
			seenCapabilities[capability] = struct{}{}
		}
	}

	seenRules := map[string]struct{}{}
	for _, rule := range cfg.PermissionPolicy {
		if strings.TrimSpace(rule.Capability) == "" {
			return fmt.Errorf("delegation permission rule capability is required")
		}
		switch rule.Decision {
		case "allow", "deny", permissionDecisionAsk:
		default:
			return fmt.Errorf("delegation permission rule for %q has invalid decision %q", rule.Capability, rule.Decision)
		}
		if rule.Decision != permissionDecisionAsk {
			if strings.TrimSpace(rule.Operation) == "" || strings.TrimSpace(rule.Scope) == "" {
				return fmt.Errorf("automatic delegation permission rule for %q requires exact operation and scope", rule.Capability)
			}
			validFor, err := time.ParseDuration(rule.ValidFor)
			if err != nil || validFor <= 0 {
				return fmt.Errorf("automatic delegation permission rule for %q requires a positive valid_for duration", rule.Capability)
			}
		}
		key := strings.Join([]string{rule.Capability, rule.Operation, rule.Scope}, "\x00")
		if _, exists := seenRules[key]; exists {
			return fmt.Errorf("duplicate delegation permission mandate for capability %q, operation %q, and scope %q", rule.Capability, rule.Operation, rule.Scope)
		}
		seenRules[key] = struct{}{}
	}
	return nil
}

// delegationCandidates retains declaration order while requiring both the SDT
// role and every capability the task marks indispensable.
func delegationCandidates(cfg DelegationConfig, role string, requiredCapabilities []string) []DelegationAgentConfig {
	var candidates []DelegationAgentConfig
	for _, agent := range cfg.Agents {
		if !delegationContains(agent.Roles, role) {
			continue
		}
		eligible := true
		for _, required := range requiredCapabilities {
			if !delegationContains(agent.Capabilities, required) {
				eligible = false
				break
			}
		}
		if eligible {
			candidates = append(candidates, agent)
		}
	}
	return candidates
}

func delegationContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
