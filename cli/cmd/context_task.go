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
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
}

func parseTaskItems(content string) []taskItem {
	var items []taskItem
	for _, it := range parseChecklistItems(content) {
		items = append(items, taskItem{Line: it.Line, Status: it.Status, Text: it.Text, ID: it.ID})
	}
	return items
}

// taskItemsInSection returns the checklist items of the `## Phase <phase>`
// section, or every item in the file when phase is empty or the section does
// not exist.

func taskItemsInSection(content, phase string) []taskItem {
	if phase == "" {
		return parseTaskItems(content)
	}
	lines := strings.Split(content, "\n")
	hi := phaseHeadingIndex(lines, phase)
	if hi < 0 {
		return nil
	}
	end := len(lines)
	for i := hi + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return parseTaskItems(strings.Join(lines[hi+1:end], "\n"))
}

// Task phase labels come from the three surfaces the model allows: the legacy
// `phase` scalar, the `phases` list and the `## Phase <label>` section
// headings. The plan side mirrors this with `### Phase <label>`.

var (
	ctxTaskPhaseSectionRegexp = regexp.MustCompile(`(?mi)^##[ \t]+phase[ \t]+([0-9a-z]+)`)
	ctxPlanPhaseSectionRegexp = regexp.MustCompile(`(?mi)^###[ \t]+phase[ \t]+([0-9a-z]+)`)
	ctxPhaseSuffixRegexp      = regexp.MustCompile(`-phase-([0-9A-Za-z]+)\.md$`)
)

// taskSectionCount is the checklist size of one `## Phase` section; the empty
// label is the unphased preamble (or the whole file when it has no sections).

type taskSectionCount struct {
	Label string
	Count int
}

// taskSectionItemCounts counts checklist items per `## Phase` section plus the
// unphased preamble, in a deterministic order (preamble first, then labels
// sorted). A file without phase sections yields a single empty-label entry.

func taskSectionItemCounts(content string) []taskSectionCount {
	counts := map[string]int{}
	var labels []string
	current := ""
	// The preamble counts (a legacy unphased checklist); a file-level record
	// section does not, so `## Deviations`/`## Review` items never inflate a
	// count or trip the oversized-section check.
	counting := true
	for _, line := range strings.Split(taskWorkContent(content), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			if m := ctxTaskPhaseSectionRegexp.FindStringSubmatch(t); m != nil {
				current = m[1]
				counting = true
				if _, seen := counts[current]; !seen {
					labels = append(labels, current)
				}
			} else {
				current = ""
				counting = false
			}
			continue
		}
		if counting && strings.HasPrefix(t, "- [") {
			counts[current]++
		}
	}
	sort.Strings(labels)
	out := make([]taskSectionCount, 0, len(labels)+1)
	if n := counts[""]; n > 0 {
		out = append(out, taskSectionCount{Label: "", Count: n})
	}
	for _, l := range labels {
		out = append(out, taskSectionCount{Label: l, Count: counts[l]})
	}
	return out
}

// parsePhaseList parses a `phases: [1, 2]`-style inline list value.

