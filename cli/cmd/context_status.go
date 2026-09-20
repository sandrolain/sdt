package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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
		{kind: ctxTypeNotes, next: ctxReviewVerb, ifClean: gitIgnoreModeNone},
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
	if row, ok := ctxStatusDeadEndRow(); ok {
		rows = append(rows, row)
	}
	return rows
}

// ctxStatusDeadEndRow counts dead-end notes and summarizes which objectives they
// belong to, so an agent reviews rejected approaches before reopening one.
func ctxStatusDeadEndRow() (ctxStatusEntry, bool) {
	files, err := dirFiles(sdtNotesDir)
	if err != nil {
		return ctxStatusEntry{}, false
	}
	counts := map[string]int{}
	total := 0
	for _, f := range files {
		kind, objective, noteType := ctxDocMeta(f)
		if kind != ctxTypeNotes || noteType != ctxNoteTypeDeadEnd {
			continue
		}
		total++
		if objective != "" {
			counts[objective]++
		}
	}
	if total == 0 {
		return ctxStatusEntry{}, false
	}
	objectives := make([]string, 0, len(counts))
	for o := range counts {
		objectives = append(objectives, o)
	}
	sort.Strings(objectives)
	parts := make([]string, 0, len(objectives))
	for _, o := range objectives {
		parts = append(parts, fmt.Sprintf("%s (%d)", o, counts[o]))
	}
	next := "read before reopening"
	if len(parts) > 0 {
		next += ": " + strings.Join(parts, ", ")
	}
	return ctxStatusEntry{Type: "dead-ends", Count: total, Next: next}, true
}

// ── context template ────────────────────────────────────────────────────────────
