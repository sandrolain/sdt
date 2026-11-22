package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/metacomp"
)

const (
	metadataLangKey       = "lang"
	metadataClaimsKey     = "claims"
	metadataFilesKey      = "files"
	metadataRoleKey       = "role"
	materialArtifactGuide = "guide"
	materialArtifactDeck  = "slide_deck"
)

var contextMetadataCmd = &cobra.Command{Use: "metadata", Short: "Validate and migrate metadata companions"}

var contextMetadataMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Convert legacy source sidecars and material ledgers",
	Long: `Convert legacy .source.yaml archive records and .ledger.json material
reviews into complete-filename .meta.yaml companions. Dry-run is the default.
The command never changes source or render artifact bytes and never deletes
legacy records. Existing valid companions are skipped; collisions are errors.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		plans, err := metadataMigrationPlan()
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		apply := getBoolFlag(cmd, "apply", false)
		for _, plan := range plans {
			verb := "would create"
			if plan.Existing {
				verb = "already valid"
			} else if apply {
				if err := createContextDocFileMode(plan.Path, plan.Bytes, 0o600); err != nil {
					exitWithError(cmd, fmt.Errorf("write %s: %w", corpusRelative(plan.Path), err))
					return
				}
				verb = statusCreated
			}
			outputString(cmd, fmt.Sprintf("%s %s from %s\n", verb, corpusRelative(plan.Path), corpusRelative(plan.Legacy)))
		}
		if !apply {
			outputString(cmd, fmt.Sprintf("dry-run: %d companion(s); pass --apply to write\n", len(plans)))
		}
	},
}

type metadataMigration struct {
	Path     string
	Legacy   string
	Bytes    []byte
	Existing bool
}

func init() {
	contextMetadataMigrateCmd.Flags().Bool("apply", false, "write the companions; without this flag, print a dry-run")
	contextMetadataCmd.AddCommand(contextMetadataMigrateCmd)
	contextCmd.AddCommand(contextMetadataCmd)
}

func metadataMigrationPlan() ([]metadataMigration, error) {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	var plans []metadataMigration
	refs := filepath.Join(root, "context", "refs")
	err := filepath.WalkDir(refs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ingestLegacySidecarSuffix) {
			return nil
		}
		plan, err := sourceSidecarMigration(root, path)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	})
	if err != nil {
		return nil, err
	}
	materials := filepath.Join(root, filepath.FromSlash(materialDir))
	err = filepath.WalkDir(materials, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ledger.json") {
			return nil
		}
		plan, err := materialLedgerMigration(root, path)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return plans, nil
}

func sourceSidecarMigration(root, legacyPath string) (metadataMigration, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return metadataMigration{}, err
	}
	legacyPath, err = filepath.Abs(legacyPath)
	if err != nil {
		return metadataMigration{}, err
	}
	raw, err := os.ReadFile(legacyPath) //#nosec G304 -- walked under context/refs
	if err != nil {
		return metadataMigration{}, err
	}
	var fields map[string]any
	if err := yaml.Unmarshal(raw, &fields); err != nil {
		return metadataMigration{}, fmt.Errorf("decode %s: %w", corpusRelative(legacyPath), err)
	}
	legacySuffix := ingestLegacySidecarSuffix
	targetPath := strings.TrimSuffix(legacyPath, legacySuffix)
	if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
		if _, renamedErr := os.Stat(targetPath + ".source"); renamedErr == nil {
			targetPath += ".source"
		}
	}
	targetRel, err := filepath.Rel(root, targetPath)
	if err != nil {
		return metadataMigration{}, err
	}
	targetBytes, err := os.ReadFile(targetPath) //#nosec G304 -- derived by removing the legacy sidecar suffix
	if err != nil {
		return metadataMigration{}, fmt.Errorf("source target %s: %w", corpusRelative(targetPath), err)
	}
	if sha := sourceField(fields, ctxSourceSheetSHA256); sha != "" && sha != metacomp.Digest(targetBytes) {
		return metadataMigration{}, fmt.Errorf("legacy sidecar %s has a stale sha256", corpusRelative(legacyPath))
	}
	doc := metacomp.New(filepath.ToSlash(targetRel), metacomp.Digest(targetBytes))
	doc["source_archive"] = fields
	if err := metacomp.ValidateTarget(root, filepath.ToSlash(targetRel), doc); err != nil {
		return metadataMigration{}, fmt.Errorf("validate source target for %s: %w", corpusRelative(legacyPath), err)
	}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		return metadataMigration{}, err
	}
	destination := targetPath + metacomp.CompanionSuffix
	return metadataMigrationPlanFor(root, destination, legacyPath, encoded)
}

func materialLedgerMigration(root, legacyPath string) (metadataMigration, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return metadataMigration{}, err
	}
	legacyPath, err = filepath.Abs(legacyPath)
	if err != nil {
		return metadataMigration{}, err
	}
	raw, err := os.ReadFile(legacyPath) //#nosec G304 -- walked under material lane
	if err != nil {
		return metadataMigration{}, err
	}
	var legacy map[string]any
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return metadataMigration{}, fmt.Errorf("decode %s: %w", corpusRelative(legacyPath), err)
	}
	guidePath := strings.TrimSuffix(legacyPath, ".ledger.json") + sdtMarkdownExt
	guideBytes, err := os.ReadFile(guidePath) //#nosec G304 -- derived beside the ledger
	if err != nil {
		return metadataMigration{}, fmt.Errorf("guide for %s: %w", corpusRelative(legacyPath), err)
	}
	first, _, _ := strings.Cut(string(guideBytes), "\n")
	header, err := parseMaterialHeader(first)
	if err != nil {
		return metadataMigration{}, fmt.Errorf("guide %s: %w", corpusRelative(guidePath), err)
	}
	if err := materialLedgerMatchesHeader(legacy, header); err != nil {
		return metadataMigration{}, fmt.Errorf("ledger %s conflicts with inline header: %w", corpusRelative(legacyPath), err)
	}
	guideRel, err := filepath.Rel(root, guidePath)
	if err != nil {
		return metadataMigration{}, err
	}
	material := map[string]any{
		ctxFrontmatterConcept: header.Concept, ctxTypeSyllabus: header.Syllabus, metadataLangKey: header.Lang,
		"classification": header.Classification,
		"artifacts":      map[string]any{materialArtifactGuide: map[string]any{metadataClaimsKey: headerClaims(header)}},
	}
	for _, key := range []string{metadataClaimsKey, metadataFilesKey, ctxTypeLesson} {
		if value, ok := legacy[key]; ok {
			material[key] = value
		}
	}
	review := map[string]any{}
	for key, value := range legacy {
		switch key {
		case "concept", ctxTypeSyllabus, metadataLangKey, metadataClaimsKey, metadataFilesKey, ctxTypeLesson:
		default:
			review[key] = value
		}
	}
	doc := metacomp.New(filepath.ToSlash(guideRel), metacomp.Digest(guideBytes))
	doc["material"] = material
	doc["review"] = review
	deckPath := strings.TrimSuffix(guidePath, sdtMarkdownExt) + ".slide" + sdtMarkdownExt
	if deckBytes, readErr := os.ReadFile(deckPath); readErr == nil { //#nosec G304 -- derived beside the guide
		deckFirst, _, _ := strings.Cut(string(deckBytes), "\n")
		deckHeader, parseErr := parseMaterialHeader(deckFirst)
		if parseErr != nil || !sameMaterialIdentity(header, deckHeader) {
			return metadataMigration{}, fmt.Errorf("deck %s has a missing or conflicting material header", corpusRelative(deckPath))
		}
		artifacts, ok := material["artifacts"].(map[string]any)
		if !ok {
			return metadataMigration{}, fmt.Errorf("material artifact map could not be built")
		}
		artifacts[materialArtifactDeck] = map[string]any{metadataClaimsKey: headerClaims(deckHeader)}
		deckRel, relErr := filepath.Rel(root, deckPath)
		if relErr != nil {
			return metadataMigration{}, relErr
		}
		if err := materialLedgerFilesMatch(legacy, guidePath, deckPath, true); err != nil {
			return metadataMigration{}, fmt.Errorf("ledger %s file list: %w", corpusRelative(legacyPath), err)
		}
		doc["related_artifacts"] = []any{map[string]any{
			metadataRoleKey: materialArtifactDeck, ctxMapPath: filepath.ToSlash(deckRel), ctxSourceSheetSHA256: metacomp.Digest(deckBytes),
		}}
	} else if !os.IsNotExist(readErr) {
		return metadataMigration{}, readErr
	} else if err := materialLedgerFilesMatch(legacy, guidePath, deckPath, false); err != nil {
		return metadataMigration{}, fmt.Errorf("ledger %s file list: %w", corpusRelative(legacyPath), err)
	}
	if err := metacomp.ValidateTarget(root, filepath.ToSlash(guideRel), doc); err != nil {
		return metadataMigration{}, fmt.Errorf("validate material target for %s: %w", corpusRelative(legacyPath), err)
	}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		return metadataMigration{}, err
	}
	return metadataMigrationPlanFor(root, guidePath+metacomp.CompanionSuffix, legacyPath, encoded)
}

func materialLedgerFilesMatch(ledger map[string]any, guidePath, deckPath string, deckExists bool) error {
	raw, present := ledger[metadataFilesKey]
	if !present {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("files must be a sequence")
	}
	seen := map[string]bool{}
	for _, item := range items {
		name, ok := item.(string)
		if !ok || filepath.Base(name) != name {
			return fmt.Errorf("file entries must be basenames")
		}
		seen[name] = true
	}
	guideName, deckName := filepath.Base(guidePath), filepath.Base(deckPath)
	if !seen[guideName] {
		return fmt.Errorf("guide %q is absent", guideName)
	}
	if seen[deckName] != deckExists {
		return fmt.Errorf("deck %q presence disagrees with the corpus", deckName)
	}
	if len(seen) != 1+btoi(deckExists) {
		return fmt.Errorf("contains artifacts beyond the guide and its slide deck")
	}
	return nil
}

func btoi(value bool) int {
	if value {
		return 1
	}
	return 0
}

func metadataMigrationPlanFor(root, destination, legacyPath string, proposed []byte) (metadataMigration, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return metadataMigration{}, err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return metadataMigration{}, err
	}
	if _, err := os.Stat(destination); err == nil {
		rootRel, relErr := filepath.Rel(root, destination)
		if relErr != nil {
			return metadataMigration{}, relErr
		}
		targetRel := strings.TrimSuffix(filepath.ToSlash(rootRel), metacomp.CompanionSuffix)
		doc, found, readErr := metacomp.ReadTarget(root, targetRel)
		if readErr != nil || !found {
			return metadataMigration{}, fmt.Errorf("companion collision at %s: %v", filepath.ToSlash(rootRel), readErr)
		}
		proposedDoc, decodeErr := metacomp.Decode(proposed)
		if decodeErr != nil {
			return metadataMigration{}, fmt.Errorf("decode proposed companion for %s: %w", filepath.ToSlash(rootRel), decodeErr)
		}
		matches, compareErr := metadataAuthoritativeFieldsMatch(doc, proposedDoc)
		if compareErr != nil {
			return metadataMigration{}, fmt.Errorf("compare companion %s: %w", filepath.ToSlash(rootRel), compareErr)
		}
		if !matches {
			return metadataMigration{}, fmt.Errorf("semantic companion conflict at %s: mapped legacy fields differ", filepath.ToSlash(rootRel))
		}
		return metadataMigration{Path: destination, Legacy: legacyPath, Existing: true}, nil
	} else if !os.IsNotExist(err) {
		return metadataMigration{}, err
	}
	return metadataMigration{Path: destination, Legacy: legacyPath, Bytes: proposed}, nil
}

func metadataAuthoritativeFieldsMatch(existing, proposed metacomp.Document) (bool, error) {
	fields := map[string]any{}
	for _, key := range []string{"schema_version", "target", materialLane, "source_archive"} {
		if value, ok := proposed[key]; ok {
			if key == materialLane {
				value = projectLegacyMaterialFields(metadataMap(value))
			}
			fields[key] = value
		}
	}
	want, err := json.Marshal(fields)
	if err != nil {
		return false, err
	}
	actualFields := map[string]any{}
	for key := range fields {
		if value, ok := existing[key]; ok {
			if key == materialLane {
				value = projectLegacyMaterialFields(metadataMap(value))
			}
			actualFields[key] = value
		}
	}
	actual, err := json.Marshal(actualFields)
	if err != nil {
		return false, err
	}
	return string(actual) == string(want), nil
}

// projectLegacyMaterialFields compares only fields representable by a legacy
// ledger. New companion fields (for example privacy declarations and review
// provenance) remain authoritative in the companion without making the
// retained ledger appear corrupt.
func projectLegacyMaterialFields(material map[string]any) map[string]any {
	projected := map[string]any{}
	for _, key := range []string{"concept", ctxTypeSyllabus, metadataLangKey, "classification", metadataClaimsKey, metadataFilesKey, ctxTypeLesson} {
		if value, ok := material[key]; ok {
			projected[key] = value
		}
	}
	if artifacts := metadataMap(material["artifacts"]); artifacts != nil {
		projectedArtifacts := map[string]any{}
		for _, name := range []string{materialArtifactGuide, materialArtifactDeck} {
			if artifact := metadataMap(artifacts[name]); artifact != nil {
				if claims, ok := artifact[metadataClaimsKey]; ok {
					projectedArtifacts[name] = map[string]any{metadataClaimsKey: claims}
				}
			}
		}
		if len(projectedArtifacts) > 0 {
			projected["artifacts"] = projectedArtifacts
		}
	}
	return projected
}

func materialLedgerMatchesHeader(ledger map[string]any, header materialHeader) error {
	if got := strings.TrimPrefix(sourceField(ledger, "concept"), "wiki/"); got != header.Concept {
		return fmt.Errorf("concept %q != %q", got, header.Concept)
	}
	for key, want := range map[string]string{ctxTypeSyllabus: header.Syllabus, metadataLangKey: header.Lang} {
		if got := sourceField(ledger, key); got != want {
			return fmt.Errorf("%s %q != %q", key, got, want)
		}
	}
	claims, ok := ledger[metadataClaimsKey].([]any)
	if !ok || len(claims) != len(header.Claims) {
		return fmt.Errorf("claim list does not match the header")
	}
	for _, raw := range claims {
		claim, ok := raw.(map[string]any)
		id := sourceField(claim, "id")
		digest := sourceField(claim, "digest")
		if !ok || id == "" || digest == "" || header.Claims[id] != digest {
			return fmt.Errorf("claim list does not match the header")
		}
	}
	return nil
}

func sameMaterialIdentity(left, right materialHeader) bool {
	return left.Concept == right.Concept && left.Syllabus == right.Syllabus && left.Lang == right.Lang &&
		left.Classification == right.Classification
}

func headerClaims(header materialHeader) []any {
	claims := make([]any, 0, len(header.Order))
	for _, id := range header.Order {
		claims = append(claims, map[string]any{"id": id, "digest": header.Claims[id]})
	}
	return claims
}

func sourceField(fields map[string]any, key string) string {
	value, ok := fields[key].(string)
	if !ok {
		return ""
	}
	return value
}
