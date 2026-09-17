package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func lintFrontmatterReferences(path, content string, prio func(string) string) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, field := range []string{"sources", "links", "derived_from", "results"} {
		for _, ref := range parseFrontmatterList(content, field) {
			if _, ok := ctxResolvePath(sdtWorkDir, ref); ok {
				continue
			}
			message := "broken " + field + " reference: " + ref
			if field == "sources" {
				message = "broken source reference: " + ref
			}
			issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: message})
		}
	}
	return issues
}

// lintRFCAndPromptContract enforces the proposal and prompt instruction
// contracts.

func lintRFCAndPromptContract(path, content, kind string, prio func(string) string) []ctxLintIssue {
	var issues []ctxLintIssue
	if kind == ctxTypePrompt && len(parseFrontmatterList(content, "derived_from")) == 0 {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "prompt must declare `derived_from` provenance"})
	}
	if kind != ctxTypeProposal && kind != ctxTypePrompt {
		return issues
	}
	for _, field := range []string{"title", "status", "created", "updated"} {
		if parseFrontmatterField(content, field) == "" {
			issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintCritical), Message: "frontmatter missing mandatory `" + field + "`"})
		}
	}
	return issues
}

// ctxIndexLine renders one index row: relative path + summary.

type ctxLintIssue struct {
	Path     string `json:"path" yaml:"path"`
	Priority string `json:"priority" yaml:"priority"`
	Message  string `json:"message" yaml:"message"`
}

// lint issue priorities.
const (
	ctxLintCritical = "CRITICAL"
	ctxLintWarning  = "WARNING"
	// ctxTasksOversizedItems is the soft checklist-size guard for task files:
	// a phase whose checklist exceeds it triggers a SUGGESTION to split.
	ctxTasksOversizedItems = 10
)

func (i ctxLintIssue) String() string {
	return fmt.Sprintf("[%s] %s: %s", i.Priority, i.Path, i.Message)
}

// ctxLinkRegexp matches a `[[path]]` wiki-style link or a plain relative path.

var ctxLinkRegexp = regexp.MustCompile(`\[\[([a-zA-Z0-9_./-]+)\]\]`)

// ctxDerivedKinds are document kinds that derive from or extend another
// document and therefore must carry a `sources` frontmatter reference. Plan and
// tasks always derive (from analysis/plan by the 5-stage lifecycle); decision
// decisions and open-question collections state their origin. A greenfield
// analysis does not derive from anything, so analysis is not required to set
// sources (a follow-up analysis should set it but is not hard-flagged).

var ctxDerivedKinds = map[string]bool{
	ctxTypePlan:      true,
	ctxTypeTasks:     true,
	ctxTypeDecision:  true,
	ctxTypeQuestions: true,
}

// ctxTaskFileStatuses is the accepted task-file frontmatter status vocabulary:
// pending (to work on), in-progress, completed, archived — plus the legacy
// `active` value kept for pre-change task lists.

var ctxTaskFileStatuses = map[string]bool{
	taskFileStatusPending:    true,
	taskFileStatusInProgress: true,
	taskFileStatusCompleted:  true,
	taskFileStatusArchived:   true,
	taskFileStatusLegacy:     true,
}

// lintTaskFileStatus flags task files whose frontmatter status is outside the
// pending | in-progress | completed | archived vocabulary (legacy `active`
// accepted). WARNING so historical lists never hard-fail the check.

func lintTaskFileStatus(path, status string) []ctxLintIssue {
	if status == "" {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: "task file missing frontmatter `status` (pending | in-progress | completed | archived)"}}
	}
	if !ctxTaskFileStatuses[status] {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("task file status %q outside vocabulary (pending | in-progress | completed | archived)", status)}}
	}
	return nil
}

// lintDoc validates one context document. priorityFn lowers CRITICAL to
// WARNING when the file is legacy (no new-style frontmatter) so old history
// does not fail the whole check.

