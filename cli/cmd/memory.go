package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func outputEntries(cmd *cobra.Command, entries []MemoryEntry) {
	format := getFormat(cmd)
	switch format {
	case fmtYAML:
		out, err := yaml.Marshal(entries)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case "text":
		for _, e := range entries {
			line := fmt.Sprintf("[%s] %s = %s", e.Project, e.Key, e.Value)
			if e.Tags != "" {
				line += fmt.Sprintf("  (tags: %s)", e.Tags)
			}
			outputString(cmd, line)
		}
	default:
		out, err := json.MarshalIndent(entries, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	}
}

// ── root memory command ───────────────────────────────────────────────────────

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Persistent key-value memory store for AI agents",
	Long: `Manage a persistent key-value memory store backed by SQLite.

Entries are scoped by project (and optionally group). Project and group are
resolved from --project/--group flags, or from .sdt.yaml discovered by
walking up from the current directory.`,
}

// ── memory set ───────────────────────────────────────────────────────────────

var memorySetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Store a key-value entry",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		project, group := getProjectAndGroup(cmd)
		if project == "" {
			exitWithError(cmd, fmt.Errorf("--project is required (or set via .sdt.yaml)"))
			return
		}
		tags := getStringFlag(cmd, "tags", false)
		tags = normalizeTags(tags)
		if err := memorySet(project, group, args[0], args[1], tags); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, "ok")
	},
}

// ── memory get ───────────────────────────────────────────────────────────────

var memoryGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Retrieve a value by key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := getProjectAndGroup(cmd)
		if project == "" {
			exitWithError(cmd, fmt.Errorf("--project is required (or set via .sdt.yaml)"))
			return
		}
		entry, err := memoryGet(project, args[0])
		exitWithError(cmd, err)
		if entry == nil {
			exitWithError(cmd, fmt.Errorf("key not found: %s", args[0]))
		}
		format := getFormat(cmd)
		switch format {
		case fmtJSON:
			out, err := json.MarshalIndent(entry, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(entry)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, entry.Value)
		}
	},
}

// ── memory list ───────────────────────────────────────────────────────────────

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memory entries for a project",
	Run: func(cmd *cobra.Command, args []string) {
		project, group := getProjectAndGroup(cmd)
		if project == "" && group == "" {
			exitWithError(cmd, fmt.Errorf("--project or --group is required (or set via .sdt.yaml)"))
			return
		}
		entries, err := memoryList(project, group)
		exitWithError(cmd, err)
		outputEntries(cmd, entries)
	},
}

// ── memory search ─────────────────────────────────────────────────────────────

var memorySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across memory entries",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		project, group := getProjectAndGroup(cmd)
		top := getIntFlag(cmd, "top", false)
		entries, err := memorySearch(args[0], project, group, top)
		exitWithError(cmd, err)
		outputEntries(cmd, entries)
	},
}

// ── memory delete ─────────────────────────────────────────────────────────────

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete [key]",
	Short: "Delete a memory entry (or all entries for the project)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := getProjectAndGroup(cmd)
		if project == "" {
			exitWithError(cmd, fmt.Errorf("--project is required (or set via .sdt.yaml)"))
		}
		all := getBoolFlag(cmd, "all", false)
		if all {
			exitWithError(cmd, memoryDeleteAll(project))
		} else if len(args) == 1 {
			exitWithError(cmd, memoryDelete(project, args[0]))
		} else {
			exitWithError(cmd, fmt.Errorf("specify a key or --all"))
		}
		outputString(cmd, "ok")
	},
}

// ── memory projects ───────────────────────────────────────────────────────────

var memoryProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all known projects",
	Run: func(cmd *cobra.Command, args []string) {
		projects, err := memoryProjects()
		exitWithError(cmd, err)
		format := getFormat(cmd)
		switch format {
		case "json":
			out, err := json.MarshalIndent(projects, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(projects)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, p := range projects {
				outputString(cmd, p)
			}
		}
	},
}

