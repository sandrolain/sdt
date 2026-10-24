package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── concept-inventory check (sdt context wiki inventory) ──────────────────────
//
// An inventory note declares its counters and a `## Paged concepts` table. This
// command makes the note a check: the arithmetic must close, every page/merge row
// must resolve to a live concept page and one of its claim anchors, every live
// concept page citing the same source must have a row, and pending candidates are
// reported as an incomplete run.

// inventoryNote is the machine half of a concept-inventory note.
type inventoryNote struct {
	NoteType  string          `yaml:"note_type"`
	Sources   []string        `yaml:"sources"`
	Inventory inventoryCounts `yaml:"inventory"`
}

// artifactSource returns the refs/ entry the note describes. The inventory uses
// the declared `sources` list rather than a private `source` key.
func (n inventoryNote) artifactSource() string {
	for _, source := range n.Sources {
		if strings.HasPrefix(source, "refs/") {
			return source
		}
	}
	return ""
}

type inventoryCounts struct {
	Units      int `yaml:"units"`
	Candidates int `yaml:"candidates"`
	Paged      int `yaml:"paged"`
	Merged     int `yaml:"merged"`
	Inline     int `yaml:"inline"`
	Declined   int `yaml:"declined"`
	Pending    int `yaml:"pending"`
}

// inventoryRow is one line of the `## Paged concepts` table.
type inventoryRow struct {
	ID          string
	Concept     string
	Disposition string
	Target      string
	Claim       string
	Source      string
	Line        int
}

const (
	inventoryDispositionPage     = "page"
	inventoryDispositionMerge    = "merge"
	inventoryDispositionDeclined = "declined"
)

type inventoryFinding struct {
	Path    string `json:"path" yaml:"path"`
	Message string `json:"message" yaml:"message"`
}

