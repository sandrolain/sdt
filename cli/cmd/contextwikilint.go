package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── wiki knowledge-graph lint (sdt context wiki lint) ──────────────────────────

// Status aliases shared beyond the wiki lint (context.go default status map).
const (
	ctxWikiStatusDraft  = contextwiki.StatusDraft
	ctxWikiStatusActive = contextwiki.StatusActive
)

// lint priorities for the wiki graph (reuses ctxLintCritical/Warning).
const ctxLintSuggestion = "SUGGESTION"

// Wiki exit codes: 0 clean / 1 warnings / 2 errors.
const (
	exitWikiWarn  = 1
	exitWikiError = 2
)

// wikiIssueBuilder gathers lint findings for one run.
type wikiIssueBuilder struct {
	issues []ctxLintIssue
}

func (b *wikiIssueBuilder) add(path, priority, format string, args ...any) {
	b.issues = append(b.issues, ctxLintIssue{Path: path, Priority: priority, Message: fmt.Sprintf(format, args...)})
}

// lintWikiLint runs all wiki checks across wiki/ and the source dirs.
func lintWikiLint() []ctxLintIssue {
	b := &wikiIssueBuilder{}
	bld := lintWikiLoad(b)
	lintWikiSchema(b, bld.Pages)
	lintWikiDuplicateKeys(b, bld.Pages)
	lintWikiClaims(b, bld.Pages)
	for _, p := range bld.Pages {
		// Wiki pages intentionally stay out of lintDoc's corpus route; apply only
		// the cross-kind axes promised to this dedicated loader using the source
		// text the shared rules expect.
		content := p.Frontmatter + "\n" + p.Body
		families := currentFrontmatterFamilies()
		if families["maturity"] {
			b.issues = append(b.issues, lintMaturity(p.Path, content, families["warrant"])...)
		}
		if families["verified"] {
			b.issues = append(b.issues, lintVerifiedStamp(p.Path, content)...)
		}
	}
	lintWikiLinks(b, bld)
	lintWikiGraph(b, bld)
	lintWikiBudget(b, bld.Pages)
	lintWikiDepth(b, bld.Pages)
	lintWikiCoverage(b, bld.Pages)
	lintWikiMarkdownLD(b, bld.Pages)
	lintWikiMarkers(b)
	return b.issues
}

// lintWikiLoad scans context/wiki recursively into a shared contextwiki.Builder
// (pages plus id/title indexes). Wiki subdirectories are allowed
// (wiki/<context>/<slug>.md); each page's FileID is the relative subpath minus
// ".md" (wiki/backend/auth.md → "backend/auth").
func lintWikiLoad(b *wikiIssueBuilder) *contextwiki.Builder {
	bld, errs, werr := contextwiki.LoadWiki(sdtWikiDir)
	if werr != nil {
		if os.IsNotExist(werr) {
			b.add(sdtWikiDir, ctxLintWarning, "directory missing: %s", sdtWikiDir)
		} else {
			b.add(sdtWikiDir, ctxLintCritical, "cannot read wiki directory: %v", werr)
		}
		return bld
	}
	for _, le := range errs {
		b.add(le.Path, ctxLintCritical, "unreadable: %v", le.Err)
	}
	return bld
}

