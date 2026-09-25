package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxFrontmatterSources is the provenance field name shared by the reference
// validation loop and the derived-document contract.
const ctxFrontmatterSources = "sources"

// ctxFrontmatterLinks is the generic-correlation reference field.
const ctxFrontmatterLinks = "links"

// ctxFrontmatterResults is the output-artifact reference field.
const ctxFrontmatterResults = "results"

// ctxReferenceFields are the frontmatter list fields that carry document
// references validated against the context tree.
var ctxReferenceFields = []string{ctxFrontmatterSources, ctxFrontmatterLinks, "derived_from", ctxFrontmatterResults, ctxTypeSupersedes, "contradicts"}

func lintFrontmatterReferences(path, content string, prio func(string) string) []ctxLintIssue {
	var issues []ctxLintIssue
	inCommands := filepath.Dir(path) == sdtCommandsDir
	for _, field := range ctxReferenceFields {
		for _, ref := range parseFrontmatterList(content, field) {
			// `links: none` is the explicit "no relation" opt-out, not a path.
			if field == ctxFrontmatterLinks && strings.TrimSpace(ref) == "none" {
				continue
			}
			if _, ok := ctxResolvePath(sdtWorkDir, ref); ok {
				continue
			}
			// Unresolvable instructions refs in command files are reported by
			// lintCommandFile with a precise remediation hint; skip the generic
			// message here to avoid a duplicate finding.
			if inCommands && strings.HasPrefix(strings.TrimSpace(ref), "instructions/") {
				continue
			}
			message := "broken " + field + " reference: " + ref
			if field == ctxFrontmatterSources {
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
	// Hint is the remediation next-action, derived from the message class (see
	// ctxLintHints). It is empty for classes without a curated hint.
	Hint string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

// ctxLintHints are remediation next-actions keyed by message prefix. Longest,
// most specific prefixes must come first so a command instruction reference is
// matched before the generic broken-reference entry.
var ctxLintHints = []struct{ prefix, hint string }{
	{"unresolved instruction reference", "fix the target path in the command frontmatter so it resolves to an existing context/instructions/*.md file"},
	{"command body lacks a `## When not to use`", "add a `## When not to use` section stating when the trigger does NOT apply"},
	{"command `summary` is", "shorten the command `summary` to at most 1024 chars; it doubles as the trigger tooltip"},
	{"command file `id` must be", "set `id` to `commands/<filename>` (the trigger identity) so links and lookup stay deterministic"},
	{"command file `kind` must be", "set `kind: commands` so the trigger filenames and the frontmatter type agree"},
	{"broken link [[", "fix the [[target]] so it resolves to an existing document (links resolve relative to the document dir; index.md resolves from context/)"},
	{"broken source reference", "fix the `sources` frontmatter reference so it resolves to an existing context document"},
	{"broken derived_from reference", "fix the `derived_from` frontmatter reference so it resolves to the prompt/analysis that produced this document"},
	{"broken links reference", "fix the `links` frontmatter reference so it resolves to an existing context document"},
	{"broken supersedes reference", "fix the `supersedes` frontmatter reference so it resolves to an existing context document"},
	{"broken contradicts reference", "fix the `contradicts` frontmatter reference so it resolves to an existing context document"},
	{"analysis declares no relation", "add `links`, `supersedes` or `contradicts`, or state the reason with `links: none`; a new analysis should always declare how it relates to prior work"},
	{"broken results reference", "fix the `results` frontmatter reference so it resolves to an existing context document"},
	{"missing `sources`", "add a `sources` frontmatter reference to the document this one derives from or extends (bidirectional traceability)"},
	{"frontmatter missing `kind`", "add `kind: <type>`; use `sdt context new --type <type>` to scaffold a compliant document"},
	{"frontmatter missing mandatory `", "add the missing mandatory frontmatter key; use `sdt context new` to scaffold a compliant document"},
	{"missing YAML frontmatter", "start the file with a YAML `---` frontmatter block carrying at least `kind` and `summary`; use `sdt context new --type <type>`"},
	{"decision filename", "rename the file to NNNN-<slug>.md so the 4-digit number stays consistent"},
	{"frontmatter number", "align `number` in the frontmatter with the 4-digit filename prefix"},
	{"outside vocabulary for kind", "set `status` to a value from the kind's vocabulary (see the status matrix: context/architecture/stack.md)"},
	{"missing frontmatter `status` for kind", "add `status: <vocab value>`; see the per-type vocabularies in the status matrix (context/architecture/stack.md)"},
	{"status-only `archived` set in place", "keep the status-only transition, or move the file with `sdt context archive` (either is allowed)"},
	{"does not parse as RFC3339 UTC", "format `created`/`updated` as RFC3339 UTC (e.g. `2026-09-25T05:00:00Z`; see `sdt time iso`)"},
	{"consider splitting the phase", "split the phase into smaller single-deliverable task files (one concern per phase)"},
	{"completed task file has no `## Review`", "record the verify-step verdicts with `sdt context task review --phase <n>` (CONFIRMED | DISPROVED | UNVERIFIED per finding)"},
	{"prompt must declare", "add a `derived_from` frontmatter reference to the prompt that produced this document"},
	{"analysis missing `objective`", "add `objective: <kebab-case-slug>`; reuse the same slug in every analysis of the same initiative so they group in the index"},
	{"analysis `objective`", "set `objective` to a lowercase kebab-case slug (letters, digits and '-'), shared across analyses of the same initiative"},
	{"notes entry missing `agent`", "add `agent: <tool/role>` to the notes frontmatter so the entry's provenance is recorded (`sdt context list --agent`)"},
	{"unknown topic", "use a canonical topic from context/topics.yaml (aliases are accepted too), or add the topic to the register"},
	{"topic ", "use a kebab-case topic slug (lowercase letters, digits and '-')"},
	{"entity ", "use a kebab-case entity slug (lowercase letters, digits and '-')"},
	{"unknown role", "use a role slug from the closed register (`sdt agent roles show`); unknown `role:` values on worklog/notes entries lose the vocabulary contract"},
	{"role profile", "run `sdt agent roles check`; fix register/profile mismatches (`sdt agent roles init`/`--force`)"},
	{"security: possible", "review the flagged content, redact or remove it, and re-ingest from a trusted source before it can influence the agent"},
	{"security: invisible", "strip the invisible/zero-width Unicode characters from the document; they can hide instructions from human review"},
}

// ctxLintHint returns the curated remediation for an issue message, or "" when
// the message class has no curated hint.
func ctxLintHint(message string) string {
	for _, h := range ctxLintHints {
		if strings.HasPrefix(message, h.prefix) {
			return h.hint
		}
	}
	return ""
}

// decorateLintHints attaches the curated Hint to every issue in place.
func decorateLintHints(issues []ctxLintIssue) []ctxLintIssue {
	for i := range issues {
		issues[i].Hint = ctxLintHint(issues[i].Message)
	}
	return issues
}

// lint issue priorities.
const (
	ctxLintCritical = "CRITICAL"
	ctxLintWarning  = "WARNING"
	// ctxTasksOversizedItems is the soft checklist-size guard for task files:
	// a phase whose checklist exceeds it triggers a SUGGESTION to split.
	ctxTasksOversizedItems = 10
	// ctxCommandSummaryMax caps the command-trigger description: the summary
	// doubles as the trigger tooltip in CLI helpers and the viewer index.
	ctxCommandSummaryMax = 1024
)

func (i ctxLintIssue) String() string {
	if i.Hint == "" {
		return fmt.Sprintf("[%s] %s: %s", i.Priority, i.Path, i.Message)
	}
	return fmt.Sprintf("[%s] %s: %s — hint: %s", i.Priority, i.Path, i.Message, i.Hint)
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

// lintStatusField flags a status-bearing document whose frontmatter status is
// missing or outside the kind's closed vocabulary (from the shared registry).
// Non-status-bearing kinds (worklog, notes, tmp) and kinds outside the registry
// (reference) are skipped. WARNING so historical documents never hard-fail.

func lintStatusField(path, content, kind string) []ctxLintIssue {
	t, ok := ctxTypeLookup(kind)
	if !ok || !ctxStatusBearing(t) {
		return nil
	}
	vocab := strings.Join(t.statuses, " | ")
	status := parseFrontmatterField(content, "status")
	if status == "" {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("missing frontmatter `status` for kind %s (%s)", kind, vocab)}}
	}
	if !ctxStatusInVocab(t, status) {
		return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("frontmatter `status` %q outside vocabulary for kind %s (%s)", status, kind, vocab)}}
	}
	return nil
}

// lintArchivedInPlace hints when a status-bearing document carries a
// status-only `archived` transition while living outside context/archive/.
// Both are legal (decision D2): set in place, or moved with `sdt context
// archive`. SUGGESTION so no existing document hard-fails.

func lintArchivedInPlace(path, content, kind string) []ctxLintIssue {
	t, ok := ctxTypeLookup(kind)
	if !ok || !ctxStatusBearing(t) {
		return nil
	}
	if parseFrontmatterField(content, "status") != statusArchived {
		return nil
	}
	if filepath.Dir(path) == sdtArchiveDir {
		return nil
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "status-only `archived` set in place; `sdt context archive` moves the file under archive/ (either is allowed)"}}
}

