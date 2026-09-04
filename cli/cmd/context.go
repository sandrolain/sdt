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
	ctxTypePlan     = "plan"
	ctxTypeAnalysis = "analysis"
	ctxTypeWorklog  = "worklog"
	ctxTypeNotes    = "notes"
	ctxTypeTasks    = "tasks"
	ctxTypeTmp      = "tmp"
	ctxTypeArchive  = "archive"
)

var contextNow = time.Now

var contextRunEditor = contextRunEditorDefault

var ctxSlugRegexp = regexp.MustCompile(`[^a-z0-9-]+`)
var ctxTaskLineRegexp = regexp.MustCompile(`^- \[([ x~!])\] (.*)$`)

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
func contextPath(typ, slug string) (string, error) {
	switch typ {
	case ctxTypePlan:
		return filepath.Join(sdtPlanDir, contextTimePrefix("2006-01-02", slug)+".md"), nil
	case ctxTypeAnalysis:
		return filepath.Join(sdtAnalysisDir, contextTimePrefix("2006-01-02", slug)+".md"), nil
	case ctxTypeWorklog:
		return filepath.Join(sdtWorklogDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeNotes:
		return filepath.Join(sdtNotesDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	case ctxTypeTasks:
		return filepath.Join(sdtTasksDir, "TODO.md"), nil
	case ctxTypeTmp:
		if slug == "" {
			return "", errors.New("--slug is required for type tmp")
		}
		return filepath.Join(sdtTmpDir, slug), nil
	case ctxTypeArchive:
		return filepath.Join(sdtArchiveDir, contextTimePrefix("20060102-150405", slug)+".md"), nil
	}
	return "", fmt.Errorf("unknown type %q (use plan|analysis|worklog|notes|tasks|tmp|archive)", typ)
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
	Short: "Print the path for a sdt.context/ work file",
	Long: `Print the full path of a sdt.context/ work file with the correct date/time
prefix. Does not create anything.

Types: plan/analysis (<YYYY-MM-DD>-<slug>.md), worklog/notes (<YYYYMMDD-HHMMSS>-<slug>.md),
tasks (TODO.md), tmp (<slug>), archive (<YYYYMMDD-HHMMSS>-<slug>.md).

Examples:
  sdt context path --type worklog --slug review-deps
  sdt context path --type plan --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		p, err := contextPath(typ, slug)
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
	Path      string `json:"path" yaml:"path"`
	Type      string `json:"type" yaml:"type"`
	CreatedAt string `json:"created_at" yaml:"created_at"`
	Project   string `json:"project,omitempty" yaml:"project,omitempty"`
	Status    string `json:"status" yaml:"status"`
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

func contextFrontmatter(typ, note, project, created string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: " + typ + "\n")
	b.WriteString("created_at: " + created + "\n")
	if note != "" {
		b.WriteString("context: " + note + "\n")
	}
	if project != "" {
		b.WriteString("project: " + project + "\n")
	}
	b.WriteString("---\n")
	return b.String()
}

var contextNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a sdt.context/ work file with frontmatter",
	Long: `Create a plan, worklog, notes or analysis file under sdt.context/ with the
correct naming and YAML frontmatter (kind, created_at, context, project). The body
comes from --input/--file or piped stdin. Existing files are preserved unless
--force is set; --edit opens the file in $EDITOR after creation.

Examples:
  sdt context new --type worklog --slug review-deps --input "reviewed deps"
  sdt context new --type plan --slug ship-memory --force
  sdt context new --type analysis --slug memory-backend --input "..."`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		if typ != ctxTypePlan && typ != ctxTypeAnalysis && typ != ctxTypeWorklog && typ != ctxTypeNotes {
			exitWithError(cmd, fmt.Errorf("new supports type plan|analysis|worklog|notes, got %q", typ))
		}
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		note := getStringFlag(cmd, "context", false)
		force := getBoolFlag(cmd, "force", false)
		edit := getBoolFlag(cmd, "edit", false)
		body := getContextBody(cmd, args)
		created := contextNow().UTC().Format(time.RFC3339)

		project := ""
		if cfg, err := findProjectConfig(); err == nil && cfg != nil {
			project = cfg.Project
		}

		path, err := contextPath(typ, slug)
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

		content := contextFrontmatter(typ, note, project, created)
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
			Path:      path,
			Type:      typ,
			CreatedAt: created,
			Project:   project,
			Status:    status,
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
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

var contextListCmd = &cobra.Command{
	Use:   useList,
	Short: "List sdt.context/ work files",
	Long: `List existing work files under sdt.context/ for a type, sorted by name
(chronological for timestamped files).

Types: plan, analysis, worklog, notes, tasks, archive.

Examples:
  sdt context list --type worklog
  sdt context list --type archive --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typ := getStringFlag(cmd, "type", true)
		dir, ok := contextDir(typ)
		if !ok || typ == ctxTypeTmp {
			exitWithError(cmd, fmt.Errorf("list supports type plan|analysis|worklog|notes|tasks|archive, got %q", typ))
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

func readTaskFile() (string, error) {
	//#nosec G304 -- fixed repo path
	data, err := os.ReadFile(sdtTasksTODO)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no active task list at %s (create one with `sdt context task add`)", sdtTasksTODO)
		}
		return "", err
	}
	return string(data), nil
}

var contextTaskListCmd = &cobra.Command{
	Use:   useList,
	Short: "Show the active task list",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := readTaskFile()
		exitWithError(cmd, err)
		outputTaskItems(cmd, parseTaskItems(content))
	},
}

func buildTaskFrontmatter(objective, project string) string {
	now := contextNow().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("kind: tasks\n")
	b.WriteString("created_at: " + now + "\n")
	if objective != "" {
		b.WriteString("objective: " + objective + "\n")
	}
	if project != "" {
		b.WriteString("project: " + project + "\n")
	}
	b.WriteString("---\n\n")
	return b.String()
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
		content := ""
		//#nosec G304 -- fixed repo path
		if data, err := os.ReadFile(sdtTasksTODO); err == nil {
			content = string(data)
		} else if os.IsNotExist(err) {
			project := ""
			if cfg, cerr := findProjectConfig(); cerr == nil && cfg != nil {
				project = cfg.Project
			}
			content = buildTaskFrontmatter(objective, project)
		} else {
			exitWithError(cmd, err)
		}
		content = strings.TrimRight(content, "\n") + "\n"
		content += "- [ ] " + step + "\n"
		if err := os.MkdirAll(sdtTasksDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(sdtTasksTODO, []byte(content), 0o644); err != nil {
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
			content, err := readTaskFile()
			exitWithError(cmd, err)
			updated, err := updateTaskStatus(content, id, status, reason)
			exitWithError(cmd, err)
			//#nosec G306 -- user work file
			if err := os.WriteFile(sdtTasksTODO, []byte(updated), 0o644); err != nil {
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
	Short: "Archive the active task list to sdt.context/archive/",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := readTaskFile()
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
		if err := os.Remove(sdtTasksTODO); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, path+"\n")
	},
}

var contextTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage the active task list (sdt.context/tasks/TODO.md)",
	Long: `Manage the active task list in sdt.context/tasks/TODO.md.

  sdt context task list                        show steps with ids
  sdt context task add "<step>" [--objective]  add a step (creates the list)
  sdt context task done|block|wip <id>         update a step status
  sdt context task archive [--slug]            archive the list and start fresh

Status markers: [ ] todo · [~] in-progress · [x] done · [!] blocked`,
}

// ── context group ──────────────────────────────────────────────────────────────

var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Context Tools (sdt.context/ work files)",
	Long: `Manage the agent working files under sdt.context/: plans, work logs, notes
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
	contextPathCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|tasks|tmp|archive")
	contextPathCmd.Flags().String("slug", "", "Slug (sanitized)")

	contextNewCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes")
	contextNewCmd.Flags().String("slug", "", "Slug (sanitized)")
	contextNewCmd.Flags().String("context", "", "What triggered this entry")
	contextNewCmd.Flags().Bool("force", false, "Overwrite existing file")
	contextNewCmd.Flags().Bool("edit", false, "Open the file in $EDITOR after creation")

	contextListCmd.Flags().String("type", "", "Type: plan|analysis|worklog|notes|tasks|archive")

	contextTaskAddCmd.Flags().String("objective", "", "Objective for the task list (used when creating)")
	contextTaskBlockCmd.Flags().String("reason", "", "Reason for blocking")
	contextTaskArchiveCmd.Flags().String("slug", "", "Archive slug (default: from objective)")

	contextTaskCmd.AddCommand(contextTaskListCmd, contextTaskAddCmd, contextTaskDoneCmd, contextTaskBlockCmd, contextTaskWipCmd, contextTaskArchiveCmd)
	contextCmd.AddCommand(contextPathCmd, contextNewCmd, contextListCmd, contextTaskCmd)
	rootCmd.AddCommand(contextCmd)
}
