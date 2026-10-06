package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/corpus"
	"github.com/spf13/cobra"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"

	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/mdstruct"
)

// ctxFrontmatterSources is the provenance field name shared by the reference
// validation loop and the derived-document contract.
const ctxFrontmatterSources = "sources"

// ctxFrontmatterLinks is the generic-correlation reference field.
const ctxFrontmatterLinks = "links"

// ctxFrontmatterResults is the output-artifact reference field.
const ctxFrontmatterResults = "results"

// ctxFrontmatterEvidence is the open-questions key holding a list of references
// backing (or deferring) a question.
const ctxFrontmatterEvidence = "evidence"

// ctxFrontmatterDeferredReason is the free-text reason required (advisory) when a
// question is `status: deferred`.
const ctxFrontmatterDeferredReason = "deferred_reason"

// ctxFrontmatterEvidenceClass is the optional scalar classification of an
// analysis/decision's dominant evidence type.
const ctxFrontmatterEvidenceClass = "evidence_class"

// ctxEvidenceClassValues is the closed vocabulary for `evidence_class`,
// confirmed by the R2 research step (plan 20261006-200432): the six evidence
// types. It deliberately mixes in no confidence/maturity axis, which is a
// separate future field.
var ctxEvidenceClassValues = []string{"fact", "observation", "inference", "hypothesis", "assumption", "decision"}

// ctxEvidenceClassValid reports whether v is in the closed vocabulary.
func ctxEvidenceClassValid(v string) bool {
	for _, c := range ctxEvidenceClassValues {
		if c == v {
			return true
		}
	}
	return false
}

// lintEvidenceClass validates the optional `evidence_class` scalar on analyses
// and decisions against the closed vocabulary. Absent is silent (the key is
// optional); out-of-vocabulary is WARNING; other kinds are ignored.
func lintEvidenceClass(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeAnalysis && kind != ctxTypeDecision {
		return nil
	}
	v := strings.TrimSpace(parseFrontmatterField(content, ctxFrontmatterEvidenceClass))
	if v == "" || ctxEvidenceClassValid(v) {
		return nil
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("`evidence_class` %q is outside the vocabulary (%s)", v, strings.Join(ctxEvidenceClassValues, " | "))}}
}

// ctxReferenceFields are the frontmatter list fields that carry document
// references validated against the context tree.
var ctxReferenceFields = []string{ctxFrontmatterSources, ctxFrontmatterLinks, "derived_from", ctxFrontmatterResults, ctxTypeSupersedes, ctxFrontmatterContradicts, ctxFrontmatterEvidence}

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

// ctxCitationRegexp matches a hash-anchored body citation in the corpus
// convention `refs/<path>@<hex>{6,}` with optional `:n` / `:n-m` line ranges.
// The optional `sha256:` prefix (the planned doc2md form) is captured so it can
// be tolerated without resolution: it carries no file path to hash. Group 1 is
// the `refs/...` path, group 2 the optional marker, group 3 the cited hex
// prefix.
var ctxCitationRegexp = regexp.MustCompile(`(refs/[A-Za-z0-9_./-]+)@(sha256:)?([0-9a-f]{6,})(?::[0-9]+(?:-[0-9]+)?)?`)

// ctxInlineCodeRegexp matches a single-backtick Markdown code span so a citation
// written as an example (`refs/x.md@fbf61784`) is not read as a claim.
var ctxInlineCodeRegexp = regexp.MustCompile("`[^`\n]*`")

// lintBodyCitations verifies the corpus's hash-anchored body citations: the
// target resolves under context/refs/ and the cited SHA-256 prefix still matches
// the file's current digest. Advisory (WARNING): it proves file presence and
// freshness only, never that the cited lines still support the claim.
func lintBodyCitations(path, content string) []ctxLintIssue {
	return lintCitationText(path, string(frontmatterBody([]byte(content))))
}

// lintCitationText is the body-only form of lintBodyCitations, shared with the
// wiki lint so `sdt context wiki lint` verifies per-page citations too.
func lintCitationText(path, body string) []ctxLintIssue {
	body = stripInlineCode(stripFencedCode(body))
	var issues []ctxLintIssue
	for _, m := range ctxCitationRegexp.FindAllStringSubmatch(body, -1) {
		ref, marker, cited := m[1], m[2], m[3]
		// The planned doc2md `@sha256:<12hex>` form is parsed but not resolved:
		// there is no path to hash yet, so it is tolerated, never flagged.
		if marker != "" {
			continue
		}
		actual, ok := ctxRefSHA8(filepath.Join(sdtWorkDir, ref))
		if !ok {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: "citation target not found: " + ref})
			continue
		}
		if !strings.HasPrefix(actual, cited) {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("stale citation %s@%s (actual %s)", ref, cited, actual)})
		}
	}
	return issues
}

// stripInlineCode blanks single-backtick code spans; a citation inside a code
// span is an example, not a claim.
func stripInlineCode(body string) string {
	return ctxInlineCodeRegexp.ReplaceAllString(body, "")
}

