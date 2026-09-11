package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ── wiki knowledge-graph lint (sdt context wiki lint) ──────────────────────────

// ctxWikiKindReference is the kind marker on the immutable source dirs.
const ctxWikiKindReference = "reference"

// Strand string literals shared across wiki checks (goconst).
const (
	ctxWikiKind           = "wiki"
	ctxWikiStatusDraft    = "draft"
	ctxWikiStatusActive   = "active"
	ctxWikiStatusArchived = "archived"
	ctxWikiMarkerPending  = "pending"
	ctxWikiVerifiedTrue   = "true"
	ctxWikiVerifiedFalse  = "false"
)

// lint priorities for the wiki graph (reuses ctxLintCritical/Warning).
const ctxLintSuggestion = "SUGGESTION"

// ctxWikiVerbs is the closed relation vocabulary. Extending the set is a schema
// change: a verb outside it fails lint.
var ctxWikiVerbs = map[string]bool{
	"supersedes":     true,
	"depends_on":     true,
	"refers_to":      true,
	"refines":        true,
	"implements":     true,
	"conflicts_with": true,
	"part_of":        true,
	"contains":       true,
}

// ctxWikiTypes and statuses are closed enums for wiki page frontmatter.
var (
	ctxWikiTypes    = map[string]bool{"concept": true, "entity": true, "decision": true, "pattern": true, "module": true}
	ctxWikiStatuses = map[string]bool{ctxWikiStatusDraft: true, ctxWikiStatusActive: true, ctxWikiStatusArchived: true}
)

// Concept budget defaults (schema-frozen): a page over either threshold is a
// split signal → WARNING, never an error.
const (
	ctxWikiBudgetLines  = 120
	ctxWikiBudgetClaims = 20
)

// Wiki exit codes: 0 clean / 1 warnings / 2 errors.
const (
	exitWikiWarn  = 1
	exitWikiError = 2
)

var (
	ctxWikiLinkRegexp     = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	ctxWikiClaimRegexp    = regexp.MustCompile(`\{#claim-\d+\}`)
	ctxWikiCitationRegexp = regexp.MustCompile(`refs/[a-zA-Z0-9_./-]+`)
	ctxWikiTagRegexp      = regexp.MustCompile(`^[a-z0-9]+(?:[-/][a-z0-9]+)*$`)
	ctxWikiMarkdownLDPrag = regexp.MustCompile(`<!--\s*markdown-ld\s*-->`)
)

// wikiPage is one parsed knowledge-graph node.
type wikiPage struct {
	Path        string
	Base        string
	FileID      string
	Frontmatter string // full outer frontmatter incl. ---- delimiters
	Body        string
	Kind        string
	ID          string
	Title       string
	Type        string
	Status      string
	Summary     string
	Verified    string
	Relations   map[string][]string // verb → raw [[id|label]] targets
	Tags        []string
	Sources     []string
	Links       []wikiLink // every [[...]] occurrence
	ClaimCount  int
	LineCount   int
	HasLD       bool // markdown-ld pragma present
}

// wikiLink is a [[...]] occurrence; Verb is "" for plain links.
type wikiLink struct {
	Raw    string
	Verb   string
	Target string
}

func (p *wikiPage) active() bool { return p.Status == ctxWikiStatusActive }

// splitFrontmatter returns the full outer "---\n...\n---" block (or "" when
// absent) and the body after it.
func splitFrontmatter(content string) (fm string, body string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	bodyStart := strings.Index(content[len("---\n"):], "\n---")
	if bodyStart < 0 {
		return content, ""
	}
	bodyStart += len("---\n") + len("\n---")
	return content[:bodyStart+2], content[bodyStart+2:]
}

