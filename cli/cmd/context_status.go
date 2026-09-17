package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type ctxStatusEntry struct {
	Type    string `json:"type" yaml:"type"`
	Count   int    `json:"count" yaml:"count"`
	Next    string `json:"next" yaml:"next"`
	IfClean string `json:"clean" yaml:"clean"`
}

var contextStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Summarize context/ documents per type with next step",
	Long: `Summarize the context/ knowledge: per-type document count and the
recommended next step (read / write / verify). Useful at session start after
reindex.

Examples:
  sdt context status
  sdt context status --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rows := ctxStatusRows()
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(rows, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(rows)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, r := range rows {
				outputString(cmd, fmt.Sprintf("%-12s %3d  %s\n", r.Type+":", r.Count, r.Next))
			}
		}
	},
}

func ctxStatusRows() []ctxStatusEntry {
	type dirInfo struct {
		typ     string
		dir     string
		next    string
		ifClean string
	}
	const (
		read = "read"
	)
	infos := []dirInfo{
		{typ: "architecture", dir: sdtArchitectureDir, next: read, ifClean: read},
		{typ: "decisions", dir: sdtDecisionsDir, next: read, ifClean: read},
		{typ: "analysis", dir: sdtAnalysisDir, next: "read if current", ifClean: "done"},
		{typ: "plan", dir: sdtPlanDir, next: "active plan", ifClean: gitIgnoreModeNone},
		{typ: "notes", dir: sdtNotesDir, next: "review", ifClean: gitIgnoreModeNone},
		{typ: "questions", dir: sdtQuestionsDir, next: "answer open questions", ifClean: gitIgnoreModeNone},
		{typ: "tasks", dir: sdtTasksDir, next: "track per-phase", ifClean: gitIgnoreModeNone},
		{typ: "worklog", dir: sdtWorklogDir, next: ctxTierHistory, ifClean: ctxTierHistory},
		{typ: "archive", dir: sdtArchiveDir, next: ctxTierHistory, ifClean: ctxTierHistory},
	}
	var rows []ctxStatusEntry
	for _, info := range infos {
		files, err := dirFiles(info.dir)
		if err != nil {
			continue
		}
		next := info.next
		if len(files) == 0 {
			next = info.ifClean
		}
		rows = append(rows, ctxStatusEntry{Type: info.typ, Count: len(files), Next: next, IfClean: info.ifClean})
	}
	return rows
}

// ── context template ────────────────────────────────────────────────────────────
