package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

const (
	codeFence          = "```"
	agentTargetDefault = "AGENTS.md"
)

var (
	sectionNameRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	sectionBeginRe = regexp.MustCompile(`(?m)^[ \t]*<!-- sdt:begin:([a-z0-9][a-z0-9-]*) -->`)
)

// ── section helpers ────────────────────────────────────────────────────────────

func sectionBeginMarker(name string) string {
	return "<!-- sdt:begin:" + name + " -->"
}

func sectionEndMarker(name string) string {
	return "<!-- sdt:end:" + name + " -->"
}

// sectionBlock renders a tagged section (body is trimmed).
func sectionBlock(name, body string) string {
	body = strings.TrimSpace(body)
	return fmt.Sprintf("%s\n\n%s\n\n%s\n", sectionBeginMarker(name), body, sectionEndMarker(name))
}

func sectionRegexp(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)\n?` + regexp.QuoteMeta(sectionBeginMarker(name)) + `.*?` + regexp.QuoteMeta(sectionEndMarker(name)) + `\n?`)
}

func hasSection(content, name string) bool {
	return sectionRegexp(name).MatchString(content)
}

func validateSectionName(name string) error {
	if !sectionNameRe.MatchString(name) {
		return fmt.Errorf("invalid section name %q (use lowercase letters, digits and hyphens)", name)
	}
	return nil
}

func addSectionBlock(content, name, body string) (string, error) {
	if hasSection(content, name) {
		return "", fmt.Errorf("section %q already exists (use update or set)", name)
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	return content + sectionBlock(name, body), nil
}

func updateSectionBlock(content, name, body string) (string, error) {
	re := sectionRegexp(name)
	if !re.MatchString(content) {
		return "", fmt.Errorf("section %q not found (use add or set)", name)
	}
	return re.ReplaceAllString(content, "\n"+sectionBlock(name, body)), nil
}

func removeSectionBlock(content, name string) (string, error) {
	re := sectionRegexp(name)
	if !re.MatchString(content) {
		return "", fmt.Errorf("section %q not found", name)
	}
	return re.ReplaceAllString(content, "\n"), nil
}

func getSectionBody(content, name string) (string, bool) {
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(sectionBeginMarker(name)) + `\n(.*?)\n` + regexp.QuoteMeta(sectionEndMarker(name)))
	m := re.FindStringSubmatch(content)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

func listSections(content string) []string {
	var names []string
	for _, m := range sectionBeginRe.FindAllStringSubmatch(content, -1) {
		names = append(names, m[1])
	}
	return names
}

// ── target file helpers ────────────────────────────────────────────────────────

func agentTargetPath(cmd *cobra.Command) string {
	path := getStringFlag(cmd, "target", false)
	if path == "" {
		path = agentTargetDefault
	}
	return path
}

func agentReadTarget(cmd *cobra.Command) (string, error) {
	path := agentTargetPath(cmd)
	data, err := os.ReadFile(path) //#nosec G304 -- user-chosen target file
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s not found (create it with 'sdt agent init' first)", path)
		}
		return "", err
	}
	return string(data), nil
}

func agentWriteTarget(cmd *cobra.Command, content string) error {
	return os.WriteFile(agentTargetPath(cmd), []byte(content), 0o644) //#nosec G306 -- user-chosen output file
}

// agentSectionInput resolves section content from extra args, stdin or flags.
func agentSectionInput(cmd *cobra.Command, args []string) string {
	if len(args) > 1 {
		return strings.Join(args[1:], " ")
	}
	body := getInputString(cmd, nil)
	if strings.TrimSpace(body) == "" {
		exitWithError(cmd, fmt.Errorf("section content is empty (provide it via stdin, --input, --file or extra args)"))
	}
	return body
}

// ── agent command group ────────────────────────────────────────────────────────

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent instruction tools (AGENTS.md, sections, guides)",
	Long: `Generate and maintain agent instruction files.

  agent init       generate an opinionated AGENTS.md
  agent section    integrate/update tagged sections inside AGENTS.md
  agent guide      generate an extended skill guide

