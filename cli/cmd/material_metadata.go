//go:build !wasm

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/sandrolain/sdt/internal/contextwrite"
	"github.com/sandrolain/sdt/internal/metacomp"
)

func materialMetadataPathOf(materialPath string) string {
	return materialPath + metacomp.CompanionSuffix
}

func readMaterialMetadata(materialPath string, header materialHeader) (metacomp.Document, bool, error) {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, false, err
	}
	materialPath, err = filepath.Abs(materialPath)
	if err != nil {
		return nil, false, err
	}
	rel, err := filepath.Rel(root, materialPath)
	if err != nil {
		return nil, false, err
	}
	doc, found, err := metacomp.ReadTarget(root, filepath.ToSlash(rel))
	if err != nil || !found {
		return doc, found, err
	}
	if err := validateLegacyMaterialLedger(root, materialPath); err != nil {
		return nil, true, err
	}
	material := metadataMap(doc["material"])
	if material == nil {
		return nil, true, fmt.Errorf("material extension is missing")
	}
	if strings.TrimPrefix(metadataString(material["concept"]), "wiki/") != header.Concept ||
		metadataString(material["syllabus"]) != header.Syllabus ||
		metadataString(material[metadataLangKey]) != header.Lang ||
		metadataString(material["classification"]) != header.Classification {
		return nil, true, fmt.Errorf("inline guide header disagrees with canonical companion")
	}
	artifacts := metadataMap(material["artifacts"])
	guide := metadataMap(artifacts["guide"])
	guideClaims := guide[metadataClaimsKey]
	if guideClaims == nil {
		guideClaims = material[metadataClaimsKey] // compatibility for early schema-1 companions
	}
	if !metadataClaimsMatch(guideClaims, header.Claims) {
		return nil, true, fmt.Errorf("guide claim header disagrees with canonical companion")
	}
	deckPath := strings.TrimSuffix(materialPath, sdtMarkdownExt) + ".slide" + sdtMarkdownExt
	deckRel, relErr := filepath.Rel(root, deckPath)
	if relErr != nil {
		return nil, true, relErr
	}
	for _, raw := range metadataSlice(doc["related_artifacts"]) {
		artifact := metadataMap(raw)
		if artifact == nil {
			continue
		}
		if metadataString(artifact[metadataRoleKey]) != "slide_deck" {
			continue
		}
		if filepath.ToSlash(filepath.Clean(metadataString(artifact[ctxMapPath]))) != filepath.ToSlash(deckRel) {
			return nil, true, fmt.Errorf("canonical related slide deck path does not match guide sibling")
		}
		data, readErr := os.ReadFile(deckPath) //#nosec G304 -- path derived beside the resolved guide
		if readErr != nil {
			return nil, true, fmt.Errorf("read related slide deck: %w", readErr)
		}
		first, _, _ := strings.Cut(string(data), "\n")
		deckHeader, parseErr := parseMaterialHeader(first)
		deck := metadataMap(artifacts["slide_deck"])
		if parseErr != nil || !sameMaterialIdentity(header, deckHeader) || !metadataClaimsMatch(deck[metadataClaimsKey], deckHeader.Claims) {
			return nil, true, fmt.Errorf("inline slide deck header disagrees with canonical companion")
		}
		return doc, true, nil
	}
	if _, statErr := os.Stat(deckPath); statErr == nil {
		return nil, true, fmt.Errorf("slide deck exists but is not bound by related_artifacts")
	} else if !os.IsNotExist(statErr) {
		return nil, true, statErr
	}
	return doc, true, nil
}

func validateLegacyMaterialLedger(root, materialPath string) error {
	// The canonical companion wins when both generations exist, but check the
	// retained ledger's mapped fields so a stale second authority is visible.
	legacyPath := strings.TrimSuffix(materialPath, sdtMarkdownExt) + ".ledger.json"
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	legacyPlan, err := materialLedgerMigration(root, legacyPath)
	if err != nil {
		return fmt.Errorf("legacy material ledger conflicts with canonical companion: %w", err)
	}
	if !legacyPlan.Existing {
		return fmt.Errorf("canonical material companion was not recognized during legacy comparison")
	}
	return nil
}

func metadataClaimsMatch(raw any, headerClaims map[string]string) bool {
	claims := metadataMap(raw)
	if claims != nil {
		if len(claims) != len(headerClaims) {
			return false
		}
		for id, digest := range headerClaims {
			if metadataString(claims[id]) != digest {
				return false
			}
		}
		return true
	}
	items := metadataSlice(raw)
	if len(items) != len(headerClaims) {
		return false
	}
	seen := map[string]string{}
	for _, item := range items {
		claim := metadataMap(item)
		if claim == nil {
			return false
		}
		seen[metadataString(claim["id"])] = metadataString(claim["digest"])
	}
	return reflect.DeepEqual(seen, headerClaims)
}

func metadataMap(value any) map[string]any {
	switch m := value.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			key, ok := k.(string)
			if !ok {
				return nil
			}
			out[key] = v
		}
		return out
	default:
		return nil
	}
}

func metadataSlice(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func metadataString(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func updateMaterialMetadataReview(path, role, verdict, by, model, at string) error {
	raw, err := os.ReadFile(path) //#nosec G304 -- resolved companion beside the material
	if err != nil {
		return err
	}
	doc, err := metacomp.Decode(raw)
	if err != nil {
		return err
	}
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	targetRel, err := filepath.Rel(root, strings.TrimSuffix(path, metacomp.CompanionSuffix))
	if err != nil {
		return err
	}
	if err := metacomp.ValidateTarget(root, filepath.ToSlash(targetRel), doc); err != nil {
		return err
	}
	review := metadataMap(doc["review"])
	if review == nil {
		review = map[string]any{}
	}
	cycle := metadataMap(review["cycle"])
	if cycle == nil {
		cycle = map[string]any{}
	}
	cycle[role] = verdict
	if by != "" {
		cycle[role+"_by"] = by
	}
	if model != "" {
		cycle[role+"_model"] = model
	}
	if at != "" {
		cycle[role+"_at"] = at
	}
	review["cycle"] = cycle
	doc["review"] = review
	proposed, err := metacomp.Encode(doc)
	if err != nil {
		return err
	}
	return contextwrite.Write(root, path, contextwrite.Digest(raw), proposed)
}