// ctxRefSHA8 reads a referenced file and returns the first 8 hex of its SHA-256
// digest, or ok=false when the file is absent or unreadable.
func ctxRefSHA8(path string) (string, bool) {
	data, err := os.ReadFile(path) //#nosec G304,G703 -- path built from a context/refs citation, never user input
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8], true
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
	{"broken evidence reference", "fix the `evidence` frontmatter reference so it resolves to an existing context document (analysis, decision, research or task)"},
	{"questions file is `status: deferred`", "add `deferred_reason: <why it is parked + its revival condition>` so the deferral is retrievable"},
	{"`evidence_class` ", "set `evidence_class` to one of fact | observation | inference | hypothesis | assumption | decision, or omit the key (optional)"},
	{"stale citation", "recompute the target's sha256 and update the citation prefix (first 8 hex), or re-verify the claim against the current source"},
	{"citation target not found", "fix the `refs/<path>` target so it resolves under context/refs/, or restore the cited file"},
	{"missing `sources`", "add a `sources` frontmatter reference to the document this one derives from or extends (bidirectional traceability)"},
	{"frontmatter missing `kind`", "add `kind: <type>`; use `sdt context new --type <type>` to scaffold a compliant document"},
	{"frontmatter missing mandatory `", "add the missing mandatory frontmatter key; use `sdt context new` to scaffold a compliant document"},
	{"missing YAML frontmatter", "start the file with a YAML `---` frontmatter block carrying at least `kind` and `summary`; use `sdt context new --type <type>`"},
	{"decision filename", "rename the file to NNNN-<slug>.md so the 4-digit number stays consistent"},
	{"frontmatter number", "align `number` in the frontmatter with the 4-digit filename prefix"},
	{"outside vocabulary for kind", "set `status` to a value from the kind's vocabulary (see the status matrix: context/architecture/stack.md)"},
	{"missing frontmatter `status` for kind", "add `status: <vocab value>`; see the per-type vocabularies in the status matrix (context/architecture/stack.md)"},
	{"does not parse as RFC3339 UTC", "format `created`/`updated` as RFC3339 UTC (e.g. `2026-09-25T05:00:00Z`; see `sdt time iso`)"},
	{"consider splitting the phase", "split the phase into smaller single-deliverable task files (one concern per phase)"},
	{"duplicate checklist id(s)", "run `sdt context checklist backfill` to renumber duplicate anchors so each id addresses one item"},
	{"completed task file has no `## Review`", "record the verify-step verdicts with `sdt context task review --phase <n>` (CONFIRMED | DISPROVED | UNVERIFIED per finding)"},
	{"task declares completed but its checklist", "tick the remaining items with `sdt context check`/`task done`, or reopen the file with `sdt context task wip`; `sdt context sync` will not regress a completed file"},
	{"task checklist is complete but the file status", "run `sdt context sync` (or `sdt context task done` on the last item) to derive `completed`"},
	{"plan is derivably completed", "run `sdt context sync` to derive the plan `completed`"},
	{"analysis is derivably completed", "run `sdt context sync` to derive the analysis `completed`"},
	{"analysis declares completed but", "finish the listed plan(s), or reopen the analysis if completion was declared by hand"},
	{"prompt must declare", "add a `derived_from` frontmatter reference to the prompt that produced this document"},
	{"analysis missing `objective`", "add `objective: <kebab-case-slug>`; reuse the same slug in every analysis of the same initiative so they group in the index"},
	{"analysis `objective`", "set `objective` to a lowercase kebab-case slug (letters, digits and '-'), shared across analyses of the same initiative"},
	{"plan missing `objective`", "add `objective: <kebab-case-slug>` matching the analysis this plan derives from so plans, tasks and analyses group in the index"},
	{"plan `objective`", "set `objective` to the lowercase kebab-case slug shared with the analysis this plan derives from"},
	{"task file carries legacy `objective`", "rename the field to `phase`; the task inherits the plan objective and never declares `objective`"},
	{"orphan task file", "run `sdt context relations backfill` to derive the parent plan from `sources`, or set `plan_id` explicitly; an orphan escapes every plan-scoped check"},
	{"plan status is ", "run `sdt context sync` (or complete the remaining phases) so the plan status matches its task files"},
	{"task file is completed but its parent plan is still active", "close the plan (`sdt context sync`, or set its status with `sdt context status set`) so the two documents agree"},
	{"stale task file", "receive the file and finish it, or reset its `[~]` items to `[ ]` and refresh `updated` with `sdt context touch`"},
	{"undeclared objective ", "declare it as `- [ ] <id> <outcome>` in the source analysis `## Objectives`, or correct the phase's `**Covers:**` claim"},
	{"uncovered objective ", "claim it from a derived plan phase with `**Covers:** <id>`, or drop it from the analysis when it is no longer intended"},
	{"completed task file has a `## Review` block but no `### Gate` record", "run the delivery gate with `sdt agent gate --record --plan <plan>` so the run's real outcome is readable next session"},
	{"finding under `### Findings` has no verdict token", "re-record it with `sdt context task review --finding \"<claim>\" --verdict " + ctxReviewVerdictHelp + "`"},
	{"unknown kind ", "record the deviation with `sdt context task deviation add` using one of fix | add | unblock | stop-and-ask"},
	{"completed task file carries an open deviation ", "resolve it (mark it done) or record why it stays open, so a completed file has no unresolved divergence"},
	{"document missing `uid`", "run `sdt context uid backfill` to stamp every existing document, or create new documents with `sdt context new`/`sdt context task`"},
	{"plan has no `analysis_id`", "run `sdt context relations backfill` to derive the typed parent relation from `sources`"},
	{"task has no `plan_id`", "run `sdt context relations backfill` to derive the typed parent relation from `sources`"},
	{"analysis_id ", "fix the typed parent relation: it must hold a parent document `uid` and be mirrored in the parent's `plans_ids`"},
	{"plan_id ", "fix the typed parent relation: it must hold a plan document `uid` and be mirrored in the plan's `tasks_ids`"},
	{"plans_ids ", "fix the typed parent relation: each entry must be a plan `uid` that names this analysis in `analysis_id`"},
	{"tasks_ids ", "fix the typed parent relation: each entry must be a task `uid` that names this plan in `plan_id`"},
	{"`uid` ", "set `uid` to a canonical lowercase UUIDv7 value (e.g. `sdt uid v7`)"},
	{"duplicate `uid`", "give the document a fresh identifier with `sdt uid v7` (a copied file duplicates the `uid`)"},
	{"notes entry missing `agent`", "add `agent: <tool/role>` to the notes frontmatter so the entry's provenance is recorded (`sdt context list --agent`)"},
	{"unknown topic", "use a canonical topic from context/topics.yaml (aliases are accepted too), or add the topic to the register"},
	{"topic ", "use a kebab-case topic slug (lowercase letters, digits and '-')"},
	{"unknown category", "use a canonical category from context/categories.yaml (aliases are accepted too), or add the category to the register"},
	{"category ", "use a kebab-case category slug (lowercase letters, digits and '-')"},
	{"entity ", "use a kebab-case entity slug (lowercase letters, digits and '-')"},
	{"unknown role", "use a role slug from the closed register (`sdt agent roles show`); unknown `role:` values on worklog/notes entries lose the vocabulary contract"},
	{"role profile", "run `sdt agent roles check`; fix register/profile mismatches (`sdt agent roles init`/`--force`)"},
	{"undeclared frontmatter key", "either keep it (any key is writable with `sdt context set`) or, if it is a recurring convention, propose it as a declared field in the type registry"},
	{"slide separator `---`", "put a blank line before the ruler (or use `***`, `___` or `- - -`) so the slides split instead of parsing as a setext heading and merging"},
	{"unknown directive", "fix the directive name (see the built-in set in `context/instructions/slides-marp.md`); the engine ignores an unknown directive silently"},
	{"relative asset path does not resolve", "fix the path so it resolves relative to the deck, or remove the image; a broken path ships a missing image"},
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
	// ctxAnalysisOversizedSections / ctxAnalysisOversizedOptions are the soft
	// split guards for analyses (rule R1 of instructions/analysis.md), expressed
	// in sections/options rather than lines: past either bound the analysis
	// probably carries more than one subject and should be a sibling set.
	ctxAnalysisOversizedSections = 12
	ctxAnalysisOversizedOptions  = 6
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
	if kind := parseFrontmatterField(content, ctxFrontmatterKind); kind != "" && kind != ctxTypeCommands {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "command file `kind` must be `" + ctxTypeCommands + "`, got " + kind})
	}
	base := strings.TrimSuffix(filepath.Base(path), sdtMarkdownExt)
	if got := parseFrontmatterField(content, "id"); got != ctxTypeCommands+"/"+base {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: "command file `id` must be `" + ctxTypeCommands + "/" + base + "`"})
	}
	if s := parseFrontmatterField(content, "summary"); len([]rune(s)) > ctxCommandSummaryMax {
		issues = append(issues, ctxLintIssue{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("command `summary` is %d chars (max %d)", len([]rune(s)), ctxCommandSummaryMax)})
	}
	for _, field := range []string{ctxFrontmatterSources, ctxFrontmatterLinks, "derived_from", ctxFrontmatterResults} {
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

// lintDoc validates the shared, per-document contract used by both corpus-wide
// and positional lint. Legacy documents without frontmatter keep the advisory
// behavior so old history does not fail the whole check.

func lintDoc(path string) []ctxLintIssue {
	var issues []ctxLintIssue
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return []ctxLintIssue{{Path: path, Priority: ctxLintCritical, Message: err.Error()}}
	}
	content := string(data)
	if issues := lintFrontmatterAndLegacyBody(path, data); issues != nil {
		return issues
	}
	kind := parseFrontmatterField(content, ctxFrontmatterKind)
	summary := parseFrontmatterField(content, "summary")

	// Legacy documents lack both kind and summary. Treat them as WARNING so the
	// historical corpus does not hard-fail the check; only new-style docs
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
	issues = append(issues, lintTimestampFields(path, content, prio)...)
	// Advisory: a top-level frontmatter key outside the declared set is allowed
	// (sdt context set is free with respect to keys) but made visible so the
	// schema stays discoverable without being closed.
	issues = append(issues, lintUndeclaredFrontmatterKeys(path, content)...)
	// Optional `objective` grouping key: WARNING on a non-kebab-case value,
	// SUGGESTION on absence so the convention is adopted gradually without
	// breaking existing analyses; a plan's objective must match its analysis.
	issues = append(issues, lintObjectiveField(path, content, kind, prio)...)
	issues = append(issues, lintPlanObjectiveConsistency(path, content, kind, prio)...)
	issues = append(issues, lintTaskObjectiveLegacy(path, content, kind)...)
	issues = append(issues, lintQuestionsDeferredReason(path, content, kind)...)
	issues = append(issues, lintEvidenceClass(path, content, kind)...)
	// Immutable identifier: presence (severity flips after the backfill) and
	// canonical UUIDv7 form. Duplicate detection is corpus-wide (below).
	issues = append(issues, lintUIDField(path, content, kind, prio)...)
	// Analyses must declare how they relate to prior work: a `links`,
	// `supersedes` or `contradicts` reference, or an explicit `links: none`.
	issues = append(issues, lintAnalysisRelations(path, content, kind, prio)...)
	// Rule R3: every weighed option records its outcome (accepted / rejected /
	// postponed). Rule R1: an analysis past the soft section/option bounds is a
	// split candidate, gated on asking the user. Both advisory SUGGESTIONs.
	issues = append(issues, lintAnalysisOptions(path, content, kind)...)
	issues = append(issues, lintAnalysisOversize(path, content, kind)...)
	// Optional controlled vocabulary: `topics`/`entities` are validated against
	// context/topics.yaml (alias canonicalization, advisory for unknown).
	issues = append(issues, lintTopicFields(path, content, ctxTopicReg)...)
	issues = append(issues, lintCategoryFields(path, content, ctxCategoryReg)...)
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
	// Body-level hash-anchored citations (`refs/<file>@<sha>:<lines>`): the
	// target must resolve under context/refs/ and its SHA-256 prefix must be
	// fresh. Advisory WARNING (file presence + hash freshness, not claim truth).
	issues = append(issues, lintBodyCitations(path, content)...)
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
	// Oversized task phases and the completed-without-review advisory: a
	// checklist beyond the soft bound (per `## Phase` section) gets a
	// SUGGESTION to split the phase (never a failure).
	if kind == ctxTypeTasks {
		issues = append(issues, lintTaskFileRules(path, content)...)
	}
	// An id must address exactly one item (decision 0014): a repeated
	// `<!-- c<N> -->` in one document is a WARNING.
	issues = append(issues, lintChecklistAnchors(path, content, kind)...)
	isMap := contextwiki.IsMapDoc(path)
	issues = append(issues, lintMarkdownBody(path, frontmatterBody(data), isMap, contextwiki.IsSlideDoc(path))...)
	if isMap {
		issues = append(issues, lintMapDoc(path, content, frontmatterBody(data))...)
	}
	if contextwiki.IsSlideDoc(path) {
		issues = append(issues, lintDeck(path, content, frontmatterBody(data))...)
	}
	return issues
}

