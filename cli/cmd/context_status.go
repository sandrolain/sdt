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

// ctxStoreVerdict is the store-completeness verdict shown by `context status`:
// CLEAN when no CRITICAL or WARNING lint findings exist, INCOMPLETE otherwise.
type ctxStoreVerdict struct {
	Verdict     string `json:"verdict" yaml:"verdict"`
	Critical    int    `json:"critical" yaml:"critical"`
	Warnings    int    `json:"warnings" yaml:"warnings"`
	Suggestions int    `json:"suggestions" yaml:"suggestions"`
}

// storeCompletenessVerdict runs the document lint and reduces the findings to a
// verdict plus counts. It is read-only and never fails.
func storeCompletenessVerdict() ctxStoreVerdict {
	v := ctxStoreVerdict{Verdict: "CLEAN"}
	for _, dir := range ctxIndexDirs {
		files, err := dirFiles(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			for _, it := range lintDoc(f) {
				switch it.Priority {
				case ctxLintCritical:
					v.Critical++
				case ctxLintWarning:
					v.Warnings++
				default:
					v.Suggestions++
				}
			}
		}
	}
	if v.Critical > 0 || v.Warnings > 0 {
		v.Verdict = "INCOMPLETE"
	}
	return v
}

var contextStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Summarize context/ documents per type with next step",
	Long: `Summarize the context/ knowledge: per-type document count and the
recommended next step (read / write / verify). Useful at session start after
reindex. Ends with a store-completeness verdict (INCOMPLETE vs CLEAN).

Examples:
  sdt context status
  sdt context status --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rows := ctxStatusRows()
		verdict := storeCompletenessVerdict()
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(map[string]any{ctxFrontmatterResults: rows, "store": verdict}, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(map[string]any{ctxFrontmatterResults: rows, "store": verdict})
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, r := range rows {
				outputString(cmd, fmt.Sprintf("%-12s %3d  %s\n", r.Type+":", r.Count, r.Next))
			}
			outputString(cmd, fmt.Sprintf("\nstore: %s (critical %d, warning %d, suggestion %d)\n",
				verdict.Verdict, verdict.Critical, verdict.Warnings, verdict.Suggestions))
		}
	},
}

// ctxStatusActiveHint surfaces the `sdt context resolve` helper for a workflow
// subject type: when the type carries an "active" status and at least one
// document holds it, the hint reports how many active candidates exist — one
// auto-resolves, several force the ask step of the ladder. Non-subject types and
// types without an active status return "" (the caller keeps its static hint).
func ctxStatusActiveHint(t ctxDocType) string {
	if !ctxStatusInVocab(t, ctxWikiStatusActive) {
		return ""
	}
	cands, err := resolveCandidates(t, ctxWikiStatusActive)
	if err != nil {
		return ""
	}
	active := 0
	for _, c := range cands {
		if c.Status == ctxWikiStatusActive {
			active++
		}
	}
	switch active {
	case 0:
		return ""
	case 1:
		return "1 active"
	default:
		return fmt.Sprintf("%d active - resolve", active)
	}
}

func ctxStatusRows() []ctxStatusEntry {
	type kindDescr struct {
		kind    string
		next    string
		ifClean string
	}
	// The tasks row names the command that actually reports stale in-progress
	// files. It used to claim `sdt context status` detected them, which it never
	// did: the reconciler gained that check in Wave 4 and the view that shows it is
	// `sdt context resume`.
	const read = "read"
	descs := []kindDescr{
		{kind: ctxTypeArchitecture, next: read, ifClean: read},
		{kind: ctxTypeDecision, next: read, ifClean: read},
		{kind: ctxTypeAnalysis, next: "read if current", ifClean: taskStatusDone},
		{kind: ctxTypePlan, next: "active plan", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeNotes, next: ctxReviewVerb, ifClean: gitIgnoreModeNone},
		{kind: ctxTypeProposal, next: "review or draft", ifClean: gitIgnoreModeNone},
		{kind: ctxTypePrompt, next: "run or review", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeResearch, next: "read findings", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeQuestions, next: "answer open questions", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeTasks, next: "sdt context resume", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeCommands, next: "review triggers", ifClean: gitIgnoreModeNone},
		{kind: ctxTypeWorklog, next: ctxTierHistory, ifClean: ctxTierHistory},
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
		// The active-candidate hint is the `sdt context resolve` helper surfaced
		// read-only: for a workflow subject type (analysis, plan) it names how
		// many active candidates exist, so `>plan`/`>execute` know whether the
		// ladder can auto-resolve or must ask.
		if hint := ctxStatusActiveHint(t); hint != "" {
			next = hint
		}
		rows = append(rows, ctxStatusEntry{Type: ctxKindLabel(t), Count: len(files), Next: next, IfClean: d.ifClean})
	}
	if row, ok := ctxStatusDeadEndRow(); ok {
		rows = append(rows, row)
	}
	if row, ok := ctxStatusPostponedRow(); ok {
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
		kind, objective, noteType, _ := ctxDocMeta(f)
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

// ctxStatusPostponedRow counts analyses parked in `status: postponed` (rule R3
// of instructions/analysis.md) and summarizes their objectives, so the residue of
// unchosen options is read before an analysis re-proposes the same option.
// Mirrors ctxStatusDeadEndRow: hidden while the count is zero, so a clean store
// gains no row.
func ctxStatusPostponedRow() (ctxStatusEntry, bool) {
	files, err := dirFiles(sdtAnalysisDir)
	if err != nil {
		return ctxStatusEntry{}, false
	}
	counts := map[string]int{}
	total := 0
	for _, f := range files {
		kind, objective, _, status := ctxDocMeta(f)
		if kind != ctxTypeAnalysis || status != statusPostponed {
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
	next := "revisit when the revival condition holds"
	if len(parts) > 0 {
		next += ": " + strings.Join(parts, ", ")
	}
	return ctxStatusEntry{Type: "postponed", Count: total, Next: next}, true
}

// ── context template ────────────────────────────────────────────────────────────
