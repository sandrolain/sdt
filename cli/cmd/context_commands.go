package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── context commands (one registry) ────────────────────────────────────────────
//
// `context/commands/` holds exactly one generated document: the registry. A
// trigger is a row, not a file — 31 near-identical files each restating the
// grammar, the gates and the payload contract were the largest duplication in
// the generated set. The registry carries that prose once and a row per trigger
// naming its durable contract.
//
// A project's own rows live outside the generated marker, so `agent init
// --force` rewrites the built-in table and leaves them alone. A trigger that
// needs prose beyond its row gets an optional detail file next to the registry.

// commandRow is one parsed registry row.
type commandRow struct {
	Trigger  string // without the leading '>'
	Kind     string
	Subject  string // "—" or a doc type
	Contract string // path as written, e.g. context/instructions/analysis.md
	Payload  string
	Examples string
}

// commandsRegistryMarker is the generated section name inside the registry.
func commandsRegistryMarker() string {
	return agentGeneratedMarkerName(filepath.Base(sdtCommandsDir), filepath.Base(sdtCommandsIndex))
}

// parseCommandRows returns every registry row of a document, in file order. It
// reads both the generated table and a project's own rows: a row is a table
// line whose first cell is a backticked `>trigger`.
func parseCommandRows(content string) []commandRow {
	var rows []commandRow
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `>") {
			continue
		}
		cells := strings.Split(trimmed, "|")
		if len(cells) < 7 {
			continue
		}
		cell := func(i int) string { return strings.TrimSpace(cells[i]) }
		rows = append(rows, commandRow{
			Trigger:  strings.TrimSuffix(strings.TrimPrefix(cell(1), "`>"), "`"),
			Kind:     cell(2),
			Subject:  strings.Trim(cell(3), "`"),
			Contract: strings.Trim(cell(4), "`"),
			Payload:  cell(5),
			Examples: cell(6),
		})
	}
	return rows
}

// commandRowLine renders a row as one registry line.
func commandRowLine(row commandRow) string {
	subject := row.Subject
	if subject == "" || subject == "—" {
		subject = "—"
	} else {
		subject = "`" + subject + "`"
	}
	contract := row.Contract
	if contract != "" && contract != "—" {
		contract = "`" + contract + "`"
	}
	payload := row.Payload
	if payload == "" {
		payload = commandPayloadUndeclared
	}
	examples := row.Examples
	if examples == "" {
		examples = "—"
	}
	return fmt.Sprintf("| `>%s` | %s | %s | %s | %s | %s |", row.Trigger, row.Kind, subject, contract, payload, examples)
}

// commandContractPath returns the contract path a contract id points at.
func commandContractPath(contract string) string {
	return filepath.Join(sdtInstrDir, contract+sdtMarkdownExt)
}

// commandsIndexSplit separates the generated registry from the project's own
// rows: head ends at the generated end marker, tail is everything after it.
func commandsIndexSplit(content string) (head, tail string) {
	end := "<!-- sdt:end:" + commandsRegistryMarker() + " -->"
	if idx := strings.Index(content, end); idx >= 0 {
		return content[:idx+len(end)], strings.TrimSpace(content[idx+len(end):])
	}
	return "", strings.TrimSpace(content)
}

// projectTriggersHeader is the table the project's own rows live in.
const projectTriggersHeader = "## Project triggers\n\n" +
	"Commands this project added. They live outside the generated block, so\n" +
	"`sdt agent init --force` rewrites the registry above without touching them.\n" +
	"A trigger here behaves exactly like a built-in one: same grammar, same gates,\n" +
	"its durable contract named in the row.\n\n" +
	"| Trigger | Kind | Subject | Durable contract | Payload | Examples |\n" +
	"|---|---|---|---|---|---|\n"

