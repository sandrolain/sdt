package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type taskItem struct {
	Line   int    `json:"line" yaml:"line"`
	Status string `json:"status" yaml:"status"`
	Text   string `json:"text" yaml:"text"`
}

func parseTaskItems(content string) []taskItem {
	lines := strings.Split(content, "\n")
	var items []taskItem
	id := 0
	for _, line := range lines {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id++
		status := taskStatusTodo
		switch m[1] {
		case "x":
			status = taskStatusDone
		case "~":
			status = taskStatusWip
		case "!":
			status = taskStatusBlocked
		}
		items = append(items, taskItem{Line: id, Status: status, Text: m[2]})
	}
	return items
}

func outputTaskItems(cmd *cobra.Command, items []taskItem) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(items, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(items)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		marker := map[string]string{
			taskStatusTodo:    " ",
			taskStatusWip:     "~",
			taskStatusDone:    "x",
			taskStatusBlocked: "!",
		}
		for _, it := range items {
			outputString(cmd, fmt.Sprintf("%d. [%s] %s\n", it.Line, marker[it.Status], it.Text))
		}
	}
}

// ctxPlanPrefix matches the leading timestamp of a canonical plan filename
// (YYYYMMDD-HHMMSS-). taskSlugFromPlan strips it to recover the plan's slug.

var ctxPlanPrefix = regexp.MustCompile(`^\d{8}-\d{6}-`)

// taskSlugFromPlan derives the slug part of a task file name from the plan
// reference: a canonical plan filename (timestamps stripped) or a standalone
// custom slug used verbatim.

func taskSlugFromPlan(plan string) string {
	s := strings.TrimSuffix(strings.TrimSpace(plan), sdtMarkdownExt)
	if loc := ctxPlanPrefix.FindStringIndex(s); loc != nil {
		s = s[loc[1]:]
	}
	return s
}

// taskFileFor builds the plan-scoped dated task file name
// (<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md) for the given plan phase
// number `<n>` and plan reference. When a matching file for the phase already
// exists it is returned (its name keeps its real creation-time prefix);
// otherwise a fresh name with timestamp = now (task creation, contextNow) is
// produced.

func taskFileFor(phase, plan string) string {
	n := sanitizeSlug(phase)
	slug := taskSlugFromPlan(plan)
	if matches, err := taskFilesForPhase(phase, slug); err == nil && len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool {
			mi, ei := os.Stat(matches[i])
			mj, ej := os.Stat(matches[j])
			if ei != nil || ej != nil {
				return matches[i] > matches[j]
			}
			return mi.ModTime().After(mj.ModTime())
		})
		return matches[0]
	}
	name := contextTimePrefix("20060102-150405", slug+"-phase-"+n)
	return filepath.Join(sdtTasksDir, name+".md")
}

// taskFilesForPhase lists existing task files matching
// *-<slug-plan>-phase-<n>.md (any timestamp prefix).

func taskFilesForPhase(phase, slug string) ([]string, error) {
	pattern := filepath.Join(sdtTasksDir, "*-"+slug+"-phase-"+sanitizeSlug(phase)+sdtMarkdownExt)
	return filepath.Glob(pattern)
}

func readTaskFile(phase, plan string) (string, error) {
	path := taskFileFor(phase, plan)
	//#nosec G304 -- fixed repo path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no task list at %s (create one with `sdt context task add --phase %s %s`)", path, phase, planFlagForHint(plan))
		}
		return "", err
	}
	return string(data), nil
}

// planFlagForHint renders the --plan fragment for an error hint, or "" when
// the plan reference is the default (not worth repeating).

func planFlagForHint(plan string) string {
	if plan == "" {
		return ""
	}
	return "--plan " + plan
}

// taskTarget resolves the confirmed --phase/--plan semantics for the
// `context task` family: --phase <n> is required (plan phase number, numeric
// or alphanumeric as written), --plan defaults to latestActivePlan() and an
// explicit `--plan <custom-slug>` enables standalone checklists.

func taskTarget(cmd *cobra.Command) (phase, plan string, err error) {
	phase = sanitizeSlug(getStringFlag(cmd, "phase", false))
	if phase == "" {
		return "", "", errors.New("--phase <n> is required (plan phase number, e.g. 1 or 1a)")
	}
	plan = getStringFlag(cmd, "plan", false)
	if plan == "" {
		plan = latestActivePlan()
		if plan == "" {
			return "", "", errors.New("no active plan found; pass --plan <slug> to create a standalone checklist")
		}
	}
	return phase, plan, nil
}

// planHasFile reports whether ref is an existing file under context/plan/
// (a real plan reference). Standalone custom slugs do not resolve, so
// frontmatter links/sources are skipped for them.

func planHasFile(ref string) bool {
	info, err := os.Stat(filepath.Join(sdtPlanDir, ref)) //#nosec G304 -- fixed repo path
	return err == nil && !info.IsDir()
}

