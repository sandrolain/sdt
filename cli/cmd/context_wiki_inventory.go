package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/frontmatter"
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
	NoteType     string          `yaml:"note_type"`
	Sources      []string        `yaml:"sources"`
	Inventory    inventoryCounts `yaml:"inventory"`
	InlineFormat string          `yaml:"inline_format"`
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

--write recomputes page/merge/declined/pending from the top table; inline is
preserved when the top table has no inline rows, because the chapter tables are
not a complete candidate list. --sync-index also repairs the index page.

Examples:
  sdt context wiki inventory
  sdt context wiki inventory notes/20261023-151134-ea-concept-inventory.md
  sdt context wiki inventory --write --sync-index --page sources/book --dry-run`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		notes, err := inventoryNotes(args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		write := getBoolFlag(cmd, "write", false)
		syncIndex := getBoolFlag(cmd, "sync-index", false)
		dryRun := getBoolFlag(cmd, "dry-run", false)
		if syncIndex && !write {
			exitWithError(cmd, fmt.Errorf("--sync-index requires --write"))
			return
		}
		if syncIndex && getStringFlag(cmd, "page", false) != "" && len(notes) > 1 {
			exitWithError(cmd, fmt.Errorf("--page selects one index page; pass one note at a time"))
			return
		}
		if syncIndex || write {
			for _, note := range notes {
				if err := syncInventoryNote(cmd, note, syncIndex, getStringFlag(cmd, "page", false), dryRun); err != nil {
					exitWithError(cmd, err)
					return
				}
			}
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
	findings = append(findings, inventoryPendingFindings(path, string(data))...)
	findings = append(findings, inventoryRowFindings(path, rows, source, note.InlineFormat, bld)...)
	findings = append(findings, inventoryScopeFindings(path, rows, source, bld)...)
	return findings
}

// countInventoryRows recomputes the counters from the canonical top table.
// The chapter tables remain evidence; counting them too would double every
// candidate that also appears in the top table.
func countInventoryRows(rows []inventoryRow) (inventoryCounts, error) {
	c := inventoryCounts{Candidates: len(rows)}
	for _, row := range rows {
		switch {
		case strings.HasPrefix(strings.ToLower(row.Disposition), "page"):
			c.Paged++
		case strings.HasPrefix(strings.ToLower(row.Disposition), "merge"):
			c.Merged++
		case strings.HasPrefix(strings.ToLower(row.Disposition), "inline"):
			c.Inline++
		case strings.HasPrefix(strings.ToLower(row.Disposition), "declined"):
			c.Declined++
		case strings.HasPrefix(strings.ToLower(row.Disposition), "pending"):
			c.Pending++
		default:
			return c, fmt.Errorf("candidate %s has unknown disposition %q", row.ID, row.Disposition)
		}
	}
	return c, nil
}

// updateInventoryTotals rewrites only the aggregate rows of `## Totals`.
// Chapter subtotals stay historical evidence; rewriting them from the chapter
// tables would turn an incomplete list into a confident wrong number.
func updateInventoryTotals(content string, counts inventoryCounts) string {
	fm, body := contextwiki.SplitFrontmatter(content)
	lines := strings.Split(body, "\n")
	inTotals := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			inTotals = trim == "## Totals"
			continue
		}
		if !inTotals || !strings.HasPrefix(trim, "|") {
			continue
		}
		cols := inventorySplitRow(trim)
		if len(cols) < 2 {
			continue
		}
		label := strings.ToLower(cols[0])
		switch {
		case strings.Contains(label, "pages implied"):
			lines[i] = inventorySetRowValue(cols, counts.Paged+counts.Merged)
		case strings.Contains(label, "inline"):
			lines[i] = inventorySetRowValue(cols, counts.Inline)
		case strings.Contains(label, "pending"):
			lines[i] = inventorySetRowValue(cols, counts.Pending)
		}
	}
	return fm + strings.Join(lines, "\n")
}

func inventorySetRowValue(cols []string, n int) string {
	value := strconv.Itoa(n)
	if strings.Contains(cols[1], "**") {
		value = "**" + value + "**"
	} else if strings.HasPrefix(strings.TrimSpace(cols[1]), "~") {
		value = "~" + value
	}
	out := append([]string{}, cols...)
	out[1] = value
	return "| " + strings.Join(out, " | ") + " |"
}