func parsePhaseList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, x := range strings.Split(v, ",") {
		x = strings.Trim(strings.TrimSpace(x), `"'`)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
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
		counts := map[string]int{}
		for _, it := range items {
			if it.ID != "" {
				counts[it.ID]++
			}
		}
		for _, it := range items {
			label := it.ID
			if label == "" {
				label = strconv.Itoa(it.Line)
			} else if counts[it.ID] > 1 {
				label += " (ambiguous)"
			}
			outputString(cmd, fmt.Sprintf("%s. [%s] %s\n", label, marker[it.Status], it.Text))
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

// taskFileFor resolves the task file for a plan and an optional plan phase.
// Resolution is legacy-first so the forward-only migration never renames an
// existing file: an explicit <...>-<slug-plan>-phase-<n>.md wins when it
// exists, otherwise the plan's whole-plan <...>-<slug-plan>.md file, otherwise
// a fresh whole-plan name with timestamp = now (task creation, contextNow).
// `phase` may be empty (whole-plan list); existing files keep their real
// creation-time prefix.

func taskFileFor(phase, plan string) string {
	return taskFileForRef(phase, "", plan)
}

// taskFileForRef resolves a task file for a plan with an optional phase and an
// optional split-file stream label. Precedence: a named stream selects its
// `*-<slug-plan>-<stream>.md` file (created when absent); otherwise the
// legacy-first phase/whole-plan resolution of taskFileFor. Naming a stream
// wins over an existing `-phase-<n>.md` for the same plan, so a deliberate
// split is never shadowed.

func taskFileForRef(phase, stream, plan string) string {
	slug := taskSlugFromPlan(plan)
	if s := sanitizeSlug(stream); s != "" {
		if matches, err := taskStreamFiles(s, slug); err == nil && len(matches) > 0 {
			return newestTaskFile(matches)
		}
		name := contextTimePrefix("20060102-150405", slug+"-"+s)
		return filepath.Join(sdtTasksDir, name+sdtMarkdownExt)
	}
	return taskFileForPhaseOrPlan(phase, plan)
}

func taskFileForPhaseOrPlan(phase, plan string) string {
	slug := taskSlugFromPlan(plan)
	if sanitizeSlug(phase) != "" {
		if matches, err := taskFilesForPhase(phase, slug); err == nil && len(matches) > 0 {
			return newestTaskFile(matches)
		}
	}
	if matches, err := taskPlanFiles(slug); err == nil && len(matches) > 0 {
		return newestTaskFile(matches)
	}
	name := contextTimePrefix("20060102-150405", slug)
	return filepath.Join(sdtTasksDir, name+sdtMarkdownExt)
}

// newestTaskFile returns the most recently modified match (lexical order as a
// tie-breaker when stat fails).

func newestTaskFile(matches []string) string {
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

// taskFilesForPhase lists existing legacy/deliberate phase files matching
// *-<slug-plan>-phase-<n>.md (any timestamp prefix).

func taskFilesForPhase(phase, slug string) ([]string, error) {
	pattern := filepath.Join(sdtTasksDir, "*-"+slug+"-phase-"+sanitizeSlug(phase)+sdtMarkdownExt)
	return filepath.Glob(pattern)
}

// taskPlanFiles lists existing whole-plan task files matching
// *-<slug-plan>.md (any timestamp prefix). Phase-suffixed and stream-suffixed
// names do not match, so the whole-plan file is unambiguous.

func taskPlanFiles(slug string) ([]string, error) {
	pattern := filepath.Join(sdtTasksDir, "*-"+slug+sdtMarkdownExt)
	return filepath.Glob(pattern)
}

// taskStreamFiles lists existing split files matching
// *-<slug-plan>-<stream>.md (any timestamp prefix).

func taskStreamFiles(stream, slug string) ([]string, error) {
	pattern := filepath.Join(sdtTasksDir, "*-"+slug+"-"+sanitizeSlug(stream)+sdtMarkdownExt)
	return filepath.Glob(pattern)
}

func readTaskFile(phase, stream, plan string) (string, error) {
	path := taskFileForRef(phase, stream, plan)
	//#nosec G304 -- fixed repo path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no task list at %s (create one with `sdt context task add \"<step>\" %s`)", path, planFlagForHint(plan))
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

// taskTarget resolves the `context task` family target: --phase <n> is
// optional (when present it targets a plan phase section; when absent the
// item lands in the file's unphased checklist), --plan defaults to
// latestActivePlan() and an explicit `--plan <custom-slug>` enables standalone
// checklists.

func taskTarget(cmd *cobra.Command) (phase, plan string, err error) {
	phase = sanitizeSlug(getStringFlag(cmd, "phase", false))
	plan = getStringFlag(cmd, "plan", false)
	if plan == "" {
		plan = latestActivePlan()
		if plan == "" {
			return "", "", errors.New("no active plan found; pass --plan <slug> to create a standalone checklist")
		}
	}
	return phase, plan, nil
}

// taskStreamFlag reads --stream, the optional split-file label.

func taskStreamFlag(cmd *cobra.Command) string {
	return sanitizeSlug(getStringFlag(cmd, "stream", false))
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
	Short: "Show a task list (optionally one phase section)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		content, err := readTaskFile(phase, taskStreamFlag(cmd), plan)
		exitWithError(cmd, err)
		outputTaskItems(cmd, taskItemsInSection(content, phase))
	},
}

// buildTaskFrontmatter emits a whole-plan task checklist header matching the
// tasks.md convention (kind/summary/status/created/updated/links/sources/
// project). A whole-plan file carries no scalar `phase`; split files may pass
// a non-empty `phases` list. The task inherits the plan's objective, so it
// never writes an `objective` field. links/sources reference the plan only
// when it is an existing real plan file (standalone custom slugs get no plan
// reference).

func buildTaskFrontmatter(project, summary, planRef string, phases []string) string {
	now := contextNow().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: tasks\n")
	b.WriteString("uid: " + newUID() + "\n")
	b.WriteString("summary: " + yamlScalar(summary) + "\n")
	if len(phases) > 0 {
		b.WriteString("phases: [" + strings.Join(phases, ", ") + "]\n")
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
	// A file with no resolvable plan gets no typed parent, so it declares the
	// standalone decision in its body: the reconciler reports an undeclared
	// orphan as a WARNING, and the CLI must not scaffold a file that is born
	// unexplained.
	if planRef == "" || !planHasFile(planRef) {
		reason := "no parent plan was named"
		if planRef != "" {
			reason = "`--plan " + planRef + "` is not a plan document"
		}
		b.WriteString(ctxStandaloneMarker + " " + reason +
			", so this file records its own lifecycle.\n\n")
	}
	return b.String()
}

// appendTaskStep appends a checklist item to `content`, placing it in the
// `## Phase <phase>` section (creating the section when absent) or, with an
// empty phase, at the end of the file just before a trailing `## Review`
// block. Non-empty phases keep each phase's items together in one file.

func appendTaskStep(content, phase, step string) string {
	item := "- [ ] " + step
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if phase == "" {
		at := firstHeadingIndex(lines)
		if at < 0 {
			lines = insertLines(lines, len(lines), item)
		} else {
			lines = insertLines(lines, at, item, "")
		}
		return strings.Join(lines, "\n") + "\n"
	}
	if hi := phaseHeadingIndex(lines, phase); hi >= 0 {
		lines = insertLines(lines, taskSectionEnd(lines, hi+1), item)
		return strings.Join(lines, "\n") + "\n"
	}
	at := reviewHeadingIndex(lines)
	lines = insertLines(lines, at, "", "## Phase "+phase, "", item)
	return strings.Join(lines, "\n") + "\n"
}

// phaseHeadingIndex returns the line index of the `## Phase <phase>` heading,
// or -1. Matching is case-insensitive and token-exact on the phase label.

func phaseHeadingIndex(lines []string, phase string) int {
	want := strings.ToLower("## phase " + phase)
	for i, line := range lines {
		t := strings.ToLower(strings.TrimSpace(line))
		if t == want || strings.HasPrefix(t, want+" ") || strings.HasPrefix(t, want+"\t") {
			return i
		}
	}
	return -1
}

// reviewHeadingIndex returns the line index of a `## Review` heading, or
// len(lines) when absent (so insertLines appends at the end).

func reviewHeadingIndex(lines []string) int {
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "## Review") {
			return i
		}
	}
	return len(lines)
}

// taskSectionEnd returns the insertion index for a section that starts at
// `from`: the next `## ` heading, backed up over trailing blank lines (or
// len(lines) when the section runs to the end).

func taskSectionEnd(lines []string, from int) int {
	end := len(lines)
	for i := from; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	for end > from && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

// firstHeadingIndex returns the line index of the first `## ` heading, or -1
// when the body has none (the unphased checklist then runs to the end).

func firstHeadingIndex(lines []string) int {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			return i
		}
	}
	return -1
}

// insertLines inserts `add` at index `at` (clamped to the slice bounds).

func insertLines(lines []string, at int, add ...string) []string {
	if at < 0 {
		at = 0
	}
	if at > len(lines) {
		at = len(lines)
	}
	out := make([]string, 0, len(lines)+len(add))
	out = append(out, lines[:at]...)
	out = append(out, add...)
	out = append(out, lines[at:]...)
	return out
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
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		stream := taskStreamFlag(cmd)
		path := taskFileForRef(phase, stream, plan)
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
				if phase != "" {
					summary = "Task checklist for phase " + phase
				} else {
					summary = "Task checklist for " + taskSlugFromPlan(plan)
				}
			}
			content = buildTaskFrontmatter(project, summary, plan, nil)
		} else {
			exitWithError(cmd, err)
		}
		content = strings.TrimRight(content, "\n") + "\n"
		content = appendTaskStep(content, phase, step)
		// Lazy identifier stamping (uid + checklist anchor), then the typed
		// parent relation (task→plan).
		if stamped, ok := stampUIDMissing(content); ok {
			content = stamped
		}
		if stamped, ok := stampChecklistIDs(content); ok {
			content = stamped
		}
		planPath, taskUID := "", ""
		if planHasFile(plan) {
			planPath = filepath.Join(sdtPlanDir, plan)
			if planUID := ctxDocUID(planPath); planUID != "" {
				content, _ = stampChildParent(content, ctxTypeTasks, planUID)
				taskUID = parseFrontmatterField(content, ctxFrontmatterUID)
			}
		}
		if err := os.MkdirAll(sdtTasksDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		if planPath != "" && taskUID != "" {
			if err := linkChildToParent(planPath, taskUID, "tasks_ids"); err != nil {
				exitWithError(cmd, err)
			}
		}
		items := parseTaskItems(content)
		last := items[len(items)-1]
		label := last.ID
		if label == "" {
			label = strconv.Itoa(last.Line)
		}
		outputString(cmd, label+"\n")
		cascadeAfterWrite(cmd, path)
	},
}