// lintWikiSchema validates frontmatter fields, enums and per-claim citations.
func lintWikiSchema(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		switch {
		case p.Kind == "":
			b.add(p.Path, ctxLintCritical, "frontmatter missing `kind: wiki`")
		case p.Kind != contextwiki.Kind:
			b.add(p.Path, ctxLintCritical, "kind %q but must be that of a wiki page", p.Kind)
		}
		switch {
		case p.ID == "":
			b.add(p.Path, ctxLintCritical, "missing mandatory `id` (must equal the file id %q)", p.FileID)
		case p.ID != p.FileID:
			b.add(p.Path, ctxLintCritical, "`id` %q does not match the file id %q", p.ID, p.FileID)
		}
		if p.Title == "" || p.Title == p.FileID && !p.ExplicitTitle() {
			b.add(p.Path, ctxLintCritical, "missing mandatory `title`")
		}
		if p.Summary == "" {
			b.add(p.Path, ctxLintCritical, "missing mandatory `summary`")
		}
		if !contextwiki.Types[p.Type] {
			b.add(p.Path, ctxLintCritical, "unknown `type` %q (use concept|entity|decision|pattern|module)", p.Type)
		}
		if !contextwiki.Statuses[p.Status] {
			b.add(p.Path, ctxLintCritical, "%s", wikiBrokenReason(p.Status))
		}
		if len(p.Tags) == 0 {
			b.add(p.Path, ctxLintCritical, "missing mandatory `tags`")
		}
		if p.Verified != "" && p.Verified != contextwiki.VerifiedTrue && p.Verified != contextwiki.VerifiedFalse {
			b.add(p.Path, ctxLintWarning, "`verified` must be true or false when present, got %q", p.Verified)
		}
		for _, cm := range contextwiki.ClaimRegexp.FindAllStringIndex(p.Body, -1) {
			if !contextwiki.CitationRegexp.MatchString(lineAt(p.Body, cm[0])) {
				b.add(p.Path, ctxLintWarning, "claim %s has no refs/ citation", p.Body[cm[0]:cm[1]])
			}
		}
		// Verify the hash of every body citation against context/refs/ (the wiki
		// pages are outside the corpus-wide lint, so the check runs here too).
		b.issues = append(b.issues, lintCitationText(p.Path, p.Body)...)
		for _, t := range p.Tags {
			if !contextwiki.CleanTag(t) {
				b.add(p.Path, ctxLintWarning, "malformed tag %q (use lowercase a-z 0-9 - /)", t)
			}
		}
	}
}

// wikiBrokenReason explains an out-of-vocabulary status value.
func wikiBrokenReason(status string) string {
	enum := contextwiki.StatusDraft + "|" + contextwiki.StatusActive + "|" + contextwiki.StatusArchived
	if status == "" {
		return "missing `status` (must be " + enum + ")"
	}
	return fmt.Sprintf("status %q is not %s", status, enum)
}

// lintWikiLinks validates closed verbs, relation targets and body wikilinks.
func lintWikiLinks(b *wikiIssueBuilder, bld *contextwiki.Builder) {
	for _, p := range bld.Pages {
		lintWikiRelations(b, p, bld)
		lintWikiBodyLinks(b, p, bld)
	}
	lintWikiTitleUniqueness(b, bld.Pages)
}

// lintWikiRelations validates the closed-verb relations block targets.
func lintWikiRelations(b *wikiIssueBuilder, p *contextwiki.Page, bld *contextwiki.Builder) {
	for verb, targets := range p.Relations {
		if verb == "" || !contextwiki.Verbs[verb] {
			b.add(p.Path, ctxLintCritical, "relation verb %q is outside the closed vocabulary", verb)
			continue
		}
		for _, t := range targets {
			m := contextwiki.LinkRegexp.FindStringSubmatch(t)
			if m == nil {
				b.add(p.Path, ctxLintWarning, "relation %s target %q is not a [[id|label]] link", verb, t)
				continue
			}
			if bld.Resolve(strings.TrimSpace(m[1])) == nil {
				b.add(p.Path, ctxLintWarning, "relation %s targets missing page %q", verb, strings.TrimSpace(m[1]))
			}
		}
	}
}

