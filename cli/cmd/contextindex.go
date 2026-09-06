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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ── context knowledge index (reindex/lint) ─────────────────────────────────────

// ctxIndexTypeTier maps a context directory to its relevance tier. Essential
// tiers (architecture, decisions) sort first; history tiers last.
// sdtMarkdownExt avoids a repeated ".md" literal across context commands.
const sdtMarkdownExt = ".md"

// ctxTierHistory is the lowest relevance tier label.
const ctxTierHistory = "history"

// map keys reused by reindex/lint/template output.
const (
	ctxMapPath   = "path"
	ctxMapStatus = "status"
)

var ctxTierOrder = []string{"essential", "important", "medium", "operational", ctxTierHistory}

func ctxTierForDir(dir string) string {
	switch dir {
	case sdtArchitectureDir, sdtDecisionsDir:
		return "essential"
	case sdtAnalysisDir:
		return "important"
	case sdtPlanDir, sdtNotesDir, sdtQuestionsDir:
		return "medium"
	case sdtTasksDir:
		return "operational"
	case sdtWorklogDir, sdtArchiveDir:
		return ctxTierHistory
	default:
		return ctxTierHistory
	}
}

// ctxIndexDirs lists the knowledge directories scanned by reindex, in a stable
// order per tier.
var ctxIndexDirs = []string{
	sdtArchitectureDir,
	sdtDecisionsDir,
	sdtAnalysisDir,
	sdtPlanDir,
	sdtNotesDir,
	sdtQuestionsDir,
	sdtTasksDir,
	sdtWorklogDir,
	sdtArchiveDir,
}

// dirFiles returns the .md files under dir sorted by name, or nil when the
// directory does not exist.
func dirFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != sdtMarkdownExt {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// parseFrontmatterField returns the value of a top-level YAML frontmatter key,
// or "" when absent or malformed.
func parseFrontmatterField(content, key string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == ctxFrontmatterDelim {
			return ""
		}
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

// parseFrontmatterList parses a YAML block-list frontmatter field (a line
// `key:` followed by `  - item` lines), returning the list items. Falls back to
// a single inline value when present. Returns nil when the key is absent.
func parseFrontmatterList(content, key string) []string {
	lines := strings.Split(content, "\n")
	var out []string
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return nil
	}
	in := false
	for _, line := range lines[1:] {
		trim := strings.TrimSpace(line)
		if trim == ctxFrontmatterDelim {
			break
		}
		if !in {
			if strings.HasPrefix(line, key+":") {
				val := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
				if val != "" {
					out = append(out, strings.TrimSpace(strings.Trim(val, `"'`)))
				}
				in = true
			}
			continue
		}
		if strings.HasPrefix(line, "  -") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "  -")))
		} else if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// a new top-level key ends the list
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ctxResolvePath resolves a frontmatter reference ([[path]] or plain path)
// relative to the document's directory, returning the absolute path if it
// exists.
func ctxResolvePath(base, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, sdtMarkdownExt)
	if ref == "" {
		return "", false
	}
	abs := filepath.Join(base, ref)
	if !strings.HasSuffix(abs, sdtMarkdownExt) {
		abs += sdtMarkdownExt
	}
	if _, err := os.Stat(abs); err != nil { //#nosec G703 -- validated against sdt.context/ tree
		return "", false
	}
	return abs, true
}

// ctxIndexLine renders one index row: relative path + summary.
func ctxIndexLine(dir, path string) string {
	rel, err := filepath.Rel(sdtWorkDir, path)
	if err != nil {
		rel = path
	}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return fmt.Sprintf("- [[%s]] — <unreadable>", rel)
	}
	summary := parseFrontmatterField(string(data), "summary")
	if summary == "" {
		summary = "<no summary>"
	}
	return fmt.Sprintf("- [[%s]] — %s", rel, summary)
}