Sections are delimited by start/end tags that make them safe to update
individually without touching the rest of the file:

  <!-- sdt:begin:NAME -->
  ...content...
  <!-- sdt:end:NAME -->`,
}

// ── agent section ──────────────────────────────────────────────────────────────

var agentSectionCmd = &cobra.Command{
	Use:   "section",
	Short: "Manage tagged sections in AGENTS.md",
	Long: `Manage tagged sections inside an instruction file (default AGENTS.md).

Sections are delimited by:

  <!-- sdt:begin:NAME -->
  ...content...
  <!-- sdt:end:NAME -->

Section content is read from extra arguments, stdin, --input or --file.

Examples:
  sdt agent section list
  sdt agent section show workflow
  echo "make test" | sdt agent section add commands
  sdt agent section update commands --file commands.md
  sdt agent section remove commands`,
}

var agentSectionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tagged sections",
	Run: func(cmd *cobra.Command, args []string) {
		content, err := agentReadTarget(cmd)
		exitWithError(cmd, err)
		names := listSections(content)
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(names, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(names)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, n := range names {
				outputString(cmd, n+"\n")
			}
		}
	},
}

var agentSectionShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a tagged section content",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		exitWithError(cmd, validateSectionName(name))
		content, err := agentReadTarget(cmd)
		exitWithError(cmd, err)
		body, ok := getSectionBody(content, name)
		if !ok {
			exitWithError(cmd, fmt.Errorf("section %q not found", name))
		}
		outputString(cmd, body+"\n")
	},
}

var agentSectionAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new tagged section",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		exitWithError(cmd, validateSectionName(name))
		content, err := agentReadTarget(cmd)
		exitWithError(cmd, err)
		body := agentSectionInput(cmd, args)
		newContent, err := addSectionBlock(content, name, body)
		exitWithError(cmd, err)
		exitWithError(cmd, agentWriteTarget(cmd, newContent))
		outputString(cmd, fmt.Sprintf("added section %q to %s", name, agentTargetPath(cmd)))
	},
}

var agentSectionUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an existing tagged section",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		exitWithError(cmd, validateSectionName(name))
		content, err := agentReadTarget(cmd)
		exitWithError(cmd, err)
		body := agentSectionInput(cmd, args)
		newContent, err := updateSectionBlock(content, name, body)
		exitWithError(cmd, err)
		exitWithError(cmd, agentWriteTarget(cmd, newContent))
		outputString(cmd, fmt.Sprintf("updated section %q in %s", name, agentTargetPath(cmd)))
	},
}

var agentSectionSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Add or update a tagged section",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		exitWithError(cmd, validateSectionName(name))
		content, err := agentReadTarget(cmd)
		exitWithError(cmd, err)
		body := agentSectionInput(cmd, args)
		if hasSection(content, name) {
			newContent, uerr := updateSectionBlock(content, name, body)
			exitWithError(cmd, uerr)
			exitWithError(cmd, agentWriteTarget(cmd, newContent))
			outputString(cmd, fmt.Sprintf("updated section %q in %s", name, agentTargetPath(cmd)))
		} else {
			newContent, aerr := addSectionBlock(content, name, body)
			exitWithError(cmd, aerr)
			exitWithError(cmd, agentWriteTarget(cmd, newContent))
			outputString(cmd, fmt.Sprintf("added section %q to %s", name, agentTargetPath(cmd)))
		}
	},
}

var agentSectionRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a tagged section",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		exitWithError(cmd, validateSectionName(name))
		content, err := agentReadTarget(cmd)
		exitWithError(cmd, err)
		newContent, err := removeSectionBlock(content, name)
		exitWithError(cmd, err)
		exitWithError(cmd, agentWriteTarget(cmd, newContent))
		outputString(cmd, fmt.Sprintf("removed section %q from %s", name, agentTargetPath(cmd)))
	},
}

// ── agent init (opinionated AGENTS.md) ─────────────────────────────────────────

type agentSectionDef struct {
	name string
	body string
}

func agentInitSections(project, group string) []agentSectionDef {
	return []agentSectionDef{
		{name: agentSectionNameProject, body: agentSectionProject(project, group)},
		{name: agentSectionNameCommands, body: agentSectionCommands()},
		{name: agentSectionNameWorkflow, body: agentSectionWorkflow()},
		{name: agentSectionNameCommunication, body: agentSectionCommunication()},
		{name: agentSectionNameMemory, body: agentSectionMemory()},
		{name: agentSectionNamePlanning, body: agentSectionPlanning(project)},
		{name: agentSectionNameAnnotations, body: agentSectionAnnotations()},
		{name: agentSectionNameSelfUpdate, body: agentSectionSelfUpdate()},
	}
}

func agentSectionProject(project, group string) string {
	var b strings.Builder
	b.WriteString("## Project\n\n")
	if project != "" {
		fmt.Fprintf(&b, "- Project: %s\n", project)
	}
	if group != "" {
		fmt.Fprintf(&b, "- Group: %s\n", group)
	}
	b.WriteString(`