// validateFrontmatter verifies that a document starts with a closed YAML
// frontmatter block whose root value is a mapping. Missing frontmatter is
// returned separately so lint can preserve its legacy warning; malformed
// blocks are errors and therefore CRITICAL.
func validateFrontmatter(data []byte) error {
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimSuffix(lines[0], "\r")) != ctxFrontmatterDelim {
		return fmt.Errorf("missing YAML frontmatter")
	}

	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(strings.TrimSuffix(lines[i], "\r")) == ctxFrontmatterDelim {
			closing = i
			break
		}
	}
	if closing < 0 {
		return fmt.Errorf("unterminated YAML frontmatter")
	}

	var value any
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:closing], "\n")), &value); err != nil {
		return fmt.Errorf("invalid YAML frontmatter: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("YAML frontmatter must be a mapping")
	}
	return nil
}

func lintFrontmatterSyntax(path string, data []byte) *ctxLintIssue {
	err := validateFrontmatter(data)
	if err == nil {
		return nil
	}
	priority := ctxLintCritical
	if err.Error() == "missing YAML frontmatter" {
		// Preserve the historical advisory for legacy documents that have no
		// frontmatter at all. A present but malformed block is a hard failure.
		priority = ctxLintWarning
	}
	return &ctxLintIssue{Path: path, Priority: priority, Message: err.Error()}
}