var contextTaskListCmd = &cobra.Command{
	Use:   useList,
	Short: "Show a per-phase task list",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		content, err := readTaskFile(phase, plan)
		exitWithError(cmd, err)
		outputTaskItems(cmd, parseTaskItems(content))
	},
}

// buildTaskFrontmatter emits a task checklist header matching the tasks.md
// convention (kind/summary/objective/status/created/updated/links/sources/
// project) so `sdt context task add` output passes lint and the index.
// links/sources reference the plan only when it is an existing real plan file
// (standalone custom slugs get no plan reference).

func buildTaskFrontmatter(objective, project, phase, summary, planRef string) string {
	now := contextNow().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: tasks\n")
	b.WriteString("summary: " + yamlScalar(summary) + "\n")
	if objective != "" {
		b.WriteString("objective: " + yamlScalar(objective) + "\n")
	}
	b.WriteString("status: " + taskFileStatusPending + "\n")
	b.WriteString("created: " + now + "\n")
	b.WriteString("updated: " + now + "\n")
	if planRef != "" && planHasFile(planRef) {
		ref := "plan/" + planRef
		b.WriteString("links:\n  - " + ref + "\n")
		b.WriteString("sources:\n  - " + ref + "\n")
	}
	if project != "" {
		b.WriteString("project: " + yamlScalar(project) + "\n")
	}
	b.WriteString("---\n\n")
	return b.String()
}

// latestActivePlan returns the newest `status: active` plan filename under
// context/plan/, or "" when none exists (standalone checklist). Plan names are
// lexically sortable (YYYYMMDD-HHMMSS-... = chronological).

func latestActivePlan() string {
	entries, err := os.ReadDir(sdtPlanDir)
	if err != nil {
		return ""
	}
	names := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), sdtMarkdownExt) {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(sdtPlanDir, e.Name())) //#nosec G304 -- fixed repo path
		if rerr != nil {
			continue
		}
		if frontmatterField(string(data), "status") != ctxWikiStatusActive {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[len(names)-1]
}

var contextTaskAddCmd = &cobra.Command{
	Use:   "add <step>",
	Short: "Add a step to the active task list",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		step := strings.TrimSpace(args[0])
		if step == "" {
			exitWithError(cmd, errors.New("step is required"))
		}
		objective := getStringFlag(cmd, "objective", false)
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		path := taskFileFor(phase, plan)
		content := ""
		//#nosec G304 -- fixed repo path
		if data, err := os.ReadFile(path); err == nil {
			content = string(data)
		} else if os.IsNotExist(err) {
			project := ""
			if cfg, cerr := findProjectConfig(); cerr == nil && cfg != nil {
				project = cfg.Project
			}
			summary := getStringFlag(cmd, "summary", false)
			if summary == "" {
				summary = "Task checklist for phase " + phase
				if objective != "" {
					summary += ": " + objective
				}
			}
			content = buildTaskFrontmatter(objective, project, phase, summary, plan)
		} else {
			exitWithError(cmd, err)
		}
		content = strings.TrimRight(content, "\n") + "\n"
		content += "- [ ] " + step + "\n"
		if err := os.MkdirAll(sdtTasksDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		items := parseTaskItems(content)
		outputString(cmd, fmt.Sprintf("%d\n", items[len(items)-1].Line))
	},
}

// setTaskFileStatus rewrites the frontmatter `status` and refreshes `updated`,
// returning the new content. setTaskFileStatus is a no-op on files without a
// status: line (legacy checklists).

func setTaskFileStatus(content, status string) string {
	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		if strings.HasPrefix(line, "status:") {
			lines[i] = "status: " + status
			changed = true
		} else if strings.HasPrefix(line, "updated:") {
			lines[i] = "updated: " + contextNow().UTC().Format(time.RFC3339)
			changed = true
		}
	}
	if !changed {
		return content
	}
	return strings.Join(lines, "\n")
}

// taskFileNextStatus derives the file status after applying one item
// transition: wip/block always leaves the file in-progress; done completes the
// file only when no [ ] or [~] item remains, and — when the completion comes
// from `task review` — only the review path records the verdict block.

func taskFileNextStatus(itemStatus string, content string) string {
	if itemStatus != taskStatusDone {
		return taskFileStatusInProgress
	}
	if hasUnfinishedTaskItem(content) {
		return taskFileStatusInProgress
	}
	return taskFileStatusCompleted
}

// hasUnfinishedTaskItem reports whether any `- [ ]` or `- [~]` item remains.

func hasUnfinishedTaskItem(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m != nil && m[1] != "x" && m[1] != "!" {
			return true
		}
	}
	return false
}