// lintTimestampFields flags a present `created`/`updated` that does not parse
// as RFC3339 UTC (decision D4). Missing fields are not flagged; offsets that
// still parse as RFC3339 are tolerated (no historical hard-fail).

func lintTimestampFields(path, content string, prio func(string) string) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, key := range []string{statusCreated, statusUpdated} {
		v := parseFrontmatterField(content, key)
		if v == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, v); err != nil {
			issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("frontmatter `%s` %q does not parse as RFC3339 UTC", key, v)})
		}
	}
	return issues
}

// lintCommandFile validates the command-trigger contract for files under
// context/commands/: kind must be `commands`, `id` must equal commands/<name>,
// `summary` is capped (it doubles as the trigger tooltip), references to
// instructions/<t>.md must resolve, and the body should state when the trigger
// does NOT apply. Commands are standardized prompts: they may reference 0..n
// instruction files, so no reverse (contract→command) check exists and a
// self-contained command is valid.
func lintCommandFile(path, content string, prio func(string) string) []ctxLintIssue {
	var issues []ctxLintIssue
	if kind := parseFrontmatterField(content, "kind"); kind != "" && kind != ctxTypeCommands {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "command file `kind` must be `" + ctxTypeCommands + "`, got " + kind})
	}
	base := strings.TrimSuffix(filepath.Base(path), sdtMarkdownExt)
	if got := parseFrontmatterField(content, "id"); got != ctxTypeCommands+"/"+base {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "command file `id` must be `" + ctxTypeCommands + "/" + base + "`"})
	}
	if s := parseFrontmatterField(content, "summary"); len([]rune(s)) > ctxCommandSummaryMax {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("command `summary` is %d chars (max %d)", len([]rune(s)), ctxCommandSummaryMax)})
	}
	for _, field := range []string{"sources", "links", "derived_from", "results"} {
		for _, ref := range parseFrontmatterList(content, field) {
			if !strings.HasPrefix(strings.TrimSpace(ref), "instructions/") {
				continue
			}
			if _, ok := ctxResolvePath(sdtWorkDir, ref); !ok {
				issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "unresolved instruction reference: " + ref + " (fix the target path in the command frontmatter)"})
			}
		}
	}
	if !strings.Contains(content, "## When not to use") {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: "command body lacks a `## When not to use` section (state when the trigger does NOT apply)"})
	}
	return issues
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
	// Generic per-type status vocabulary check for every status-bearing kind
	// (from the shared registry); documented in the stack.md status matrix.
	issues = append(issues, lintStatusField(path, content, kind)...)
	issues = append(issues, lintArchivedInPlace(path, content, kind)...)
	issues = append(issues, lintTimestampFields(path, content, prio)...)
	// Optional `objective` grouping key: WARNING on a non-kebab-case value,
	// SUGGESTION on absence so the convention is adopted gradually without
	// breaking existing analyses.
	issues = append(issues, lintObjectiveField(path, content, kind, prio)...)
	// Analyses must declare how they relate to prior work: a `links`,
	// `supersedes` or `contradicts` reference, or an explicit `links: none`.
	issues = append(issues, lintAnalysisRelations(path, content, kind, prio)...)
	// Optional controlled vocabulary: `topics`/`entities` are validated against
	// context/topics.yaml (alias canonicalization, advisory for unknown).
	issues = append(issues, lintTopicFields(path, content, ctxTopicReg)...)
	// Notes provenance: record who produced the entry. Advisory (SUGGESTION) so
	// existing notes are never hard-flagged and no backfill is forced.
	if kind == ctxTypeNotes && parseFrontmatterField(content, "agent") == "" {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: "notes entry missing `agent` provenance (record who produced it)"})
	}
	// Role provenance vocabulary: `role:` is validated against the closed
	// register. An unknown slug is advisory (SUGGESTION) so historical entries
	// never hard-fail the check.
	issues = append(issues, lintRoleFrontmatter(path, content)...)
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
	if ctxDerivedKinds[kind] && len(parseFrontmatterList(content, ctxFrontmatterSources)) == 0 {
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
	// Command dir: trigger contract (kind/id consistency, summary cap,
	// resolvable instruction refs, when-not-to-use advisory).
	if dir == sdtCommandsDir {
		issues = append(issues, lintCommandFile(path, content, prio)...)
	}
	// Oversized task phases: a checklist beyond the soft bound gets a
	// SUGGESTION to split the phase (never a failure).
	if kind == ctxTypeTasks {
		if n := len(parseTaskItems(content)); n > ctxTasksOversizedItems {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: fmt.Sprintf("task file has %d checklist items (>%d); consider splitting the phase", n, ctxTasksOversizedItems)})
		}
		if parseFrontmatterField(content, "status") == taskFileStatusCompleted && !hasReviewBlock(content) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: "completed task file has no `## Review` verify-step block (record verdicts; see `sdt context task review`)"})
		}
	}
	return issues
}