This project is managed with **SDT** (Smart Developer Tools), a CLI toolset for
AI agents. Every command is deterministic, composable and has machine-readable
output.

Discover all capabilities:
`)
	b.WriteString(codeFence)
	b.WriteString(`
sdt manifest --format json
sdt schema --command "<command>"
`)
	b.WriteString(codeFence)

	if project != "" {
		b.WriteString("\n## Project Configuration\n\n")
		b.WriteString("SDT project identity lives in `.sdt.yaml`:\n\n")
		b.WriteString(codeFence + "yaml\n")
		fmt.Fprintf(&b, "project: %s\n", project)
		if group != "" {
			fmt.Fprintf(&b, "group: %s\n", group)
		}
		b.WriteString(codeFence)
	}
	return b.String()
}

func agentSectionCommands() string {
	return `## Build, Test and Lint

Document the exact commands an agent must run to build, test, lint and format
this project. Keep this section minimal and accurate.

` + codeFence + `
# replace with the real commands for this project
make build
make test
make lint
` + codeFence + `

Whenever these commands change, update this section (see Self-Update below).
`
}

func agentSectionWorkflow() string {
	return `## Agent Workflow (opinionated)

Follow this loop for any non-trivial task:

1. **Plan** — write a short plan in ` + "`sdt.context/plan/`" + ` before starting (see Planning and Work Logs).
2. **Investigate** — read this AGENTS.md, search memory (` + "`sdt memory search`" + `) and inspect the code before changing anything.
3. **Act** — make the smallest change that satisfies the task.
4. **Verify** — run the project's build, test and lint commands.
5. **Annotate** — append a ` + "`sdt.context/worklog/`" + ` entry describing what changed and why.
6. **Remember** — store durable decisions in ` + "`sdt memory`" + `.
7. **Update** — keep this AGENTS.md current when conventions change.

Use ` + "`sdt.context/tmp/`" + ` for any temporary or scratch files. Never write
or execute temporary files outside the project (e.g. do not use ` + "`/tmp`" + `).
`
}

func agentSectionCommunication() string {
	return `## Communication (default)

Code work: terse caveman ultra. Drop articles/filler/pleasantries/hedging.
Fragments OK, short synonyms, technical terms exact, code unchanged.
Pattern: ` + "`[thing] [action] [reason]. [next step]`" + `.
Not: "Sure! I'd be happy to help you with that."
Yes: "Bug in auth middleware. Fix:"
Code only — user-requested docs written normal (concise).

Commits: Conventional Commits. Subject ≤50 chars, imperative, lowercase after
type. Body only when "why" unclear. No period on subject.

Files in ` + "`sdt.context/`" + `: concise technical language. Cut fluff, keep
meaning and readability. These instructions and docs are concise on purpose.
`
}

func agentSectionMemory() string {
	return `## Persistent Memory

Use ` + "`sdt memory`" + ` to store facts that must survive across sessions:
decisions, constraints, conventions and known-good choices.

` + codeFence + `
sdt memory set <key> <value> [--tags a,b]   # remember a fact
sdt memory get <key>                         # retrieve a fact
sdt memory search <query> --format json      # full-text search (BM25)
sdt memory list --format json                # list all entries
sdt memory delete <key>                      # forget a fact
` + codeFence + `

Rules:
- Use stable, descriptive keys (e.g. ` + "`arch:database`" + `, ` + "`decision:auth`" + `).
- Prefer durable project facts over transient data.
- Store work progress in ` + "`sdt.context/worklog/`" + `, not in memory.
`
}

func agentSectionPlanning(project string) string {
	var b strings.Builder
	b.WriteString(`## Planning and Work Logs