// lintWikiBodyLinks validates the [[...]] occurrences in the page body.
func lintWikiBodyLinks(b *wikiIssueBuilder, p *contextwiki.Page, bld *contextwiki.Builder) {
	for _, l := range p.Links {
		if l.Verb != "" && !contextwiki.Verbs[l.Verb] {
			b.add(p.Path, ctxLintCritical, "body link verb %q is outside the closed vocabulary", l.Verb)
			continue
		}
		if l.Verb != "" && l.Target == "" {
			b.add(p.Path, ctxLintWarning, "typed link %q has no target title", l.Raw)
			continue
		}
		if l.Verb == "" && strings.HasPrefix(l.Target, "http") {
			continue // external URL
		}
		switch {
		case len(bld.ByTitle[l.Target]) > 1:
			b.add(p.Path, ctxLintCritical, "link target %q is ambiguous (%d pages share the title)", l.Target, len(bld.ByTitle[l.Target]))
		case len(bld.ByTitle[l.Target]) == 1, bld.ByID[l.Target] != nil:
		default:
			b.add(p.Path, ctxLintWarning, "broken link %q (no page with id or title %q)", l.Raw, l.Target)
		}
	}
}

// lintWikiDuplicateKeys flags a top-level frontmatter key repeated in one page.
// The single-value readers keep the first occurrence and drop the rest, so the
// page cannot be read the way it is written; that is a fact about the document,
// hence CRITICAL, reported without touching the parse.
func lintWikiDuplicateKeys(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		for _, dup := range contextwiki.DuplicateFrontmatterKeys(p.Frontmatter) {
			b.add(p.Path, ctxLintCritical, "duplicate frontmatter key `%s` %s", dup.Key, ctxLineList(dup.Lines))
		}
	}
}

// claimAnchor is one `{#claim-N}` occurrence with its 1-based file line.
type claimAnchor struct {
	Number int
	Line   int
}

// claimAnchors reads a page's claim anchors with file line numbers. The body
// starts after the frontmatter block, so the offset's newline count is shifted by
// the block's own line span.
func claimAnchors(p *contextwiki.Page) []claimAnchor {
	base := strings.Count(p.Frontmatter, "\n") + 1
	var anchors []claimAnchor
	for _, loc := range contextwiki.ClaimRegexp.FindAllStringIndex(p.Body, -1) {
		text := strings.TrimSuffix(strings.TrimPrefix(p.Body[loc[0]:loc[1]], "{#claim-"), "}")
		n, err := strconv.Atoi(text)
		if err != nil {
			continue
		}
		anchors = append(anchors, claimAnchor{
			Number: n,
			Line:   base + strings.Count(p.Body[:loc[0]], "\n"),
		})
	}
	return anchors
}

// lintWikiClaims flags claim anchors that are repeated (CRITICAL: a citation to
// that number is ambiguous) or that leave a gap in the numbering (WARNING: the
// residue of a deleted claim, a hint rather than a lie). A page with a duplicate
// reports only the duplicate — the gap it implies adds nothing.
func lintWikiClaims(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		byNumber := map[int][]int{}
		var order []int
		for _, a := range claimAnchors(p) {
			if _, known := byNumber[a.Number]; !known {
				order = append(order, a.Number)
			}
			byNumber[a.Number] = append(byNumber[a.Number], a.Line)
		}
		duplicated := false
		for _, n := range order {
			if lines := byNumber[n]; len(lines) > 1 {
				duplicated = true
				b.add(p.Path, ctxLintCritical, "duplicate claim anchor {#claim-%d} %s", n, ctxLineList(lines))
			}
		}
		if duplicated || len(order) == 0 {
			continue
		}
		highest := order[0]
		for _, n := range order[1:] {
			if n > highest {
				highest = n
			}
		}
		var missing []int
		for n := 1; n <= highest; n++ {
			if _, present := byNumber[n]; !present {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			b.add(p.Path, ctxLintWarning, "claim anchors are not contiguous: %s", ctxAnchorList(missing))
		}
	}
}