func lintFrontmatterAndLegacyBody(path string, data []byte) []ctxLintIssue {
	issue := lintFrontmatterSyntax(path, data)
	if issue == nil {
		return nil
	}
	issues := []ctxLintIssue{*issue}
	if issue.Priority == ctxLintWarning {
		// A legacy document without frontmatter still has a Markdown body.
		isMap := contextwiki.IsMapDoc(path)
		issues = append(issues, lintMarkdownBody(path, data, isMap, contextwiki.IsSlideDoc(path))...)
		if isMap {
			issues = append(issues, lintMapDoc(path, string(data), data)...)
		}
	}
	return issues
}

func frontmatterBody(data []byte) []byte {
	lines := bytes.SplitAfter(data, []byte{'\n'})
	seenOpening := false
	for i, line := range lines {
		line = bytes.TrimSuffix(line, []byte{'\n'})
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if !seenOpening {
			seenOpening = true
			continue
		}
		if strings.TrimSpace(string(line)) == ctxFrontmatterDelim {
			return bytes.Join(lines[i+1:], nil)
		}
	}
	return nil
}

func lintMarkdownBody(path string, body []byte, isMap, isDeck bool) []ctxLintIssue {
	root := parser.New().Parse(body)
	var issues []ctxLintIssue
	if err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		// A map legitimately has one `#` root; lintMapDoc owns the single-root
		// check, so the generic no-H1 rule is skipped for `.map.md` documents.
		// A deck's `#` is a slide title (one per slide), not a document H1, so
		// the rule is skipped for `.slide.md` too.
		if heading, ok := node.(*ast.Heading); ok && heading.Level == 1 && !isMap && !isDeck {
			line := markdownLineNumber(body, heading.Pos())
			issues = append(issues, ctxLintIssue{
				Path: path, Priority: ctxLintWarning,
				Message: fmt.Sprintf("markdown H1 heading at line %d; document bodies start at H2", line),
			})
		}
		if block, ok := node.(*ast.CodeBlock); ok && block.CodeBlockKind == ast.CodeBlockKindFenced && !codeBlockHasFenceCloser(body, block) {
			line := markdownLineNumber(body, block.Pos())
			issues = append(issues, ctxLintIssue{
				Path: path, Priority: ctxLintCritical,
				Message: fmt.Sprintf("unbalanced fenced code block opened at line %d", line),
			})
		}
		return ast.WalkContinue, nil
	}); err != nil {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintCritical, Message: "markdown AST traversal failed: " + err.Error()})
	}
	return issues
}

func markdownLineNumber(source []byte, pos int) int {
	if pos < 0 {
		return 1
	}
	if pos > len(source) {
		pos = len(source)
	}
	return bytes.Count(source[:pos], []byte{'\n'}) + 1
}

func codeBlockHasFenceCloser(source []byte, block *ast.CodeBlock) bool {
	pos := block.Pos()
	if pos < 0 || pos >= len(source) {
		return false
	}
	openLineEnd := bytes.IndexByte(source[pos:], '\n')
	if openLineEnd < 0 {
		openLineEnd = len(source)
	} else {
		openLineEnd += pos
	}
	marker, length, ok := openingFence(source[pos:openLineEnd])
	if !ok {
		return false
	}

	lineStart, ok := nextMarkdownLine(source, openLineEnd)
	if !ok {
		return false
	}
	segments := block.Value.Segments()
	if len(segments) > 0 {
		lineStart, ok = nextMarkdownLine(source, segments[len(segments)-1].Stop)
		if !ok {
			return false
		}
	}
	if lineStart >= len(source) {
		return false
	}
	lineEnd := bytes.IndexByte(source[lineStart:], '\n')
	if lineEnd < 0 {
		lineEnd = len(source)
	} else {
		lineEnd += lineStart
	}
	line := bytes.TrimSuffix(source[lineStart:lineEnd], []byte{'\r'})
	return closingFence(line, marker, length)
}

func openingFence(line []byte) (byte, int, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] != '`' && line[i] != '~' {
			continue
		}
		marker := line[i]
		j := i + 1
		for j < len(line) && line[j] == marker {
			j++
		}
		if j-i >= 3 {
			return marker, j - i, true
		}
		i = j - 1
	}
	return 0, 0, false
}

func closingFence(line []byte, marker byte, minimum int) bool {
	for {
		line = bytes.TrimLeft(line, " \t")
		if len(line) == 0 || line[0] != '>' {
			break
		}
		line = line[1:]
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			line = line[1:]
		}
	}
	line = bytes.TrimLeft(line, " \t")
	i := 0
	for i < len(line) && line[i] == marker {
		i++
	}
	if i < minimum {
		return false
	}
	return len(bytes.Trim(line[i:], " \t")) == 0
}

func nextMarkdownLine(source []byte, offset int) (int, bool) {
	if offset < 0 || offset >= len(source) {
		return 0, false
	}
	if offset > 0 && source[offset-1] == '\n' {
		return offset, true
	}
	n := bytes.IndexByte(source[offset:], '\n')
	if n < 0 {
		return 0, false
	}
	return offset + n + 1, true
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
// analysis and plan documents: WARNING on a non-kebab-case value, SUGGESTION
// when the key is absent so the grouping convention is adopted gradually.
func lintObjectiveField(path, content, kind string, prio func(string) string) []ctxLintIssue {
	if kind != ctxTypeAnalysis && kind != ctxTypePlan {
		return nil
	}
	if o := parseFrontmatterField(content, ctxFrontmatterObjective); o == "" {
		return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: kind + " missing `objective` group key (kebab-case slug)"}}
	} else if !ctxObjectiveRegexp.MatchString(o) {
		return []ctxLintIssue{{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("%s `objective` %q must be a kebab-case slug (lowercase letters, digits and '-')", kind, o)}}
	}
	return nil
}

// lintPlanObjectiveConsistency warns when a plan's `objective` differs from the
// objective of the analysis it sources (advisory so it never blocks).
func lintPlanObjectiveConsistency(path, content, kind string, prio func(string) string) []ctxLintIssue {
	if kind != ctxTypePlan {
		return nil
	}
	o := parseFrontmatterField(content, ctxFrontmatterObjective)
	if o == "" {
		return nil
	}
	for _, ref := range parseFrontmatterList(content, ctxFrontmatterSources) {
		abs, ok := ctxResolvePath(sdtWorkDir, ref)
		if !ok {
			continue // broken source references are reported elsewhere
		}
		data, err := os.ReadFile(abs) //#nosec G304 -- path resolved within context/
		if err != nil {
			continue
		}
		if parseFrontmatterField(string(data), ctxFrontmatterKind) != ctxTypeAnalysis {
			continue
		}
		if ao := parseFrontmatterField(string(data), ctxFrontmatterObjective); ao != "" && ao != o {
			return []ctxLintIssue{{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("plan `objective` %q differs from its analysis %s objective %q", o, ref, ao)}}
		}
		return nil
	}
	return nil
}

