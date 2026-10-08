package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/ctxvocab"
)

const (
	ctxTypePlan         = "plan"
	ctxTypeAnalysis     = "analysis"
	ctxTypeWorklog      = "worklog"
	ctxTypeNotes        = "notes"
	ctxTypeTasks        = "tasks"
	ctxTypeTmp          = "tmp"
	ctxTypeArchitecture = "architecture"
	ctxTypeDecision     = "decision"
	ctxTypeQuestions    = "questions"
	ctxTypeProposal     = "proposal"
	ctxTypePrompt       = "prompt"
	ctxTypeResearch     = "research"
	ctxTypeCommands     = "commands"
	ctxTypeWiki         = "wiki"
	// ctxTypeSupersedes is the forward-only relation field: a document names
	// the older document it replaces (the reverse is computed, never written).
	ctxTypeSupersedes = "supersedes"
	// ctxNoteTypeDeadEnd marks a notes entry as a rejected/dead-end approach
	// tied to an objective; reindex surfaces it in that objective's bucket.
	ctxNoteTypeDeadEnd = "dead-end"
)

var contextNow = time.Now

var contextRunEditor = contextRunEditorDefault

var ctxTaskLineRegexp = regexp.MustCompile(`^- \[([ x~!])\] (.*)$`)

// ctxWikiIDRegexp is the wiki page id grammar: kebab-case segments joined by
// "/" (a page id equals its relative subpath, e.g. "backend/auth").

var ctxWikiIDRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*$`)

// ctxSummaryPlaceholder is emitted as the `summary` value when --summary is
// omitted, so the generated file is lint-parseable and the agent knows to fill
// it in.

const ctxSummaryPlaceholder = "<one-line summary — MANDATORY, fill in>"

// ctxResearchSubjectPlaceholder is emitted as the `subject` value for a new
// research document (the question the research run answers).

const ctxResearchSubjectPlaceholder = "<research subject — the question this run answers, fill in>"

// ctxDefaultStatusFor / ctxHasUpdatedFor come from the shared type registry
// (context_types.go): the documented status used when `context new` bootstraps
// a type and whether the instruction contract requires an `updated` field.

func sanitizeSlug(s string) string {
	return ctxvocab.Slug(s)
}

// sanitizeSlugList normalizes a list of slugs, dropping empties and duplicates
// while preserving order.
func sanitizeSlugList(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range in {
		s := sanitizeSlug(v)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// contextDir returns the working directory for a context type.

func contextDir(typ string) (string, bool) {
	t, ok := ctxTypeLookup(typ)
	if !ok {
		return "", false
	}
	return t.dir, true
}

func contextTimePrefix(format, slug string) string {
	p := contextNow().Format(format)
	if slug != "" {
		p += "-" + slug
	}
	return p
}

// contextPath computes the full path of a context work file without touching the
// filesystem. It is deterministic for a given time and slug. The tasks type
// needs the plan reference (plan filename or standalone slug) and the plan
// phase number in `phase`.

func contextPath(typ, slug, phase, stream, plan string) (string, error) {
	t, ok := ctxTypeLookup(typ)
	if !ok || !t.pathSupported {
		return "", fmt.Errorf("unknown type %q (use %s)", typ, ctxTypeHelpText(ctxPathTypes()))
	}
	switch t.scheme {
	case ctxSchemeDated:
		return filepath.Join(t.dir, contextTimePrefix("20060102-150405", slug)+sdtMarkdownExt), nil
	case ctxSchemeBare:
		return filepath.Join(t.dir, slug+sdtMarkdownExt), nil
	case ctxSchemePhase:
		return taskFileForRef(phase, stream, plan), nil
	case ctxSchemeTmpBySlug:
		if slug == "" {
			return "", errors.New("--slug is required for type tmp")
		}
		return filepath.Join(t.dir, slug), nil
	case ctxSchemeSubpath:
		if slug == "" {
			return "", errors.New("--slug is required for type wiki")
		}
		if !ctxWikiIDRegexp.MatchString(slug) {
			return "", fmt.Errorf("invalid wiki id %q (kebab-case segments joined by %q)", slug, string(filepath.Separator))
		}
		return filepath.Join(t.dir, filepath.FromSlash(slug)+sdtMarkdownExt), nil
	case ctxSchemeDecision:
		return "", errors.New("decision type is append-only; create via `sdt context new --type decision`")
	}
	return "", fmt.Errorf("type %q has no path scheme", typ)
}

// ── context path ───────────────────────────────────────────────────────────────

var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Context Tools (context/ work files)",
	Long: `Manage the agent working files under context/: plans, work logs, notes
