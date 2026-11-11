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
)

// The delivery boundary has three independent states: freshness says the
// derivation still matches its page, review says the ledger records a critic and
// a reader, and publishable says both hold and the rights position is declared.
// A fresh material is not a reviewed one, and an unrun review is never "passed".

var materialStatusCmd = &cobra.Command{
	Use:   "status [<concept>]",
	Short: "Report freshness, review and publishability",
	Long: `Report the three states of each guide, one row per concept and language.

Freshness is the render lane's verdict (fresh/stale/missing/orphan). Review comes
from the ledger beside the guide: "not run" when the critic or the reader is
empty or says so, "pending" when one says "to run", "recorded" otherwise — the
raw text is carried, never rewritten into "passed". Publishable requires all
three: fresh, a recorded review, and a classification that is not "undeclared".
Exit 0: a status is a report, not a gate.`,
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
			data, _ := json.MarshalIndent(rows, "", "  ")
			outputBytes(cmd, data)
		case fmtYAML:
			data, _ := yaml.Marshal(rows)
			outputBytes(cmd, data)
		default:
			for _, row := range rows {
				line := fmt.Sprintf("%s  %s  %s  %s", row.Concept, row.Lang, row.Syllabus, row.Freshness)
				if row.Deck != "" {
					line += "  deck: " + row.Deck
				}
				line += fmt.Sprintf("  review: %s  publishable: %s", row.Review, yesNo(row.Publishable))
				if reason := publishableReason(row); reason != "" {
					line += " — " + reason
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
	Concept        string `json:"concept" yaml:"concept"`
	Syllabus       string `json:"syllabus" yaml:"syllabus"`
	Lang           string `json:"lang" yaml:"lang"`
	Freshness      string `json:"freshness" yaml:"freshness"`
	Deck           string `json:"deck,omitempty" yaml:"deck,omitempty"`
	Review         string `json:"review" yaml:"review"`
	Publishable    bool   `json:"publishable" yaml:"publishable"`
	Classification string `json:"classification" yaml:"classification"`
	Ledger         string `json:"ledger,omitempty" yaml:"ledger,omitempty"`
	Detail         string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// publishableReason names the first unmet gate, so a reader can tell an
// unreviewed material from one whose rights are undeclared.
func publishableReason(row materialStatusRow) string {
	if row.Publishable {
		return ""
	}
	var reasons []string
	if row.Freshness != renderFresh {
		reasons = append(reasons, "freshness "+row.Freshness)
	}
	if row.Deck != "" && row.Deck != renderFresh {
		reasons = append(reasons, "deck "+row.Deck)
	}
	if row.Review != "recorded" {
		reasons = append(reasons, "review "+row.Review)
	}
	if row.Classification == "" || row.Classification == "undeclared" {
		reasons = append(reasons, "classification undeclared")
	}
	return strings.Join(reasons, "; ")
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
	for _, row := range report {
		freshness[filepath.ToSlash(row.Path)] = row.Status
	}

	concept = strings.TrimPrefix(concept, "wiki/")
	syllabus = strings.TrimPrefix(syllabus, "syllabi/")
	rows := make([]materialStatusRow, 0, len(files))
	for _, file := range files {
		if concept != "" && file.Header.Concept != concept {
			continue
		}
		if syllabus != "" && strings.TrimPrefix(file.Header.Syllabus, "syllabi/") != syllabus {
			continue
		}
		if lang != "" && file.Header.Lang != lang {
			continue
		}
		rel, relErr := filepath.Rel(root, file.Path)
		if relErr != nil {
			rel = file.Path
		}
		status := freshness[filepath.ToSlash(rel)]
		if status == "" {
			status = "unknown"
		}
		deckRel := filepath.ToSlash(strings.TrimSuffix(rel, sdtMarkdownExt) + ".slide" + sdtMarkdownExt)
		deckStatus := freshness[deckRel]
		ledgerPath := strings.TrimSuffix(file.Path, sdtMarkdownExt) + ".ledger.json"
		review, detail, found := materialReviewState(ledgerPath)
		row := materialStatusRow{
			Concept:        file.Header.Concept,
			Syllabus:       strings.TrimPrefix(file.Header.Syllabus, "syllabi/"),
			Lang:           file.Header.Lang,
			Freshness:      status,
			Deck:           deckStatus,
			Review:         review,
			Classification: file.Header.Classification,
			Detail:         detail,
		}
		if found {
			if ledgerRel, ledgerErr := filepath.Rel(root, ledgerPath); ledgerErr == nil {
				row.Ledger = filepath.ToSlash(ledgerRel)
			}
		}
		row.Publishable = status == renderFresh &&
			(deckStatus == "" || deckStatus == renderFresh) &&
			review == "recorded" &&
			file.Header.Classification != "" && file.Header.Classification != "undeclared"
		rows = append(rows, row)
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

// materialReviewState reads the free-text cycle. It never calls an unrun review
// passed: only "recorded" is earned, and the raw text travels in the detail.
func materialReviewState(path string) (state, detail string, found bool) {
	data, err := os.ReadFile(path) //#nosec G304 -- the ledger beside the resolved guide
	if err != nil {
		return "not run", "no ledger beside the guide", false
	}
	var ledger materialLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return "not run", "ledger unreadable: " + err.Error(), true
	}
	critic := strings.TrimSpace(ledger.Cycle.Critic)
	reader := strings.TrimSpace(ledger.Cycle.Reader)
	detail = fmt.Sprintf("critic=%q reader=%q", critic, reader)
	switch {
	case critic == "" || reader == "" ||
		strings.EqualFold(critic, "not run") || strings.EqualFold(reader, "not run"):
		return "not run", detail, true
	case strings.EqualFold(critic, "to run") || strings.EqualFold(reader, "to run"):
		return "pending", detail, true
	default:
		return "recorded", detail, true
	}
}