// lintUIDField validates the immutable `uid` identifier of a CLI-created kind:
// a missing value is a SUGGESTION while the one-shot backfill is pending and a
// WARNING after it (`ctxUIDBackfillMarker`); a present but non-canonical UUIDv7
// is a WARNING. Excluded kinds (commands, tmp, index, legacy) are skipped.
func lintUIDField(path, content, kind string, prio func(string) string) []ctxLintIssue {
	if !ctxUIDEligibleKind(kind) {
		return nil
	}
	uid := parseFrontmatterField(content, ctxFrontmatterUID)
	if uid == "" {
		severity := ctxLintSuggestion
		if uidBackfillDone() {
			severity = ctxLintWarning
		}
		return []ctxLintIssue{{Path: path, Priority: severity, Message: "document missing `uid` (immutable identifier; stamp with `sdt context uid backfill`)"}}
	}
	if !validUIDv7(uid) {
		return []ctxLintIssue{{Path: path, Priority: prio(ctxLintWarning), Message: fmt.Sprintf("`uid` %q is not a canonical UUIDv7 (RFC 9562, lowercase 8-4-4-4-12)", uid)}}
	}
	return nil
}

// lintUIDDuplicates flags every eligible document after the first that reuses
// another document's `uid` (a copied file duplicates the identifier). The
// check is corpus-wide because uniqueness cannot be decided per document.
func lintUIDDuplicates(files []string) []ctxLintIssue {
	byUID := map[string][]string{}
	for _, path := range files {
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		if !ctxUIDEligibleKind(parseFrontmatterField(content, ctxFrontmatterKind)) {
			continue
		}
		uid := parseFrontmatterField(content, ctxFrontmatterUID)
		if uid == "" || !validUIDv7(uid) {
			continue
		}
		byUID[uid] = append(byUID[uid], path)
	}
	var issues []ctxLintIssue
	for uid, paths := range byUID {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		for _, dup := range paths[1:] {
			issues = append(issues, ctxLintIssue{Path: dup, Priority: ctxLintWarning, Message: fmt.Sprintf("duplicate `uid` %s (also used by %s)", uid, paths[0])})
		}
	}
	return issues
}

// lintTaskObjectiveLegacy flags a task file still carrying the legacy
// `objective` field (renamed to `phase`; the objective is inherited from the
// plan, never declared on the task).
func lintTaskObjectiveLegacy(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeTasks || parseFrontmatterField(content, ctxFrontmatterObjective) == "" {
		return nil
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "task file carries legacy `objective`; rename it to `phase` (the plan objective is inherited, not declared)"}}
}

// lintQuestionsDeferredReason asks a deferred open question to record why: a
// `status: deferred` without a `deferred_reason` hides the revival condition.
// Advisory SUGGESTION, questions kind only.
func lintQuestionsDeferredReason(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeQuestions || parseFrontmatterField(content, ctxMapStatus) != questionStatusDeferred {
		return nil
	}
	if strings.TrimSpace(parseFrontmatterField(content, ctxFrontmatterDeferredReason)) != "" {
		return nil
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "questions file is `status: deferred` without a `deferred_reason` (record why it is parked and its revival condition)"}}
}

// ctxHeadingRegexp matches a markdown ATX heading, capturing its level and text.
var ctxHeadingRegexp = regexp.MustCompile(`(?m)^(#{2,6})\s+(.*)$`)

// ctxSection returns the body of the `## <name>` section, from its heading up to
// the next heading of the same or a higher level. ok is false when the section
// is absent.
func ctxSection(content, name string) (string, bool) {
	content = stripFencedCode(content)
	start := -1
	level := 0
	for _, m := range ctxHeadingRegexp.FindAllStringSubmatchIndex(content, -1) {
		text := strings.TrimSpace(content[m[4]:m[5]])
		if start < 0 {
			if text == name {
				start = m[0]
				level = len(content[m[2]:m[3]])
			}
			continue
		}
		// Stop at the next heading of the same or a higher level.
		if len(content[m[2]:m[3]]) <= level {
			return content[start:m[0]], true
		}
	}
	if start < 0 {
		return "", false
	}
	return content[start:], true
}

// ctxSectionCount counts the `## ` sections of a document body, the size unit
// used by the oversized-analysis guard. Fenced code blocks are blanked first so
// a `## ` line inside a sample snippet never inflates the count.
func ctxSectionCount(body string) int {
	n := 0
	for _, ln := range strings.Split(stripFencedCode(body), "\n") {
		if strings.HasPrefix(ln, "## ") {
			n++
		}
	}
	return n
}