and the active task list.

  sdt context path [--type ...]   print a work file path
  sdt context new --type ...      create a work file with frontmatter
  sdt context list --type ...     list existing work files
  sdt context task ...            manage the active task list`,
}

func contextRunEditorDefault(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("EDITOR is not set")
	}
	//#nosec G204,G702 -- editor comes from $EDITOR, path is an SDT work file
	cmd := exec.Command(editor, path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func init() {
	contextPathCmd.Flags().String("type", "", "Type: "+ctxTypeHelpText(ctxPathTypes()))
	contextPathCmd.Flags().String("slug", "", "Slug (sanitized)")
	contextPathCmd.Flags().String("phase", "", "Phase for type tasks (plan phase number, e.g. 1 or 1a)")
	contextPathCmd.Flags().String("stream", "", "Split-file label for type tasks (kebab-case)")
	contextPathCmd.Flags().String("plan", "", "Plan reference for type tasks (plan file or standalone slug)")
	contextPathCmd.Flags().String("number", "", "Number for type decision (4-digit NNNN)")

	contextNewCmd.Flags().String("type", "", "Type: "+ctxTypeHelpText(ctxNewTypes()))
	contextNewCmd.Flags().String("title", "", "Title (slug derived from it when --slug omitted)")
	contextNewCmd.Flags().String("slug", "", "Slug (sanitized; overrides --title-derived slug)")
	contextNewCmd.Flags().String("summary", "", "Summary for the frontmatter (default: MANDATORY-fill placeholder)")
	contextNewCmd.Flags().String("context", "", "What triggered this entry")
	contextNewCmd.Flags().String("objective", "", "Group key (kebab-case slug; analysis, notes or plan)")
	contextNewCmd.Flags().String("status", "", "Initial status (validated against the type's vocabulary; default: the type's default)")
	contextNewCmd.Flags().StringArray("source", nil, "Source reference(s) written to `sources` and `links` (repeatable; a plan defaults its objective from the first analysis source)")
	contextNewCmd.Flags().String("agent", "", "Provenance: agent/tool that produced the entry (notes/worklog)")
	contextNewCmd.Flags().String("role", "", "Provenance: role that produced the entry (notes/worklog)")
	contextNewCmd.Flags().String("note-type", "", "Notes subtype, e.g. dead-end (notes only)")
	contextNewCmd.Flags().StringArray("topic", nil, "Controlled topic slug (repeatable)")
	contextNewCmd.Flags().StringArray("category", nil, "Analysis category slug from context/categories.yaml (repeatable)")
	contextNewCmd.Flags().StringArray("entity", nil, "Entity slug the document mentions (repeatable)")
	contextNewCmd.Flags().Bool("prior-art", false, "Analysis: search and propose related documents in a Prior art section")
	contextNewCmd.Flags().Bool("prior-art-links", false, "Analysis: also pre-fill `links` with the top prior-art candidates")
	contextNewCmd.Flags().Bool("semantic", false, "Prior-art: use the semantic (embeddings) branch; degrades to lexical when unavailable")
	contextNewCmd.Flags().String("semantic-model", "", "Embedding model for --semantic (default BASE8M)")
	contextNewCmd.Flags().String("number", "", "Override for the decision number (default: next NNNN from decisions/)")
	contextNewCmd.Flags().Bool("force", false, "Overwrite existing file")
	contextNewCmd.Flags().Bool("edit", false, "Open the file in $EDITOR after creation")

	contextListCmd.Flags().String("type", "", "Type: "+ctxListHelpText())
	contextListCmd.Flags().String("agent", "", "Filter by frontmatter `agent` provenance")
	contextListCmd.Flags().String("role", "", "Filter by frontmatter `role` provenance")
	contextListCmd.Flags().StringArray("category", nil, "Filter by frontmatter `categories` (repeatable; any-match)")
	contextListCmd.Flags().String("status", "", "Filter by frontmatter status")
	contextListCmd.Flags().String("after", "", "Match documents dated on/after (YYYY-MM-DD, RFC3339, now, now-<N><unit>)")
	contextListCmd.Flags().String("before", "", "Match documents dated on/before (same forms as --after)")
	contextListCmd.Flags().String("last", "", "Match documents in the last window, e.g. 6h, 7d, 3w (exclusive with --after)")
	contextListCmd.Flags().String("since", "", "Alias of --after")
	contextListCmd.Flags().String("until", "", "Alias of --before")
	contextListCmd.Flags().String("date", "", "Reference timestamp: created (default) or updated")
	contextListCmd.Flags().StringArray("where", nil, "Generic frontmatter filter key=value or key!=value (repeatable; AND-ed)")

	contextLintCmd.Flags().Bool("security", false, "Also scan for prompt-injection, credential and invisible-Unicode patterns (advisory WARNING)")
	contextLintCmd.Flags().Int("stale-days", ctxStaleInProgressDays, "Days before an `in-progress` task file is reported as stale (advisory SUGGESTION)")

	contextResumeCmd.Flags().String("plan", "", "Plan reference to report (default: every active plan)")
	contextResumeCmd.Flags().Int("stale-days", ctxStaleInProgressDays, "Days before an `in-progress` task file is reported as stale")

	contextDeviationCmd.AddCommand(contextDeviationAddCmd, contextDeviationListCmd)
	contextDeviationAddCmd.Flags().String("kind", ctxDeviationDefaultKind, "Deviation kind: "+ctxDeviationKindHelp())
	contextDeviationAddCmd.Flags().String("id", "", "Checklist id of the phase item the deviation concerns (marked, never deleted)")
	contextDeviationAddCmd.Flags().String("reason", "", "Why the plan was departed from (recorded with the item)")
	for _, c := range []*cobra.Command{contextDeviationAddCmd, contextDeviationListCmd} {
		c.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
		c.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
		c.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	}
	addCascadeFlag(contextDeviationAddCmd)
	contextSearchCmd.Flags().String("type", "", "Filter by frontmatter kind")
	contextSearchCmd.Flags().String("status", "", "Filter by frontmatter status (default: active; use --all for any)")
	contextSearchCmd.Flags().String("objective", "", "Filter by frontmatter objective")
	contextSearchCmd.Flags().String("topic", "", "Filter by controlled topic")
	contextSearchCmd.Flags().String("category", "", "Filter by frontmatter category")
	contextSearchCmd.Flags().String("after", "", "Dated on/after (YYYY-MM-DD, RFC3339, now, now-<N><unit>)")
	contextSearchCmd.Flags().String("before", "", "Dated on/before (same forms as --after)")
	contextSearchCmd.Flags().String("last", "", "Dated in the last window, e.g. 6h, 7d, 3w (exclusive with --after)")
	contextSearchCmd.Flags().String("date", "", "Reference timestamp: created (default) or updated")
	contextSearchCmd.Flags().StringArray("where", nil, "Generic frontmatter filter key=value or key!=value (repeatable; AND-ed)")
	contextSearchCmd.Flags().String("since", "", "Alias of --after")
	contextSearchCmd.Flags().String("until", "", "Alias of --before")
	contextSearchCmd.Flags().Int("limit", 10, "Maximum results (1-100)")
	contextSearchCmd.Flags().Bool("all", false, "Include superseded/archived documents")
	contextSearchCmd.Flags().Bool("semantic", false, "Fuse lexical with semantic (embeddings) via RRF; degrades to lexical if unavailable")
	contextSearchCmd.Flags().String("semantic-model", "", "Embedding model for --semantic (default BASE8M)")
	contextShowCmd.Flags().String("section", "", "Print only the section with this id/heading")
	contextShowCmd.Flags().String("lines", "", "Print only this line range (from:to, 1-based)")

	contextTaskAddCmd.Flags().String("summary", "", "Summary for the checklist frontmatter (default: derived from the phase)")
	contextTaskAddCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskListCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskDoneCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskBlockCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskWipCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskReviewCmd.Flags().StringArray("finding", nil, "Finding claim to record under `### Findings` (repeatable; pair by position with --verdict)")
	contextTaskReviewCmd.Flags().StringArray("verdict", nil, "Verdict for the matching --finding: "+ctxReviewVerdictHelp+" (repeatable)")
	contextTaskReviewCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskBlockCmd.Flags().String("reason", "", "Reason for blocking")
	contextTaskAddCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskListCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; filters to that section)")
	contextTaskDoneCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskBlockCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskWipCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskReviewCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskAddCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	contextTaskListCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	contextTaskDoneCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	contextTaskBlockCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	contextTaskWipCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	for _, c := range []*cobra.Command{contextTaskDoneCmd, contextTaskBlockCmd, contextTaskWipCmd} {
		c.Flags().Bool("all", false, "Select every item of the phase, or the whole file without --phase (batch)")
		c.Flags().String("grep", "", "Select the items whose text contains this substring (batch)")
		addProvenanceFlags(c)
	}
	addProvenanceFlags(contextStatusSetCmd)
	contextTaskReviewCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	contextTaskClaimCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskClaimCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskClaimCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")
	contextTaskReleaseCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskReleaseCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (optional; targets the ## Phase section)")
	contextTaskReleaseCmd.Flags().String("stream", "", "Split-file label (kebab-case); selects <slug-plan>-<stream>.md")

	contextTemplateCmd.Flags().String("type", "", "Type: "+ctxTypeHelpText(ctxTemplateTypes()))

	contextCloseCmd.Flags().Bool("allow-unfinished", false, "Close despite unfinished children/tasks (requires --reason)")
	contextCloseCmd.Flags().String("reason", "", "Why the document is closed early (required with --allow-unfinished)")
	addCascadeFlag(contextCloseCmd)

	addCascadeFlag(contextTaskAddCmd)
	addCascadeFlag(contextTaskDoneCmd)
	addCascadeFlag(contextTaskBlockCmd)
	addCascadeFlag(contextTaskWipCmd)
	addCascadeFlag(contextTaskReviewCmd)

	contextTaskCmd.AddCommand(contextTaskListCmd, contextTaskAddCmd, contextTaskDoneCmd, contextTaskBlockCmd, contextTaskWipCmd, contextTaskReviewCmd, contextDeviationCmd, contextTaskClaimCmd, contextTaskReleaseCmd)
	contextCmd.AddCommand(contextPathCmd, contextNewCmd, contextListCmd, contextTaskCmd, contextCheckCmd, contextChecklistCmd, contextSyncCmd, contextTouchCmd, contextCloseCmd, contextReindexCmd, contextLintCmd, contextResumeCmd, contextStatusCmd, contextTemplateCmd, contextSearchCmd, contextShowCmd, contextUIDCmd, contextRelationsCmd, contextMemoCmd, contextTodoCmd)
	rootCmd.AddCommand(contextCmd)
}