// parseFrontmatterMap reads a YAML block-map frontmatter field
// (key:\n  verb:\n    - item). Returns nil when absent; a map when present.
// An inline value on the key line is reported via warnOnly.
func parseFrontmatterMap(fm, key string) (map[string][]string, bool) {
	out := map[string][]string{}
	in := false
	cur := ""
	for _, line := range strings.Split(fm, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if !in {
			if strings.HasPrefix(line, key+":") {
				val := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
				if val != "" {
					return nil, true // inline map; not supported
				}
				in = true
			}
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			break // next top-level key
		}
		if indent >= 2 && cur != "" && strings.HasPrefix(trim, "-") {
			out[cur] = append(out[cur], strings.TrimSpace(strings.TrimPrefix(trim, "-")))
			continue
		}
		if indent == 2 && strings.HasSuffix(trim, ":") {
			cur = strings.TrimSuffix(trim, ":")
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, false
}

// parseWikiPage parses a wiki/refs/ingestion file into a wikiPage. dir is the
// directory the file lives in (or its root for nested wiki pages); the FileID
// is the dir-relative path minus ".md" (flat files keep their basename slug).
func parseWikiPage(dir, path string) (*wikiPage, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- fixed repo path
	if err != nil {
		return nil, err
	}
	content := string(data)
	fm, body := splitFrontmatter(content)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	fileID := filepath.ToSlash(strings.TrimSuffix(rel, sdtMarkdownExt))
	p := &wikiPage{
		Path:        path,
		Base:        filepath.Base(path),
		FileID:      fileID,
		Frontmatter: fm,
		Body:        body,
		Kind:        parseFrontmatterField(fm, "kind"),
		ID:          parseFrontmatterField(fm, "id"),
		Title:       parseFrontmatterField(fm, "title"),
		Type:        parseFrontmatterField(fm, "type"),
		Status:      parseFrontmatterField(fm, "status"),
		Summary:     parseFrontmatterField(fm, "summary"),
		Verified:    parseFrontmatterField(fm, "verified"),
		Tags:        parseFrontmatterList(fm, "tags"),
		Sources:     parseFrontmatterList(fm, "sources"),
		LineCount:   strings.Count(content, "\n") + 1,
		ClaimCount:  len(ctxWikiClaimRegexp.FindAllString(body, -1)),
		HasLD:       ctxWikiMarkdownLDPrag.MatchString(body),
	}
	if rels, inline := parseFrontmatterMap(fm, "relations"); rels != nil || inline {
		p.Relations = rels
	}
	if strings.TrimSpace(p.Title) == "" {
		p.Title = fileID // plain link resolution falls back to the slug
	}
	for _, m := range ctxWikiLinkRegexp.FindAllStringSubmatch(fm+"\n"+body, -1) {
		l := wikiLink{Raw: m[0], Target: strings.TrimSpace(m[1])}
		if i := strings.Index(l.Target, "::"); i >= 0 {
			l.Verb = strings.TrimSpace(l.Target[:i])
			l.Target = strings.TrimSpace(l.Target[i+2:])
		}
		p.Links = append(p.Links, l)
	}
	return p, nil
}

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
	pages, byID, byTitle := lintWikiLoad(b)
	lintWikiSchema(b, pages)
	lintWikiLinks(b, pages, byID, byTitle)
	lintWikiGraph(b, pages, byID, byTitle)
	lintWikiBudget(b, pages)
	lintWikiMarkdownLD(b, pages)
	lintWikiMarkers(b)
	return b.issues
}

// lintWikiLoad scans context/wiki recursively into pages plus id/title indexes.
// Wiki subdirectories are allowed (wiki/<context>/<slug>.md); each page's FileID
// is the relative subpath minus ".md" (wiki/backend/auth.md → "backend/auth").
func lintWikiLoad(b *wikiIssueBuilder) ([]*wikiPage, map[string]*wikiPage, map[string][]*wikiPage) {
	var pages []*wikiPage
	byID := map[string]*wikiPage{}
	byTitle := map[string][]*wikiPage{}
	var rel []string
	err := filepath.WalkDir(sdtWikiDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != sdtMarkdownExt {
			return nil
		}
		r, err := filepath.Rel(sdtWikiDir, path)
		if err != nil {
			return err
		}
		rel = append(rel, r)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			b.add(sdtWikiDir, ctxLintWarning, "directory missing: %s", sdtWikiDir)
		} else {
			b.add(sdtWikiDir, ctxLintCritical, "cannot read wiki directory: %v", err)
		}
		return pages, byID, byTitle
	}
	sort.Strings(rel)
	for _, r := range rel {
		path := filepath.Join(sdtWikiDir, r)
		p, err := parseWikiPage(sdtWikiDir, path)
		if err != nil {
			b.add(path, ctxLintCritical, "unreadable: %v", err)
			continue
		}
		pages = append(pages, p)
		if p.ID != "" {
			byID[p.ID] = p
		}
		byTitle[p.Title] = append(byTitle[p.Title], p)
	}
	return pages, byID, byTitle
}

