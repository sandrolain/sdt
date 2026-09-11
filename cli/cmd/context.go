package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
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
	ctxTypeAdr          = "adr"
	ctxTypeQuestions    = "questions"
)

var contextNow = time.Now

var contextRunEditor = contextRunEditorDefault

var ctxSlugRegexp = regexp.MustCompile(`[^a-z0-9-]+`)
var ctxTaskLineRegexp = regexp.MustCompile(`^- \[([ x~!])\] (.*)$`)

// ctxSummaryPlaceholder is emitted as the `summary` value when --summary is
// omitted, so the generated file is lint-parseable and the agent knows to fill
// it in.
const ctxSummaryPlaceholder = "<one-line summary — MANDATORY, fill in>"

// ctxDefaultStatus maps a bootstrappable type to its initial `status`. Types
// without an entry do not carry a `status` field (worklog/notes).
var ctxDefaultStatus = map[string]string{
	ctxTypePlan:         ctxWikiStatusActive,
	ctxTypeAnalysis:     ctxWikiStatusActive,
	ctxTypeQuestions:    ctxWikiStatusActive,
	ctxTypeArchitecture: "draft",
}

// ctxHasUpdated lists types whose instruction contract requires an `updated`
// field (notes does not).
var ctxHasUpdated = map[string]bool{
	ctxTypePlan:         true,
	ctxTypeAnalysis:     true,
	ctxTypeWorklog:      true,
	ctxTypeQuestions:    true,
	ctxTypeArchitecture: true,
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
// filesystem. It is deterministic for a given time and slug.
func contextPath(typ, slug, phase string) (string, error) {
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
		return taskFileFor(phase), nil
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
		return "", errors.New("decision type is append-only ADR; create via `sdt context new --type analysis` or edit decisions/")
	case ctxTypeQuestions:
		return filepath.Join(sdtQuestionsDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	}
	return "", fmt.Errorf("unknown type %q (use plan|analysis|worklog|notes|tasks|tmp|archive|architecture|decisions|questions)", typ)
}

// ── context path ───────────────────────────────────────────────────────────────

type contextPathResult struct {
	Path string `json:"path" yaml:"path"`
	Type string `json:"type" yaml:"type"`
	Slug string `json:"slug,omitempty" yaml:"slug,omitempty"`
}

func outputContextPath(cmd *cobra.Command, res contextPathResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, res.Path+"\n")
	}
}

var contextPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path for a context/ work file",
	Long: `Print the full path of a context/ work file with the correct date/time
prefix. Does not create anything.

Types: plan/analysis/worklog/notes/archive (<YYYYMMDD-HHMMSS>-<slug>.md),
tasks (<phase>.md with --phase, default plan), tmp (<slug>),
architecture (<slug>.md).

Examples:
  sdt context path --type worklog --slug review-deps
  sdt context path --type tasks --phase execution
  sdt context path --type plan --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		phase := getStringFlag(cmd, "phase", false)
		p, err := contextPath(typ, slug, phase)
		exitWithError(cmd, err)
		outputContextPath(cmd, contextPathResult{Path: p, Type: typ, Slug: slug})
	},
}

// ── context new ────────────────────────────────────────────────────────────────

// getContextBody returns the file body from --input/--file/--inb64 or piped
// stdin. On a terminal with no input flags it returns "" without blocking.
func getContextBody(cmd *cobra.Command, args []string) string {
	f := cmd.Flags()
	if f.Lookup("input").Changed || f.Lookup("file").Changed || f.Lookup("inb64").Changed {
		return getInputString(cmd, args)
	}
	if stdinIsTTY() {
		return ""
	}
	return getInputString(cmd, args)
}

type contextNewResult struct {
	Path    string `json:"path" yaml:"path"`
	Type    string `json:"type" yaml:"type"`
	Created string `json:"created" yaml:"created"`
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	Status  string `json:"status" yaml:"status"`
}

func outputContextNew(cmd *cobra.Command, res contextNewResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, res.Path+"\n")
	}
}

// yamlScalar quotes s when the plain form would not parse as a valid YAML
// block scalar (e.g. contains ": ", " #", leading/trailing space, newline, or
// starts with a special indicator). Plain form is kept for ordinary values so
// generated frontmatter stays readable.
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	special := strings.ContainsAny(s, ":#\n\t")
	indicatorPrefix := strings.HasPrefix(s, "-") || strings.HasPrefix(s, "?") ||
		strings.HasPrefix(s, "[") || strings.HasPrefix(s, "]") ||
		strings.HasPrefix(s, "{") || strings.HasPrefix(s, "}") ||
		strings.HasPrefix(s, ",") || strings.HasPrefix(s, "&") ||
		strings.HasPrefix(s, "*") || strings.HasPrefix(s, "!") ||
		strings.HasPrefix(s, "|") || strings.HasPrefix(s, ">") ||
		strings.HasPrefix(s, "%") || strings.HasPrefix(s, "@") ||
		strings.HasPrefix(s, "`") || strings.HasPrefix(s, `"`) ||
		strings.HasPrefix(s, "'")
	if !special && !indicatorPrefix && s == strings.TrimSpace(s) {
		return s
	}
	return strconv.Quote(s)
}

// contextFrontmatter builds the YAML frontmatter for a freshly created
// context/ work file, matching the per-type instruction contracts
// (context/instructions/*.md). summary falls back to ctxSummaryPlaceholder so
// the file stays lint-parseable; title/component are emitted only where the
// type requires them and the value is non-empty.
func contextFrontmatter(typ, title, summary, note, project, component, created string) string {
	if summary == "" {
		summary = ctxSummaryPlaceholder
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: " + typ + "\n")
	if typ == ctxTypeAnalysis && title != "" {
		b.WriteString("title: " + yamlScalar(title) + "\n")
	}
	b.WriteString("summary: " + yamlScalar(summary) + "\n")
	if note != "" {
		b.WriteString("context: " + yamlScalar(note) + "\n")
	}
	if st, ok := ctxDefaultStatus[typ]; ok {
		b.WriteString("status: " + st + "\n")
	}
	if typ == ctxTypeArchitecture && component != "" {
		b.WriteString("component: " + yamlScalar(component) + "\n")
	}
	b.WriteString("created: " + created + "\n")
	if ctxHasUpdated[typ] {
		b.WriteString("updated: " + created + "\n")
	}
	if project != "" {
		b.WriteString("project: " + yamlScalar(project) + "\n")
	}
	b.WriteString("---\n")
	return b.String()
}

var contextNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a context/ work file with frontmatter",
	Long: `Create a plan, analysis, worklog, notes, questions or architecture
file under context/ with the correct naming and the full per-type YAML
frontmatter (kind, summary, context, status, created, updated, project plus
per-type fields). The body comes from --input/--file or piped stdin. Existing
files are preserved unless --force is set; --edit opens the file in $EDITOR
after creation.

The slug is derived from --title when --slug is omitted; --summary is optional
and falls back to a MANDATORY-fill placeholder so the file passes lint. The
command prints the created file path (--format text|json|yaml).