func lintDoc(path string) []ctxLintIssue {
	var issues []ctxLintIssue
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return []ctxLintIssue{{Path: path, Priority: ctxLintCritical, Message: err.Error()}}
	}
	content := string(data)
	// frontmatter: must start with --- and contain kind + summary.
	if !strings.HasPrefix(content, "---\n") {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: "missing YAML frontmatter"}}
	}
	kind := parseFrontmatterField(content, "kind")
	summary := parseFrontmatterField(content, "summary")

	// Legacy documents lack both kind and summary. Treat them as WARNING so the
	// historical archive does not hard-fail the check; only new-style docs
	// (with kind) require a mandatory summary.
	legacy := kind == "" && summary == ""
	prio := func(sev string) string {
		if legacy {
			return ctxLintWarning
		}
		return sev
	}
	if kind == "" {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintCritical), Message: "frontmatter missing `kind`"})
	}
	if summary == "" {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintCritical), Message: "frontmatter missing mandatory `summary`"})
	}
	if kind == ctxTypeTasks {
		issues = append(issues, lintTaskFileStatus(path, parseFrontmatterField(content, "status"))...)
	}
	// resolve [[links]] and links: array to existing documents.
	// Files under context dirs link relative to their own directory; the
	// generated index.md links relative to the context/ root.
	dir := filepath.Dir(path)
	linkBase := dir
	if path == sdtContextIndex {
		linkBase = sdtWorkDir
	}
	for _, m := range ctxLinkRegexp.FindAllStringSubmatch(content, -1) {
		target := m[1]
		abs := filepath.Join(linkBase, target)
		if !strings.HasSuffix(abs, sdtMarkdownExt) {
			abs += sdtMarkdownExt
		}
		if _, err := os.Stat(abs); os.IsNotExist(err) { //#nosec G703 -- validated against context/ tree
			issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "broken link [[" + target + "]]"})
		}
	}
	issues = append(issues, lintFrontmatterReferences(path, content, prio)...)
	if ctxDerivedKinds[kind] && len(parseFrontmatterList(content, "sources")) == 0 {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "document derives from/extend another; missing `sources` frontmatter"})
	}
	issues = append(issues, lintRFCAndPromptContract(path, content, kind, prio)...)
	// Decision directory: filename must match NNNN-slug.md and number must match.
	if dir == sdtDecisionsDir {
		base := filepath.Base(path)
		m := regexp.MustCompile(`^(\d{4})-`).FindStringSubmatch(base)
		if m == nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: "decision filename must start with a 4-digit number (NNNN-<slug>.md)"})
		} else if n := parseFrontmatterField(content, "number"); n != "" && n != m[1] {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: fmt.Sprintf("frontmatter number %s does not match filename %s", n, m[1])})
		}
	}
	// Oversized task phases: a checklist beyond the soft bound gets a
	// SUGGESTION to split the phase (never a failure).
	if kind == ctxTypeTasks {
		if n := len(parseTaskItems(content)); n > ctxTasksOversizedItems {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: fmt.Sprintf("task file has %d checklist items (>%d); consider splitting the phase", n, ctxTasksOversizedItems)})
		}
	}
	return issues
}

var contextLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate context frontmatter and links",
	Long: `Validate the context/ documents: frontmatter well-formed (kind, mandatory
summary), [[links]] resolve to existing files, and decision filenames/numbers are
consistent. Exits non-zero when CRITICAL issues are found.

Examples:
  sdt context lint
  sdt context lint --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var issues []ctxLintIssue
		for _, dir := range ctxIndexDirs {
			files, err := dirFiles(dir)
			exitWithError(cmd, err)
			for _, f := range files {
				issues = append(issues, lintDoc(f)...)
			}
		}
		// index.md itself is validated as a document too.
		if _, err := os.Stat(sdtContextIndex); err == nil {
			issues = append(issues, lintDoc(sdtContextIndex)...)
		}
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].Priority != issues[j].Priority {
				prio := map[string]int{ctxLintCritical: 0, ctxLintWarning: 1, "SUGGESTION": 2}
				return prio[issues[i].Priority] < prio[issues[j].Priority]
			}
			return issues[i].Path < issues[j].Path
		})
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(issues, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(issues)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, it := range issues {
				outputString(cmd, it.String()+"\n")
			}
		}
		critical := 0
		for _, it := range issues {
			if it.Priority == ctxLintCritical {
				critical++
			}
		}
		if critical > 0 {
			exitWithError(cmd, fmt.Errorf("%d CRITICAL lint issue(s)", critical))
		}
	},
}

// ── context status ──────────────────────────────────────────────────────────────