// lintWikiSchema validates frontmatter fields, enums and per-claim citations.
func lintWikiSchema(b *wikiIssueBuilder, pages []*wikiPage) {
	for _, p := range pages {
		switch {
		case p.Kind == "":
			b.add(p.Path, ctxLintCritical, "frontmatter missing `kind: wiki`")
		case p.Kind != ctxWikiKind:
			b.add(p.Path, ctxLintCritical, "kind %q but must be that of a wiki page", p.Kind)
		}
		switch {
		case p.ID == "":
			b.add(p.Path, ctxLintCritical, "missing mandatory `id` (must equal the file id %q)", p.FileID)
		case p.ID != p.FileID:
			b.add(p.Path, ctxLintCritical, "`id` %q does not match the file id %q", p.ID, p.FileID)
		}
		if p.Title == "" || p.Title == p.FileID && !hasTitleField(p) {
			b.add(p.Path, ctxLintCritical, "missing mandatory `title`")
		}
		if p.Summary == "" {
			b.add(p.Path, ctxLintCritical, "missing mandatory `summary`")
		}
		if !ctxWikiTypes[p.Type] {
			b.add(p.Path, ctxLintCritical, "unknown `type` %q (use concept|entity|decision|pattern|module)", p.Type)
		}
		if !ctxWikiStatuses[p.Status] {
			b.add(p.Path, ctxLintCritical, "%s", p.brokenReason())
		}
		if len(p.Tags) == 0 {
			b.add(p.Path, ctxLintCritical, "missing mandatory `tags`")
		}
		if p.Verified != "" && p.Verified != ctxWikiVerifiedTrue && p.Verified != ctxWikiVerifiedFalse {
			b.add(p.Path, ctxLintWarning, "`verified` must be true or false when present, got %q", p.Verified)
		}
		for _, cm := range ctxWikiClaimRegexp.FindAllStringIndex(p.Body, -1) {
			if !ctxWikiCitationRegexp.MatchString(lineAt(p.Body, cm[0])) {
				b.add(p.Path, ctxLintWarning, "claim %s has no refs/ citation", p.Body[cm[0]:cm[1]])
			}
		}
		for _, t := range p.Tags {
			if !cleanTag(t) {
				b.add(p.Path, ctxLintWarning, "malformed tag %q (use lowercase a-z 0-9 - /)", t)
			}
		}
	}
}

// lintWikiLinks validates closed verbs, relation targets and body wikilinks.
func lintWikiLinks(b *wikiIssueBuilder, pages []*wikiPage, byID map[string]*wikiPage, byTitle map[string][]*wikiPage) {
	for _, p := range pages {
		lintWikiRelations(b, p, byID, byTitle)
		lintWikiBodyLinks(b, p, byID, byTitle)
	}
	lintWikiTitleUniqueness(b, pages)
}

// lintWikiRelations validates the closed-verb relations block targets.
func lintWikiRelations(b *wikiIssueBuilder, p *wikiPage, byID map[string]*wikiPage, byTitle map[string][]*wikiPage) {
	for verb, targets := range p.Relations {
		if verb == "" || !ctxWikiVerbs[verb] {
			b.add(p.Path, ctxLintCritical, "relation verb %q is outside the closed vocabulary", verb)
			continue
		}
		for _, t := range targets {
			m := ctxWikiLinkRegexp.FindStringSubmatch(t)
			if m == nil {
				b.add(p.Path, ctxLintWarning, "relation %s target %q is not a [[id|label]] link", verb, t)
				continue
			}
			if resolveWikiTarget(byID, byTitle, strings.TrimSpace(m[1])) == nil {
				b.add(p.Path, ctxLintWarning, "relation %s targets missing page %q", verb, strings.TrimSpace(m[1]))
			}
		}
	}
}