Keep planning, work logs and annotations under the ` + "`sdt.context/`" + ` directory:

` + codeFence + `
sdt.context/plan/<YYYY-MM-DD>-<slug>.md              # plan before starting work
sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md      # ordered log of completed work
sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md        # free-form annotations
` + codeFence + `

Temporary and scratch files live in ` + "`sdt.context/tmp/`" + ` — never outside the
project (no ` + "`/tmp`" + `, no other absolute paths).

Date/time prefixes keep files sortable and provide a durable history.

Every file starts with YAML frontmatter carrying metadata:

`)
	b.WriteString(codeFence + "yaml\n")
	b.WriteString(`---
kind: worklog      # plan | worklog | notes
created_at: 2026-08-02T10:00:00Z
context: what triggered this entry
`)
	if project != "" {
		fmt.Fprintf(&b, "project: %s\n", project)
	}
	b.WriteString("---\n")
	b.WriteString(codeFence)
	return b.String()
}

func agentSectionAnnotations() string {
	return `## Work Annotations

Annotate continuously while working:

- Before starting, create the plan file in ` + "`sdt.context/plan/`" + `.
- After each change, append a dated entry to ` + "`sdt.context/worklog/`" + `.
- Record decisions, dead-ends and constraints with ` + "`sdt memory`" + `.

The goal is a chronological, searchable history that lets a future session
(agent or human) reconstruct what happened and why.
`
}

func agentSectionSelfUpdate() string {
	return `## Keeping AGENTS.md Up To Date

This file is organized in tagged sections:

` + codeFence + `
<!-- sdt:begin:NAME -->
...content...
<!-- sdt:end:NAME -->
` + codeFence + `

Manage sections with:

` + codeFence + `
sdt agent section list                 # show sections
sdt agent section show <name>          # show one section
sdt agent section add <name>           # add a section (stdin/--input/--file)
sdt agent section update <name>        # update a section
sdt agent section set <name>           # add or update
sdt agent section remove <name>        # remove a section
sdt agent init --force                 # refresh generated sections
` + codeFence + `

When you change project conventions, commands or workflows, update the relevant
section with ` + "`sdt agent section update <name>`" + ` and record the change in
` + "`sdt.context/worklog/`" + `.
`
}

var agentInitCmd = &cobra.Command{
	Use:   useInit,
	Short: "Bootstrap an SDT-managed project for AI agents",
	Long: `Bootstrap the current directory with everything an AI agent needs:

  .sdt.yaml                  project identity (project/group)
  AGENTS.md                  opinionated instructions in tagged sections
  .agents/skills/sdt/        extended skill guide (SKILL/REFERENCE/WORKFLOWS)
  sdt.context/plan sdt.context/worklog sdt.context/notes sdt.context/tmp   working directories

The command is idempotent and non-destructive: a second run fills in missing
content and never overwrites or removes existing files. Use --force to refresh
generated template content.

Values not provided via flags are prompted interactively with sensible defaults.
Use --yes to accept defaults without prompting (CI/non-interactive).

