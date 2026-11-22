//go:build !wasm

package cmd

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/materialpackage"
	"github.com/sandrolain/sdt/internal/metacomp"
)

func packagePrivacyFromDocument(data []byte) (string, *materialpackage.PrivacyClearance, error) {
	frontmatter, _ := contextwiki.SplitFrontmatter(string(data))
	if frontmatter == "" {
		return "", nil, fmt.Errorf("resource has no privacy declaration")
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(frontmatter, "---"), "---")
	return packagePrivacyFromYAML([]byte(inner))
}

func packagePrivacyFromYAML(data []byte) (string, *materialpackage.PrivacyClearance, error) {
	var metadata map[string]any
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return "", nil, fmt.Errorf("parse resource privacy metadata: %w", err)
	}
	return packagePrivacyFromMap(metadata)
}

func packagePrivacyFromMaterial(metadata map[string]any, role string) (string, *materialpackage.PrivacyClearance, error) {
	material := metadataMap(metadata["material"])
	artifacts := metadataMap(material["artifacts"])
	resource := metadataMap(artifacts[role])
	if resource == nil {
		return "", nil, fmt.Errorf("material companion has no %s privacy declaration", role)
	}
	return packagePrivacyFromMap(resource)
}

func packagePrivacyFromAsset(root, contextRelativePath string) (string, *materialpackage.PrivacyClearance, error) {
	document, found, err := metacomp.ReadTarget(root, contextRelativePath)
	if err != nil {
		return "", nil, fmt.Errorf("asset privacy companion: %w", err)
	}
	if !found {
		return "", nil, fmt.Errorf("asset %s has no privacy companion", contextRelativePath)
	}
	return packagePrivacyFromMap(document)
}

func packagePrivacyFromMap(metadata map[string]any) (string, *materialpackage.PrivacyClearance, error) {
	privacy := metadataString(metadata["privacy"])
	if privacy == "" {
		return "", nil, fmt.Errorf("privacy declaration is required; use none, personal or real")
	}
	var clearance *materialpackage.PrivacyClearance
	if raw, exists := metadata["privacy_clearance"]; exists {
		fields := metadataMap(raw)
		if fields == nil {
			return privacy, nil, fmt.Errorf("privacy_clearance must be a mapping")
		}
		clearance = &materialpackage.PrivacyClearance{
			Status: metadataString(fields["status"]),
			Basis:  metadataString(fields["basis"]),
		}
	}
	if err := materialpackage.ValidatePrivacy(privacy, clearance); err != nil {
		return privacy, clearance, err
	}
	return privacy, clearance, nil
}