Examples:
  sdt context new --type worklog --title "review deps" --input "reviewed deps"
  sdt context new --type plan --title "ship memory" --force
  sdt context new --type analysis --title "memory backend" --input "..."
  sdt context new --type architecture --title "config loading" --summary "config loading component"
  sdt context new --type questions --title "open api questions"`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		switch typ {
		case ctxTypePlan, ctxTypeAnalysis, ctxTypeWorklog, ctxTypeNotes, ctxTypeQuestions, ctxTypeArchitecture:
		default:
			exitWithError(cmd, fmt.Errorf("new supports type plan|analysis|worklog|notes|questions|architecture, got %q", typ))
		}
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		title := getStringFlag(cmd, "title", false)
		if slug == "" && title != "" {
			slug = sanitizeSlug(title)
		}
		if typ == ctxTypeArchitecture && slug == "" {
			exitWithError(cmd, errors.New("--title or --slug is required for type architecture"))
		}
		note := getStringFlag(cmd, "context", false)
		summary := getStringFlag(cmd, "summary", false)
		force := getBoolFlag(cmd, "force", false)
		edit := getBoolFlag(cmd, "edit", false)
		body := getContextBody(cmd, args)
		created := contextNow().UTC().Format(time.RFC3339)

		project := ""
		if cfg, err := findProjectConfig(); err == nil && cfg != nil {
			project = cfg.Project
		}

		path, err := contextPath(typ, slug, "")
		exitWithError(cmd, err)

		status := statusCreated
		if _, err := os.Stat(path); err == nil {
			if !force {
				exitWithError(cmd, fmt.Errorf("%s already exists (use --force to overwrite)", path))
			}
			status = statusUpdated
		} else if !os.IsNotExist(err) {
			exitWithError(cmd, err)
		}

		component := ""
		if typ == ctxTypeArchitecture {
			component = slug
		}

		content := contextFrontmatter(typ, title, summary, note, project, component, created)
		if body != "" {
			content += "\n" + strings.TrimRight(body, "\n") + "\n"
		}

		dir, _ := filepath.Split(path)
		if err := os.MkdirAll(dir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}

		if edit {
			if err := contextRunEditor(path); err != nil {
				exitWithError(cmd, err)
			}
		}

		outputContextNew(cmd, contextNewResult{
			Path:    path,
			Type:    typ,
			Created: created,
			Project: project,
			Status:  status,
		})
	},
}

// ── context list ───────────────────────────────────────────────────────────────

func outputStringList(cmd *cobra.Command, items []string) {
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
		for _, it := range items {
			outputString(cmd, it+"\n")
		}
	}
}

func listContextFiles(dir string) ([]string, error) {
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

var contextListCmd = &cobra.Command{
	Use:   useList,
	Short: "List context/ work files",
	Long: `List existing work files under context/ for a type, sorted by name
(chronological for timestamped files).

Types: plan, analysis, worklog, notes, tasks, archive, architecture, decisions.

Examples:
  sdt context list --type worklog
  sdt context list --type decisions --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		switch typ {
		case "decisions", ctxTypeAdr:
			typ = ctxTypeDecision
		}
		dir, ok := contextDir(typ)
		if !ok || typ == ctxTypeTmp {
			exitWithError(cmd, fmt.Errorf("list supports type plan|analysis|worklog|notes|tasks|archive|architecture|decisions, got %q", typ))
		}
		files, err := listContextFiles(dir)
		exitWithError(cmd, err)
		outputStringList(cmd, files)
	},
}

// ── context task ───────────────────────────────────────────────────────────────

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

func taskFileFor(phase string) string {
	name := sanitizeSlug(phase)
	if name == "" {
		name = "plan"
	}
	return filepath.Join(sdtTasksDir, name+".md")
}

func readTaskFile(phase string) (string, error) {
	path := taskFileFor(phase)
	//#nosec G304 -- fixed repo path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no task list at %s (create one with `sdt context task add --phase %s`)", path, phase)
		}
		return "", err
	}
	return string(data), nil
}

var contextTaskListCmd = &cobra.Command{
	Use:   useList,
	Short: "Show a per-phase task list",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		phase := getStringFlag(cmd, "phase", false)
		content, err := readTaskFile(phase)
		exitWithError(cmd, err)
		outputTaskItems(cmd, parseTaskItems(content))
	},
}

// buildTaskFrontmatter emits a task checklist header matching the tasks.md
// convention (kind/summary/objective/status/created/updated/links/sources/
// project) so `sdt context task add` output passes lint and the index.
func buildTaskFrontmatter(objective, project, phase, summary, planRef string) string {
	now := contextNow().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: tasks\n")
	b.WriteString("summary: " + yamlScalar(summary) + "\n")
	if objective != "" {
		b.WriteString("objective: " + yamlScalar(objective) + "\n")
	}
	b.WriteString("status: active\n")
	b.WriteString("created: " + now + "\n")
	b.WriteString("updated: " + now + "\n")
	if planRef != "" {
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
		phase := getStringFlag(cmd, "phase", false)
		path := taskFileFor(phase)
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
			planRef := sanitizeSlug(getStringFlag(cmd, "plan", false))
			if planRef == "" {
				planRef = latestActivePlan()
			}
			content = buildTaskFrontmatter(objective, project, phase, summary, planRef)
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
			phase := getStringFlag(cmd, "phase", false)
			content, err := readTaskFile(phase)
			exitWithError(cmd, err)
			updated, err := updateTaskStatus(content, id, status, reason)
			exitWithError(cmd, err)
			//#nosec G306 -- user work file
			if err := os.WriteFile(taskFileFor(phase), []byte(updated), 0o644); err != nil {
				exitWithError(cmd, err)
			}
			outputString(cmd, "ok\n")
		},
	}
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
		phase := getStringFlag(cmd, "phase", false)
		content, err := readTaskFile(phase)
		exitWithError(cmd, err)
		slug := taskArchiveSlug(content, getStringFlag(cmd, "slug", false))
		path := filepath.Join(sdtArchiveDir, contextTimePrefix("20060102-150405", slug)+".md")
		if err := os.MkdirAll(sdtArchiveDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
		}
		if err := os.Remove(taskFileFor(phase)); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, path+"\n")
	},
}

var contextTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage per-phase task checklists (context/tasks/<phase>.md)",
	Long: `Manage per-phase task checklists in context/tasks/<phase>.md. Each plan
phase gets its own checklist file; --phase defaults to "plan".

  sdt context task list [--phase <phase>]             show steps with ids
  sdt context task add "<step>" [--phase] [--objective] [--summary] [--plan]  add a step (creates the list)
  sdt context task done|block|wip <id> [--phase]      update a step status
  sdt context task archive [--phase] [--slug]         archive the list and start fresh

Status markers: [ ] todo · [~] in-progress · [x] done · [!] blocked`,
}

// ── context group ──────────────────────────────────────────────────────────────

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

var contextTaskDoneCmd = taskSetStatusCmd("done")
var contextTaskBlockCmd = taskSetStatusCmd("block")
var contextTaskWipCmd = taskSetStatusCmd("wip")

func init() {
	contextPathCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|tasks|tmp|archive|architecture|decisions")
	contextPathCmd.Flags().String("slug", "", "Slug (sanitized)")
	contextPathCmd.Flags().String("phase", "plan", "Phase for type tasks (checklist file name)")

	contextNewCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|questions|architecture")
	contextNewCmd.Flags().String("title", "", "Title (slug derived from it when --slug omitted)")
	contextNewCmd.Flags().String("slug", "", "Slug (sanitized; overrides --title-derived slug)")
	contextNewCmd.Flags().String("summary", "", "Summary for the frontmatter (default: MANDATORY-fill placeholder)")
	contextNewCmd.Flags().String("context", "", "What triggered this entry")
	contextNewCmd.Flags().Bool("force", false, "Overwrite existing file")
	contextNewCmd.Flags().Bool("edit", false, "Open the file in $EDITOR after creation")

	contextListCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|tasks|archive")

	contextTaskAddCmd.Flags().String("objective", "", "Objective for the task list (used when creating)")
	contextTaskAddCmd.Flags().String("summary", "", "Summary for the checklist frontmatter (default: derived from phase/objective)")
	contextTaskAddCmd.Flags().String("plan", "", "Plan reference for links/sources (default: latest active plan)")
	contextTaskBlockCmd.Flags().String("reason", "", "Reason for blocking")
	contextTaskArchiveCmd.Flags().String("slug", "", "Archive slug (default: from objective)")
	contextTaskAddCmd.Flags().String("phase", "plan", "Phase for the checklist file (default plan)")
	contextTaskListCmd.Flags().String("phase", "plan", "Phase for the checklist file (default plan)")
	contextTaskDoneCmd.Flags().String("phase", "plan", "Phase for the checklist file (default plan)")
	contextTaskBlockCmd.Flags().String("phase", "plan", "Phase for the checklist file (default plan)")
	contextTaskWipCmd.Flags().String("phase", "plan", "Phase for the checklist file (default plan)")
	contextTaskArchiveCmd.Flags().String("phase", "plan", "Phase for the checklist file (default plan)")

	contextTemplateCmd.Flags().String("type", "", "Type: analysis|plan|tasks|adr|architecture|worklog|notes")

	contextTaskCmd.AddCommand(contextTaskListCmd, contextTaskAddCmd, contextTaskDoneCmd, contextTaskBlockCmd, contextTaskWipCmd, contextTaskArchiveCmd)
	contextCmd.AddCommand(contextPathCmd, contextNewCmd, contextListCmd, contextTaskCmd, contextReindexCmd, contextLintCmd, contextStatusCmd, contextTemplateCmd)
	rootCmd.AddCommand(contextCmd)
}