// setTaskFileStatus rewrites the frontmatter `status` and refreshes `updated`,
// returning the new content. setTaskFileStatus is a no-op on files without a
// status: line (legacy checklists).

func setTaskFileStatus(content, status string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return content
	}
	changed := false
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == ctxFrontmatterDelim {
			break
		}
		if strings.HasPrefix(line, "status:") {
			lines[i] = "status: " + status
			changed = true
		} else if strings.HasPrefix(line, "status ") {
			// Recover the malformed legacy shape instead of silently preserving
			// it when a task file status changes.
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

// hasUnfinishedTaskItem reports whether any phase item is still open. Only the
// work part of the file counts: `## Deviations` and `## Review` are file-level
// records the CLI writes, and their checklist items (an open deviation, a
// finding) are traces rather than work, so neither may keep a file from
// completing.
func hasUnfinishedTaskItem(content string) bool {
	for _, line := range strings.Split(taskWorkContent(content), "\n") {
		m := ctxTaskLineRegexp.FindStringSubmatch(line)
		if m != nil && m[1] != "x" && m[1] != "!" {
			return true
		}
	}
	return false
}

// taskWorkContent returns the work part of a task file: everything before the
// file-level record sections `## Deviations` and `## Review`. Every scan that
// asks "what work is left in this file" — unfinished items, per-section item
// counts, the derived status behind the cascade checks — goes through here, so a
// record section is never mistaken for a phase. `## Review` is ignored too, even
// though it predates the deviations section: once `### Findings` put checklist
// items under it, a whole-document scan started seeing verdicts as open work.
func taskWorkContent(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if t := strings.TrimSpace(line); t == ctxDeviationsHeader || t == ctxReviewBlockHeader {
			return strings.Join(lines[:i], "\n")
		}
	}
	return content
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
			if _, ok := parseChecklistID(args[0]); !ok {
				if _, err := strconv.Atoi(strings.TrimSpace(args[0])); err != nil {
					exitWithError(cmd, fmt.Errorf("invalid task id %q", args[0]))
				}
			}
			reason := ""
			if status == taskStatusBlock {
				reason = getStringFlag(cmd, "reason", false)
			}
			phase, plan, err := taskTarget(cmd)
			exitWithError(cmd, err)
			stream := taskStreamFlag(cmd)
			content, err := readTaskFile(phase, stream, plan)
			exitWithError(cmd, err)
			updated, err := updateChecklistItem(content, args[0], status, reason)
			if err != nil {
				exitWithError(cmd, err)
				return
			}
			updated = setTaskFileStatus(updated, taskFileNextStatus(status, updated))
			path := taskFileForRef(phase, stream, plan)
			//#nosec G306 -- user work file
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				exitWithError(cmd, err)
			}
			outputString(cmd, "ok\n")
			cascadeAfterWrite(cmd, path)
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
self-approve. Use --input/--file/piped stdin for the findings prose.

