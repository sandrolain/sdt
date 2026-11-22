//go:build !wasm

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwrite"
	"github.com/sandrolain/sdt/internal/materialbrief"
	"github.com/sandrolain/sdt/internal/materialpackage"
	"github.com/sandrolain/sdt/internal/metacomp"
)

var materialPrivacyCmd = &cobra.Command{
	Use:   "privacy",
	Short: "Set privacy declarations on material and asset companions",
}

var materialPrivacySetCmd = &cobra.Command{
	Use:   "set <material-id> <guide|slide_deck> <none|personal|real>",
	Short: "Set privacy for one material artifact",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		basis := getStringFlag(cmd, "basis", false)
		if err := setMaterialArtifactPrivacy(args[0], args[1], args[2], basis); err != nil {
			return err
		}
		outputString(cmd, fmt.Sprintf("set %s privacy for %s\n", args[1], args[0]))
		return nil
	},
}

var materialPrivacyAssetCmd = &cobra.Command{
	Use:   "asset <context-relative-path> <none|personal|real>",
	Short: "Set privacy in an asset's metadata companion",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		basis := getStringFlag(cmd, "basis", false)
		if err := setAssetPrivacy(args[0], args[1], basis); err != nil {
			return err
		}
		outputString(cmd, fmt.Sprintf("set asset privacy for %s\n", args[0]))
		return nil
	},
}

func init() {
	materialPrivacySetCmd.Flags().String("basis", "", "short clearance basis required for personal or real")
	materialPrivacyAssetCmd.Flags().String("basis", "", "short clearance basis required for personal or real")
	materialPrivacyCmd.AddCommand(materialPrivacySetCmd, materialPrivacyAssetCmd)
	materialCmd.AddCommand(materialPrivacyCmd)
}

func setMaterialArtifactPrivacy(materialID, artifact, value, basis string) error {
	if !materialbrief.ValidMaterialID(materialID) {
		return fmt.Errorf("invalid material id %q", materialID)
	}
	if artifact != materialArtifactGuide && artifact != materialArtifactDeck {
		return fmt.Errorf("artifact must be guide or slide_deck")
	}
	clearance, err := privacyClearance(value, basis)
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
	guidePath := filepath.Join(root, "context", "renders", "materials", filepath.FromSlash(materialID)+sdtMarkdownExt)
	guideData, err := os.ReadFile(guidePath) //#nosec G304 -- path is rooted in the validated material id
	if err != nil {
		return fmt.Errorf("read material guide: %w", err)
	}
	first, _, _ := strings.Cut(string(guideData), "\n")
	header, err := parseMaterialHeader(first)
	if err != nil {
		return fmt.Errorf("material %s has no valid header: %w", materialID, err)
	}
	materialBase, err := filepath.Abs(materialBaseDir())
	if err != nil {
		return err
	}
	materialRel, err := filepath.Rel(materialBase, guidePath)
	if err != nil {
		return err
	}
	pathID := strings.TrimSuffix(filepath.ToSlash(materialRel), sdtMarkdownExt)
	syllabusID := strings.TrimPrefix(header.Syllabus, "syllabi/")
	if pathID != materialID || !strings.HasPrefix(pathID, syllabusID+"/") || !strings.HasSuffix(pathID, "."+header.Lang) {
		return fmt.Errorf("material id %q does not match its path or header identity", materialID)
	}
	_, found, err := readMaterialMetadata(guidePath, header)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("material %s has no canonical metadata companion", materialID)
	}
	companionPath := materialMetadataPathOf(guidePath)
	raw, err := os.ReadFile(companionPath) //#nosec G304 -- companion path derives from validated material id
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, guidePath)
	if err != nil {
		return err
	}
	doc, found, err := metacomp.ReadTarget(root, filepath.ToSlash(rel))
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("material %s companion disappeared while updating privacy", materialID)
	}
	material := metadataMap(doc["material"])
	artifacts := metadataMap(material["artifacts"])
	entry := metadataMap(artifacts[artifact])
	if entry == nil {
		return fmt.Errorf("material %s has no %s artifact entry", materialID, artifact)
	}
	applyPrivacy(entry, value, clearance)
	proposed, err := metacomp.Encode(doc)
	if err != nil {
		return err
	}
	return contextwrite.Write(root, companionPath, contextwrite.Digest(raw), proposed)
}

func setAssetPrivacy(targetPath, value, basis string) error {
	clearance, err := privacyClearance(value, basis)
	if err != nil {
		return err
	}
	targetPath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(targetPath)))
	if !strings.HasPrefix(targetPath, "context/") || strings.HasPrefix(targetPath, "../") || !packageAssetRE.MatchString(targetPath) {
		return fmt.Errorf("asset path must be a supported context-relative file")
	}
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open project root: %w", err)
	}
	data, readErr := rootFS.ReadFile(filepath.FromSlash(targetPath))
	closeErr := rootFS.Close()
	if readErr != nil || closeErr != nil {
		return fmt.Errorf("read asset %s: %w", targetPath, errors.Join(readErr, closeErr))
	}
	doc, found, err := metacomp.ReadTarget(root, targetPath)
	if err != nil {
		return err
	}
	if !found {
		doc = metacomp.New(targetPath, metacomp.Digest(data))
	}
	applyPrivacy(doc, value, clearance)
	proposed, err := metacomp.Encode(doc)
	if err != nil {
		return err
	}
	companionRel := targetPath + metacomp.CompanionSuffix
	if !found {
		return contextwrite.Create(root, companionRel, proposed, 0o644)
	}
	companionPath := filepath.Join(root, filepath.FromSlash(companionRel))
	current, err := os.ReadFile(companionPath) //#nosec G304 -- path is context-relative and validated by ReadTarget
	if err != nil {
		return err
	}
	return contextwrite.Write(root, companionPath, contextwrite.Digest(current), proposed)
}

func privacyClearance(value, basis string) (*materialpackage.PrivacyClearance, error) {
	basis = strings.TrimSpace(basis)
	if value == materialpackage.PrivacyNone && basis != "" {
		return nil, fmt.Errorf("privacy none must not include a clearance basis")
	}
	var clearance *materialpackage.PrivacyClearance
	if value == materialpackage.PrivacyPersonal || value == materialpackage.PrivacyReal {
		if basis == "" {
			return nil, fmt.Errorf("privacy %s requires --basis", value)
		}
		clearance = &materialpackage.PrivacyClearance{Status: materialpackage.PrivacyCleared, Basis: basis}
	}
	if err := materialpackage.ValidatePrivacy(value, clearance); err != nil {
		return nil, err
	}
	return clearance, nil
}

func applyPrivacy(target map[string]any, value string, clearance *materialpackage.PrivacyClearance) {
	target["privacy"] = value
	if clearance == nil {
		delete(target, "privacy_clearance")
		return
	}
	target["privacy_clearance"] = map[string]any{ctxMapStatus: clearance.Status, "basis": clearance.Basis}
}
