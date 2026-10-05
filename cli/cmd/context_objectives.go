package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/mdstruct"
)

// Objective-to-phase traceability: an analysis that declares an `## Objectives`
// checklist says what it must deliver, and a plan phase that writes
// `**Covers:** <id>` says which of those it delivers. `lintObjectivePhaseCoverage`
// compares the two in both directions, so an objective nobody covers and a
// phase covering an objective nobody declared are both visible.
//
// Only the `## Objectives` **checklist** is parsed. A prose scrape (`**B1.`
// sentences, `| B1 |` table cells) was measured on this corpus and yields 87
// false positives: documents discuss objective ids far more often than they
// declare them.

// ctxObjectivesHeading is the analysis section declaring the objectives.
const ctxObjectivesHeading = "Objectives"

// ctxObjectiveIDRegexp is the closed identity grammar of one objective: an
// uppercase letter and digits (`O1`, `B3`). The prefix is free so an analysis
// can keep its own naming (`B1`-`B6`) instead of renumbering to `O1`-`On`.
// Nothing outside the `## Objectives` checklist is read, so a document may use
// these shapes freely in prose.
var ctxObjectiveIDRegexp = regexp.MustCompile(`\b([A-Z][0-9]+)\b`)

// ctxPhaseCoversRegexp finds the `**Covers:**` token on a phase line. Any words
// between the prefix and the id are prose ("analysis B1"), so the ids are
// scanned from the whole value rather than matched positionally.
var ctxPhaseCoversRegexp = regexp.MustCompile(`\*\*Covers:\*\*\s*([^\n]*)`)

// analysisObjective is one declared objective: its id, the item text and whether
// it is already ticked.
type analysisObjective struct {
	ID   string `json:"id" yaml:"id"`
	Text string `json:"text" yaml:"text"`
	Done bool   `json:"done" yaml:"done"`
}

// parseAnalysisObjectives reads the `## Objectives` checklist of an analysis
// document. It reports false when the section is absent or holds no checklist
// item: an analysis without the section is a valid, graceful state, not a
// finding.
func parseAnalysisObjectives(content string) ([]analysisObjective, bool) {
	var objectives []analysisObjective
	found := false
	for _, section := range mdstruct.SplitSections(content) {
		if section.Level != 2 || !strings.EqualFold(section.Heading, ctxObjectivesHeading) {
			continue
		}
		found = true
		for _, entry := range parseChecklistEntries(strings.Split(section.Body, "\n")) {
			body, _ := splitChecklistAnchor(entry.Body)
			body = strings.TrimSpace(body)
			ids := ctxObjectiveIDRegexp.FindAllStringSubmatch(body, -1)
			if len(ids) == 0 {
				continue
			}
			// Only a leading id declares an objective; an id quoted later in the
			// sentence is part of the prose.
			id := ids[0][1]
			if !strings.HasPrefix(body, id) {
				continue
			}
			objectives = append(objectives, analysisObjective{
				ID:   id,
				Text: strings.TrimSpace(body[len(id):]),
				Done: entry.Marker == "x",
			})
		}
	}
	if !found || len(objectives) == 0 {
		return nil, false
	}
	return objectives, true
}

// phaseCoverage is one plan phase's `**Covers:**` claim.
type phaseCoverage struct {
	Path  string   `json:"path" yaml:"path"`
	Phase string   `json:"phase" yaml:"phase"`
	IDs   []string `json:"covers" yaml:"covers"`
}

// parsePlanPhaseCoverage reads the `**Covers:**` claim of every `### Phase`
// section of a plan document. A plan that claims nothing is normal, so it yields
// no coverage and raises nothing.
func parsePlanPhaseCoverage(path, content string) []phaseCoverage {
	var out []phaseCoverage
	for _, section := range mdstruct.SplitSections(content) {
		if section.Level != 3 || !strings.HasPrefix(strings.ToLower(section.Heading), "phase") {
			continue
		}
		label := phaseHeadingLabel(section.Heading)
		if label == "" {
			continue
		}
		ids := coversIDs(section.Body)
		if len(ids) == 0 {
			continue
		}
		out = append(out, phaseCoverage{Path: path, Phase: label, IDs: ids})
	}
	return out
}

// ctxPhaseHeadingLabelRegexp captures the identity token of a phase heading
// (`### Phase 2 — title` -> `2`). The title is prose; the token is the address
// the finding names.
var ctxPhaseHeadingLabelRegexp = regexp.MustCompile(`(?i)^phase[ \t]+([0-9a-z]+)`)

// phaseHeadingLabel is the label a finding names for a phase: its identity token
// when the heading carries one, otherwise the whole heading.
func phaseHeadingLabel(heading string) string {
	if m := ctxPhaseHeadingLabelRegexp.FindStringSubmatch(strings.TrimSpace(heading)); m != nil {
		return m[1]
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(heading), "Phase"))
}