// ctxLineList renders cited file lines as a reader says them: "at line 7",
// "at lines 7 and 9", "at lines 7, 9 and 12".
func ctxLineList(lines []int) string {
	if len(lines) == 0 {
		return ""
	}
	parts := make([]string, len(lines))
	for i, n := range lines {
		parts[i] = strconv.Itoa(n)
	}
	if len(parts) == 1 {
		return "at line " + parts[0]
	}
	return "at lines " + strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// ctxAnchorList names the missing claim numbers.
func ctxAnchorList(numbers []int) string {
	parts := make([]string, len(numbers))
	for i, n := range numbers {
		parts[i] = fmt.Sprintf("{#claim-%d}", n)
	}
	noun := "missing claim anchor"
	if len(parts) > 1 {
		noun = "missing claim anchors"
	}
	return noun + " " + strings.Join(parts, ", ")
}

// lintWikiTitleUniqueness flags duplicate explicit titles across wiki pages.
func lintWikiTitleUniqueness(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	seen := map[string]bool{}
	for _, p := range pages {
		if !p.ExplicitTitle() {
			continue
		}
		ttl := strings.TrimSpace(p.Title)
		if seen[ttl] {
			b.add(p.Path, ctxLintCritical, "duplicate title %q across wiki pages", ttl)
		}
		seen[ttl] = true
	}
}

// lintWikiGraph checks orphans, depends_on cycles and supersede semantics.
func lintWikiGraph(b *wikiIssueBuilder, bld *contextwiki.Builder) {
	inbound := bld.Inbound()
	for _, p := range bld.Pages {
		if p.Active() && len(inbound[p.ID]) == 0 {
			b.add(p.Path, ctxLintSuggestion, "orphan page: no inbound relations/wikilinks from active pages")
		}
	}
	for path := range bld.Cycles() {
		b.add(path, ctxLintCritical, "depends_on cycle detected (page participates in a dependency loop)")
	}
	for _, p := range bld.Pages {
		lintWikiSupersedes(b, p, bld, inbound)
	}
}

// lintWikiSupersedes checks supersede semantics against the target page state.
func lintWikiSupersedes(b *wikiIssueBuilder, p *contextwiki.Page, bld *contextwiki.Builder, inbound map[string][]string) {
	for _, t := range p.Relations["supersedes"] {
		m := contextwiki.LinkRegexp.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		tgt := bld.Resolve(strings.TrimSpace(m[1]))
		if tgt == nil {
			continue
		}
		if tgt.Active() {
			b.add(p.Path, ctxLintWarning, "supersedes %q but target is still active", tgt.ID)
		}
		if tgt.Status == contextwiki.StatusArchived && len(inbound[tgt.ID]) > 0 {
			b.add(p.Path, ctxLintWarning, "archived superseded page %q still has inbound relations", tgt.ID)
		}
	}
}

// lintWikiBudget flags pages over the frozen concept budget as a split signal.
func lintWikiBudget(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		// A source's index page carries the coverage record's `## Structure`
		// list — one line per unit — so its length is the source's, never a
		// concept's; the concept budget does not apply to it (wiki.md, *The
		// coverage record*). A block that does not parse is lintWikiCoverage's
		// finding, not a reason to apply the line budget here.
		_, hasCoverage, covErr := contextwiki.CoverageBlock(p.Frontmatter)
		if !hasCoverage && covErr == nil && p.LineCount > contextwiki.BudgetLines {
			b.add(p.Path, ctxLintWarning, "page is %d lines (budget ~%d; counted as `sdt read --count`): over budget → split", p.LineCount, contextwiki.BudgetLines)
		}
		if p.ClaimCount > contextwiki.BudgetClaims {
			b.add(p.Path, ctxLintWarning, "page has %d claims (budget ~%d): over budget → split", p.ClaimCount, contextwiki.BudgetClaims)
		}
	}
}

// Density floor for a distilled concept page. The concept budget is a ceiling;
// without a floor a 7-claim, 40-word page passes every check while carrying one
// umbrella concept. The numbers are calibrated on the 2026-10-19 pages (11-16
// claims, 64-89 summary words) and are a SUGGESTION, never a target to pad.
const (
	wikiTypeConcept          = "concept"
	wikiDepthMinClaims       = 8
	wikiDepthMinSummaryWords = 50
)

