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
)

var contextNow = time.Now

var contextRunEditor = contextRunEditorDefault

var ctxSlugRegexp = regexp.MustCompile(`[^a-z0-9-]+`)

var ctxTaskLineRegexp = regexp.MustCompile(`^- \[([ x~!])\] (.*)$`)

// ctxSummaryPlaceholder is emitted as the `summary` value when --summary is
// omitted, so the generated file is lint-parseable and the agent knows to fill
// it in.

const ctxSummaryPlaceholder = "<one-line summary — MANDATORY, fill in>"

// ctxResearchSubjectPlaceholder is emitted as the `subject` value for a new
// research document (the question the research run answers).

const ctxResearchSubjectPlaceholder = "<research subject — the question this run answers, fill in>"

// ctxDefaultStatus maps a bootstrappable type to its initial `status`. Types
// without an entry do not carry a `status` field (worklog/notes).

var ctxDefaultStatus = map[string]string{
	ctxTypePlan:         ctxWikiStatusActive,
	ctxTypeAnalysis:     ctxWikiStatusActive,
	ctxTypeQuestions:    ctxWikiStatusActive,
	ctxTypeArchitecture: ctxWikiStatusDraft,
	ctxTypeProposal:     ctxWikiStatusDraft,
	ctxTypePrompt:       ctxWikiStatusDraft,
	ctxTypeResearch:     ctxWikiStatusDraft,
}

// ctxHasUpdated lists types whose instruction contract requires an `updated`
// field (notes does not).

var ctxHasUpdated = map[string]bool{
	ctxTypePlan:         true,
	ctxTypeAnalysis:     true,
	ctxTypeWorklog:      true,
	ctxTypeQuestions:    true,
	ctxTypeArchitecture: true,
	ctxTypeProposal:     true,
	ctxTypePrompt:       true,
	ctxTypeResearch:     true,
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = ctxSlugRegexp.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// contextDir returns the working directory for a context type.

func contextDir(typ string) (string, bool) {
	switch typ {
	case ctxTypePlan:
		return sdtPlanDir, true
	case ctxTypeAnalysis:
		return sdtAnalysisDir, true
	case ctxTypeWorklog:
		return sdtWorklogDir, true
	case ctxTypeNotes:
		return sdtNotesDir, true
	case ctxTypeTasks:
		return sdtTasksDir, true
	case ctxTypeTmp:
		return sdtTmpDir, true
	case ctxTypeArchive:
		return sdtArchiveDir, true
	case ctxTypeArchitecture:
		return sdtArchitectureDir, true
	case ctxTypeDecision:
		return sdtDecisionsDir, true
	case ctxTypeQuestions:
		return sdtQuestionsDir, true
	case ctxTypeProposal:
		return sdtProposalsDir, true
	case ctxTypePrompt:
		return sdtPromptsDir, true
	case ctxTypeResearch:
		return sdtResearchDir, true
	}
	return "", false
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
	switch typ {
	case ctxTypePlan:
		return filepath.Join(sdtPlanDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeAnalysis:
		return filepath.Join(sdtAnalysisDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeWorklog:
		return filepath.Join(sdtWorklogDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeNotes:
		return filepath.Join(sdtNotesDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeTasks:
		if phase == "" {
			return "", errors.New("--phase <n> is required for type tasks")
		}
		return taskFileFor(phase, plan), nil
	case ctxTypeTmp:
		if slug == "" {
			return "", errors.New("--slug is required for type tmp")
		}
		return filepath.Join(sdtTmpDir, slug), nil
	case ctxTypeArchive:
		return filepath.Join(sdtArchiveDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeArchitecture:
		return "context/architecture/" + slug + sdtMarkdownExt, nil
	case ctxTypeDecision:
		return "", errors.New("decision type is append-only; create via `sdt context new --type decision`")
	case ctxTypeQuestions:
		return filepath.Join(sdtQuestionsDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeProposal:
		return filepath.Join(sdtProposalsDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypePrompt:
		return filepath.Join(sdtPromptsDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeResearch:
		return filepath.Join(sdtResearchDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	}
	return "", fmt.Errorf("unknown type %q (use plan|analysis|worklog|notes|tasks|tmp|archive|architecture|decision|questions|proposal|prompt|research)", typ)
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
	contextPathCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|tasks|tmp|archive|architecture|decision")
	contextPathCmd.Flags().String("slug", "", "Slug (sanitized)")
	contextPathCmd.Flags().String("phase", "", "Phase for type tasks (plan phase number, e.g. 1 or 1a)")
	contextPathCmd.Flags().String("plan", "", "Plan reference for type tasks (plan file or standalone slug)")
	contextPathCmd.Flags().String("number", "", "Number for type decision (4-digit NNNN)")

	contextNewCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|questions|architecture|decision")
	contextNewCmd.Flags().String("title", "", "Title (slug derived from it when --slug omitted)")
	contextNewCmd.Flags().String("slug", "", "Slug (sanitized; overrides --title-derived slug)")
	contextNewCmd.Flags().String("summary", "", "Summary for the frontmatter (default: MANDATORY-fill placeholder)")
	contextNewCmd.Flags().String("context", "", "What triggered this entry")
	contextNewCmd.Flags().String("number", "", "Override for the decision number (default: next NNNN from decisions/)")
	contextNewCmd.Flags().Bool("force", false, "Overwrite existing file")
	contextNewCmd.Flags().Bool("edit", false, "Open the file in $EDITOR after creation")

	contextListCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|tasks|archive")

	contextTaskAddCmd.Flags().String("objective", "", "Objective for the task list (used when creating)")
	contextTaskAddCmd.Flags().String("summary", "", "Summary for the checklist frontmatter (default: derived from phase/objective)")
	contextTaskAddCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskListCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskDoneCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskBlockCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskWipCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskArchiveCmd.Flags().String("plan", "", "Plan reference (plan file; default: latest active plan; custom slug for standalone)")
	contextTaskBlockCmd.Flags().String("reason", "", "Reason for blocking")
	contextTaskArchiveCmd.Flags().String("slug", "", "Archive slug (default: from objective)")
	contextTaskAddCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskListCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskDoneCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskBlockCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskWipCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")
	contextTaskArchiveCmd.Flags().String("phase", "", "Phase number from the plan, e.g. 1 or 1a (required)")

	contextTemplateCmd.Flags().String("type", "", "Type: analysis|plan|tasks|decision|architecture|worklog|notes")

	contextTaskCmd.AddCommand(contextTaskListCmd, contextTaskAddCmd, contextTaskDoneCmd, contextTaskBlockCmd, contextTaskWipCmd, contextTaskArchiveCmd)
	contextCmd.AddCommand(contextPathCmd, contextNewCmd, contextListCmd, contextTaskCmd, contextReindexCmd, contextLintCmd, contextStatusCmd, contextTemplateCmd)
	rootCmd.AddCommand(contextCmd)
}