// buildIndex renders the full sdt.context/index.md content grouped by tier.
func buildIndex() (string, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: index\n")
	b.WriteString("summary: Generated knowledge index of sdt.context documents grouped by relevance tier\n")
	b.WriteString("_generated: auto\n")
	b.WriteString("---\n\n")
	b.WriteString("# sdt.context — Knowledge Index\n\n")
	b.WriteString("_Managed by `sdt context reindex`. Each row lists the file and its frontmatter `summary`._\n\n")
	for _, tier := range ctxTierOrder {
		var rows []string
		for _, dir := range ctxIndexDirs {
			if ctxTierForDir(dir) != tier {
				continue
			}
			files, err := dirFiles(dir)
			if err != nil {
				return "", err
			}
			for _, f := range files {
				rows = append(rows, ctxIndexLine(dir, f))
			}
		}
		if len(rows) == 0 {
			continue
		}
		b.WriteString("## " + cases.Title(language.English).String(tier) + "\n\n")
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func writeIndex(content string) error {
	if err := os.MkdirAll(sdtWorkDir, 0o750); err != nil { //#nosec G301 -- user work dir
		return err
	}
	//#nosec G306 -- user work file
	return os.WriteFile(sdtContextIndex, []byte(content), 0o644)
}

var contextReindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Regenerate sdt.context/index.md from frontmatter summaries",
	Long: `Scan the sdt.context/ knowledge directories, read the mandatory frontmatter
summary of every document and regenerate sdt.context/index.md grouped by
relevance tier (essential, important, medium, operational, history).

Examples:
  sdt context reindex
  sdt context reindex --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := buildIndex()
		exitWithError(cmd, err)
		written, err := writeIndexIfChanged(content)
		exitWithError(cmd, err)
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(map[string]string{ctxMapPath: sdtContextIndex, ctxMapStatus: written}, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(map[string]string{ctxMapPath: sdtContextIndex, ctxMapStatus: written})
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, sdtContextIndex+" ("+written+")\n")
		}
	},
}

func writeIndexIfChanged(content string) (string, error) {
	if data, err := os.ReadFile(sdtContextIndex); err == nil && string(data) == content {
		return "skipped", nil
	}
	if err := writeIndex(content); err != nil {
		return "", err
	}
	return "written", nil
}

// ctxLintIssue is one validation finding from lint.
type ctxLintIssue struct {
	Path     string `json:"path" yaml:"path"`
	Priority string `json:"priority" yaml:"priority"`
	Message  string `json:"message" yaml:"message"`
}

// lint issue priorities.
const (
	ctxLintCritical = "CRITICAL"
	ctxLintWarning  = "WARNING"
)

func (i ctxLintIssue) String() string {
	return fmt.Sprintf("[%s] %s: %s", i.Priority, i.Path, i.Message)
}

// ctxLinkRegexp matches a `[[path]]` wiki-style link or a plain relative path.
var ctxLinkRegexp = regexp.MustCompile(`\[\[([a-zA-Z0-9_./-]+)\]\]`)

// ctxDerivedKinds are document kinds that derive from or extend another
// document and therefore must carry a `sources` frontmatter reference. Plan and
// tasks always derive (from analysis/plan by the 5-phase lifecycle); ADR
// decisions and open-question collections state their origin. A greenfield
// analysis does not derive from anything, so analysis is not required to set
// sources (a follow-up analysis should set it but is not hard-flagged).
var ctxDerivedKinds = map[string]bool{
	ctxTypePlan:      true,
	ctxTypeTasks:     true,
	ctxTypeAdr:       true,
	ctxTypeQuestions: true,
}

// lintDoc validates one sdt.context document. priorityFn lowers CRITICAL to
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
	// resolve [[links]] and links: array to existing documents.
	// Files under context dirs link relative to their own directory; the
	// generated index.md links relative to the sdt.context/ root.
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
		if _, err := os.Stat(abs); os.IsNotExist(err) { //#nosec G703 -- validated against sdt.context/ tree
			issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "broken link [[" + target + "]]"})
		}
	}
	// sources: backward-provenance references (root-relative to sdt.context/,
	// like the index). Resolve each entry; require the field on documents known
	// to derive from/extend another document.
	for _, ref := range parseFrontmatterList(content, "sources") {
		if _, ok := ctxResolvePath(sdtWorkDir, ref); !ok {
			issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "broken source reference: " + ref})
		}
	}
	if ctxDerivedKinds[kind] && len(parseFrontmatterList(content, "sources")) == 0 {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "document derives from/extend another; missing `sources` frontmatter"})
	}
	// ADR directory: filename must match NNNN-slug.md and number must match.
	if dir == sdtDecisionsDir {
		base := filepath.Base(path)
		m := regexp.MustCompile(`^(\d{4})-`).FindStringSubmatch(base)
		if m == nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: "ADR filename must start with a 4-digit number (NNNN-<slug>.md)"})
		} else if n := parseFrontmatterField(content, "number"); n != "" && n != m[1] {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: fmt.Sprintf("frontmatter number %s does not match filename %s", n, m[1])})
		}
	}
	return issues
}

var contextLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate sdt.context frontmatter and links",
	Long: `Validate the sdt.context/ documents: frontmatter well-formed (kind, mandatory
summary), [[links]] resolve to existing files, and ADR filenames/numbers are
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

type ctxStatusEntry struct {
	Type    string `json:"type" yaml:"type"`
	Count   int    `json:"count" yaml:"count"`
	Next    string `json:"next" yaml:"next"`
	IfClean string `json:"clean" yaml:"clean"`
}

var contextStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Summarize sdt.context/ documents per type with next step",
	Long: `Summarize the sdt.context/ knowledge: per-type document count and the
recommended next step (read / write / verify). Useful at session start after
reindex.

Examples:
  sdt context status
  sdt context status --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rows := ctxStatusRows()
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(rows, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(rows)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, r := range rows {
				outputString(cmd, fmt.Sprintf("%-12s %3d  %s\n", r.Type+":", r.Count, r.Next))
			}
		}
	},
}

func ctxStatusRows() []ctxStatusEntry {
	type dirInfo struct {
		typ     string
		dir     string
		next    string
		ifClean string
	}
	const (
		read = "read"
	)
	infos := []dirInfo{
		{typ: "architecture", dir: sdtArchitectureDir, next: read, ifClean: read},
		{typ: "decisions", dir: sdtDecisionsDir, next: read, ifClean: read},
		{typ: "analysis", dir: sdtAnalysisDir, next: "read if current", ifClean: "done"},
		{typ: "plan", dir: sdtPlanDir, next: "active plan", ifClean: gitIgnoreModeNone},
		{typ: "notes", dir: sdtNotesDir, next: "review", ifClean: gitIgnoreModeNone},
		{typ: "questions", dir: sdtQuestionsDir, next: "answer open questions", ifClean: gitIgnoreModeNone},
		{typ: "tasks", dir: sdtTasksDir, next: "track per-phase", ifClean: gitIgnoreModeNone},
		{typ: "worklog", dir: sdtWorklogDir, next: ctxTierHistory, ifClean: ctxTierHistory},
		{typ: "archive", dir: sdtArchiveDir, next: ctxTierHistory, ifClean: ctxTierHistory},
	}
	var rows []ctxStatusEntry
	for _, info := range infos {
		files, err := dirFiles(info.dir)
		if err != nil {
			continue
		}
		next := info.next
		if len(files) == 0 {
			next = info.ifClean
		}
		rows = append(rows, ctxStatusEntry{Type: info.typ, Count: len(files), Next: next, IfClean: info.ifClean})
	}
	return rows
}

// ── context template ────────────────────────────────────────────────────────────

var contextTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Print the per-type instruction file for a context type",
	Long: `Print the content of sdt.context/instructions/<tipo>.md for one document
type (analysis, plan, tasks, adr, architecture, worklog, notes). Read-only: the
CLI never writes documents.

Examples:
  sdt context template --type adr
  sdt context template --type plan --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		path, err := contextInstrPath(typ)
		exitWithError(cmd, err)
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if os.IsNotExist(err) {
			exitWithError(cmd, fmt.Errorf("no instruction file at %s (types: analysis|plan|tasks|adr|architecture|worklog|notes|questions)", path))
		}
		exitWithError(cmd, err)
		switch getFormat(cmd) {
		case fmtJSON, fmtYAML:
			m := map[string]string{"type": typ, ctxMapPath: path, "content": string(data)}
			var out []byte
			var merr error
			if getFormat(cmd) == fmtJSON {
				out, merr = json.MarshalIndent(m, "", "  ")
			} else {
				out, merr = yaml.Marshal(m)
			}
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			outputString(cmd, string(data))
		}
	},
}

func contextInstrPath(typ string) (string, error) {
	var name string
	switch typ {
	case ctxTypeAnalysis:
		name = "analysis.md"
	case ctxTypePlan:
		name = "plan.md"
	case ctxTypeTasks:
		name = "tasks.md"
	case ctxTypeAdr, "decision":
		name = "adr.md"
	case "architecture":
		name = "architecture.md"
	case ctxTypeWorklog:
		name = "worklog.md"
	case ctxTypeNotes:
		name = "notes.md"
	case ctxTypeQuestions:
		name = "questions.md"
	default:
		return "", fmt.Errorf("unknown type %q (use analysis|plan|tasks|adr|architecture|worklog|notes|questions)", typ)
	}
	return filepath.Join(sdtInstrDir, name), nil
}