// ── memory groups ─────────────────────────────────────────────────────────────

var memoryGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List all known groups",
	Run: func(cmd *cobra.Command, args []string) {
		groups, err := memoryGroups()
		exitWithError(cmd, err)
		format := getFormat(cmd)
		switch format {
		case "json":
			out, err := json.MarshalIndent(groups, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(groups)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, g := range groups {
				outputString(cmd, g)
			}
		}
	},
}

// ── memory export ─────────────────────────────────────────────────────────────

var memoryExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export memory entries as JSON",
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := getProjectAndGroup(cmd)
		entries, err := memoryExport(project)
		exitWithError(cmd, err)
		out, err := json.MarshalIndent(entries, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	},
}

// ── memory import ─────────────────────────────────────────────────────────────

var memoryImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import memory entries from JSON (stdin or --file)",
	Run: func(cmd *cobra.Command, args []string) {
		data := getInputBytes(cmd, args)
		if len(data) == 0 {
			exitWithError(cmd, fmt.Errorf("no input provided"))
		}
		var entries []MemoryEntry
		exitWithError(cmd, json.Unmarshal(data, &entries))
		exitWithError(cmd, memoryImport(entries))
		outputString(cmd, fmt.Sprintf("imported %d entries", len(entries)))
	},
}

// ── memory init ───────────────────────────────────────────────────────────────

var memoryInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .sdt.yaml in the current directory",
	Run: func(cmd *cobra.Command, args []string) {
		projectFlag := getStringFlag(cmd, "project", false)
		groupFlag := getStringFlag(cmd, "group", false)

		if projectFlag == "" {
			exitWithError(cmd, fmt.Errorf("--project is required"))
		}

		lines := []string{
			fmt.Sprintf("project: %s", projectFlag),
		}
		if groupFlag != "" {
			lines = append(lines, fmt.Sprintf("group: %s", groupFlag))
		}
		content := strings.Join(lines, "\n") + "\n"

		const configFile = ".sdt.yaml"
		if _, err := os.Stat(configFile); err == nil {
			exitWithError(cmd, fmt.Errorf(".sdt.yaml already exists in current directory"))
		}
		if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, fmt.Sprintf("created %s", configFile))
	},
}

// ── registration ──────────────────────────────────────────────────────────────

func init() {
	// set
	memorySetCmd.Flags().String("project", "", "Project name")
	memorySetCmd.Flags().String("group", "", "Group name")
	memorySetCmd.Flags().String("tags", "", "Comma-separated tags")

	// get
	memoryGetCmd.Flags().String("project", "", "Project name")
	memoryGetCmd.Flags().String("group", "", "Group name")

	// list
	memoryListCmd.Flags().String("project", "", "Project name")
	memoryListCmd.Flags().String("group", "", "Group name")

	// search
	memorySearchCmd.Flags().String("project", "", "Project name (optional filter)")
	memorySearchCmd.Flags().String("group", "", "Group name (optional filter)")
	memorySearchCmd.Flags().Int("top", 20, "Maximum results to return")

	// delete
	memoryDeleteCmd.Flags().String("project", "", "Project name")
	memoryDeleteCmd.Flags().String("group", "", "Group name")
	memoryDeleteCmd.Flags().Bool("all", false, "Delete all entries for the project")

	// export
	memoryExportCmd.Flags().String("project", "", "Project name (empty = all projects)")
	memoryExportCmd.Flags().String("group", "", "Group name")

	// init
	memoryInitCmd.Flags().String("project", "", "Project name")
	memoryInitCmd.Flags().String("group", "", "Group name")

	// subcommands
	memoryCmd.AddCommand(
		memorySetCmd,
		memoryGetCmd,
		memoryListCmd,
		memorySearchCmd,
		memoryDeleteCmd,
		memoryProjectsCmd,
		memoryGroupsCmd,
		memoryExportCmd,
		memoryImportCmd,
		memoryInitCmd,
	)

	rootCmd.AddCommand(memoryCmd)
}