Examples:
  sdt agent init
  sdt agent init --project myapp
  sdt agent init --project myapp --group platform --yes`,
	Run: func(cmd *cobra.Command, args []string) {
		yes := getBoolFlag(cmd, "yes", false)
		project := getStringFlag(cmd, "project", false)
		group := getStringFlag(cmd, "group", false)
		force := getBoolFlag(cmd, "force", false)
		target := agentTargetPath(cmd)
		guideDir := getStringFlag(cmd, "dir", false)
		if guideDir == "" {
			guideDir = agentGuideDefaultDir
		}

		// 1. Resolve project identity: flags > interactive prompt > default.
		if project == "" {
			project = agentPrompt(cmd, yes, "Project name", defaultProjectName())
		}
		if group == "" {
			group = agentPrompt(cmd, yes, "Group name", defaultProjectName())
		}

		cfg := &ProjectConfig{Project: project, Group: group}
		if _, err := os.Stat(sdtConfigFile); err == nil {
			existing := loadProjectConfig(sdtConfigFile)
			if existing != nil {
				cfg.fill(existing)
			}
		}
		configResult := FileResult{Path: sdtConfigFile}
		if err := os.WriteFile(sdtConfigFile, []byte(buildProjectConfigContent(cfg.Project, cfg.Group)), 0o600); err != nil { //#nosec G306 -- user project config
			exitWithError(cmd, err)
		}
		configResult.Status = statusWritten

		// 2. sdt/ working directories (non-destructive).
		dirResults := ensureWorkDirs(force)

		// 3. AGENTS.md: add/update standard sections, keep custom content.
		mdResult, mdBody := agentMergeTarget(target, cfg, force)
		if err := os.WriteFile(target, []byte(mdBody), 0o644); err != nil { //#nosec G306 -- user-chosen output file
			exitWithError(cmd, err)
		}

		// 4. Extended skill guide.
		guideResults := writeGuideFiles(guideDir, force, false)

		results := []FileResult{configResult}
		results = append(results, dirResults...)
		results = append(results, mdResult)
		results = append(results, guideResults...)
		outputGuideResults(cmd, results)
	},
}

// agentPrompt asks for a value on the terminal. When yes is set, or stdin is
// not a terminal, the default is returned without prompting.
func agentPrompt(cmd *cobra.Command, yes bool, label, def string) string {
	if yes || !stdinIsTTY() {
		return def
	}
	if _, ferr := fmt.Fprintf(cmd.ErrOrStderr(), "%s [%s]: ", label, def); ferr != nil {
		_ = ferr
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func defaultProjectName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(cwd))
	return fmt.Sprintf("%s_%x", filepath.Base(cwd), sum[:4])
}

// loadProjectConfig reads a project config file, returning nil on any error.
func loadProjectConfig(path string) *ProjectConfig {
	data, err := os.ReadFile(path) //#nosec G304 -- user-chosen config path
	if err != nil {
		return nil
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// fill copies non-empty fields from existing into cfg, preserving prior values.
func (cfg *ProjectConfig) fill(existing *ProjectConfig) {
	if existing == nil {
		return
	}
	if existing.Project != "" {
		cfg.Project = existing.Project
	}
	if existing.Group != "" {
		cfg.Group = existing.Group
	}
}

// ensureWorkDirs creates the sdt.context/ working directory layout.
func ensureWorkDirs(force bool) []FileResult {
	dirs := []string{sdtWorkDir, sdtPlanDir, sdtWorklogDir, sdtNotesDir, sdtTmpDir}
	var results []FileResult
	for _, d := range dirs {
		res := FileResult{Path: d + "/"}
		fi, err := os.Stat(d)
		if err == nil {
			if fi.IsDir() {
				res.Status = statusSkipped
				res.Reason = "directory already exists"
			} else {
				res.Status = statusError
				res.Reason = "exists and is not a directory"
			}
		} else if os.IsNotExist(err) {
			if err := os.MkdirAll(d, 0o750); err != nil { //#nosec G301
				res.Status = statusError
				res.Reason = err.Error()
			} else {
				res.Status = statusCreated
			}
		} else {
			res.Status = statusError
			res.Reason = err.Error()
		}
		results = append(results, res)
	}

	path := sdtWorkReadme
	res := FileResult{Path: path}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(sdtWorkReadmeTemplate), 0o644); err != nil { //#nosec G306 -- user-chosen output
			res.Status = statusError
			res.Reason = err.Error()
		} else {
			res.Status = statusCreated
		}
	} else {
		res.Status = statusSkipped
		res.Reason = "file already exists"
	}
	results = append(results, res)
	return results
}

const sdtWorkReadmeTemplate = `# sdt.context/ — Working Directory

This directory holds the agent's planning, work logs, notes and temporary
files for this project.

## Layout

- ` + "`plan/`" + ` — plans written before starting non-trivial work
- ` + "`worklog/`" + ` — chronological log of completed work
- ` + "`notes/`" + ` — free-form annotations
- ` + "`tmp/`" + ` — temporary and scratch files (never outside this project)

## Conventions