// stripFencedCode replaces the content of every ```-fenced block with blank
// lines, preserving line numbers so callers can still reason about positions.
func stripFencedCode(body string) string {
	lines := strings.Split(body, "\n")
	fence := ""
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		switch {
		case fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			fence = trimmed[:3]
			lines[i] = ""
		case fence != "" && strings.HasPrefix(trimmed, fence):
			fence = ""
			lines[i] = ""
		case fence != "":
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// ctxOptionBlock is one `###` option subsection of an `## Options considered`
// section: its heading text and its own body.
type ctxOptionBlock struct {
	heading string
	body    string
}

// ctxOptionBlocks splits a `## Options considered` section into its `###` option
// subsections. A section written without `###` subsections yields no block, so a
// flat or missing option list is not reported by the outcome rule.
func ctxOptionBlocks(section string) []ctxOptionBlock {
	lines := strings.Split(section, "\n")
	optionHeading := regexp.MustCompile(`^###\s+(.*)$`)
	var idx []int
	var titles []string
	for i, ln := range lines {
		if m := optionHeading.FindStringSubmatch(ln); m != nil {
			idx = append(idx, i)
			titles = append(titles, strings.TrimSpace(m[1]))
		}
	}
	blocks := make([]ctxOptionBlock, 0, len(idx))
	for n, start := range idx {
		end := len(lines)
		if n+1 < len(idx) {
			end = idx[n+1]
		}
		blocks = append(blocks, ctxOptionBlock{heading: titles[n], body: strings.Join(lines[start:end], "\n")})
	}
	return blocks
}

// ctxOptionOutcomeRegexp matches a recorded verdict line for one option, in the
// `**Outcome:** rejected (reason)` form rule R3 asks for, and in the terser
// `**Accepted**` / `**Rejected**` / `**Postponed**` bold-verdict form.
var ctxOptionOutcomeRegexp = regexp.MustCompile(`(?im)^\s*[-*]?\s*\**\s*(outcome|fate|verdict|decision)\s*\**\s*:|^\s*\**\s*(accepted|rejected|postponed)\b`)

// lintAnalysisOptions requires every option in an `## Options considered`
// section to carry a recorded outcome (rule R3): the analysis must say which
// option was accepted, which were rejected and why, and which are postponed.
// Advisory SUGGESTION: it checks that a verdict is present, never its quality
// (a known and accepted limit).
func lintAnalysisOptions(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeAnalysis {
		return nil
	}
	section, ok := ctxSection(content, "Options considered")
	if !ok {
		return nil
	}
	var unrecorded []string
	for _, b := range ctxOptionBlocks(section) {
		if !ctxOptionOutcomeRegexp.MatchString(b.body) {
			unrecorded = append(unrecorded, b.heading)
		}
	}
	if len(unrecorded) == 0 {
		return nil
	}
	names := unrecorded
	if len(names) > 3 {
		names = append(names[:3:3], "…")
	}
	return []ctxLintIssue{{
		Path:     path,
		Priority: ctxLintSuggestion,
		Message:  fmt.Sprintf("analysis lists %d option(s) with no recorded outcome (%s)", len(unrecorded), strings.Join(names, "; ")),
		Hint:     "record the fate of every option as accepted / rejected (+ reason) / postponed — R3 in `context/instructions/analysis.md`",
	}}
}

// lintAnalysisOversize flags an analysis past the soft section/option bounds so
// a too-broad analysis becomes a sibling set instead of a monolith. Advisory
// SUGGESTION mirroring the oversized-task-file rule, and the ask-first gate is
// part of the hint: the split decision stays with the user (rule R1).
func lintAnalysisOversize(path, content, kind string) []ctxLintIssue {
	if kind != ctxTypeAnalysis {
		return nil
	}
	body := string(frontmatterBody([]byte(content)))
	sections := ctxSectionCount(body)
	options := 0
	if section, ok := ctxSection(body, "Options considered"); ok {
		options = len(ctxOptionBlocks(section))
	}
	switch {
	case sections > ctxAnalysisOversizedSections:
		return []ctxLintIssue{{
			Path:     path,
			Priority: ctxLintSuggestion,
			Message:  fmt.Sprintf("analysis has %d sections (>%d); it likely carries more than one subject", sections, ctxAnalysisOversizedSections),
			Hint:     "consider splitting it into sibling analyses under the same `objective` — ask the user first (R1 in `context/instructions/analysis.md`)",
		}}
	case options > ctxAnalysisOversizedOptions:
		return []ctxLintIssue{{
			Path:     path,
			Priority: ctxLintSuggestion,
			Message:  fmt.Sprintf("analysis weighs %d options (>%d); more than one domain here is a split signal", options, ctxAnalysisOversizedOptions),
			Hint:     "consider splitting it into sibling analyses under the same `objective` — ask the user first (R1 in `context/instructions/analysis.md`)",
		}}
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
	for _, field := range []string{ctxFrontmatterLinks, ctxTypeSupersedes, ctxFrontmatterContradicts} {
		if len(parseFrontmatterList(content, field)) > 0 {
			return nil
		}
	}
	return []ctxLintIssue{{Path: path, Priority: ctxLintSuggestion, Message: "analysis declares no relation to prior work (add `" + ctxFrontmatterLinks + "`, `supersedes`/`contradicts`, or `" + ctxFrontmatterLinks + ": none` with a reason)"}}
}

func resolveContextLintPath(ref string) (string, error) {
	indexRef := strings.TrimSpace(ref)
	if filepath.IsAbs(indexRef) {
		rel, err := filepath.Rel(ctxCwd(), indexRef)
		if err != nil {
			return "", err
		}
		indexRef = rel
	}
	indexRef = filepath.Clean(indexRef)
	if indexRef != sdtContextIndex && !strings.HasPrefix(filepath.ToSlash(indexRef), sdtWorkDir+"/") {
		indexRef = filepath.Join(sdtWorkDir, indexRef)
	}
	if filepath.ToSlash(filepath.Clean(indexRef)) == sdtContextIndex {
		info, err := os.Stat(sdtContextIndex)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("%s is a directory", sdtContextIndex)
		}
		return sdtContextIndex, nil
	}

	doc, err := resolveContextDocPath(ref)
	if err != nil {
		return "", err
	}
	if corpus.ExcludedPath(filepath.ToSlash(doc.Path)) {
		return "", fmt.Errorf("path %q is outside the lint corpus", ref)
	}
	if !contextLintCorpusPath(doc.Path) {
		return "", fmt.Errorf("path %q is not covered by corpus-wide context lint; use the specialized lint command if available", ref)
	}
	return doc.Path, nil
}

func contextLintCorpusPath(path string) bool {
	if filepath.Clean(path) == filepath.Clean(sdtContextIndex) {
		return true
	}
	dir := filepath.Clean(filepath.Dir(path))
	for _, indexedDir := range ctxIndexDirs {
		if dir == filepath.Clean(indexedDir) {
			return true
		}
	}
	return false
}

// ctxDoneStatuses mirrors the viewer's "done" vocabulary
// (web/src/lib/statusDot.ts DONE) so lint and the dot agree on completion.
var ctxDoneStatuses = map[string]bool{
	taskFileStatusCompleted: true,
	"complete":              true,
	taskStatusDone:          true,
	"executed":              true,
	taskFileStatusArchived:  true,
}

// normalizeContextRef mirrors web/src/lib/statusDot.ts normalizeRef: strip a
// leading ./ or /, add .md, and prefix context/ when absent.
func normalizeContextRef(ref string) string {
	clean := strings.TrimSpace(ref)
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimLeft(clean, "/")
	if !strings.HasSuffix(clean, sdtMarkdownExt) {
		clean += sdtMarkdownExt
	}
	if !strings.HasPrefix(clean, sdtWorkDir+"/") {
		clean = sdtWorkDir + "/" + clean
	}
	return clean
}

// lintPlanTaskAgreement reports a plan declaring `completed` whose referenced
// tasks are not all done, and distinguishes a plan with no resolvable task
// reference. WARNING so a plan completed in the same change that archives its
// tasks never hard-fails lint.
func lintPlanTaskAgreement(planFiles, taskFiles []string, edges *ctxrel.Edges) []ctxLintIssue {
	type taskDoc struct {
		path   string
		status string
	}
	tasksByPlan := map[string][]taskDoc{}
	for _, path := range taskFiles {
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		if parseFrontmatterField(content, "kind") != ctxTypeTasks {
			continue
		}
		// The association is the typed `plan_id`, resolved once by ctxrel: a
		// task file derives from exactly one plan, whatever its `sources` cite.
		ref := edges.ParentOf(filepath.ToSlash(path))
		if ref == "" {
			continue
		}
		tasksByPlan[ref] = append(tasksByPlan[ref], taskDoc{path: path, status: strings.ToLower(strings.TrimSpace(parseFrontmatterField(content, "status")))})
	}

	var issues []ctxLintIssue
	for _, path := range planFiles {
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		if parseFrontmatterField(content, "kind") != ctxTypePlan {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parseFrontmatterField(content, "status"))) != taskFileStatusCompleted {
			continue
		}
		tasks := tasksByPlan[normalizeContextRef(path)]
		if len(tasks) == 0 {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: "plan declares completed but no task file references it (cannot verify completion)"})
			continue
		}
		var unfinished []string
		for _, task := range tasks {
			if !ctxDoneStatuses[task.status] {
				unfinished = append(unfinished, filepath.Base(task.path))
			}
		}
		if len(unfinished) > 0 {
			sort.Strings(unfinished)
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintWarning, Message: fmt.Sprintf("plan declares completed but %d task(s) are not done: %s", len(unfinished), strings.Join(unfinished, ", "))})
		}
	}
	return issues
}

