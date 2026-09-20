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
	type kindDescr struct {
		kind    string
		next    string
		ifClean string
	}
	const read = "read"
	descs := []kindDescr{
		{kind: ctxTypeArchitecture, next: read, ifClean: read},
		{kind: ctxTypeDecision, next: read, ifClean: read},
		{kind: ctxTypeAnalysis, next: "read if current", ifClean: "done"},
		{kind: ctxTypePlan, next: "active plan", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeNotes, next: "review", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeProposal, next: "review or draft", ifClean: gitIgnoreModeNone},
		{kind: ctxTypePrompt, next: "run or review", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeResearch, next: "read findings", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeQuestions, next: "answer open questions", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeTasks, next: "track per-phase", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeCommands, next: "review triggers", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeWorklog, next: ctxTierHistory, ifClean: ctxTierHistory},
		{kind: ctxTypeArchive, next: ctxTierHistory, ifClean: ctxTierHistory},
	}
	var rows []ctxStatusEntry
	for _, d := range descs {
		t, ok := ctxTypeLookup(d.kind)
		if !ok || !t.statusRow {
			continue
		}
		files, err := dirFiles(t.dir)
		if err != nil {
			continue
		}
		next := d.next
		if len(files) == 0 {
			next = d.ifClean
		}
		rows = append(rows, ctxStatusEntry{Type: ctxKindLabel(t), Count: len(files), Next: next, IfClean: d.ifClean})
	}
	return rows
}

// ── context template ────────────────────────────────────────────────────────────
