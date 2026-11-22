//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwrite"
	"github.com/sandrolain/sdt/internal/materialbrief"
	"github.com/sandrolain/sdt/internal/materialprofile"
	"github.com/sandrolain/sdt/internal/metacomp"
)

var materialProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage reusable versioned recipient profiles",
}

var materialProfileCreateCmd = &cobra.Command{
	Use:   "create <profile-id> <revision>",
	Short: "Create an immutable recipient profile revision from YAML input",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		profile, err := materialprofile.Decode(getInputBytes(cmd, nil), args[0], args[1])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		data, err := materialprofile.Encode(profile)
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		root := sdtProjectRoot()
		if root == "" {
			root = "."
		}
		path := filepath.Join("context", "learning", "profiles", profile.ID, profile.Revision+".yaml")
		if err := contextwrite.Create(root, path, data, 0o644); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, fmt.Sprintf("created recipient profile %s@%s\n", profile.ID, profile.Revision))
	},
}

var materialProfileCheckCmd = &cobra.Command{
	Use:   "check <profile-id> <revision>",
	Short: "Validate one immutable recipient profile revision",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		root := sdtProjectRoot()
		if root == "" {
			root = "."
		}
		profile, err := materialprofile.Read(root, args[0], args[1])
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			data, err := json.MarshalIndent(profile, "", "  ")
			exitWithError(cmd, err)
			if err == nil {
				outputBytes(cmd, data)
			}
		case fmtYAML:
			data, err := yaml.Marshal(profile)
			exitWithError(cmd, err)
			if err == nil {
				outputBytes(cmd, data)
			}
		default:
			outputString(cmd, fmt.Sprintf("valid recipient profile %s@%s — %s\n", profile.ID, profile.Revision, profile.Title))
		}
	},
}

var materialBriefCmd = &cobra.Command{
	Use:   "brief",
	Short: "Manage immutable, source-linked material composition briefs",
}

var materialBriefCreateCmd = &cobra.Command{
	Use:   "create <material-id> <revision>",
	Short: "Create an immutable material brief revision from YAML input",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		brief, err := materialbrief.Decode(getInputBytes(cmd, nil), args[0], args[1])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		root := sdtProjectRoot()
		if root == "" {
			root = "."
		}
		if brief.Profile != nil {
			if _, err := materialprofile.Read(root, brief.Profile.ID, brief.Profile.Revision); err != nil {
				exitWithError(cmd, fmt.Errorf("brief profile binding: %w", err))
				return
			}
		}
		data, err := materialbrief.Encode(brief)
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		path := filepath.Join("context", "learning", "briefs", filepath.FromSlash(brief.ID), brief.Revision+".yaml")
		if err := contextwrite.Create(root, path, data, 0o644); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, fmt.Sprintf("created material brief %s@%s\n", brief.ID, brief.Revision))
	},
}

var materialBriefCheckCmd = &cobra.Command{
	Use:   "check <material-id> <revision>",
	Short: "Validate one immutable material brief and its profile binding",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		root := sdtProjectRoot()
		if root == "" {
			root = "."
		}
		brief, err := materialbrief.Read(root, args[0], args[1])
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		if brief.Profile != nil {
			if _, err := materialprofile.Read(root, brief.Profile.ID, brief.Profile.Revision); err != nil {
				exitWithError(cmd, fmt.Errorf("brief profile binding: %w", err))
				return
			}
		}
		switch getFormat(cmd) {
		case fmtJSON:
			data, err := json.MarshalIndent(brief, "", "  ")
			exitWithError(cmd, err)
			if err == nil {
				outputBytes(cmd, data)
			}
		case fmtYAML:
			data, err := yaml.Marshal(brief)
			exitWithError(cmd, err)
			if err == nil {
				outputBytes(cmd, data)
			}
		default:
			outputString(cmd, fmt.Sprintf("valid material brief %s@%s — %s\n", brief.ID, brief.Revision, brief.Title))
		}
	},
}

var materialBriefBindCmd = &cobra.Command{
	Use:   "bind <material-id> <revision>",
	Short: "Bind a guide companion to an exact immutable brief revision",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := bindMaterialBrief(args[0], args[1]); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, fmt.Sprintf("bound material %s to brief revision %s\n", args[0], args[1]))
	},
}

func bindMaterialBrief(materialID, revision string) error {
	if !materialbrief.ValidMaterialID(materialID) {
		return fmt.Errorf("invalid material id %q", materialID)
	}
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	brief, err := materialbrief.Read(root, materialID, revision)
	if err != nil {
		return err
	}
	if brief.Profile != nil {
		if _, err := materialprofile.Read(root, brief.Profile.ID, brief.Profile.Revision); err != nil {
			return fmt.Errorf("brief profile binding: %w", err)
		}
	}
	guide := filepath.Join(root, "context", "renders", "materials", filepath.FromSlash(materialID)+sdtMarkdownExt)
	guideBytes, err := os.ReadFile(guide) //#nosec G304 -- material id is validated and rooted in the material lane
	if err != nil {
		return fmt.Errorf("read material %s: %w", materialID, err)
	}
	first, _, _ := strings.Cut(string(guideBytes), "\n")
	header, err := parseMaterialHeader(first)
	if err != nil {
		return fmt.Errorf("material %s has no valid header: %w", materialID, err)
	}
	canonicalID := strings.TrimPrefix(header.Syllabus, "syllabi/") + "/" + header.Concept + "." + header.Lang
	if canonicalID != materialID {
		return fmt.Errorf("material id %q does not match its header identity %q", materialID, canonicalID)
	}
	_, found, err := readMaterialMetadata(guide, header)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("material %s has no canonical metadata companion", materialID)
	}
	companionPath := materialMetadataPathOf(guide)
	companionBytes, err := os.ReadFile(companionPath) //#nosec G304 -- companion path is derived beside the validated material
	if err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	guideRel, err := filepath.Rel(rootAbs, guide)
	if err != nil {
		return err
	}
	doc, found, err := metacomp.ReadTarget(rootAbs, filepath.ToSlash(guideRel))
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("material %s companion disappeared while binding", materialID)
	}
	material := metadataMap(doc["material"])
	if material == nil {
		return fmt.Errorf("material %s companion has no material extension", materialID)
	}
	material["brief"] = map[string]any{"id": brief.ID, "revision": brief.Revision}
	doc["material"] = material
	proposed, err := metacomp.Encode(doc)
	if err != nil {
		return err
	}
	if err := contextwrite.Write(rootAbs, companionPath, contextwrite.Digest(companionBytes), proposed); err != nil {
		return err
	}
	return nil
}

func init() {
	materialProfileCmd.AddCommand(materialProfileCreateCmd, materialProfileCheckCmd)
	materialBriefCmd.AddCommand(materialBriefCreateCmd, materialBriefCheckCmd, materialBriefBindCmd)
	materialCmd.AddCommand(materialProfileCmd, materialBriefCmd)
}