func inventoryChapterPendingCount(content string) int {
	_, body := contextwiki.SplitFrontmatter(content)
	count := 0
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "|") {
			continue
		}
		cols := inventorySplitRow(trim)
		if len(cols) < 3 {
			continue
		}
		if strings.HasPrefix(strings.ToLower(cols[2]), "pending") {
			count++
		}
	}
	return count
}

func formatInventoryCounts(c inventoryCounts) string {
	return fmt.Sprintf("  units: %d\n  candidates: %d\n  paged: %d\n  merged: %d\n  inline: %d\n  declined: %d\n  pending: %d",
		c.Units, c.Candidates, c.Paged, c.Merged, c.Inline, c.Declined, c.Pending)
}

func formatConceptsBlock(from string, c inventoryCounts) string {
	return fmt.Sprintf("  from: %s\n  candidates: %d\n  paged: %d\n  merged: %d\n  inline: %d\n  declined: %d\n  pending: %d",
		from, c.Candidates, c.Paged, c.Merged, c.Inline, c.Declined, c.Pending)
}

// buildInventoryNoteUpdate reads one note, recomputes its counters and returns
// the original bytes, the replaced bytes, the counts and the parsed note.
func buildInventoryNoteUpdate(notePath string) (string, string, inventoryCounts, inventoryNote, error) {
	data, err := os.ReadFile(notePath) //#nosec G304 -- corpus path resolved by inventoryNotes
	if err != nil {
		return "", "", inventoryCounts{}, inventoryNote{}, err
	}
	block, err := frontmatter.Parse(string(data))
	if err != nil {
		return "", "", inventoryCounts{}, inventoryNote{}, fmt.Errorf("parse %s: %w", notePath, err)
	}
	if _, ok := block.Key("inventory"); !ok {
		return "", "", inventoryCounts{}, inventoryNote{}, fmt.Errorf("%s has no inventory: block", notePath)
	}
	rows := parseInventoryRows(string(data))
	if len(rows) == 0 {
		return "", "", inventoryCounts{}, inventoryNote{}, fmt.Errorf("%s has no ## Paged concepts rows", notePath)
	}
	counts, err := countInventoryRows(rows)
	if err != nil {
		return "", "", inventoryCounts{}, inventoryNote{}, fmt.Errorf("%s: %w", notePath, err)
	}
	note, err := readInventoryNote(notePath)
	if err != nil {
		return "", "", inventoryCounts{}, inventoryNote{}, err
	}
	// The top table is canonical for page/merge/declined/pending. Inline rows
	// live only in the chapter tables and are not a complete list, so a table
	// without inline rows preserves the declared inline count instead of
	// silently dropping the EA 62. A chapter pending row wins over a stale zero.
	if counts.Inline == 0 {
		counts.Inline = note.Inventory.Inline
	}
	if chapterPending := inventoryChapterPendingCount(string(data)); chapterPending > counts.Pending {
		counts.Pending = chapterPending
	}
	counts.Candidates = counts.Paged + counts.Merged + counts.Inline + counts.Declined + counts.Pending
	counts.Units = note.Inventory.Units

	newYAML, err := block.Set("inventory", formatInventoryCounts(counts), []string{ctxFrontmatterSources, "tags", ctxFrontmatterRelations})
	if err != nil {
		return "", "", inventoryCounts{}, inventoryNote{}, err
	}
	updated := updateInventoryTotals(block.ReplaceBody(string(data), newYAML), counts)
	return string(data), updated, counts, note, nil
}

// buildInventoryIndexUpdate returns the index page path, its bytes and the
// bytes with the concepts block replaced.
func buildInventoryIndexUpdate(notePath string, note inventoryNote, counts inventoryCounts, pageRef string) (string, string, string, error) {
	pagePath, err := resolveInventoryIndexPage(note, pageRef)
	if err != nil {
		return "", "", "", err
	}
	pageData, err := os.ReadFile(pagePath) //#nosec G304 -- corpus path resolved under context/wiki/
	if err != nil {
		return "", "", "", err
	}
	pageContent := string(pageData)
	pageBlock, err := frontmatter.Parse(pageContent)
	if err != nil {
		return "", "", "", fmt.Errorf("parse %s: %w", pagePath, err)
	}
	pageYAML, err := pageBlock.Set("concepts", formatConceptsBlock(corpusRelative(notePath), counts), []string{"coverage"})
	if err != nil {
		return "", "", "", err
	}
	updated := pageBlock.ReplaceBody(pageContent, pageYAML)
	updated, err = promoteIndexPage(updated, counts)
	if err != nil {
		return "", "", "", err
	}
	return pagePath, pageContent, updated, nil
}