// lintWikiBodyLinks validates the [[...]] occurrences in the page body.
func lintWikiBodyLinks(b *wikiIssueBuilder, p *wikiPage, byID map[string]*wikiPage, byTitle map[string][]*wikiPage) {
	for _, l := range p.Links {
		if l.Verb != "" && !ctxWikiVerbs[l.Verb] {
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
		case len(byTitle[l.Target]) > 1:
			b.add(p.Path, ctxLintCritical, "link target %q is ambiguous (%d pages share the title)", l.Target, len(byTitle[l.Target]))
		case len(byTitle[l.Target]) == 1, byID[l.Target] != nil:
		default:
			b.add(p.Path, ctxLintWarning, "broken link %q (no page with id or title %q)", l.Raw, l.Target)
		}
	}
}

// lintWikiTitleUniqueness flags duplicate explicit titles across wiki pages.
func lintWikiTitleUniqueness(b *wikiIssueBuilder, pages []*wikiPage) {
	seen := map[string]bool{}
	for _, p := range pages {
		if !hasTitleField(p) {
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
func lintWikiGraph(b *wikiIssueBuilder, pages []*wikiPage, byID map[string]*wikiPage, byTitle map[string][]*wikiPage) {
	inbound := wikiInbound(pages, byID, byTitle)
	for _, p := range pages {
		if p.active() && len(inbound[p.ID]) == 0 {
			b.add(p.Path, ctxLintSuggestion, "orphan page: no inbound relations/wikilinks from active pages")
		}
	}
	for path := range wikiCycles(pages, byID) {
		b.add(path, ctxLintCritical, "depends_on cycle detected (page participates in a dependency loop)")
	}
	for _, p := range pages {
		lintWikiSupersedes(b, p, byID, byTitle, inbound)
	}
}

// wikiInbound maps each page id to the ids of pages referencing it through
// relations or typed body links.
func wikiInbound(pages []*wikiPage, byID map[string]*wikiPage, byTitle map[string][]*wikiPage) map[string][]string {
	inbound := map[string][]string{}
	for _, p := range pages {
		for _, targets := range p.Relations {
			for _, raw := range targets {
				if m := ctxWikiLinkRegexp.FindStringSubmatch(raw); m != nil {
					if t := resolveWikiTarget(byID, byTitle, strings.TrimSpace(m[1])); t != nil {
						inbound[t.ID] = append(inbound[t.ID], p.ID)
					}
				}
			}
		}
		for _, l := range p.Links {
			if l.Verb == "" {
				continue
			}
			if t := resolveWikiTarget(byID, byTitle, l.Target); t != nil {
				inbound[t.ID] = append(inbound[t.ID], p.ID)
			}
		}
	}
	return inbound
}

// lintWikiSupersedes checks supersede semantics against the target page state.
func lintWikiSupersedes(b *wikiIssueBuilder, p *wikiPage, byID map[string]*wikiPage, byTitle map[string][]*wikiPage, inbound map[string][]string) {
	for _, t := range p.Relations["supersedes"] {
		m := ctxWikiLinkRegexp.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		tgt := resolveWikiTarget(byID, byTitle, strings.TrimSpace(m[1]))
		if tgt == nil {
			continue
		}
		if tgt.active() {
			b.add(p.Path, ctxLintWarning, "supersedes %q but target is still active", tgt.ID)
		}
		if tgt.Status == ctxWikiStatusArchived && len(inbound[tgt.ID]) > 0 {
			b.add(p.Path, ctxLintWarning, "archived superseded page %q still has inbound relations", tgt.ID)
		}
	}
}

// lintWikiBudget flags pages over the frozen concept budget as a split signal.
func lintWikiBudget(b *wikiIssueBuilder, pages []*wikiPage) {
	for _, p := range pages {
		if p.LineCount > ctxWikiBudgetLines {
			b.add(p.Path, ctxLintWarning, "page is %d lines (budget ~%d): over budget → split", p.LineCount, ctxWikiBudgetLines)
		}
		if p.ClaimCount > ctxWikiBudgetClaims {
			b.add(p.Path, ctxLintWarning, "page has %d claims (budget ~%d): over budget → split", p.ClaimCount, ctxWikiBudgetClaims)
		}
	}
}

// lintWikiMarkdownLD validates optional markdown-ld JSON blocks.
func lintWikiMarkdownLD(b *wikiIssueBuilder, pages []*wikiPage) {
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

// lintWikiMarkers validates the immutable source dirs (ingestion/ pending, refs/
// archived, both kind: reference).
func lintWikiMarkers(b *wikiIssueBuilder) {
	dirs := []struct {
		dir, wantStatus string
	}{
		{sdtIngestionDir, ctxWikiMarkerPending},
		{sdtRefsDir, ctxWikiStatusArchived},
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
			p, err := parseWikiPage(d.dir, f)
			if err != nil {
				b.add(f, ctxLintCritical, "unreadable: %v", err)
				continue
			}
			base := filepath.Base(d.dir)
			if p.Kind != ctxWikiKindReference {
				b.add(f, ctxLintCritical, "marker `kind` %q must be %q in %s/", p.Kind, ctxWikiKindReference, base)
			}
			if p.Status != d.wantStatus {
				b.add(f, ctxLintCritical, "marker `status` %q must be %q in %s/", p.Status, d.wantStatus, base)
			}
		}
	}
}

func (p *wikiPage) brokenReason() string {
	enum := ctxWikiStatusDraft + "|" + ctxWikiStatusActive + "|" + ctxWikiStatusArchived
	if p.Status == "" {
		return "missing `status` (must be " + enum + ")"
	}
	return fmt.Sprintf("status %q is not %s", p.Status, enum)
}

// hasTitleField reports whether the frontmatter declares an explicit title.
func hasTitleField(p *wikiPage) bool {
	return parseFrontmatterField(p.Frontmatter, "title") != ""
}

// cleanTag reports whether a tag matches the strict form.
func cleanTag(t string) bool { return ctxWikiTagRegexp.MatchString(t) }

// resolveWikiTarget resolves a relation/link target by id (primary), then by a
// unique matching title.
func resolveWikiTarget(byID map[string]*wikiPage, byTitle map[string][]*wikiPage, target string) *wikiPage {
	s := strings.TrimSpace(target)
	if p, ok := byID[s]; ok {
		return p
	}
	if list := byTitle[s]; len(list) == 1 {
		return list[0]
	}
	return nil
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

// wikiCycles detects depends_on dependency loops with a pure-Go DFS. Returns the
// paths of pages participating in a cycle.
func wikiCycles(pages []*wikiPage, byID map[string]*wikiPage) map[string]bool {
	adj := map[string][]string{}
	pathOf := map[string]string{}
	for _, p := range pages {
		pathOf[p.ID] = p.Path
		for _, t := range p.Relations["depends_on"] {
			if m := ctxWikiLinkRegexp.FindStringSubmatch(t); m != nil {
				if tgt := byID[strings.TrimSpace(m[1])]; tgt != nil {
					adj[p.ID] = append(adj[p.ID], tgt.ID)
				}
			}
		}
	}
	var stack []string
	onStack := map[string]bool{}
	visited := map[string]bool{}
	inCycle := map[string]bool{}
	var dfs func(id string)
	dfs = func(id string) {
		if onStack[id] {
			inCycle[id] = true
			return
		}
		if visited[id] {
			return
		}
		visited[id] = true
		onStack[id] = true
		stack = append(stack, id)
		for _, n := range adj[id] {
			dfs(n)
		}
		stack = stack[:len(stack)-1]
		onStack[id] = false
	}
	for _, p := range pages {
		dfs(p.ID)
	}
	res := map[string]bool{}
	for id := range inCycle {
		if ph, ok := pathOf[id]; ok {
			res[ph] = true
		}
	}
	return res
}

var contextWikiLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate the wiki knowledge graph and source markers",
	Long: `Validate the knowledge pipeline: wiki/ pages (schema, closed relation
verbs, link resolution, unique titles, depends_on cycles, supersede semantics,
claim citations, tags, concept budget, optional markdown-ld JSON), plus the
immutable source markers under ingestion/ (status: pending) and refs/
(status: archived).

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
	Use:   "wiki",
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
