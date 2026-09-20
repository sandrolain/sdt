package cmd

import (
	"fmt"
	"os"
	"strings"
)

// Cross-analysis overlap lint: two active analyses that share an `objective`
// and have highly similar title/summary are likely the same line of work.
// Advisory only (SUGGESTION): it never merges documents and never fails lint.

// ctxAnalysisOverlapThreshold is the token-set Jaccard similarity of the
// title+summary signature above which two analyses sharing an objective are
// reported as overlapping.
const ctxAnalysisOverlapThreshold = 0.60

// analysisSignature builds the comparison text of an analysis from its title
// (or filename stem when no title) and its summary.
func analysisSignature(path, content string) string {
	title := parseFrontmatterField(content, "title")
	if title == "" {
		title = strings.TrimSuffix(strings.TrimPrefix(path, sdtAnalysisDir+"/"), sdtMarkdownExt)
	}
	return normalizedBody(title + " " + parseFrontmatterField(content, "summary"))
}

// lintOverlappingAnalyses scans the analysis files and reports pairs that share
// an `objective` and whose title+summary signature is at least
// ctxAnalysisOverlapThreshold similar. Each file is reported at most once.
func lintOverlappingAnalyses(files []string) []ctxLintIssue {
	type analysis struct {
		path      string
		objective string
		sig       map[string]struct{}
	}
	byObjective := map[string][]analysis{}
	for _, f := range files {
		data, err := os.ReadFile(f) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		objective := parseFrontmatterField(content, "objective")
		if objective == "" {
			continue
		}
		sig := analysisSignature(f, content)
		if sig == "" {
			continue
		}
		byObjective[objective] = append(byObjective[objective], analysis{path: f, objective: objective, sig: tokenSet(sig)})
	}
	var issues []ctxLintIssue
	reported := map[string]bool{}
	for _, group := range byObjective {
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				a, b := group[i], group[j]
				if reported[b.path] {
					continue
				}
				if jc := jaccard(a.sig, b.sig); jc >= ctxAnalysisOverlapThreshold {
					reported[b.path] = true
					issues = append(issues, ctxLintIssue{
						Path:     b.path,
						Priority: ctxLintSuggestion,
						Message:  fmt.Sprintf("overlaps analysis %s (objective %s, similarity %.2f); link it (`links`/`supersedes`) or state why they differ", a.path, a.objective, jc),
					})
				}
			}
		}
	}
	return issues
}
