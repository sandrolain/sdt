//go:build unix

package cmd

import (
	"fmt"
	"github.com/sandrolain/sdt/internal/delegation"
	"strings"
)

func validatePermissionAuthority(source, rationale, mandate string) error {
	if source != delegation.PermissionSourcePrincipal && source != delegation.PermissionUser {
		return fmt.Errorf("--source must be principal or user")
	}
	if strings.TrimSpace(rationale) == "" {
		return fmt.Errorf("--rationale must explain the decision")
	}
	if source == delegation.PermissionSourcePrincipal && strings.TrimSpace(mandate) == "" {
		return fmt.Errorf("--mandate must reference the existing authority; otherwise ask the user")
	}
	return nil
}

func filterPermissionHistory(records []delegation.PermissionRecord, source, outcome string, escalated bool) []delegation.PermissionRecord {
	result := []delegation.PermissionRecord{}
	for _, record := range records {
		if source != "" && record.Source != source || outcome != "" && record.Disposition != outcome || escalated && record.Escalation == "" {
			continue
		}
		result = append(result, record)
	}
	return result
}