// lintWikiDepth flags a concept page below the density floor. Index pages (a
// coverage block) and non-concept pages are exempt: the floor is about the
// knowledge layer, not source frames or maps.
func lintWikiDepth(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		if p.Type != wikiTypeConcept {
			continue
		}
		if _, hasCoverage, covErr := contextwiki.CoverageBlock(p.Frontmatter); hasCoverage || covErr != nil {
			continue
		}
		if p.ClaimCount < wikiDepthMinClaims {
			b.add(p.Path, ctxLintSuggestion, "page has %d claim(s); the density floor is %d — the concept inventory expects more", p.ClaimCount, wikiDepthMinClaims)
		}
		if words := wikiSummaryWords(p.Body); words < wikiDepthMinSummaryWords {
			b.add(p.Path, ctxLintSuggestion, "summary is %d word(s); the density floor is %d — restate the knowledge, do not name the topic", words, wikiDepthMinSummaryWords)
		}
	}
}

// wikiSummaryWords counts the words of the page's `## Summary` section. A
// missing section is 0, so the finding names the absence instead of skipping it.
func wikiSummaryWords(body string) int {
	in := false
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			if in {
				break
			}
			if trim == "## Summary" {
				in = true
			}
			continue
		}
		if in {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return len(strings.Fields(b.String()))
}

// lintWikiCoverage checks the countable half of a book-scale run's coverage
// record: a malformed block, a sum that does not match `units`, and a non-zero
// gap. A page with no block is not a finding — whether a source was book-scale
// is the declared route's job (ingestion.md, Ingestion receipt), not a shape the
// tool can infer.
func lintWikiCoverage(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		c, present, err := contextwiki.CoverageBlock(p.Frontmatter)
		if !present {
			continue
		}
		if err != nil {
			b.add(p.Path, ctxLintWarning, "coverage block is malformed: %v", err)
			continue
		}
		if sum := c.OK + c.Part + c.Gap + c.NoOp; sum != c.Units {
			b.add(p.Path, ctxLintWarning, "coverage does not add up: ok+part+gap+no_op = %d, units = %d", sum, c.Units)
		}
		if c.Gap > 0 {
			b.add(p.Path, ctxLintWarning, "coverage leaves %d unit(s) as gap: the run is incomplete — distil them or disposition them", c.Gap)
		}
	}
}

// lintWikiMarkdownLD validates optional markdown-ld JSON blocks.
func lintWikiMarkdownLD(b *wikiIssueBuilder, pages []*contextwiki.Page) {
	for _, p := range pages {
		if !p.HasLD && !strings.Contains(p.Body, "\"@context\"") && !strings.Contains(p.Body, "\"@type\"") {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(markdownLDJSON(p.Body)), &v); err != nil {
			b.add(p.Path, ctxLintWarning, "markdown-ld JSON is not well-formed")
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := m["name"].(string); ok && name != "" && name != p.Title {
			b.add(p.Path, ctxLintWarning, "markdown-ld name %q does not match title %q", name, p.Title)
		}
	}
}

var ctxWikiMarkdownLDPrag = regexp.MustCompile(`<!--\s*markdown-ld\s*-->`)

// lintWikiMarkers validates the immutable source dirs that carry a required
// marker (ingestion/ pending, kind: reference). refs/ is deliberately excluded
// from lint: it holds large immutable external captures that are not required
// to follow the marker contract.
func lintWikiMarkers(b *wikiIssueBuilder) {
	dirs := []struct {
		dir, wantStatus string
	}{
		{sdtIngestionDir, contextwiki.MarkerPending},
	}
	for _, d := range dirs {
		entries, err := os.ReadDir(d.dir)
		if err != nil {
			if os.IsNotExist(err) {
				b.add(d.dir, ctxLintWarning, "directory missing: %s", d.dir)
				continue
			}
			b.add(d.dir, ctxLintCritical, "cannot read %s: %v", d.dir, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != sdtMarkdownExt {
				continue
			}
			f := filepath.Join(d.dir, e.Name())
			p, err := contextwiki.ParsePage(d.dir, f)
			if err != nil {
				b.add(f, ctxLintCritical, "unreadable: %v", err)
				continue
			}
			base := filepath.Base(d.dir)
			if p.Kind != contextwiki.KindReference {
				b.add(f, ctxLintCritical, "marker `kind` %q must be %q in %s/", p.Kind, contextwiki.KindReference, base)
			}
			if p.Status != d.wantStatus {
				b.add(f, ctxLintCritical, "marker `status` %q must be %q in %s/", p.Status, d.wantStatus, base)
			}
		}
	}
}

// lineAt returns the content of the line containing byte offset i.
func lineAt(s string, i int) string {
	start := strings.LastIndex(s[:i], "\n") + 1
	end := strings.Index(s[i:], "\n")
	if end < 0 {
		end = len(s)
	} else {
		end = i + end
	}
	return s[start:end]
}

// markdownLDJSON locates the markdown-ld pragma and returns the outermost
// balanced JSON object from just past it; without a pragma it scans the whole
// body (gated by an @context/@type hint in the caller).
func markdownLDJSON(body string) string {
	from := 0
	if m := ctxWikiMarkdownLDPrag.FindStringIndex(body); m != nil {
		from = m[1]
	}
	return extractJSONAt(body, from)
}

// extractJSONAt scans s from the given byte offset for the first '{' and
// returns the outermost balanced JSON object; falls back to trimmed s.
func extractJSONAt(s string, from int) string {
	i := strings.Index(s[from:], "{")
	if i < 0 {
		return s
	}
	i += from
	s = s[i:]
	depth := 0
	for j, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:j+1]
			}
		}
	}
	return s
}

