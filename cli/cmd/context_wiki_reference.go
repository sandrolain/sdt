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

// ── conditional reference bridge (sdt context wiki reference) ────────────────
//
// The reference lane is optional until a page links a bundle: then the bundle
// is a contract and every file in it must be cited. A bundle no page links is
// reported as optional, not as a defect.

const referenceStatusOptional = "optional"

type referenceReport struct {
	Dir         string   `json:"dir" yaml:"dir"`
	Status      string   `json:"status" yaml:"status"`
	Files       int      `json:"files" yaml:"files"`
	CitingPages []string `json:"citing_pages,omitempty" yaml:"citing_pages,omitempty"`
	IndexPages  []string `json:"index_pages,omitempty" yaml:"index_pages,omitempty"`
	Missing     []string `json:"missing,omitempty" yaml:"missing,omitempty"`
}

var contextWikiReferenceCmd = &cobra.Command{
	Use:   "reference [--strict]",
	Short: "Check the conditional reference→index bridge",
	Long: `Check the reference bundles under context/refs/reference/.

A bundle is mandatory once a wiki page links at least one file in it: every
.md file in the bundle must then be cited by some wiki page. A bundle no page
links is optional and reports no finding; --strict turns an optional bundle
into a finding.

The check is read-only. Exit 1 on a mandatory gap (or any optional bundle with
--strict), 0 otherwise.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld := &contextwiki.Builder{}
		if _, statErr := os.Stat(sdtWikiDir); statErr == nil {
			loaded, _, err := contextwiki.LoadWiki(sdtWikiDir)
			if err != nil {
				exitWithError(cmd, fmt.Errorf("load wiki: %w", err))
				return
			}
			bld = loaded
		} else if !os.IsNotExist(statErr) {
			exitWithError(cmd, statErr)
			return
		}
		reports, err := referenceReports(bld)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		strict := getBoolFlag(cmd, "strict", false)
		failed := false
		for _, r := range reports {
			if r.Status == doctorStatusFail || (strict && r.Status == referenceStatusOptional) {
				failed = true
			}
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(reports, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(reports)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if len(reports) == 0 {
				outputString(cmd, "reference: none\n")
			}
			for _, r := range reports {
				switch r.Status {
				case doctorStatusPass:
					outputString(cmd, fmt.Sprintf("[OK] %s (%d files, %d pages)\n", r.Dir, r.Files, len(r.CitingPages)))
				case referenceStatusOptional:
					outputString(cmd, fmt.Sprintf("[OPTIONAL] %s (%d files, no citing index page)\n", r.Dir, r.Files))
				default:
					outputString(cmd, fmt.Sprintf("[FAIL] %s: missing %s\n", r.Dir, strings.Join(r.Missing, ", ")))
				}
			}
		}
		if failed {
			exit(exitWikiError)
		}
	},
}

func referenceReports(bld *contextwiki.Builder) ([]referenceReport, error) {
	root := filepath.Join(sdtRefsDir, "reference")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var reports []referenceReport
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		files, ferr := referenceFiles(dir)
		if ferr != nil {
			return nil, ferr
		}
		if len(files) == 0 {
			continue
		}
		reports = append(reports, referenceReportForDir(dir, files, bld))
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Dir < reports[j].Dir })
	return reports, nil
}

func referenceReportForDir(dir string, files []string, bld *contextwiki.Builder) referenceReport {
	report := referenceReport{Dir: corpusRelative(dir), Files: len(files), Status: referenceStatusOptional}
	cited := map[string]bool{}
	pageSet := map[string]bool{}
	indexSet := map[string]bool{}
	for _, page := range bld.Pages {
		for _, file := range files {
			ref := corpusRelative(file)
			if strings.Contains(page.Body, ref) || inventoryContainsString(page.Sources, ref) {
				cited[ref] = true
				pageSet[corpusRelative(page.Path)] = true
				if page.Type == wikiTypeEntity {
					indexSet[corpusRelative(page.Path)] = true
				}
			}
		}
	}
	for page := range pageSet {
		report.CitingPages = append(report.CitingPages, page)
	}
	for page := range indexSet {
		report.IndexPages = append(report.IndexPages, page)
	}
	sort.Strings(report.CitingPages)
	sort.Strings(report.IndexPages)
	if len(report.IndexPages) == 0 {
		return report
	}
	report.Status = doctorStatusPass
	for _, file := range files {
		ref := corpusRelative(file)
		if !cited[ref] {
			report.Missing = append(report.Missing, ref)
		}
	}
	if len(report.Missing) > 0 {
		report.Status = doctorStatusFail
		sort.Strings(report.Missing)
	}
	return report
}

func referenceFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func init() {
	contextWikiReferenceCmd.Flags().Bool("strict", false, "Treat an optional bundle with no citing page as a finding")
	contextWikiCmd.AddCommand(contextWikiReferenceCmd)
}