- Files are prefixed with date/time so they sort naturally and keep history:
  - ` + "`sdt.context/plan/<YYYY-MM-DD>-<slug>.md`" + `
  - ` + "`sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`" + `
  - ` + "`sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`" + `
- ` + "`sdt.context/`" + ` files use concise technical language. Cut fluff,
  keep meaning and readability (token-efficient).
- Every file starts with YAML frontmatter:

` + "```yaml" + `
---
kind: worklog      # plan | worklog | notes
created_at: <ISO 8601>
context: what triggered this entry
project: <project>
---
` + "```" + `

Store durable facts in ` + "`sdt memory`" + `, not here.
`

// agentInitContent builds the full AGENTS.md body from the standard sections.
func agentInitContent(cfg *ProjectConfig) string {
	var b strings.Builder
	if cfg.Project != "" {
		fmt.Fprintf(&b, "# AGENTS.md — %s\n\n", cfg.Project)
	} else {
		b.WriteString("# AGENTS.md\n\n")
	}
	b.WriteString("> Generated by `sdt agent init`. Manage sections with `sdt agent section ...`.\n\n")
	for _, s := range agentInitSections(cfg.Project, cfg.Group) {
		b.WriteString(sectionBlock(s.name, s.body))
		b.WriteString("\n")
	}
	return b.String()
}

// agentMergeTarget builds the AGENTS.md body without destroying custom content:
// existing sections are kept, missing standard sections are appended, and with
// force the standard sections are refreshed.
func agentMergeTarget(target string, cfg *ProjectConfig, force bool) (FileResult, string) {
	res := FileResult{Path: target}
	existing, err := os.ReadFile(target) //#nosec G304 -- user-chosen target file
	if err != nil {
		res.Status = statusCreated
		return res, agentInitContent(cfg)
	}
	content := string(existing)
	if strings.TrimSpace(content) == "" {
		res.Status = statusCreated
		return res, agentInitContent(cfg)
	}

	added, updated := 0, 0
	for _, s := range agentInitSections(cfg.Project, cfg.Group) {
		if hasSection(content, s.name) {
			if force {
				content, err = updateSectionBlock(content, s.name, s.body)
				if err != nil {
					res.Status = statusError
					res.Reason = err.Error()
					return res, content
				}
				updated++
			}
			continue
		}
		content, err = addSectionBlock(content, s.name, s.body)
		if err != nil {
			res.Status = statusError
			res.Reason = err.Error()
			return res, content
		}
		added++
	}

	if added == 0 && updated == 0 {
		res.Status = statusSkipped
		res.Reason = "AGENTS.md already up to date"
	} else {
		res.Status = statusUpdated
		parts := []string{}
		if added > 0 {
			parts = append(parts, fmt.Sprintf("%d added", added))
		}
		if updated > 0 {
			parts = append(parts, fmt.Sprintf("%d refreshed", updated))
		}
		res.Reason = "sections " + strings.Join(parts, ", ")
	}
	return res, content
}

// ── registration ───────────────────────────────────────────────────────────────

func init() {
	agentSectionCmd.PersistentFlags().String("target", agentTargetDefault, "Instruction file to manage")
	agentSectionCmd.AddCommand(agentSectionListCmd)
	agentSectionCmd.AddCommand(agentSectionShowCmd)
	agentSectionCmd.AddCommand(agentSectionAddCmd)
	agentSectionCmd.AddCommand(agentSectionUpdateCmd)
	agentSectionCmd.AddCommand(agentSectionSetCmd)
	agentSectionCmd.AddCommand(agentSectionRemoveCmd)

	agentInitCmd.Flags().String("project", "", "Project name")
	agentInitCmd.Flags().String("group", "", "Group name")
	agentInitCmd.Flags().String("target", agentTargetDefault, "Output instruction file")
	agentInitCmd.Flags().String("dir", agentGuideDefaultDir, "Output directory for the skill guide")
	agentInitCmd.Flags().Bool("force", false, "Refresh generated template content")
	agentInitCmd.Flags().Bool("yes", false, "Accept defaults without prompting")

	agentCmd.AddCommand(agentInitCmd)
	agentCmd.AddCommand(agentSectionCmd)
	agentCmd.AddCommand(agentGuideCmd)
	rootCmd.AddCommand(agentCmd)
}
