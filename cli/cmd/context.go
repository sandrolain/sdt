package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	ctxTypePlan         = "plan"
	ctxTypeAnalysis     = "analysis"
	ctxTypeWorklog      = "worklog"
	ctxTypeNotes        = "notes"
	ctxTypeTasks        = "tasks"
	ctxTypeTmp          = "tmp"
	ctxTypeArchive      = "archive"
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

var ctxSlugRegexp = regexp.MustCompile(`[^a-z0-9-]+`)

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
	s = strings.ToLower(strings.TrimSpace(s))
	s = ctxSlugRegexp.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
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

func contextPath(typ, slug, phase, plan string) (string, error) {
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
		if phase == "" {
			return "", errors.New("--phase <n> is required for type tasks")
		}
		return taskFileFor(phase, plan), nil
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
	contextPathCmd.Flags().String("plan", "", "Plan reference for type tasks (plan file or standalone slug)")
	contextPathCmd.Flags().String("number", "", "Number for type decision (4-digit NNNN)")

	contextNewCmd.Flags().String("type", "", "Type: "+ctxTypeHelpText(ctxNewTypes()))
	contextNewCmd.Flags().String("title", "", "Title (slug derived from it when --slug omitted)")
	contextNewCmd.Flags().String("slug", "", "Slug (sanitized; overrides --title-derived slug)")
	contextNewCmd.Flags().String("summary", "", "Summary for the frontmatter (default: MANDATORY-fill placeholder)")
	contextNewCmd.Flags().String("context", "", "What triggered this entry")
	contextNewCmd.Flags().String("objective", "", "Analysis group key (kebab-case slug; analysis only)")
	contextNewCmd.Flags().String("agent", "", "Provenance: agent/tool that produced the entry (notes/worklog)")
	contextNewCmd.Flags().String("role", "", "Provenance: role that produced the entry (notes/worklog)")
	contextNewCmd.Flags().String("note-type", "", "Notes subtype, e.g. dead-end (notes only)")
	contextNewCmd.Flags().StringArray("topic", nil, "Controlled topic slug (repeatable)")
	contextNewCmd.Flags().StringArray("entity", nil, "Entity slug the document mentions (repeatable)")
	contextNewCmd.Flags().Bool("prior-art", false, "Analysis: search and propose related documents in a Prior art section")
	contextNewCmd.Flags().Bool("prior-art-links", false, "Analysis: also pre-fill `links` with the top prior-art candidates")
	contextNewCmd.Flags().String("number", "", "Override for the decision number (default: next NNNN from decisions/)")
	contextNewCmd.Flags().Bool("force", false, "Overwrite existing file")
	contextNewCmd.Flags().Bool("edit", false, "Open the file in $EDITOR after creation")

	contextListCmd.Flags().String("type", "", "Type: "+ctxListHelpText())
	contextListCmd.Flags().String("agent", "", "Filter by frontmatter `agent` provenance")
	contextListCmd.Flags().String("role", "", "Filter by frontmatter `role` provenance")

	contextLintCmd.Flags().Bool("security", false, "Also scan for prompt-injection, credential and invisible-Unicode patterns (advisory WARNING)")
	contextSearchCmd.Flags().String("type", "", "Filter by frontmatter kind")
	contextSearchCmd.Flags().String("status", "", "Filter by frontmatter status (default: active; use --all for any)")
	contextSearchCmd.Flags().String("objective", "", "Filter by frontmatter objective")
	contextSearchCmd.Flags().String("topic", "", "Filter by controlled topic")
	contextSearchCmd.Flags().String("since", "", "Created on/after YYYY-MM-DD")
	contextSearchCmd.Flags().String("until", "", "Created on/before YYYY-MM-DD")
	contextSearchCmd.Flags().Int("limit", 10, "Maximum results (1-100)")
	contextSearchCmd.Flags().Bool("all", false, "Include superseded/archived documents")
	contextSearchCmd.Flags().Bool("semantic", false, "Fuse lexical with semantic (embeddings) via RRF; degrades to lexical if unavailable")
	contextSearchCmd.Flags().String("semantic-model", "", "Embedding model for --semantic (default BASE8M)")
	contextShowCmd.Flags().String("section", "", "Print only the section with this id/heading")
	contextShowCmd.Flags().String("lines", "", "Print only this line range (from:to, 1-based)")

	contextTaskAddCmd.Flags().String("objective", "", "Objective for the task list (used when creating)")
	contextTaskAddCmd.Flags().String("summary", "", "Summary for the checklist frontmatter (default: derived from phase/objective)")
	contextTaskAddCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskListCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskDoneCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskBlockCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskWipCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskReviewCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskArchiveCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskBlockCmd.Flags().String("reason", "", "Reason for blocking")
	contextTaskArchiveCmd.Flags().String("slug", "", "Archive slug (default: from objective)")
	contextTaskAddCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskListCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskDoneCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskBlockCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskWipCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskReviewCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskArchiveCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")

	contextTemplateCmd.Flags().String("type", "", "Type: "+ctxTypeHelpText(ctxTemplateTypes()))

	contextTaskCmd.AddCommand(contextTaskListCmd, contextTaskAddCmd, contextTaskDoneCmd, contextTaskBlockCmd, contextTaskWipCmd, contextTaskReviewCmd, contextTaskArchiveCmd)
	contextCmd.AddCommand(contextPathCmd, contextNewCmd, contextListCmd, contextTaskCmd, contextReindexCmd, contextLintCmd, contextStatusCmd, contextTemplateCmd, contextSearchCmd, contextShowCmd)
	rootCmd.AddCommand(contextCmd)
}