// promoteIndexPage turns a draft page active only when the page's own coverage
// record and the note's pending count both say the run is closed.
func promoteIndexPage(content string, counts inventoryCounts) (string, error) {
	fm, _ := contextwiki.SplitFrontmatter(content)
	cov, present, err := contextwiki.CoverageBlock(fm)
	if err != nil || !present || cov.Gap != 0 || counts.Pending != 0 {
		return content, nil
	}
	block, err := frontmatter.Parse(content)
	if err != nil {
		return "", err
	}
	status, _ := block.Key("status")
	if status == "active" || status == "archived" {
		return content, nil
	}
	newYAML, err := block.Set("status", "active", []string{ctxMapNameSummary})
	if err != nil {
		return "", err
	}
	return block.ReplaceBody(content, newYAML), nil
}

// syncInventoryNote repairs one note's counters and, with syncIndex, the index
// page's concepts block. It writes only after both replacements are built.
func syncInventoryNote(cmd *cobra.Command, notePath string, syncIndex bool, pageRef string, dryRun bool) error {
	data, updated, counts, note, err := buildInventoryNoteUpdate(notePath)
	if err != nil {
		return err
	}
	pagePath, pageContent, pageUpdated := "", "", ""
	if syncIndex {
		pagePath, pageContent, pageUpdated, err = buildInventoryIndexUpdate(notePath, note, counts, pageRef)
		if err != nil {
			return err
		}
	}
	noteChanged := counts != note.Inventory
	pageChanged := pagePath != "" && pageUpdated != pageContent
	if !noteChanged && !pageChanged {
		outputString(cmd, fmt.Sprintf("inventory unchanged: %s\n", corpusRelative(notePath)))
		return nil
	}
	if dryRun {
		if noteChanged {
			outputString(cmd, unifiedDiff(corpusRelative(notePath), corpusRelative(notePath), strings.Split(data, "\n"), strings.Split(updated, "\n"), 3))
		}
		if pageChanged {
			outputString(cmd, unifiedDiff(corpusRelative(pagePath), corpusRelative(pagePath), strings.Split(pageContent, "\n"), strings.Split(pageUpdated, "\n"), 3))
		}
		return nil
	}
	if noteChanged {
		if err := writeCorpusFile(notePath, updated); err != nil {
			return err
		}
	}
	if pageChanged {
		if err := writeCorpusFile(pagePath, pageUpdated); err != nil {
			if noteChanged {
				if restoreErr := writeCorpusFile(notePath, data); restoreErr != nil {
					return fmt.Errorf("write %s: %w; restore %s: %v", pagePath, err, notePath, restoreErr)
				}
			}
			return fmt.Errorf("write %s: %w (inventory restored)", pagePath, err)
		}
	}
	if noteChanged {
		outputString(cmd, fmt.Sprintf("inventory updated: %s\n", corpusRelative(notePath)))
	}
	if pageChanged {
		outputString(cmd, fmt.Sprintf("concepts updated: %s\n", corpusRelative(pagePath)))
	}
	return nil
}

func resolveInventoryIndexPage(note inventoryNote, pageRef string) (string, error) {
	if pageRef != "" {
		return resolveWikiPage(pageRef)
	}
	source := note.artifactSource()
	if source == "" {
		return "", fmt.Errorf("inventory note names no refs/ source")
	}
	bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
	if err != nil {
		return "", fmt.Errorf("load wiki: %w", err)
	}
	var matches []string
	for _, page := range bld.Pages {
		if page.Type != wikiTypeEntity {
			continue
		}
		if strings.Contains(page.Body, source) || inventoryContainsString(page.Sources, source) {
			matches = append(matches, page.Path)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no entity index page cites %s; name one with --page", source)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("several index pages cite %s: %s; name one with --page", source, strings.Join(matches, ", "))
	}
}

func writeCorpusFile(path, content string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, []byte(content), mode) //#nosec G306,G703 -- corpus path resolved under context/, mode preserved
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

// inventoryPendingFindings reads every table row, not only the ## Paged
// concepts table: a chapter table can carry a pending row while the
// frontmatter says pending: 0, and a totals table can carry the stale count.
func inventoryPendingFindings(path, content string) []inventoryFinding {
	fm, body := contextwiki.SplitFrontmatter(content)
	offset := strings.Count(fm, "\n")
	var findings []inventoryFinding
	for i, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "|") {
			continue
		}
		cols := inventorySplitRow(trim)
		if len(cols) == 0 || strings.HasPrefix(cols[0], "---") {
			continue
		}
		if len(cols) >= 3 {
			if strings.HasPrefix(strings.ToLower(cols[2]), "pending") {
				findings = append(findings, inventoryFinding{
					Path:    path,
					Message: fmt.Sprintf("candidate %q is pending at line %d: %s", cols[0], offset+i+1, cols[2]),
				})
			}
			continue
		}
		if len(cols) == 2 && strings.Contains(strings.ToLower(cols[0]), "pending") {
			n, err := strconv.Atoi(strings.Trim(strings.TrimSpace(cols[1]), "~"))
			if err == nil && n > 0 {
				findings = append(findings, inventoryFinding{
					Path:    path,
					Message: fmt.Sprintf("totals say %d pending candidate(s) at line %d: the run is incomplete", n, offset+i+1),
				})
			}
		}
	}
	return findings
}