var contextWikiLintCmd = &cobra.Command{
	Use:   gateStepLint,
	Short: "Validate the wiki knowledge graph and source markers",
	Long: `Validate the knowledge pipeline: wiki/ pages (schema, closed relation
verbs, link resolution, unique titles, depends_on cycles, supersede semantics,
claim citations, tags, concept budget, optional markdown-ld JSON), plus the
immutable source markers under ingestion/ (status: pending). refs/ is excluded
from lint (immutable external captures).

wiki/ is scanned recursively (wiki/<context>/<slug>.md allowed): a page's id
must equal its relative subpath minus ".md" (wiki/backend/auth.md → id:
backend/auth).

Exit codes: 0 clean, 1 warnings only, 2 errors.

Examples:
  sdt context wiki lint
  sdt context wiki lint --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		issues := lintWikiLint()
		if issues == nil {
			issues = []ctxLintIssue{}
		}
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].Priority != issues[j].Priority {
				prio := map[string]int{ctxLintCritical: 0, ctxLintWarning: 1, ctxLintSuggestion: 2}
				return prio[issues[i].Priority] < prio[issues[j].Priority]
			}
			return issues[i].Path < issues[j].Path
		})
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(issues, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(issues)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, it := range issues {
				outputString(cmd, it.String()+"\n")
			}
		}
		errCount, warnCount := 0, 0
		for _, it := range issues {
			switch it.Priority {
			case ctxLintCritical:
				errCount++
			case ctxLintWarning:
				warnCount++
			}
		}
		if errCount > 0 {
			exit(exitWikiError)
			return
		}
		if warnCount > 0 {
			exit(exitWikiWarn)
		}
	},
}

var contextWikiCmd = &cobra.Command{
	Use:   ctxTypeWiki,
	Short: "Knowledge graph operations (wiki lint)",
	Long: `Knowledge-pipeline commands for the wiki/ knowledge graph under context/.
Currently provides "lint": recursive schema/graph validation of wiki pages
(including wiki/<context>/<slug>.md subdirectories) and the immutable source
markers (ingestion/ status: pending, refs/ status: archived).`,
}

func init() {
	contextWikiCmd.AddCommand(contextWikiLintCmd)
	contextCmd.AddCommand(contextWikiCmd)
}
