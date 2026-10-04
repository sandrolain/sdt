package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/mdindex"
	"github.com/spf13/cobra"
)

// ctxChecklistBackfillMarker records that the one-shot
// `context checklist backfill` has run; lint flips a missing id anchor from
// SUGGESTION to WARNING once it exists.
var ctxChecklistBackfillMarker = filepath.Join(mdindex.CacheDir, "checklist-backfill.json")

// checklistBackfillDone reports whether the checklist backfill marker exists.
func checklistBackfillDone() bool {
	_, err := os.Stat(ctxChecklistBackfillMarker)
	return err == nil
}

// ctxChecklistFiles lists every document that can carry a checklist: all
// context/ work-file types except the tmp scratch dir and the generated
// command triggers, recursively (wiki subpaths included), in stable order.
func ctxChecklistFiles() ([]string, error) {
	var files []string
	for _, t := range contextTypeList {
		if t.kind == ctxTypeTmp || t.kind == ctxTypeCommands {
			continue
		}
		walkErr := filepath.WalkDir(t.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || filepath.Ext(path) != sdtMarkdownExt {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	sort.Strings(files)
	return files, nil
}

// ── context checklist backfill ─────────────────────────────────────────────────

type ctxChecklistBackfillResult struct {
	Action   string   `json:"action" yaml:"action"`
	Scanned  int      `json:"scanned" yaml:"scanned"`
	Stamped  int      `json:"stamped" yaml:"stamped"`
	Skipped  int      `json:"skipped" yaml:"skipped"`
	Paths    []string `json:"paths,omitempty" yaml:"paths,omitempty"`
	Marker   string   `json:"marker,omitempty" yaml:"marker,omitempty"`
	Finished string   `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
}

// stampChecklistFiles is the backfill core: it stamps a missing id anchor on
// every un-anchored checklist item of every document. With dryRun it reports
// the would-change paths without writing (and without touching the marker).
func stampChecklistFiles(dryRun bool) (ctxChecklistBackfillResult, error) {
	res := ctxChecklistBackfillResult{Action: statusWritten, Marker: ctxChecklistBackfillMarker}
	if dryRun {
		res.Action = statusDryRun
	}
	files, err := ctxChecklistFiles()
	if err != nil {
		return res, err
	}
	for _, path := range files {
		data, rerr := os.ReadFile(path) //#nosec G304 -- fixed repo path
		if rerr != nil {
			return res, rerr
		}
		content := string(data)
		res.Scanned++
		stamped, sChanged := stampChecklistIDs(content)
		repaired, rChanged := repairChecklistIDs(stamped)
		if !sChanged && !rChanged {
			res.Skipped++
			continue
		}
		res.Stamped++
		res.Paths = append(res.Paths, path)
		if dryRun {
			continue
		}
		if werr := writeWorkFile(path, repaired); werr != nil {
			return res, werr
		}
	}
	if dryRun {
		return res, nil
	}
	res.Finished = time.Now().UTC().Format(time.RFC3339)
	if err := writeBackfillMarker(ctxChecklistBackfillMarker, res.Finished, res.Stamped); err != nil {
		return res, err
	}
	return res, nil
}

func outputChecklistBackfill(cmd *cobra.Command, res ctxChecklistBackfillResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		verb := "stamped"
		if res.Action == statusDryRun {
			verb = "would stamp"
		}
		outputString(cmd, fmt.Sprintf("%s %d checklist item(s) in %d file(s); %d scanned, %d already stamped\n", verb, res.Stamped, len(res.Paths), res.Scanned, res.Skipped))
		for _, p := range res.Paths {
			outputString(cmd, p+"\n")
		}
	}
}

var contextChecklistCmd = &cobra.Command{
	Use:   "checklist",
	Short: "Manage checklist item identifiers",
	Long: `Manage the stable identifiers of context/ checklist items.

  sdt context checklist backfill [--dry-run]   stamp a missing id anchor on every
                                               checklist item (idempotent; writes
                                               .sdt/cache/checklist-backfill.json)`,
}

var contextChecklistBackfillCmd = &cobra.Command{
	Use:   ctxBackfillVerb,
	Short: "Stamp a missing id anchor on every context/ checklist item",
	Long: `Stamp a missing ` + "`<!-- c<N> -->`" + ` anchor on every checklist item of every
context/ document (task files, plan Phases/Completion criteria, questions and
any other checklist), normalize an item's anchor onto its checklist line, and
renumber a repeated ` + "`c<N>`" + ` so every id addresses exactly one item.
Existing distinct anchors are preserved, so the command is idempotent. A real run
writes the marker
` + "`.sdt/cache/checklist-backfill.json`" + ` so lint starts treating a missing
anchor as a WARNING; --dry-run only reports.

Examples:
  sdt context checklist backfill
  sdt context checklist backfill --dry-run
  sdt context checklist backfill --dry-run --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		res, err := stampChecklistFiles(getBoolFlag(cmd, "dry-run", false))
		exitWithError(cmd, err)
		outputChecklistBackfill(cmd, res)
	},
}

func init() {
	contextChecklistBackfillCmd.Flags().Bool("dry-run", false, "Report without writing anything")
	contextChecklistCmd.AddCommand(contextChecklistBackfillCmd)
}
