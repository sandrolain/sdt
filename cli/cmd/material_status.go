//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// Material status reports evidence by dimension. It deliberately has no overall
// readiness verdict: a material can be current and reviewed while its package
// has not been built or checked for a destination.

var materialStatusCmd = &cobra.Command{
	Use:   "status [<concept>]",
	Short: "Report material readiness dimensions",
	Long: `Report currency, content review, technical usability and distributability
for each guide, one row per concept and language.

Currency is the render lane's freshness verdict. Review comes from the guide's
review record. Technical status reports source-level findings and whether a
deck is present, current and renderable. This status targets the guide and its
deck. Distributability reports the declared rights
classification, but does not claim a package is ready: run
"sdt material package export" to build and validate the destination package.
There is no overall ready/publishable verdict. Exit 0: this is a report, not a gate.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		concept := ""
		if len(args) > 0 {
			concept = args[0]
		}
		rows, err := materialStatusRows(concept, getStringFlag(cmd, "syllabus", false), getStringFlag(cmd, "lang", false))
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			data, err := json.MarshalIndent(rows, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, data)
		case fmtYAML:
			data, err := yaml.Marshal(rows)
			exitWithError(cmd, err)
			outputBytes(cmd, data)
		default:
			for _, row := range rows {
				line := fmt.Sprintf("%s  %s  %s  currency: %s", row.Concept, row.Lang, row.Syllabus, row.Freshness)
				if row.CurrencyDetail != "" {
					line += " (" + row.CurrencyDetail + ")"
				}
				if row.Deck != "" {
					line += "  deck: " + row.Deck
				}
				line += fmt.Sprintf("  review: %s  technical: %s  distribution: %s", row.Review, row.Technical, row.Distributability)
				if row.Detail != "" {
					line += " — review detail: " + row.Detail
				}
				if row.TechnicalDetail != "" {
					line += " — technical detail: " + row.TechnicalDetail
				}
				if row.DistributionDetail != "" {
					line += " — distribution detail: " + row.DistributionDetail
				}
				outputString(cmd, line+"\n")
			}
		}
	},
}

func init() {
	materialStatusCmd.Flags().String("syllabus", "", "syllabus to filter, with or without syllabi/")
	materialStatusCmd.Flags().String("lang", "", "language to filter, e.g. it")
	materialCmd.AddCommand(materialStatusCmd)
}

type materialLedger struct {
	Cycle struct {
		Critic string `json:"critic"`
		Reader string `json:"reader"`
	} `json:"cycle"`
}

type materialStatusRow struct {
	Concept            string `json:"concept" yaml:"concept"`
	Syllabus           string `json:"syllabus" yaml:"syllabus"`
	Lang               string `json:"lang" yaml:"lang"`
	Freshness          string `json:"currency" yaml:"currency"`
	CurrencyDetail     string `json:"currency_detail,omitempty" yaml:"currency_detail,omitempty"`
	Deck               string `json:"deck,omitempty" yaml:"deck,omitempty"`
	DeckDetail         string `json:"deck_detail,omitempty" yaml:"deck_detail,omitempty"`
	Review             string `json:"review" yaml:"review"`
	Classification     string `json:"classification" yaml:"classification"`
	Technical          string `json:"technical" yaml:"technical"`
	Distributability   string `json:"distributability" yaml:"distributability"`
	TechnicalDetail    string `json:"technical_detail,omitempty" yaml:"technical_detail,omitempty"`
	DistributionDetail string `json:"distribution_detail,omitempty" yaml:"distribution_detail,omitempty"`
	Ledger             string `json:"ledger,omitempty" yaml:"ledger,omitempty"`
	Detail             string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

func materialStatusRows(concept, syllabus, lang string) ([]materialStatusRow, error) {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	files, err := loadMaterialFiles()
	if err != nil {
		return nil, err
	}
	builder := loadMaterialWiki()
	if builder == nil {
		return nil, fmt.Errorf("load the wiki: the status reads the page a material derives from")
	}
	report, err := materialReport(builder)
	if err != nil {
		return nil, err
	}
	freshness := map[string]string{}
	currencyDetails := map[string]string{}
	for _, row := range report {
		freshness[filepath.ToSlash(row.Path)] = row.Status
		currencyDetails[filepath.ToSlash(row.Path)] = row.Detail
	}

	concept = strings.TrimPrefix(concept, "wiki/")
	syllabus = strings.TrimPrefix(syllabus, "syllabi/")
	rows := make([]materialStatusRow, 0, len(files))
	for _, file := range files {
		if row, ok := materialStatusRowFor(file, root, concept, syllabus, lang, freshness, currencyDetails, builder); ok {
			rows = append(rows, row)
		}
	}
	if concept != "" && len(rows) == 0 {
		return nil, fmt.Errorf("no material for %q with syllabus %q and language %q", concept, syllabus, lang)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Concept != rows[j].Concept {
			return rows[i].Concept < rows[j].Concept
		}
		return rows[i].Lang < rows[j].Lang
	})
	return rows, nil
}

func materialStatusRowFor(file materialFile, root, concept, syllabus, lang string, freshness, currencyDetails map[string]string, wiki *contextwiki.Builder) (materialStatusRow, bool) {
	if !materialStatusMatches(file, concept, syllabus, lang) {
		return materialStatusRow{}, false
	}
	rel, relErr := filepath.Rel(root, file.Path)
	if relErr != nil {
		rel = file.Path
	}
	status := freshness[filepath.ToSlash(rel)]
	if status == "" {
		status = learnProvenanceUnknown
	}
	ledgerPath := strings.TrimSuffix(file.Path, sdtMarkdownExt) + ".ledger.json"
	metadataPath := materialMetadataPathOf(file.Path)
	if file.Metadata != nil {
		ledgerPath = metadataPath
	}
	review, detail, found := materialStatusReview(file, ledgerPath)
	row := materialStatusRow{
		Concept: file.Header.Concept, Syllabus: strings.TrimPrefix(file.Header.Syllabus, "syllabi/"),
		Lang: file.Header.Lang, Freshness: status, CurrencyDetail: currencyDetails[filepath.ToSlash(rel)],
		Review: review, Classification: file.Header.Classification, Detail: detail,
	}
	fillMaterialTechnical(&row, file.Path, wiki)
	fillMaterialDistribution(&row)
	if found {
		if ledgerRel, err := filepath.Rel(root, ledgerPath); err == nil {
			row.Ledger = filepath.ToSlash(ledgerRel)
		}
	}
	deckRel := filepath.ToSlash(strings.TrimSuffix(rel, sdtMarkdownExt) + ".slide" + sdtMarkdownExt)
	row.Deck, row.DeckDetail = materialStatusDeck(root, deckRel, freshness)
	fillMaterialDeckTechnical(&row)
	return row, true
}

func materialStatusMatches(file materialFile, concept, syllabus, lang string) bool {
	return (concept == "" || file.Header.Concept == concept) &&
		(syllabus == "" || strings.TrimPrefix(file.Header.Syllabus, "syllabi/") == syllabus) &&
		(lang == "" || file.Header.Lang == lang)
}

func materialStatusReview(file materialFile, ledgerPath string) (string, string, bool) {
	if file.Metadata != nil {
		return materialMetadataReviewState(file.Metadata)
	}
	return materialReviewState(ledgerPath)
}

func materialStatusDeck(root, deckRel string, freshness map[string]string) (string, string) {
	laneStatus := freshness[deckRel]
	status, detail := deckFreshnessOf(root, deckRel, laneStatus)
	if laneStatus != "" {
		return status, detail
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(deckRel))); os.IsNotExist(err) {
		return renderMissing, "deck file is absent"
	}
	return learnProvenanceUnknown, "deck is not present in the freshness report"
}

func fillMaterialTechnical(row *materialStatusRow, path string, wiki *contextwiki.Builder) {
	row.Technical = materialTechnicalSourceCheck
	content, err := os.ReadFile(path) //#nosec G304 -- path came from loadMaterialFiles walking the material lane
	if err != nil {
		row.Technical = learnProvenanceUnknown
		row.TechnicalDetail = "could not read guide: " + err.Error()
		return
	}
	clean, _ := cleanMaterialText(string(content), wiki)
	findings := checkMaterialContents(path, clean, wiki)
	if len(findings) == 0 {
		row.TechnicalDetail = "source checks passed; package link, asset and consumer rendering validation not run"
		return
	}
	row.Technical = "issues"
	messages := make([]string, 0, len(findings))
	for _, finding := range findings {
		messages = append(messages, finding.Message)
	}
	row.TechnicalDetail = strings.Join(messages, "; ")
}

func fillMaterialDistribution(row *materialStatusRow) {
	row.Distributability = "not assessed"
	if row.Classification == "" || row.Classification == "undeclared" {
		row.Distributability = "blocked"
		row.DistributionDetail = "rights classification undeclared"
		return
	}
	row.DistributionDetail = "declared classification does not prove destination compatibility; package export evaluates rights and validates the ZIP"
}

func fillMaterialDeckTechnical(row *materialStatusRow) {
	if row.Deck == renderFresh {
		if row.Technical == materialTechnicalSourceCheck {
			row.TechnicalDetail = "source checks passed; deck is fresh; package link, asset and consumer rendering validation not run"
		}
		return
	}
	row.Technical = "issues"
	reason := "deck " + row.Deck
	if row.DeckDetail != "" {
		reason += ": " + row.DeckDetail
	}
	if row.TechnicalDetail != "" {
		row.TechnicalDetail += "; "
	}
	row.TechnicalDetail += reason
}

func materialMetadataReviewState(doc map[string]any) (state, detail string, found bool) {
	review := metadataMap(doc["review"])
	if review == nil {
		return materialReviewNotRun, "review extension missing", true
	}
	cycle := metadataMap(review["cycle"])
	if cycle == nil {
		return materialReviewNotRun, "review cycle missing", true
	}
	critic := strings.TrimSpace(metadataString(cycle["critic"]))
	reader := strings.TrimSpace(metadataString(cycle["reader"]))
	detail = fmt.Sprintf("critic=%q reader=%q", critic, reader)
	return materialReviewStateOf(critic, reader, detail)
}

// deckFreshnessOf keeps the render lane's verdict for the deck beside a guide
// and adds the one structural check on top: a deck the lane calls fresh but
// that cannot render reports render-broken, and the row says why. Every other
// lane verdict stands.
func deckFreshnessOf(root, deckRel, laneStatus string) (string, string) {
	if laneStatus != renderFresh {
		return laneStatus, ""
	}
	reason, checked := deckRenderIssue(filepath.Join(root, filepath.FromSlash(deckRel)))
	if checked && reason != "" {
		return deckRenderBroken, reason
	}
	return laneStatus, ""
}

// materialReviewState reads the free-text cycle. It never calls an unrun review
// passed: only materialReviewRecorded is earned, and the raw text travels in the detail.
func materialReviewState(path string) (state, detail string, found bool) {
	data, err := os.ReadFile(path) //#nosec G304 -- the ledger beside the resolved guide
	if err != nil {
		return materialReviewNotRun, "no ledger beside the guide", false
	}
	var ledger materialLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return materialReviewNotRun, "ledger unreadable: " + err.Error(), true
	}
	critic := strings.TrimSpace(ledger.Cycle.Critic)
	reader := strings.TrimSpace(ledger.Cycle.Reader)
	detail = fmt.Sprintf("critic=%q reader=%q", critic, reader)
	return materialReviewStateOf(critic, reader, detail)
}

func materialReviewStateOf(critic, reader, detail string) (state, detailResult string, found bool) {
	switch {
	case critic == "" || reader == "" ||
		strings.EqualFold(critic, materialReviewNotRun) || strings.EqualFold(reader, materialReviewNotRun):
		return materialReviewNotRun, detail, true
	case strings.EqualFold(critic, "to run") || strings.EqualFold(reader, "to run"):
		return taskFileStatusPending, detail, true
	default:
		return materialReviewRecorded, detail, true
	}
}
