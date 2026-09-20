package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── context commands (trigger files) ─────────────────────────────────────────────

// contextCommandIDs scans context/commands/*.md (excluding the generated index)
// and returns the sorted trigger ids. It is the source of truth for index
// regeneration so user-created triggers survive `sdt agent init --force`.

func contextCommandIDs() []string {
	entries, err := os.ReadDir(sdtCommandsDir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == filepath.Base(sdtCommandsIndex) {
			continue
		}
		if filepath.Ext(name) != sdtMarkdownExt {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, sdtMarkdownExt))
	}
	sort.Strings(ids)
	return ids
}

// commandsIndexContent renders the index body for the current command files:
// the union of generated triggers and whatever the directory scan finds.

func commandsIndexContent(project string, now time.Time) string {
	set := map[string]bool{}
	for _, id := range contextCommandIDs() {
		set[id] = true
	}
	for _, id := range agentCommandIDs {
		set[id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return instrCommandsIndexTemplate(ids, project, now)
}

// commandsRewriteIndex regenerates context/commands/index.md from the current
// directory scan, keeping the generated-marker format used by agent init.

func commandsRewriteIndex(project string) error {
	name := agentGeneratedMarkerName(filepath.Base(sdtCommandsDir), filepath.Base(sdtCommandsIndex))
	body := agentRenderGenerated(name, commandsIndexContent(project, contextNow()))
	return os.WriteFile(sdtCommandsIndex, []byte(body), 0o644) //#nosec G306 -- generated index
}

// contextCommandProject returns the project identity for generated frontmatter,
// or "" when no .sdt.yaml is found.

func contextCommandProject() string {
	if cfg, err := findProjectConfig(); err == nil && cfg != nil {
		return cfg.Project
	}
	return ""
}

// ── sdt context commands new ────────────────────────────────────────────────────

var contextCommandsNewCmd = &cobra.Command{
	Use:   "new <trigger>",
	Short: "Create a thin command trigger under context/commands/",
	Long: `Write a thin agent-invokable trigger file under context/commands/
(named by its trigger, e.g. >triage) and regenerate the commands index. The
durable contract stays in context/instructions/<contract>.md and is referenced,
never duplicated. The file uses the same generated format as ` + "`sdt agent init`" + `.

Examples:
  sdt context commands new triage
  sdt context commands new triage --contract research
  sdt context commands new triage --force`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := strings.TrimSpace(args[0])
		if !ctxObjectiveRegexp.MatchString(id) {
			exitWithError(cmd, fmt.Errorf("invalid trigger %q (single kebab-case segment)", id))
			return
		}
		if id == strings.TrimSuffix(filepath.Base(sdtCommandsIndex), sdtMarkdownExt) {
			exitWithError(cmd, fmt.Errorf("%s is reserved for the generated index", id))
			return
		}
		contract := getStringFlag(cmd, "contract", false)
		if contract == "" {
			contract = id
		}
		if !ctxObjectiveRegexp.MatchString(contract) {
			exitWithError(cmd, fmt.Errorf("invalid --contract %q (single kebab-case segment)", contract))
			return
		}
		project := contextCommandProject()
		path := filepath.Join(sdtCommandsDir, id+sdtMarkdownExt)
		if _, err := os.Stat(path); err == nil && !getBoolFlag(cmd, "force", false) {
			exitWithError(cmd, fmt.Errorf("%s already exists (use --force to overwrite)", path))
			return
		}
		content := agentRenderGenerated(agentGeneratedMarkerName(filepath.Base(sdtCommandsDir), id+sdtMarkdownExt), instrCommandStubTemplate(id, contract, project, contextNow()))
		if err := os.MkdirAll(sdtCommandsDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
			return
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G306 -- user work file
			exitWithError(cmd, err)
			return
		}
		if err := commandsRewriteIndex(project); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, path+"\n")
		outputString(cmd, "regenerated "+sdtCommandsIndex+"\n")
	},
}

// ── sdt context commands rm ─────────────────────────────────────────────────────

var contextCommandsRmCmd = &cobra.Command{
	Use:   "rm <trigger>",
	Short: "Archive a command trigger and regenerate the index",
	Long: `Move a trigger file from context/commands/ to context/archive/ with a
dated name, flag it ` + "`status: archived`" + `, and regenerate the commands index.

Examples:
  sdt context commands rm triage`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := strings.TrimSpace(args[0])
		if id == strings.TrimSuffix(filepath.Base(sdtCommandsIndex), sdtMarkdownExt) {
			exitWithError(cmd, fmt.Errorf("%s is the generated index; it cannot be removed", id))
			return
		}
		src := filepath.Join(sdtCommandsDir, id+sdtMarkdownExt)
		data, err := os.ReadFile(src) //#nosec G304 -- fixed repo path
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		dst := filepath.Join(sdtArchiveDir, contextTimePrefix("20060102-150405", "commands-"+id)+sdtMarkdownExt)
		if _, err := os.Stat(dst); err == nil {
			exitWithError(cmd, fmt.Errorf("target already exists: %s", dst))
			return
		}
		content, _ := setFrontmatterFields(string(data), []frontmatterPatch{{key: ctxMapStatus, value: statusArchived}})
		if err := os.MkdirAll(sdtArchiveDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
			return
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil { //#nosec G306 -- user work file
			exitWithError(cmd, err)
			return
		}
		if err := os.Remove(src); err != nil {
			exitWithError(cmd, err)
			return
		}
		project := contextCommandProject()
		if err := commandsRewriteIndex(project); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, dst+"\n")
		outputString(cmd, "regenerated "+sdtCommandsIndex+"\n")
	},
}

// ── command group ───────────────────────────────────────────────────────────────

var contextCommandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "Manage the context/commands trigger files",
	Long: `Manage the thin agent-invokable trigger files under context/commands/:
one per >trigger, each referencing its durable contract under
context/instructions/. The index (context/commands/index.md) is regenerated
from a directory scan on every change so user triggers survive
agent init --force.

  sdt context commands new <trigger>   create a trigger stub
  sdt context commands rm <trigger>    archive a trigger stub`,
}

func init() {
	contextCommandsNewCmd.Flags().String("contract", "", "Durable instruction id referenced by the stub (default: the trigger)")
	contextCommandsNewCmd.Flags().Bool("force", false, "Overwrite an existing trigger file")
	contextCommandsCmd.AddCommand(contextCommandsNewCmd, contextCommandsRmCmd)
	contextCmd.AddCommand(contextCommandsCmd)
}