// ctxChecklistKinds are the kinds whose bodies carry addressable checklist
// items and therefore must keep their ids unique per document.
var ctxChecklistKinds = map[string]bool{
	ctxTypeTasks:     true,
	ctxTypePlan:      true,
	ctxTypeAnalysis:  true,
	ctxTypeQuestions: true,
}

// lintChecklistAnchors flags a repeated `<!-- c<N> -->` id in one document: an
// id must address exactly one item (decision 0014). Advisory WARNING, never a
// failure; the repair is `sdt context checklist backfill`.
func lintChecklistAnchors(path, content, kind string) []ctxLintIssue {
	if !ctxChecklistKinds[kind] {
		return nil
	}
	counts := map[string]int{}
	for _, it := range parseChecklistItems(content) {
		if it.ID != "" {
			counts[it.ID]++
		}
	}
	var ids []string
	for id, n := range counts {
		if n > 1 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Slice(ids, func(i, j int) bool {
		a, _ := parseChecklistID(ids[i])
		b, _ := parseChecklistID(ids[j])
		return a < b
	})
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%s ×%d", id, counts[id]))
	}
	return []ctxLintIssue{{
		Path:     path,
		Priority: ctxLintWarning,
		Message:  "duplicate checklist id(s): " + strings.Join(parts, ", ") + " (each id must address one item)",
		Hint:     "run `sdt context checklist backfill` to renumber duplicate anchors so each id addresses one item",
	}}
}

// lintTaskFileRules covers the per-file task advisories: an oversized
// `## Phase` section (SUGGESTION, never a failure) and a completed file with
// no `## Review` block.
func lintTaskFileRules(path, content string) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, sc := range taskSectionItemCounts(content) {
		if sc.Count <= ctxTasksOversizedItems {
			continue
		}
		msg := fmt.Sprintf("task file has %d checklist items (>%d); consider splitting the phase", sc.Count, ctxTasksOversizedItems)
		if sc.Label != "" {
			msg = fmt.Sprintf("task file phase %s has %d checklist items (>%d); consider splitting the phase", sc.Label, sc.Count, ctxTasksOversizedItems)
		}
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: msg})
	}
	if parseFrontmatterField(content, "status") == taskFileStatusCompleted && !hasReviewBlock(content) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: "completed task file has no `## Review` verify-step block (record verdicts; see `sdt context task review`)"})
	}
	issues = append(issues, lintFindingVerdict(path, content)...)
	issues = append(issues, lintDeviation(path, content)...)
	return issues
}

// lintFindingVerdict reports a `### Findings` item of a completed task file whose
// text carries no verdict token. The verify-step vocabulary is closed by decision
// 0013 and the verdict is the item's trailing token, so an item without one
// records a claim nobody judged. SUGGESTION — the finding is still readable, it
// just cannot be checked.
func lintFindingVerdict(path, content string) []ctxLintIssue {
	if parseFrontmatterField(content, "status") != taskFileStatusCompleted {
		return nil
	}
	var issues []ctxLintIssue
	for _, section := range mdstruct.SplitSections(content) {
		if section.Level != 3 || !strings.EqualFold(section.Heading, "Findings") {
			continue
		}
		for _, entry := range parseChecklistEntries(strings.Split(section.Body, "\n")) {
			body, _ := splitChecklistAnchor(entry.Body)
			if hasVerdictToken(body) {
				continue
			}
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion,
				Message: fmt.Sprintf("finding under `### Findings` has no verdict token (want %s): %s", ctxReviewVerdictHelp, truncateLintText(body))})
		}
	}
	return issues
}

// truncateLintText keeps a finding's claim short enough to read in a lint line.
func truncateLintText(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if len(flat) <= 80 {
		return flat
	}
	return flat[:77] + "..."
}

// taskPlanCoverage unions the phase labels a task file covers (legacy scalar
// `phase`, `phases` list, `-phase-<label>` filename, `## Phase` sections) and
// reports whether any section surface is used at all.

type taskPlanCoverage struct {
	covers       map[string]bool
	usesSections bool
}

func taskCoverageFor(content, path string) taskPlanCoverage {
	pc := taskPlanCoverage{covers: map[string]bool{}}
	if p := parseFrontmatterField(content, "phase"); p != "" {
		pc.covers[p] = true
	}
	if m := ctxPhaseSuffixRegexp.FindStringSubmatch(path); m != nil {
		pc.covers[m[1]] = true
	}
	if ph := parseFrontmatterField(content, "phases"); ph != "" {
		pc.usesSections = true
		for _, x := range parsePhaseList(ph) {
			pc.covers[x] = true
		}
	}
	for _, m := range ctxTaskPhaseSectionRegexp.FindAllStringSubmatch(content, -1) {
		pc.covers[m[1]] = true
		pc.usesSections = true
	}
	return pc
}

// uncoveredPlanPhases returns the plan's `### Phase` labels missing from the
// covered set, in document order.

func uncoveredPlanPhases(content string, covers map[string]bool) []string {
	seen := map[string]bool{}
	var missing []string
	for _, m := range ctxPlanPhaseSectionRegexp.FindAllStringSubmatch(content, -1) {
		label := m[1]
		if seen[label] {
			continue
		}
		seen[label] = true
		if !covers[label] {
			missing = append(missing, label)
		}
	}
	return missing
}