func updateTaskStatus(content string, id int, status, reason string) (string, error) {
	lines := strings.Split(content, "\n")
	count := 0
	for i, line := range lines {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		count++
		if count != id {
			continue
		}
		marker := " "
		switch status {
		case taskStatusDone:
			marker = "x"
		case taskStatusWip:
			marker = "~"
		case taskStatusBlock:
			marker = "!"
		}
		updated := fmt.Sprintf("- [%s] %s", marker, m[2])
		if status == taskStatusBlock && reason != "" {
			updated += fmt.Sprintf(" (blocked: %s)", reason)
		}
		lines[i] = updated
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("task id %d out of range", id)
}

func taskSetStatusCmd(status string) *cobra.Command {
	var use, short string
	switch status {
	case taskStatusDone:
		use, short = "done <id>", "Mark a task step done"
	case taskStatusBlock:
		use, short = "block <id>", "Mark a task step blocked"
	case taskStatusWip:
		use, short = "wip <id>", "Mark a task step in progress"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				exitWithError(cmd, fmt.Errorf("invalid task id %q", args[0]))
			}
			reason := ""
			if status == taskStatusBlock {
				reason = getStringFlag(cmd, "reason", false)
			}
			phase, plan, err := taskTarget(cmd)
			exitWithError(cmd, err)
			content, err := readTaskFile(phase, plan)
			exitWithError(cmd, err)
			updated, err := updateTaskStatus(content, id, status, reason)
			exitWithError(cmd, err)
			updated = setTaskFileStatus(updated, taskFileNextStatus(status, updated))
			//#nosec G306 -- user work file
			if err := os.WriteFile(taskFileFor(phase, plan), []byte(updated), 0o644); err != nil {
				exitWithError(cmd, err)
			}
			outputString(cmd, "ok\n")
		},
	}
}

// contextTaskReviewCmd records the verify-step verdict. It appends the fixed
// protocol reminder plus the caller's findings and, when every checklist item is
// done, completes the file (a completed task file should carry a Review block).
var contextTaskReviewCmd = &cobra.Command{
	Use:   ctxReviewVerb,
	Short: "Record the verify-step review verdicts in the phase task file",
	Long: `Append the verify-step review block to the phase task file and, when every
checklist item is done, mark the file completed.

Each finding ends as one of the closed verdicts (` + ctxReviewVerdictHelp + `) with
evidence; an independent pass validates findings and the phase author does not
self-approve. Use --input/--file/piped stdin for the findings text.

Examples:
  sdt context task review --phase 1 --plan plan.md --input "all gates green (CONFIRMED)"`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		path := taskFileFor(phase, plan)
		content, err := readTaskFile(phase, plan)
		exitWithError(cmd, err)
		body := getContextBody(cmd, args)
		content = appendReviewBlock(content, body)
		if !hasUnfinishedTaskItem(content) {
			content = setTaskFileStatus(content, taskFileStatusCompleted)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(strings.TrimRight(content, "\n")+"\n"), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, path+"\n")
	},
}

func taskArchiveSlug(content, flagSlug string) string {
	if s := sanitizeSlug(flagSlug); s != "" {
		return s
	}
	if s := sanitizeSlug(frontmatterField(content, "objective")); s != "" {
		return s
	}
	return "tasks"
}

func frontmatterField(content, key string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == ctxFrontmatterDelim {
			break
		}
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

var contextTaskArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive the active task list to context/archive/",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		content, err := readTaskFile(phase, plan)
		exitWithError(cmd, err)
		slug := taskArchiveSlug(content, getStringFlag(cmd, "slug", false))
		path := filepath.Join(sdtArchiveDir, contextTimePrefix("20060102-150405", slug)+".md")
		if err := os.MkdirAll(sdtArchiveDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		content = setTaskFileStatus(content, taskFileStatusArchived)
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		if err := os.Remove(taskFileFor(phase, plan)); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, path+"\n")
	},
}

var contextTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage per-phase task checklists",
	Long: `Manage plan-scoped task checklists in
context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>-phase-<n>.md. Each plan phase maps
to its own checklist; --phase <n> is required and --plan defaults to the
latest active plan (or an explicit --plan <plan-file> / --plan <slug> for a
standalone checklist).

  sdt context task list [--phase <n>] [--plan <ref>]      show steps with ids
  sdt context task add "<step>" --phase <n> [--plan <ref>] [--objective] [--summary]
  sdt context task done|block|wip <id> --phase <n> [--plan <ref>]
  sdt context task review --phase <n> [--plan <ref>]      record verdicts + complete
  sdt context task archive --phase <n> [--plan <ref>] [--slug]

Status markers: [ ] todo · [~] in-progress · [x] done · [!] blocked`,
}

// ── context group ──────────────────────────────────────────────────────────────

var contextTaskDoneCmd = taskSetStatusCmd("done")

var contextTaskBlockCmd = taskSetStatusCmd("block")

var contextTaskWipCmd = taskSetStatusCmd("wip")