// lintRoleFrontmatter validates a document's `role:` provenance field against
// the closed role register. Unknown slugs are advisory (SUGGESTION) so no
// historical entry hard-fails the check.
func lintRoleFrontmatter(path, content string) []ctxLintIssue {
	role := parseFrontmatterField(content, "role")
	if role == "" {
		return nil
	}
	if _, ok := roleLookup(role); ok {
		return nil
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "unknown role " + role + " in `role:` frontmatter (closed register: " + strings.Join(roleSlugs(), ", ") + ")"}}
}

// lintObjectiveField validates the optional `objective` grouping key carried by
// analysis documents: WARNING on a non-kebab-case value, SUGGESTION when the
// key is absent so the grouping convention is adopted gradually.
func lintObjectiveField(path, content, kind string, prio func(string) string) []ctxLintIssue {
	if kind != ctxTypeAnalysis {
		return nil
	}
	if o := parseFrontmatterField(content, "objective"); o == "" {
		return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "analysis missing `objective` group key (kebab-case slug)"}}
	} else if !ctxObjectiveRegexp.MatchString(o) {
		return []ctxLintIssue{{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("analysis `objective` %q must be a kebab-case slug (lowercase letters, digits and '-')", o)}}
	}
	return nil
}

// lintAnalysisRelations requires an analysis to declare how it relates to prior
// work: at least one of links/supersedes/contradicts, or an explicit
// `links: none` opt-out. Advisory (SUGGESTION) so the convention is adopted
// gradually.
func lintAnalysisRelations(path, content, kind string, prio func(string) string) []ctxLintIssue {
	if kind != ctxTypeAnalysis {
		return nil
	}
	if parseFrontmatterField(content, "links") == "none" {
		return nil
	}
	for _, field := range []string{ctxFrontmatterLinks, ctxTypeSupersedes, "contradicts"} {
		if len(parseFrontmatterList(content, field)) > 0 {
			return nil
		}
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "analysis declares no relation to prior work (add `" + ctxFrontmatterLinks + "`, `supersedes`/`contradicts`, or `" + ctxFrontmatterLinks + ": none` with a reason)"}}
}