// upsertProjectRow adds or replaces a project row in the section after the
// generated block, creating the section when absent.
func upsertProjectRow(section string, row commandRow) string {
	line := commandRowLine(row)
	lines := strings.Split(section, "\n")
	replaced := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "| `>"+row.Trigger+"`") {
			lines[i] = line
			replaced = true
			break
		}
	}
	if replaced {
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(section) == "" {
		return projectTriggersHeader + line
	}
	// Append after the last row of the project table, so a preamble keeps its
	// place and the new row stays inside the table.
	lastRow := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "| `>") {
			lastRow = i
		}
	}
	if lastRow >= 0 {
		lines = append(lines[:lastRow+1], append([]string{line}, lines[lastRow+1:]...)...)
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	if !strings.Contains(section, "## Project triggers") {
		return strings.TrimSpace(section) + "\n\n" + projectTriggersHeader + line
	}
	return strings.TrimSpace(section) + "\n" + line
}

// removeProjectRow drops a project row, leaving the section (and its header) in
// place so the next addition has a home.
func removeProjectRow(section, trigger string) string {
	lines := strings.Split(section, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "| `>"+trigger+"`") {
			continue
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// commandsIndexContent renders the generated registry for the project's current
// selection (no project rows: they live after the marker).
func commandsIndexContent(project string, now time.Time) string {
	return commandsIndexContentFor(nil, project, now)
}

// commandsIndexContentFor builds the registry for a profile selection: a stub
// whose contract module is not selected is not a row, because the project does
// not have that contract.
func commandsIndexContentFor(profiles []string, project string, now time.Time) string {
	if len(profiles) == 0 {
		profiles = currentProfiles()
	}
	return instrCommandsIndexTemplate(commandEntriesFor(profiles), project, now)
}

// contextCommandProject returns the project identity for generated frontmatter,
// or "" when no .sdt.yaml is found.

func contextCommandProject() string {
	if cfg, err := findProjectConfig(); err == nil && cfg != nil {
		return cfg.Project
	}
	return ""
}

// commandDetailContent renders the optional per-trigger detail file: the place
// for payload prose a table cell cannot hold. It is a project document, not a
// generated one: `agent init --force` never rewrites it.
func commandDetailContent(id, contract, kind, subject, payload, project string, now time.Time) string {
	var b strings.Builder
	stamp := now.UTC().Format(time.RFC3339)
	b.WriteString("---\n")
	b.WriteString("kind: commands\n")
	b.WriteString("id: commands/" + id + "\n")
	b.WriteString("title: \">" + id + " — command detail\"\n")
	b.WriteString("summary: \"Payload detail for the >" + id + " trigger; the durable contract is " + commandContractPath(contract) + ".\"\n")
	b.WriteString("# statuses for the commands kind: see sdt context vocab\n")
	b.WriteString("status: active\n")
	b.WriteString("links:\n  - commands/index.md\n")
	b.WriteString("sources:\n  - instructions/" + contract + ".md\n")
	if project != "" {
		b.WriteString("project: " + project + "\n")
	}
	b.WriteString("created: \"" + stamp + "\"\n")
	b.WriteString("updated: \"" + stamp + "\"\n")
	b.WriteString("---\n\n")
	b.WriteString("## Payload\n\n")
	b.WriteString("- **Form:** `>" + id + "` for no payload · `>" + id + ": <payload>` for one; the\n")
	b.WriteString("  payload is everything after the first colon, verbatim.\n")
	if kind == commandKindWorkflow && subject != "" {
		b.WriteString("- **Subject:** the `" + subject + "` document this command advances; resolve it\n")
		b.WriteString("  through the ladder in `context/commands/index.md`.\n")
	}
	if payload != "" {
		b.WriteString("- **This trigger accepts:** " + payload + ".\n")
	}
	b.WriteString("- **Empty payload:** apply the declared default; if there is none, ask **one**\n")
	b.WriteString("  question. Never infer the subject silently.\n")
	b.WriteString("- **Precedence:** explicit payload > working-context resolution > ask the user.\n")
	return b.String()
}

// ── sdt context commands new ────────────────────────────────────────────────────

var contextCommandsNewCmd = &cobra.Command{
	Use:   "new <trigger>",
	Short: "Add a row to the commands registry",
	Long: `Add a trigger row to context/commands/index.md, under *Project triggers*.

A trigger is a registry row, not a file: the row names its durable contract
under context/instructions/ (or any project document) and the grammar, gates
and resolution ladder are the registry's single copy. The row lives outside the
generated block, so ` + "`sdt agent init --force`" + ` rewrites the built-in table and
keeps it.

A trigger is a document command by default (it produces/edits a document of a
fixed type). Pass --kind workflow --subject <type> to declare a lifecycle
transition that resolves a subject document from the working context.

Pass --detail to also write context/commands/<trigger>.md for payload prose a
table cell cannot hold; the file is never regenerated.

Examples:
  sdt context commands new triage
  sdt context commands new triage --contract research --payload "the topic to look up"
  sdt context commands new triage --kind workflow --subject analysis --detail
  sdt context commands new triage --force`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := strings.TrimSpace(args[0])
		if !ctxObjectiveRegexp.MatchString(id) {
			exitWithError(cmd, fmt.Errorf("invalid trigger %q (single kebab-case segment)", id))
			return
		}
		if id == strings.TrimSuffix(filepath.Base(sdtCommandsIndex), sdtMarkdownExt) {
			exitWithError(cmd, fmt.Errorf("%s is reserved for the generated registry", id))
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
		kind := getStringFlag(cmd, "kind", false)
		if kind == "" {
			kind = commandKindDocument
		}
		if kind != commandKindDocument && kind != commandKindWorkflow {
			exitWithError(cmd, fmt.Errorf("invalid --kind %q (use document or workflow)", kind))
			return
		}
		subject := getStringFlag(cmd, "subject", false)
		if subject != "" {
			if !ctxObjectiveRegexp.MatchString(subject) {
				exitWithError(cmd, fmt.Errorf("invalid --subject %q (single kebab-case segment)", subject))
				return
			}
			if kind != commandKindWorkflow {
				exitWithError(cmd, fmt.Errorf("--subject requires --kind workflow"))
				return
			}
		}

		existing := ""
		if data, err := os.ReadFile(sdtCommandsIndex); err == nil { //#nosec G304 -- fixed registry path
			existing = string(data)
		}
		force := getBoolFlag(cmd, "force", false)
		for _, row := range parseCommandRows(existing) {
			if row.Trigger == id && !force {
				exitWithError(cmd, fmt.Errorf(">%s is already registered (use --force to update the row)", id))
				return
			}
		}

		detail := getBoolFlag(cmd, "detail", false)
		detailPath := filepath.Join(sdtCommandsDir, id+sdtMarkdownExt)
		if detail {
			if _, err := os.Stat(detailPath); err == nil && !force {
				exitWithError(cmd, fmt.Errorf("%s already exists (use --force to overwrite)", detailPath))
				return
			}
		}

		row := commandRow{
			Trigger:  id,
			Kind:     kind,
			Subject:  subject,
			Contract: filepath.Join(sdtInstrDir, contract+sdtMarkdownExt),
			Payload:  payload,
		}
		_, tail := commandsIndexSplit(existing)
		tail = upsertProjectRow(tail, row)
		head := agentRenderGenerated(commandsRegistryMarker(), commandsIndexContentFor(currentProfiles(), project, contextNow()))
		content := head
		if tail != "" {
			content = strings.TrimRight(content, "\n") + "\n\n" + tail + "\n"
		}
		if err := os.MkdirAll(sdtCommandsDir, 0o750); err != nil { //#nosec G301 -- user work dir
			exitWithError(cmd, err)
			return
		}
		if err := //#nosec G306 G703 -- path resolved from the corpus, not from user input
			os.WriteFile(sdtCommandsIndex, []byte(content), 0o644); err != nil { //#nosec G306 -- user work file
			exitWithError(cmd, err)
			return
		}
		if detail {
			body := commandDetailContent(id, contract, kind, subject, payload, project, contextNow())
			if err := os.WriteFile(detailPath, []byte(body), 0o644); err != nil { //#nosec G306 -- user work file
				exitWithError(cmd, err)
				return
			}
		}
		outputString(cmd, "registered >"+id+" in "+sdtCommandsIndex+"\n")
		if detail {
			outputString(cmd, "wrote "+detailPath+"\n")
		}
	},
}

// validateCommandPayload rejects a --payload value that cannot be stored as a
// single table cell: it must be non-empty after trimming and carry no newline.

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

// ── sdt context commands rm ─────────────────────────────────────────────────────

var contextCommandsRmCmd = &cobra.Command{
	Use:   "rm <trigger>",
	Short: "Remove a trigger row from the registry",
	Long: `Remove a trigger's row from context/commands/index.md and delete its detail
file, when one exists. A built-in trigger is refused: it comes from the
installed template set, not from the project.

Examples:
  sdt context commands rm triage`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := strings.TrimSpace(args[0])
		if id == strings.TrimSuffix(filepath.Base(sdtCommandsIndex), sdtMarkdownExt) {
			exitWithError(cmd, fmt.Errorf("%s is the generated registry; it cannot be removed", id))
			return
		}
		for _, s := range agentCommandStubs {
			if s.id == id {
				exitWithError(cmd, fmt.Errorf(">%s is a built-in trigger: it is not a project row", id))
				return
			}
		}
		existing := ""
		if data, err := os.ReadFile(sdtCommandsIndex); err == nil { //#nosec G304 -- fixed registry path
			existing = string(data)
		}
		head, tail := commandsIndexSplit(existing)
		if !hasProjectRow(tail, id) {
			exitWithError(cmd, fmt.Errorf(">%s is not registered in %s", id, sdtCommandsIndex))
			return
		}
		tail = removeProjectRow(tail, id)
		if head == "" {
			head = agentRenderGenerated(commandsRegistryMarker(), commandsIndexContentFor(currentProfiles(), contextCommandProject(), contextNow()))
		}
		content := strings.TrimRight(head, "\n")
		if tail != "" {
			content += "\n\n" + tail
		}
		if err := os.WriteFile(sdtCommandsIndex, []byte(content+"\n"), 0o644); err != nil { //#nosec G306 -- generated registry
			exitWithError(cmd, err)
			return
		}
		detailPath := filepath.Join(sdtCommandsDir, id+sdtMarkdownExt)
		if _, err := os.Stat(detailPath); err == nil {
			if err := os.Remove(detailPath); err != nil {
				exitWithError(cmd, err)
				return
			}
			outputString(cmd, "removed "+detailPath+"\n")
		}
		outputString(cmd, "removed >"+id+" from "+sdtCommandsIndex+"\n")
	},
}

// hasProjectRow reports whether the project section registers a trigger.
func hasProjectRow(section, trigger string) bool {
	for _, row := range parseCommandRows(section) {
		if row.Trigger == trigger {
			return true
		}
	}
	return false
}

// ── command group ───────────────────────────────────────────────────────────────

var contextCommandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "Manage the agent-invokable commands registry",
	Long: `Manage the single registry of agent-invokable commands,
` + "`context/commands/index.md`" + `: its grammar, its gates and one row per trigger
naming the durable contract under ` + "`context/instructions/`" + `.

  sdt context commands new <trigger>   add a project row
  sdt context commands rm <trigger>    remove a project row`,
}

func init() {
	contextCommandsNewCmd.Flags().String("contract", "", "Durable instruction id the row names (default: the trigger)")
	contextCommandsNewCmd.Flags().String("payload", "", "One phrase describing what the trigger accepts after the colon in `>trigger: payload`")
	contextCommandsNewCmd.Flags().String("kind", "", "Command kind: document (default) or workflow (advances the lifecycle of the --subject type)")
	contextCommandsNewCmd.Flags().String("subject", "", "Subject doc type a workflow trigger resolves (requires --kind workflow)")
	contextCommandsNewCmd.Flags().Bool("detail", false, "Also write context/commands/<trigger>.md for payload prose a table cell cannot hold")
	contextCommandsNewCmd.Flags().Bool("force", false, "Update an existing row (and overwrite an existing detail file)")
	contextCommandsCmd.AddCommand(contextCommandsNewCmd, contextCommandsRmCmd)
	contextCmd.AddCommand(contextCommandsCmd)
}