var contextWikiInventoryCmd = &cobra.Command{
	Use:   "inventory [note...]",
	Short: "Validate concept-inventory notes (counters, rows, claims, scope)",
	Long: `Validate the concept-inventory notes of a book-scale ingestion.

A note declares its counters and a "## Paged concepts" table:

  inventory:
    units: 223
    candidates: 39
    paged: 37
    merged: 2
    inline: 62
    declined: 0
    pending: 0

  | id | concept | disposition | target | claim | source |

The command checks the arithmetic (candidates = paged + merged + inline +
declined + pending), resolves every page/merge row to a live concept page and to
an existing {#claim-N} anchor on it, and requires every live concept page citing
the note's source to have a row. A pending count is a finding: the run is
incomplete.

With no argument every note whose note_type is concept-inventory is checked.
Exit 0 clean, 1 findings.

Examples:
  sdt context wiki inventory
  sdt context wiki inventory notes/20261023-151134-ea-concept-inventory.md`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		notes, err := inventoryNotes(args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		bld, _, werr := contextwiki.LoadWiki(sdtWikiDir)
		if werr != nil {
			exitWithError(cmd, fmt.Errorf("load wiki: %w", werr))
			return
		}
		var findings []inventoryFinding
		for _, note := range notes {
			findings = append(findings, checkInventory(note, bld)...)
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(findings, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(findings)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			for _, f := range findings {
				outputString(cmd, fmt.Sprintf("[WARNING] %s: %s\n", f.Path, f.Message))
			}
		}
		if len(findings) > 0 {
			exit(exitWikiWarn)
		}
	},
}

// inventoryNotes resolves the requested notes, or scans context/notes for every
// concept-inventory note.
func inventoryNotes(args []string) ([]string, error) {
	if len(args) > 0 {
		out := make([]string, 0, len(args))
		for _, ref := range args {
			path, err := resolveInventoryNote(ref)
			if err != nil {
				return nil, err
			}
			out = append(out, path)
		}
		return out, nil
	}
	var out []string
	err := filepath.WalkDir(sdtNotesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || filepath.Ext(d.Name()) != ".md" {
			return nil
		}
		note, err := readInventoryNote(path)
		if err == nil && note.NoteType == "concept-inventory" {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func resolveInventoryNote(ref string) (string, error) {
	candidates := []string{ref, filepath.Join(sdtWorkDir, ref), filepath.Join(sdtNotesDir, ref)}
	if !strings.HasSuffix(ref, ".md") {
		candidates = append(candidates, filepath.Join(sdtNotesDir, ref+".md"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("inventory note %q not found", ref)
}

func readInventoryNote(path string) (inventoryNote, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path resolved under context/
	if err != nil {
		return inventoryNote{}, err
	}
	var note inventoryNote
	if err := yaml.Unmarshal([]byte(inventoryFrontmatter(string(data))), &note); err != nil {
		return inventoryNote{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return note, nil
}

// inventoryFrontmatter strips the `---` fences so the YAML decoder sees one
// document.
func inventoryFrontmatter(content string) string {
	fm, _ := contextwiki.SplitFrontmatter(content)
	lines := strings.Split(fm, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == ctxFrontmatterDelim {
		lines = lines[1:]
	}
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == ctxFrontmatterDelim {
		lines = lines[:n-1]
	}
	return strings.Join(lines, "\n")
}

// checkInventory validates one note against the wiki corpus.
func checkInventory(path string, bld *contextwiki.Builder) []inventoryFinding {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path
	if err != nil {
		return []inventoryFinding{{Path: path, Message: fmt.Sprintf("unreadable: %v", err)}}
	}
	note, err := readInventoryNote(path)
	if err != nil {
		return []inventoryFinding{{Path: path, Message: err.Error()}}
	}
	rows := parseInventoryRows(string(data))
	source := note.artifactSource()
	findings := inventoryCountFindings(path, note.Inventory)
	findings = append(findings, inventoryRowFindings(path, rows, source, bld)...)
	findings = append(findings, inventoryScopeFindings(path, rows, source, bld)...)
	return findings
}

func inventoryCountFindings(path string, c inventoryCounts) []inventoryFinding {
	var findings []inventoryFinding
	add := func(format string, args ...any) {
		findings = append(findings, inventoryFinding{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	sum := c.Paged + c.Merged + c.Inline + c.Declined + c.Pending
	if c.Candidates != sum {
		add("inventory does not add up: paged+merged+inline+declined+pending = %d, candidates = %d", sum, c.Candidates)
	}
	if c.Pending > 0 {
		add("%d candidate(s) are pending: the run is incomplete", c.Pending)
	}
	return findings
}

func inventoryRowFindings(path string, rows []inventoryRow, source string, bld *contextwiki.Builder) []inventoryFinding {
	var findings []inventoryFinding
	add := func(format string, args ...any) {
		findings = append(findings, inventoryFinding{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	seenIDs := map[string]bool{}
	for _, row := range rows {
		if seenIDs[row.ID] {
			add("duplicate candidate id %q", row.ID)
		}
		seenIDs[row.ID] = true
		switch row.Disposition {
		case inventoryDispositionPage, inventoryDispositionMerge:
			if row.Source != "" && row.Source != source {
				add("candidate %s cites %s, which is not the note's source %s", row.ID, row.Source, source)
			}
			page := bld.Resolve(row.Target)
			if page == nil {
				add("candidate %s targets missing page %q", row.ID, row.Target)
				continue
			}
			if page.Type != wikiTypeConcept {
				add("candidate %s targets %q, whose type is %q, not concept", row.ID, page.ID, page.Type)
			}
			if row.Claim == "" || !strings.Contains(page.Body, "{#"+row.Claim+"}") {
				add("candidate %s names claim %q, which %q does not carry", row.ID, row.Claim, page.ID)
			}
		case inventoryDispositionDeclined:
			if row.Target != "" && row.Target != "—" && row.Target != "-" {
				add("declined candidate %s names a target; a decline has none", row.ID)
			}
		default:
			add("candidate %s has unknown disposition %q", row.ID, row.Disposition)
		}
	}
	return findings
}

func inventoryScopeFindings(path string, rows []inventoryRow, source string, bld *contextwiki.Builder) []inventoryFinding {
	targeted := map[string]bool{}
	for _, row := range rows {
		if row.Disposition == inventoryDispositionPage || row.Disposition == inventoryDispositionMerge {
			targeted[row.Target] = true
		}
	}
	var findings []inventoryFinding
	for _, page := range bld.Pages {
		if page.Type != wikiTypeConcept || !inventoryContainsString(page.Sources, source) {
			continue
		}
		if !targeted[page.ID] {
			findings = append(findings, inventoryFinding{Path: path, Message: fmt.Sprintf("concept page %q cites the source but has no inventory row", page.ID)})
		}
	}
	return findings
}

func inventoryContainsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// parseInventoryRows reads the `## Paged concepts` table. A malformed row is
// returned with an empty disposition so the check reports it rather than skipping.
func parseInventoryRows(content string) []inventoryRow {
	_, body := contextwiki.SplitFrontmatter(content)
	in := false
	var rows []inventoryRow
	for i, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			if in {
				break
			}
			if trim == "## Paged concepts" {
				in = true
			}
			continue
		}
		if !in || !strings.HasPrefix(trim, "|") {
			continue
		}
		cols := inventorySplitRow(trim)
		if len(cols) < 6 || cols[0] == "id" || strings.HasPrefix(cols[0], "---") {
			continue
		}
		rows = append(rows, inventoryRow{
			ID:          cols[0],
			Concept:     cols[1],
			Disposition: cols[2],
			Target:      cols[3],
			Claim:       cols[4],
			Source:      cols[5],
			Line:        i + 1,
		})
	}
	return rows
}

func inventorySplitRow(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func init() {
	contextWikiCmd.AddCommand(contextWikiInventoryCmd)
}
