package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// setupAgentFile maps each agent name to the conventional file path it should create.
// The order of setupAgentAll controls creation order when --agent all is used.
var setupAgentFile = map[string]string{
	"copilot": ".github/copilot-instructions.md",
	"claude":  "CLAUDE.md",
	"generic": "AGENTS.md",
	"skill":   ".agents/skills/sdt/SKILL.md",
}

// setupAgentAll is the ordered list of agents created by --agent all.
// Only generic (AGENTS.md) and skill (.agents/skills/sdt/SKILL.md) are included by default.
// copilot and claude are available via --agent copilot / --agent claude.
var setupAgentAll = []string{"generic", "skill"}

// SetupFileResult represents the outcome of creating one file during setup.
type SetupFileResult struct {
	Path    string `json:"path"    yaml:"path"`
	Status  string `json:"status"  yaml:"status"` // "created", "skipped", "dry-run"
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// SetupResult is the top-level output of the setup command.
type SetupResult struct {
	Project string            `json:"project,omitempty" yaml:"project,omitempty"`
	Group   string            `json:"group,omitempty"   yaml:"group,omitempty"`
	Files   []SetupFileResult `json:"files"             yaml:"files"`
}

func setupWriteFile(path string, content []byte, force bool, dryRun bool) SetupFileResult {
	res := SetupFileResult{Path: path}

	if dryRun {
		res.Status = "dry-run"
		return res
	}

	if _, err := os.Stat(path); err == nil && !force {
		res.Status = "skipped"
		res.Reason = "file already exists (use --force to overwrite)"
		return res
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil { //#nosec G301
			res.Status = "error"
			res.Reason = err.Error()
			return res
		}
	}

	if err := os.WriteFile(path, content, 0o600); err != nil { //#nosec G306
		res.Status = "error"
		res.Reason = err.Error()
		return res
	}

	res.Status = "created"
	return res
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Scaffold agent files for the current project",
	Long: `Create agent instruction files and .sdt.yaml in the current directory.

By default (--agent all), the following files are created:

  .sdt.yaml                       — project identity for sdt memory
  AGENTS.md                       — generic agent instructions
  .agents/skills/sdt/SKILL.md     — open agent skills ecosystem

Use --agent to create specific files:
  --agent generic                 AGENTS.md only
  --agent skill                   .agents/skills/sdt/SKILL.md only
  --agent copilot                 .github/copilot-instructions.md
  --agent claude                  CLAUDE.md
  --agent copilot,claude          multiple agents (comma-separated)

Use --dry-run to preview without writing anything.
Use --force to overwrite existing files.

Examples:
  sdt setup --project myapp
  sdt setup --project myapp --group platform --agent copilot
  sdt setup --project myapp --agent all --force
  sdt setup --project myapp --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		project := getStringFlag(cmd, "project", false)
		group := getStringFlag(cmd, "group", false)
		agent := getStringFlag(cmd, "agent", false)
		force := getBoolFlag(cmd, "force", false)
		dryRun := getBoolFlag(cmd, "dry-run", false)
		format := getFormat(cmd)

		if agent == "" {
			agent = "all"
		}

		// Resolve which agents to set up
		var agentsToSetup []string
		if agent == "all" {
			agentsToSetup = setupAgentAll
		} else {
			for _, a := range strings.Split(agent, ",") {
				a = strings.TrimSpace(a)
				if _, ok := setupAgentFile[a]; !ok {
					supported := append([]string(nil), setupAgentAll...)
					exitWithError(cmd, fmt.Errorf(
						"unknown agent %q; supported: %s, all", a, strings.Join(supported, ", "),
					))
					return
				}
				agentsToSetup = append(agentsToSetup, a)
			}
		}

		result := SetupResult{
			Project: project,
			Group:   group,
		}

		// 1. Create .sdt.yaml if a project name was provided
		if project != "" {
			const sdtConfig = ".sdt.yaml"
			lines := []string{fmt.Sprintf("project: %s", project)}
			if group != "" {
				lines = append(lines, fmt.Sprintf("group: %s", group))
			}
			content := []byte(strings.Join(lines, "\n") + "\n")
			result.Files = append(result.Files, setupWriteFile(sdtConfig, content, force, dryRun))
		}

		// 2. Create agent instruction files
		for _, a := range agentsToSetup {
			path := setupAgentFile[a]
			content := []byte(skillTemplates[a])
			result.Files = append(result.Files, setupWriteFile(path, content, force, dryRun))
		}

		// Output
		switch format {
		case fmtJSON:
			out, err := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(result)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, f := range result.Files {
				line := fmt.Sprintf("%-10s %s", "["+f.Status+"]", f.Path)
				if f.Reason != "" {
					line += "  # " + f.Reason
				}
				outputString(cmd, line)
			}
		}
	},
}

func init() {
	setupCmd.Flags().String("project", "", "Project name for .sdt.yaml (optional)")
	setupCmd.Flags().String("group", "", "Group/team name for .sdt.yaml (optional)")
	setupCmd.Flags().String("agent", "all", "Agent type(s): copilot|claude|generic|skill|all (comma-separated)")
	setupCmd.Flags().Bool("force", false, "Overwrite existing files")
	setupCmd.Flags().Bool("dry-run", false, "Preview files without writing")
	rootCmd.AddCommand(setupCmd)
}