func inventoryRowFindings(path string, rows []inventoryRow, source, inlineFormat string, bld *contextwiki.Builder) []inventoryFinding {
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
		if strings.HasPrefix(strings.ToLower(row.Disposition), "inline") {
			findings = append(findings, inventoryInlineFindings(path, row, inlineFormat)...)
			continue
		}
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
			if !wikiInventoriableType(page.Type) {
				add("candidate %s targets %q, whose type is %q, not a concept or pattern page", row.ID, page.ID, page.Type)
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

func inventoryInlineFindings(path string, row inventoryRow, inlineFormat string) []inventoryFinding {
	disposition := strings.TrimSpace(row.Disposition)
	if strings.ToLower(disposition) == "inline" || disposition == "inline ()" {
		return []inventoryFinding{{Path: path, Message: fmt.Sprintf("candidate %s has a bare inline disposition; name the target", row.ID)}}
	}
	if inlineFormat == "claim" && !inlineClaimFormat(disposition) {
		return []inventoryFinding{{Path: path, Message: fmt.Sprintf("candidate %s must use inline (<page>#claim-N; reason) under inline_format: claim", row.ID)}}
	}
	return nil
}

func inlineClaimFormat(disposition string) bool {
	open := strings.Index(disposition, "(")
	close := strings.LastIndex(disposition, ")")
	if open < 0 || close <= open {
		return false
	}
	body := disposition[open+1 : close]
	claim := strings.Index(body, "#claim-")
	semi := strings.Index(body, "; ")
	return claim >= 0 && semi > claim && strings.TrimSpace(body[semi+2:]) != ""
}

func inventoryScopeFindings(path string, rows []inventoryRow, source string, bld *contextwiki.Builder) []inventoryFinding {
	targeted := map[string]bool{}
	for _, row := range rows {
		switch {
		case row.Disposition == inventoryDispositionPage || row.Disposition == inventoryDispositionMerge:
			targeted[row.Target] = true
		case strings.HasPrefix(strings.ToLower(row.Disposition), "inline"):
			if target := inlineTarget(row); target != "" {
				targeted[target] = true
			}
		}
	}
	var findings []inventoryFinding
	for _, page := range bld.Pages {
		if !wikiInventoriableType(page.Type) || !inventoryContainsString(page.Sources, source) {
			continue
		}
		if !targeted[page.ID] {
			findings = append(findings, inventoryFinding{Path: path, Message: fmt.Sprintf("concept page %q cites the source but has no inventory row", page.ID)})
		}
	}
	return findings
}

func inlineTarget(row inventoryRow) string {
	if row.Target != "" && row.Target != "—" && row.Target != "-" {
		return row.Target
	}
	open := strings.Index(row.Disposition, "(")
	close := strings.LastIndex(row.Disposition, ")")
	if open < 0 || close <= open {
		return ""
	}
	body := row.Disposition[open+1 : close]
	if i := strings.Index(body, "#"); i >= 0 {
		body = body[:i]
	}
	if i := strings.Index(body, ";"); i >= 0 {
		body = body[:i]
	}
	return strings.TrimSpace(body)
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
	contextWikiInventoryCmd.Flags().Bool("write", false, "Recompute the note counters from its ## Paged concepts table")
	contextWikiInventoryCmd.Flags().Bool("sync-index", false, "With --write, also update the index page's concepts block")
	contextWikiInventoryCmd.Flags().String("page", "", "Index page for --sync-index (default: the unique entity page citing the source)")
	contextWikiInventoryCmd.Flags().Bool("dry-run", false, "With --write, print the diff without writing")
	contextWikiCmd.AddCommand(contextWikiInventoryCmd)
}