var contextLintCmd = &cobra.Command{
	Use:   gateStepLint,
	Short: "Validate context frontmatter and links",
	Long: `Validate the context/ documents: frontmatter well-formed (kind, mandatory
summary), [[links]] resolve to existing files, and decision filenames/numbers are
consistent. Exits non-zero when CRITICAL issues are found.

With --security, additionally scan every document for prompt-injection phrases,
credential/secret literals, invisible/zero-width Unicode and exfil patterns
(advisory WARNING; the default scan is unchanged).

Examples:
  sdt context lint
  sdt context lint --security
  sdt context lint --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var issues []ctxLintIssue
		security := getBoolFlag(cmd, "security", false)
		// Load the controlled topic register once; a malformed file is reported
		// as a lint issue on the register itself.
		reg, regErr := loadTopicRegister()
		if regErr != nil {
			issues = append(issues, ctxLintIssue{Path: ctxTopicsFilePath, Priority: ctxLintWarning, Message: regErr.Error()})
			reg = &ctxTopicRegister{Topics: map[string][]string{}, Aliases: map[string]string{}}
		}
		ctxTopicReg = reg
		for _, dir := range ctxIndexDirs {
			files, err := dirFiles(dir)
			exitWithError(cmd, err)
			for _, f := range files {
				issues = append(issues, lintDoc(f)...)
				if security {
					issues = append(issues, lintSecurity(f)...)
				}
			}
		}
		// index.md itself is validated as a document too.
		if _, err := os.Stat(sdtContextIndex); err == nil {
			issues = append(issues, lintDoc(sdtContextIndex)...)
			if security {
				issues = append(issues, lintSecurity(sdtContextIndex)...)
			}
		}
		// Notes-only dedup-before-write advisory (SUGGESTION, never a failure).
		if files, err := dirFiles(sdtNotesDir); err == nil {
			issues = append(issues, lintDuplicateNotes(files)...)
		}
		// Cross-analysis overlap advisory: same objective, similar title/summary.
		if files, err := dirFiles(sdtAnalysisDir); err == nil {
			issues = append(issues, lintOverlappingAnalyses(files)...)
		}
		// Role-profile advisory: mirror the deterministic role checks as
		// SUGGESTIONs (never failing), so profile health is visible in lint
		// while `agent roles check` remains the strict gate.
		issues = append(issues, lintRoleProfiles()...)
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].Priority != issues[j].Priority {
				prio := map[string]int{ctxLintCritical: 0, ctxLintWarning: 1, "SUGGESTION": 2}
				return prio[issues[i].Priority] < prio[issues[j].Priority]
			}
			return issues[i].Path < issues[j].Path
		})
		decorateLintHints(issues)
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