// coversIDs extracts every objective id a `**Covers:**` line claims, in first
// appearance order and without duplicates.
func coversIDs(body string) []string {
	m := ctxPhaseCoversRegexp.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	for _, match := range ctxObjectiveIDRegexp.FindAllStringSubmatch(m[1], -1) {
		id := match[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// objectiveCoverage is the analysis/plan reconciliation: which objectives the
// analysis declares, and which of them any derived plan phase claims.
type objectiveCoverage struct {
	AnalysisRef string
	objectives  []analysisObjective
	covered     map[string][]string // objective id -> "<plan ref> phase <n>"
	undeclared  []undeclaredCover
}

type undeclaredCover struct {
	Path  string
	Phase string
	ID    string
}

// buildObjectiveCoverage reconciles every analysis that declares `## Objectives`
// against the plan phases derived from it. It returns nothing when no analysis
// declares the section — the graceful case.
func buildObjectiveCoverage(planFiles []string, edges *ctxrel.Edges) []objectiveCoverage {
	analyses := declaringAnalyses()
	if len(analyses) == 0 {
		return nil
	}
	coverages := make([]objectiveCoverage, 0, len(analyses))
	for _, a := range analyses {
		cov := objectiveCoverage{AnalysisRef: a.ref, objectives: a.objectives, covered: map[string][]string{}}
		declaredIDs := map[string]bool{}
		for _, o := range a.objectives {
			declaredIDs[o.ID] = true
		}
		// Derived plans are resolved by the typed `analysis_id` relation, so a
		// plan that merely cites the analysis in `sources` is not counted.
		for _, planRef := range edges.ChildrenOf(a.ref) {
			path, found := ctxRefToPath(planRef)
			if !found || !slicesContains(planFiles, path) {
				continue
			}
			content, ok := readReconcileDoc(path)
			if !ok {
				continue
			}
			collectPhaseCoverage(&cov, planRef, path, content, declaredIDs)
		}
		coverages = append(coverages, cov)
	}
	return coverages
}

// declaringAnalysis is one analysis that declares an `## Objectives` checklist.
type declaringAnalysis struct {
	ref        string
	objectives []analysisObjective
}

// declaringAnalyses lists the analyses that actually declare objectives. An
// unreadable directory yields none, so the check degrades to silence rather than
// reporting every objective as uncovered.
func declaringAnalyses() []declaringAnalysis {
	analysisFiles, err := dirFiles(sdtAnalysisDir)
	if err != nil {
		return nil
	}
	var analyses []declaringAnalysis
	for _, path := range analysisFiles {
		content, ok := readReconcileDoc(path)
		if !ok || parseFrontmatterField(content, "kind") != ctxTypeAnalysis {
			continue
		}
		objectives, ok := parseAnalysisObjectives(content)
		if !ok {
			continue
		}
		analyses = append(analyses, declaringAnalysis{ref: normalizeContextRef(path), objectives: objectives})
	}
	return analyses
}

// collectPhaseCoverage folds one derived plan's phase claims into the coverage:
// a claimed id the analysis does not declare is undeclared (WARNING), a declared
// one is recorded with the phase that claims it.
func collectPhaseCoverage(cov *objectiveCoverage, planRef, path, content string, declaredIDs map[string]bool) {
	for _, pc := range parsePlanPhaseCoverage(path, content) {
		for _, id := range pc.IDs {
			if !declaredIDs[id] {
				cov.undeclared = append(cov.undeclared, undeclaredCover{Path: pc.Path, Phase: pc.Phase, ID: id})
				continue
			}
			cov.covered[id] = append(cov.covered[id], fmt.Sprintf("%s phase %s", planRef, pc.Phase))
		}
	}
}

// ctxRefToPath maps a corpus-relative reference back to the path dirFiles
// yields. Both carry the `context/` prefix (ctxrel.CorpusDir), so the reference
// already *is* the project-relative path; the check exists so a reference from
// outside the corpus is rejected rather than read.
func ctxRefToPath(ref string) (string, bool) {
	if !strings.HasPrefix(ref, sdtWorkDir+"/") {
		return "", false
	}
	return ref, true
}

// slicesContains reports whether needle is in haystack.
func slicesContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// lintObjectivePhaseCoverage reports both directions of the traceability gap:
// a plan phase claiming an objective the source analysis does not declare
// (WARNING — the claim cannot be checked), and a declared objective no derived
// plan phase covers (SUGGESTION — nobody claimed it). An analysis without the
// section is skipped in both directions.
func lintObjectivePhaseCoverage(planFiles []string, edges *ctxrel.Edges) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, cov := range buildObjectiveCoverage(planFiles, edges) {
		analysisPath, _ := ctxRefToPath(cov.AnalysisRef)
		for _, u := range cov.undeclared {
			issues = append(issues, ctxLintIssue{Path: u.Path, Priority: ctxLintWarning,
				Message: fmt.Sprintf("undeclared objective %s is claimed by phase %s; %s does not declare it in `## Objectives`",
					u.ID, u.Phase, cov.AnalysisRef)})
		}
		for _, o := range cov.objectives {
			if len(cov.covered[o.ID]) == 0 {
				issues = append(issues, ctxLintIssue{Path: analysisPath, Priority: ctxLintSuggestion,
					Message: fmt.Sprintf("uncovered objective %s is declared in `## Objectives` but no derived plan phase claims it", o.ID)})
			}
		}
	}
	return issues
}