// lintTaskPhaseCoverage suggests when a plan that already uses the section
// model lists a phase no task file covers. A plan whose task files are all
// legacy (no `## Phase` section or `phases` list) is skipped, so the advisory
// only fires once a plan opts into the lean file model.
func lintTaskPhaseCoverage(planFiles, taskFiles []string, edges *ctxrel.Edges) []ctxLintIssue {
	byPlan := map[string]taskPlanCoverage{}
	for _, path := range taskFiles {
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		if parseFrontmatterField(content, "kind") != ctxTypeTasks {
			continue
		}
		ref := edges.ParentOf(filepath.ToSlash(path))
		if ref == "" {
			continue
		}
		pc := byPlan[ref]
		if pc.covers == nil {
			pc.covers = map[string]bool{}
		}
		one := taskCoverageFor(content, path)
		for label := range one.covers {
			pc.covers[label] = true
		}
		pc.usesSections = pc.usesSections || one.usesSections
		byPlan[ref] = pc
	}

	var issues []ctxLintIssue
	for _, path := range planFiles {
		data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if err != nil {
			continue
		}
		content := string(data)
		if parseFrontmatterField(content, "kind") != ctxTypePlan {
			continue
		}
		pc := byPlan[normalizeContextRef(path)]
		if pc.covers == nil || !pc.usesSections {
			continue
		}
		if missing := uncoveredPlanPhases(content, pc.covers); len(missing) > 0 {
			issues = append(issues, ctxLintIssue{Path: path, Priority: ctxLintSuggestion, Message: fmt.Sprintf("plan phase(s) %s have no `## Phase` section in any task file", strings.Join(missing, ", "))})
		}
	}
	return issues
}

var contextLintCmd = &cobra.Command{
	Use:   gateStepLint,
	Short: "Validate context frontmatter and links",
	Long: `Validate context frontmatter, Markdown rules and links. With no path,
scans the context corpus; with one or more paths, validates only those documents.
Paths are relative to the project root (context/plan/file.md) or context/
(plan/file.md); targets must be in the same top-level corpus directories scanned
by the default command, or be context/index.md. Wiki pages use
sdt context wiki lint. Exits non-zero when CRITICAL issues are found.

With --security, additionally scan the selected documents (or the whole corpus)
for prompt-injection phrases, credential/secret literals, invisible/zero-width
Unicode and exfil patterns (advisory WARNING).

Examples:
  sdt context lint
  sdt context lint context/plan/example.md
  sdt context lint plan/example.md --format json
  sdt context lint --security
  sdt context lint --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var issues []ctxLintIssue
		security := getBoolFlag(cmd, "security", false)
		staleDays := getIntFlag(cmd, "stale-days", false)
		if staleDays <= 0 {
			staleDays = ctxStaleInProgressDays
		}
		// Load the controlled topic register once; a malformed file is reported
		// as a lint issue on the register itself.
		reg, regErr := loadTopicRegister()
		if regErr != nil {
			issues = append(issues, ctxLintIssue{Path: ctxTopicsFilePath, Priority: ctxLintWarning, Message: regErr.Error()})
			reg = &ctxTopicRegister{Topics: map[string][]string{}, Aliases: map[string]string{}}
		}
		ctxTopicReg = reg
		catReg, catErr := loadCategoryRegister()
		if catErr != nil {
			issues = append(issues, ctxLintIssue{Path: ctxCategoriesFilePath, Priority: ctxLintWarning, Message: catErr.Error()})
			catReg = &ctxCategoryRegister{Categories: map[string][]string{}, Aliases: map[string]string{}}
		}
		ctxCategoryReg = catReg
		if memoErr := validateMemoRegister(); memoErr != nil {
			issues = append(issues, ctxLintIssue{Path: ctxMemoFilePath, Priority: ctxLintWarning, Message: memoErr.Error()})
		}
		if todoErr := validateTodoRegister(); todoErr != nil {
			issues = append(issues, ctxLintIssue{Path: ctxTodoFilePath, Priority: ctxLintWarning, Message: todoErr.Error()})
		}
		if len(args) > 0 {
			for _, ref := range args {
				path, err := resolveContextLintPath(ref)
				exitWithError(cmd, err)
				issues = append(issues, lintDoc(path)...)
				if security {
					issues = append(issues, lintSecurity(path)...)
				}
			}
		} else {
			var allFiles []string
			for _, dir := range ctxIndexDirs {
				files, err := dirFiles(dir)
				exitWithError(cmd, err)
				allFiles = append(allFiles, files...)
				for _, f := range files {
					issues = append(issues, lintDoc(f)...)
					if security {
						issues = append(issues, lintSecurity(f)...)
					}
				}
			}
			// Corpus-wide `uid` uniqueness (a copied file duplicates the id).
			issues = append(issues, lintUIDDuplicates(allFiles)...)
			// Typed parent relations: bidirectional agreement keyed by uid.
			issues = append(issues, lintParentRelations(allFiles)...)
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
			// Plan/task disagreement guard (D4): needs both document sets; a
			// task file lives in its own type directory for its whole life, so
			// tasks/ plus plan/ is the complete association set.
			planFiles, err := dirFiles(sdtPlanDir)
			exitWithError(cmd, err)
			taskFiles, err := dirFiles(sdtTasksDir)
			exitWithError(cmd, err)
			edges, err := ctxrel.Load(sdtWorkDir)
			exitWithError(cmd, err)
			issues = append(issues, lintPlanTaskAgreement(planFiles, taskFiles, edges)...)
			// Advisory phase-coverage check: a plan already using the section
			// model whose phase no task file covers.
			issues = append(issues, lintTaskPhaseCoverage(planFiles, taskFiles, edges)...)
			// Reconciler completion (B1): the orphan, status-drift and
			// staleness checks, all reading the one shared report that
			// `sdt context resume` renders.
			reconcile := reconcileCorpus(staleDays, contextNow())
			issues = append(issues, lintTaskOrphans(reconcile)...)
			issues = append(issues, lintPlanTaskStatusDrift(reconcile)...)
			issues = append(issues, lintStaleInProgress(reconcile)...)
			// Gate evidence (B2): a closed phase whose Review block never
			// recorded a delivery-gate run.
			issues = append(issues, lintGateEvidence(reconcile)...)
			// Objective-to-phase traceability (B4): a plan phase claiming an
			// objective its source analysis does not declare, and a declared
			// objective no derived plan phase covers.
			issues = append(issues, lintObjectivePhaseCoverage(planFiles, edges)...)
			// Declared-vs-derived drift across the whole chain (task, plan,
			// analysis) at advisory WARNING severity (analysis Q4).
			issues = append(issues, lintCascadeDrift()...)
			// Role-profile advisory: mirror deterministic role checks as SUGGESTIONs.
			issues = append(issues, lintRoleProfiles()...)
		}
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
