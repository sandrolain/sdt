package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
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

// contextCommandPayload returns the payload phrase declared in a command
// file's frontmatter, or "" when the file declares none. The field lives
// outside the generated markers, so `sdt agent init --force` refreshes the
// body and keeps it — that is what makes it the persistence point for a
// user-created trigger.

func contextCommandPayload(path string) string {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed commands dir, listed entry
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(frontmatterField(string(data), "payload")), `"`)
}

// commandPayloadFor resolves the payload phrase of one trigger: the
// agentCommandStubs table first, then the command file's own `payload:`
// frontmatter, else the explicit undeclared placeholder. The order is a
// decision, not an accident — a generated trigger's phrase lives in the table,
// and only a user trigger reaches the frontmatter.

func commandPayloadFor(id string) string {
	if p := declaredCommandPayload(id); p != commandPayloadUndeclared {
		return p
	}
	if p := contextCommandPayload(filepath.Join(sdtCommandsDir, id+sdtMarkdownExt)); p != "" {
		return p
	}
	return commandPayloadUndeclared
}

// commandsIndexContent renders the index body for the current command files:
// the union of generated triggers and whatever the directory scan finds.

func commandsIndexContent(project string, now time.Time) string {
	set := map[string]bool{}
	for _, id := range contextCommandIDs() {
		set[id] = true
	}
	for _, s := range agentCommandStubs {
		set[s.id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	entries := make([]commandIndexEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, commandIndexEntry{Trigger: id, Contract: declaredCommandContract(id), Payload: commandPayloadFor(id)})
	}
	return instrCommandsIndexTemplate(entries, project, now)
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
		payload := getStringFlag(cmd, "payload", false)
		if payload != "" {
			if err := validateCommandPayload(payload); err != nil {
				exitWithError(cmd, err)
				return
			}
		}
		path := filepath.Join(sdtCommandsDir, id+sdtMarkdownExt)
		if _, err := os.Stat(path); err == nil && !getBoolFlag(cmd, "force", false) {
			exitWithError(cmd, fmt.Errorf("%s already exists (use --force to overwrite)", path))
			return
		}
		body := instrCommandStubTemplate(id, contract, "", nil, project, contextNow())
		if payload != "" {
			// The declared payload goes in the frontmatter, which --force
			// preserves, so the phrase survives every later regeneration.
			body = addCommandPayloadField(body, payload)
		}
		content := agentRenderGenerated(agentGeneratedMarkerName(filepath.Base(sdtCommandsDir), id+sdtMarkdownExt), body)
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

// validateCommandPayload rejects a --payload value that cannot be stored as a
// single frontmatter line: it must be non-empty after trimming and carry no
// newline, so the `payload:` field stays one readable scalar.

func validateCommandPayload(payload string) error {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return fmt.Errorf("--payload must not be empty")
	}
	if strings.ContainsAny(trimmed, "\n\r") {
		return fmt.Errorf("--payload must be a single line")
	}
	return nil
}

// addCommandPayloadField inserts `payload: <value>` into a rendered command
// file's frontmatter, just before the closing delimiter. agentRenderGenerated
// keeps the frontmatter outside the generated markers, so the field is
// preserved by `sdt agent init --force`.

func addCommandPayloadField(rendered, payload string) string {
	fm, body := contextwiki.SplitFrontmatter(rendered)
	if fm == "" || !strings.HasSuffix(strings.TrimRight(fm, "\n"), ctxFrontmatterDelim) {
		// No frontmatter to extend (the template changed): leave the file
		// alone rather than write a field outside the frontmatter.
		return rendered
	}
	line := "payload: " + yamlScalar(strings.TrimSpace(payload))
	fm = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(fm, "\n"), ctxFrontmatterDelim), "\n") +
		"\n" + line + "\n" + ctxFrontmatterDelim + "\n"
	return fm + body
}

// ── sdt context commands rm ─────────────────────────────────────────────────────

var contextCommandsRmCmd = &cobra.Command{
	Use:   "rm <trigger>",
	Short: "Delete a command trigger and regenerate the index",
	Long: `Delete a trigger file from context/commands/ and regenerate the commands
index. The stub is removed, not archived.

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
		if err := os.Remove(src); err != nil {
			exitWithError(cmd, err)
			return
		}
		project := contextCommandProject()
		if err := commandsRewriteIndex(project); err != nil {
			exitWithError(cmd, err)
			return
		}
		outputString(cmd, "removed "+src+"\n")
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
  sdt context commands rm <trigger>    delete a trigger stub`,
}

func init() {
	contextCommandsNewCmd.Flags().String("contract", "", "Durable instruction id referenced by the stub (default: the trigger)")
	contextCommandsNewCmd.Flags().String("payload", "", "One phrase describing what the trigger accepts after the colon in `>trigger: payload`; stored in the stub frontmatter and shown in the commands index")
	contextCommandsNewCmd.Flags().Bool("force", false, "Overwrite an existing trigger file")
	contextCommandsCmd.AddCommand(contextCommandsNewCmd, contextCommandsRmCmd)
	contextCmd.AddCommand(contextCommandsCmd)
}