Record per-finding verdicts with --finding and --verdict: the two flags pair by
position and are repeatable, and each pair becomes one item under the block's
` + "`### Findings`" + ` subsection with a CLI-assigned id. Re-running the same pair is
a no-op, so a phase's finding identity is stable; a claim re-stated with a
different verdict is a new finding and gets a new id.

Examples:
  sdt context task review --phase 1 --plan plan.md --input "all gates green (CONFIRMED)"
  sdt context task review --phase 1 --plan plan.md \
    --finding "vet is clean" --verdict CONFIRMED \
    --finding "coverage target met" --verdict UNVERIFIED`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase, plan, err := taskTarget(cmd)
		exitWithError(cmd, err)
		stream := taskStreamFlag(cmd)
		path := taskFileForRef(phase, stream, plan)
		content, err := readTaskFile(phase, stream, plan)
		exitWithError(cmd, err)
		findings, err := reviewFindings(cmd)
		exitWithError(cmd, err)
		body := getContextBody(cmd, args)
		content = appendReviewFindings(content, body)
		content = appendFindingItems(content, findings)
		if !hasUnfinishedTaskItem(content) {
			content = setTaskFileStatus(content, taskFileStatusCompleted)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(strings.TrimRight(content, "\n")+"\n"), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, path+"\n")
		cascadeAfterWrite(cmd, path)
	},
}

func reviewFindings(cmd *cobra.Command) ([]reviewFinding, error) {
	claims := getStringArrayFlag(cmd, "finding", false)
	verdicts := getStringArrayFlag(cmd, "verdict", false)
	if len(claims) == 0 && len(verdicts) == 0 {
		return nil, nil
	}
	if len(claims) == 0 {
		return nil, errors.New("--verdict given without --finding; each finding needs a claim")
	}
	if len(claims) != len(verdicts) {
		return nil, fmt.Errorf("%d --finding claim(s) but %d --verdict value(s); the flags pair by position", len(claims), len(verdicts))
	}
	findings := make([]reviewFinding, 0, len(claims))
	for i, claim := range claims {
		claim = strings.TrimSpace(claim)
		if claim == "" {
			return nil, fmt.Errorf("--finding %d is empty; a finding must name the claim under review", i+1)
		}
		verdict := strings.TrimSpace(verdicts[i])
		if !validReviewVerdict(verdict) {
			return nil, fmt.Errorf("--verdict %q is not a verify-step verdict; want one of %s", verdict, ctxReviewVerdictHelp)
		}
		findings = append(findings, reviewFinding{Claim: claim, Verdict: verdict})
	}
	return findings, nil
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
		if strings.HasPrefix(line, key+" ") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+" "))
		}
	}
	return ""
}

var contextTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage plan task checklists",
	Long: `Manage plan-scoped task checklists in
context/tasks/<YYYYMMDD-HHMMSS>-<slug-plan>.md (one file per plan by default,
with one ` + "`## Phase <n>`" + ` section per phase; a legacy
<...>-<slug-plan>-phase-<n>.md file still wins, and --stream <label> selects an
opt-in split file <...>-<slug-plan>-<label>.md). --phase <n> targets a phase
section and is optional; --plan defaults to the latest active plan (or an
explicit --plan <plan-file> / --plan <slug> for a standalone checklist).

  sdt context task list [--phase <n>] [--stream <label>] [--plan <ref>]   show steps with ids
  sdt context task add "<step>" [--phase <n>] [--stream <label>] [--plan <ref>] [--summary]
  sdt context task done|block|wip <id> [--phase <n>] [--stream <label>] [--plan <ref>]
  sdt context task review [--phase <n>] [--stream <label>] [--plan <ref>]  record verdicts + complete

Status markers: [ ] todo · [~] in-progress · [x] done · [!] blocked`,
}

// ── context group ──────────────────────────────────────────────────────────────

var contextTaskDoneCmd = taskSetStatusCmd("done")

var contextTaskBlockCmd = taskSetStatusCmd("block")

var contextTaskWipCmd = taskSetStatusCmd("wip")
