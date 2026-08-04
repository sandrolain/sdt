package cmd

import (
	"bufio"
	"crypto/sha256"
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

// ── tagged block helpers ───────────────────────────────────────────────────────

func sectionBlock(name, body string) string {
	body = strings.TrimSpace(body)
	return fmt.Sprintf("<!-- sdt:begin:%s -->\n\n%s\n\n<!-- sdt:end:%s -->\n", name, body, name)
}

func sectionBeginMarker(name string) string {
	return "<!-- sdt:begin:" + name + " -->"
}

func sectionEndMarker(name string) string {
	return "<!-- sdt:end:" + name + " -->"
}

func sectionRegexp(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)\n?` + regexp.QuoteMeta(sectionBeginMarker(name)) + `.*?` + regexp.QuoteMeta(sectionEndMarker(name)) + `\n?`)
}

func hasSection(content, name string) bool {
	return sectionRegexp(name).MatchString(content)
}

// agentMergeBlock ensures the named block exists in content. With force it is
// refreshed, otherwise left untouched when already present. Returns the new
// content and whether it changed.
func agentMergeBlock(content, name, body string, force bool) (string, bool) {
	if hasSection(content, name) {
		if force {
			return sectionRegexp(name).ReplaceAllString(content, "\n"+sectionBlock(name, body)), true
		}
		return content, false
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	return content + sectionBlock(name, body), true
}

// ── target file helpers ────────────────────────────────────────────────────────

func agentTargetPath(cmd *cobra.Command) string {
	path := getStringFlag(cmd, "target", false)
	if path == "" {
		path = agentTargetDefault
	}
	return path
}

// ── agent command group ────────────────────────────────────────────────────────

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent instruction tools (AGENTS.md, instruction files)",
	Long: `Generate and maintain agent instruction files.

  agent init       bootstrap AGENTS.md + sdt.context/ instruction files

AGENTS.md is a thin index: it carries a single tagged block that points to the
instruction files under ` + "`sdt.context/instructions/`" + `. Each instruction
file is the single source of truth for one topic.`,
}

// ── agent init (AGENTS.md + instruction files) ─────────────────────────────────

// instructionFile is one generated instruction file under sdt.context/instructions/.
type instructionFile struct {
	name string
	body string
}

func instructionFiles(project, group string) []instructionFile {
	return []instructionFile{
		{name: filepath.Base(sdtInstrReadme), body: instrReadmeTemplate},
		{name: filepath.Base(sdtInstrProject), body: instrProjectTemplate(project, group)},
		{name: filepath.Base(sdtInstrCommands), body: instrCommandsTemplate},
		{name: filepath.Base(sdtInstrWorkflow), body: instrWorkflowTemplate},
		{name: filepath.Base(sdtInstrComm), body: instrCommunicationTemplate},
		{name: filepath.Base(sdtInstrMemory), body: instrMemoryTemplate},
		{name: filepath.Base(sdtInstrPlanning), body: instrPlanningTemplate(project)},
		{name: filepath.Base(sdtInstrAnnotate), body: instrAnnotationsTemplate},
		{name: filepath.Base(sdtInstrSelfUpd), body: instrSelfUpdateTemplate},
		{name: filepath.Base(sdtInstrReference), body: instrReferenceTemplate},
	}
}

// writeInstructionFiles creates the instruction files under sdt.context/instructions/.
// It is non-destructive: existing files are preserved unless force is set.
func writeInstructionFiles(project, group string, force bool) []FileResult {
	var results []FileResult
	for _, f := range instructionFiles(project, group) {
		path := filepath.Join(sdtInstrDir, f.name)
		res := FileResult{Path: path}
		existed := false
		if _, err := os.Stat(path); err == nil {
			existed = true
			if !force {
				res.Status = statusSkipped
				res.Reason = "file already exists (use --force to overwrite)"
				results = append(results, res)
				continue
			}
		} else if !os.IsNotExist(err) {
			res.Status = statusError
			res.Reason = err.Error()
			results = append(results, res)
			continue
		}
		if err := os.MkdirAll(sdtInstrDir, 0o750); err != nil { //#nosec G301
			res.Status = statusError
			res.Reason = err.Error()
			results = append(results, res)
			continue
		}
		if err := os.WriteFile(path, []byte(f.body), 0o644); err != nil { //#nosec G306 -- user-chosen output
			res.Status = statusError
			res.Reason = err.Error()
			results = append(results, res)
			continue
		}
		if existed {
			res.Status = statusUpdated
		} else {
			res.Status = statusCreated
		}
		results = append(results, res)
	}
	return results
}

var agentInitCmd = &cobra.Command{
	Use:   useInit,
	Short: "Bootstrap an SDT-managed project for AI agents",
	Long: `Bootstrap the current directory with everything an AI agent needs:

  .sdt.yaml                           project identity (project/group)
  AGENTS.md                           thin index with a single tagged block
  sdt.context/plan|worklog|notes|tmp  working directories
  sdt.context/instructions/           instruction files (single source of truth)

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

		// 2. sdt.context/ working directories (non-destructive).
		dirResults := ensureWorkDirs(force)

		// 3. Instruction files under sdt.context/instructions/.
		instrResults := writeInstructionFiles(cfg.Project, cfg.Group, force)

		// 4. AGENTS.md: ensure the single instructions block.
		mdResult, mdBody := agentMergeTarget(target, force)
		if err := os.WriteFile(target, []byte(mdBody), 0o644); err != nil { //#nosec G306 -- user-chosen output file
			exitWithError(cmd, err)
		}

		results := []FileResult{configResult}
		results = append(results, dirResults...)
		results = append(results, instrResults...)
		results = append(results, mdResult)
		outputFileResults(cmd, results)
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
	dirs := []string{sdtWorkDir, sdtPlanDir, sdtWorklogDir, sdtNotesDir, sdtTmpDir, sdtInstrDir}
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

This directory holds the agent's planning, work logs, notes, instruction files
and temporary files for this project.

## Layout

- ` + "`plan/`" + ` — plans written before starting non-trivial work
- ` + "`worklog/`" + ` — chronological log of completed work
- ` + "`notes/`" + ` — free-form annotations
- ` + "`instructions/`" + ` — agent instruction files (referenced by AGENTS.md)
- ` + "`tmp/`" + ` — temporary and scratch files (never outside this project)

## Conventions

- Files are prefixed with date/time so they sort naturally and keep history:
  - ` + "`sdt.context/plan/<YYYY-MM-DD>-<slug>.md`" + `
  - ` + "`sdt.context/worklog/<YYYYMMDD-HHMMSS>-<slug>.md`" + `
  - ` + "`sdt.context/notes/<YYYYMMDD-HHMMSS>-<slug>.md`" + `
- ` + "`sdt.context/`" + ` files use concise technical language. Cut fluff,
  keep meaning and readability (token-efficient).
- Every work file starts with YAML frontmatter:

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

// agentMergeTarget ensures AGENTS.md carries the single instructions block
// without destroying custom content. With force the block is refreshed.
func agentMergeTarget(target string, force bool) (FileResult, string) {
	res := FileResult{Path: target}
	content := ""
	if _, err := os.Stat(target); err == nil {
		data, rerr := os.ReadFile(target) //#nosec G304 -- user-chosen target file
		if rerr != nil {
			res.Status = statusError
			res.Reason = rerr.Error()
			return res, content
		}
		content = string(data)
	}

	newContent, changed := agentMergeBlock(content, agentSectionNameInstructions, agentBlockInstructions(), force)
	if !changed {
		res.Status = statusSkipped
		res.Reason = "AGENTS.md already up to date"
	} else if strings.TrimSpace(content) == "" {
		res.Status = statusCreated
	} else {
		res.Status = statusUpdated
	}
	return res, newContent
}

func agentBlockInstructions() string {
	return `## Instructions

This project is managed with SDT. Read the relevant instruction file before acting:

- ` + "`sdt.context/instructions/project.md`" + ` — project identity and configuration
- ` + "`sdt.context/instructions/commands.md`" + ` — build, test and lint commands
- ` + "`sdt.context/instructions/workflow.md`" + ` — agent workflow loop
- ` + "`sdt.context/instructions/communication.md`" + ` — response style, commits, conciseness
- ` + "`sdt.context/instructions/memory.md`" + ` — persistent memory usage
- ` + "`sdt.context/instructions/planning.md`" + ` — planning and work log conventions
- ` + "`sdt.context/instructions/annotations.md`" + ` — work annotation rules
- ` + "`sdt.context/instructions/self-update.md`" + ` — keeping instructions current
- ` + "`sdt.context/instructions/reference.md`" + ` — SDT command reference

Work directories live under ` + "`sdt.context/`" + ` (plan/, worklog/, notes/,
tmp/). Never write or execute temporary files outside the project. Keep all
instruction files concise and technical.
`
}

// ── registration ───────────────────────────────────────────────────────────────

func init() {
	agentInitCmd.Flags().String("project", "", "Project name")
	agentInitCmd.Flags().String("group", "", "Group name")
	agentInitCmd.Flags().String("target", agentTargetDefault, "Output instruction file")
	agentInitCmd.Flags().Bool("force", false, "Refresh generated template content")
	agentInitCmd.Flags().Bool("yes", false, "Accept defaults without prompting")

	agentCmd.AddCommand(agentInitCmd)
	rootCmd.AddCommand(agentCmd)
}
