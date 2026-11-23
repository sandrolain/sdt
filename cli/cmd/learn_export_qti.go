package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/qti"
)

var learnExportQTICmd = &cobra.Command{
	Use:   "export-qti <concept>",
	Short: "Export a frozen choice assessment as a QTI 3.0 package",
	Long: `Export one current, frozen choice assessment as an IMS QTI 3.0 item-bank ZIP.
The archive contains prompts, options, scoring and answer keys for an assessment
consumer; it never contains learner identity or attempts. Redirect stdout to a
file, for example: sdt learn export-qti architecture/example > bank.zip.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		archive, err := qtiPackageForConceptOptions(cmd, args[0], getBoolFlag(cmd, "supported-only", false))
		if err != nil {
			return err
		}
		outputBytes(cmd, archive)
		return nil
	},
}

func qtiPackageForConcept(cmd *cobra.Command, concept string) ([]byte, error) {
	return qtiPackageForConceptOptions(cmd, concept, false)
}

func qtiPackageForConceptOptions(cmd *cobra.Command, concept string, supportedOnly bool) ([]byte, error) {
	items, ok := learnAssessmentItems(concept)
	if !ok {
		return nil, fmt.Errorf("no assessment bank for %q", concept)
	}
	if stale, found := learnAssessmentStaleItem(concept); found {
		return nil, fmt.Errorf("item %q key changed since it was frozen; refresh its key_hash before export", stale.ID)
	}
	page := loadLearnWiki(cmd).Resolve(concept)
	if page == nil {
		return nil, fmt.Errorf("unknown concept %q", concept)
	}
	packageItems := make([]qti.Item, 0, len(items))
	for _, item := range items {
		if supportedOnly && item.Interaction == "" {
			continue
		}
		if item.KeyHash == "" || item.KeyHash != item.KeyDigest {
			return nil, fmt.Errorf("item %q has no current key_hash; freeze the key before export", item.ID)
		}
		if item.Interaction == "" {
			return nil, fmt.Errorf("item %q has no QTI interaction; add the optional choice columns before export", item.ID)
		}
		if findings := lintChoiceFields(learnAssessmentPath(concept), item.ID, item.Interaction, item.Options, item.Expected, item.Scoring, item.ExplanationPrompt); len(findings) > 0 {
			return nil, fmt.Errorf("item %q cannot be exported: %s", item.ID, findings[0].Message)
		}
		var correct []string
		if err := json.Unmarshal([]byte(item.Expected), &correct); err != nil {
			return nil, fmt.Errorf("item %q expected is not a JSON array of option ids: %w", item.ID, err)
		}
		choices := make([]qti.Option, 0, len(item.ChoiceOptions))
		for _, option := range item.ChoiceOptions {
			choices = append(choices, qti.Option{ID: option.ID, Text: option.Text})
		}
		packageItems = append(packageItems, qti.Item{
			ID: item.ID, Title: item.ID, Prompt: item.Prompt,
			Objective: item.Objective, Claim: item.Claim, Provenance: item.Provenance,
			Interaction: item.Interaction,
			Options:     choices, Correct: correct, Scoring: item.Scoring,
			ExplanationPrompt: item.ExplanationPrompt,
		})
	}
	if len(packageItems) == 0 && supportedOnly {
		return nil, fmt.Errorf("assessment %q has no supported QTI items", concept)
	}
	return qti.WritePackage(qti.Package{ID: qtiSafeID(page.ID), Title: page.Title, Items: packageItems})
}

func qtiSafeID(value string) string {
	clean := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			clean = append(clean, r)
		} else {
			clean = append(clean, '-')
		}
	}
	if len(clean) == 0 {
		return "sdt-"
	}
	if !isASCIIAlpha(clean[0]) {
		return "sdt-" + string(clean)
	}
	return string(clean)
}

func isASCIIAlpha(value rune) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func init() {
	learnExportQTICmd.Flags().Bool("supported-only", false, "Skip assessment items without a QTI interaction")
	learnCmd.AddCommand(learnExportQTICmd)
}
