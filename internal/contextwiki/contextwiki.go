// Package contextwiki is the shared wiki knowledge-graph builder used by both
// the sdt CLI (context wiki lint) and the sdtviewer binary (ADR-0003). It
// parses wiki pages, resolves id/title targets, validates relations and
// computes inbound neighbors so lint and visualization never drift.
package contextwiki

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Closed vocabulary and schema-frozen constants for wiki page frontmatter.
// Extending the verb set is a schema change: a verb outside Verbs fails lint.
const (
	// Kind is the frontmatter kind of a wiki page.
	Kind = "wiki"
	// KindReference is the kind marker on the immutable source dirs
	// (ingestion/ + refs/).
	KindReference = "reference"
	// Status lifecycle values for wiki pages and reference markers.
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"
	// MarkerPending is the expected status inside ingestion/.
	MarkerPending = "pending"
	// VerifiedTrue / VerifiedFalse are the accepted `verified` field values.
	VerifiedTrue  = "true"
	VerifiedFalse = "false"
	// MarkdownExt is the corpus file extension.
	MarkdownExt = ".md"
	// frontmatterDelim delimits a YAML frontmatter block.
	frontmatterDelim = "---"
)

// Verbs is the closed relation vocabulary.
var Verbs = map[string]bool{
	"supersedes":     true,
	"depends_on":     true,
	"refers_to":      true,
	"refines":        true,
	"implements":     true,
	"conflicts_with": true,
	"part_of":        true,
	"contains":       true,
}

// Types are the closed wiki page `type` values.
var Types = map[string]bool{"concept": true, "entity": true, "decision": true, "pattern": true, "module": true}

// Statuses are the closed wiki page `status` values.
var Statuses = map[string]bool{StatusDraft: true, StatusActive: true, StatusArchived: true}

// Budget defaults (schema-frozen): a page over either threshold is a split
// signal, never an error.
const (
	BudgetLines  = 120
	BudgetClaims = 20
)

var (
	// LinkRegexp matches a [[target|label]] wikilink occurrence.
	LinkRegexp = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	// ClaimRegexp matches a claim anchor in the page body.
	ClaimRegexp = regexp.MustCompile(`\{#claim-\d+\}`)
	// CitationRegexp matches a refs/ citation path on the claim line.
	CitationRegexp = regexp.MustCompile(`refs/[a-zA-Z0-9_./-]+`)
	// TagRegexp is the strict tag form (lowercase a-z 0-9 - /).
	TagRegexp = regexp.MustCompile(`^[a-z0-9]+(?:[-/][a-z0-9]+)*$`)
	// MarkdownLDPrag matches the optional markdown-ld pragma.
	MarkdownLDPrag = regexp.MustCompile(`<!--\s*markdown-ld\s*-->`)
)

// Page is one parsed wiki knowledge-graph node.
type Page struct {
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
	Links       []Link // every [[...]] occurrence
	ClaimCount  int
	LineCount   int
	HasLD       bool // markdown-ld pragma present
}

// Link is a [[...]] occurrence; Verb is "" for plain links.
type Link struct {
	Raw    string
	Verb   string
	Target string
}

// Active reports whether the page status is active.
func (p *Page) Active() bool { return p.Status == StatusActive }

// ExplicitTitle reports whether the frontmatter declares an explicit title.
func (p *Page) ExplicitTitle() bool { return FrontmatterField(p.Frontmatter, "title") != "" }

// SplitFrontmatter returns the full outer "---\n...\n---" block (or "" when
// absent) and the body after it.
func SplitFrontmatter(content string) (fm string, body string) {
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

// FrontmatterField returns the value of a top-level YAML frontmatter key, or ""
// when absent or malformed.
func FrontmatterField(content, key string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != frontmatterDelim {
		return ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == frontmatterDelim {
			return ""
		}
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

// FrontmatterList parses a YAML block-list frontmatter field (a line `key:`
// followed by `  - item` lines), returning the list items. Falls back to a
// single inline value when present. Returns nil when the key is absent.
func FrontmatterList(content, key string) []string {
	lines := strings.Split(content, "\n")
	var out []string
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != frontmatterDelim {
		return nil
	}
	in := false
	for _, line := range lines[1:] {
		trim := strings.TrimSpace(line)
		if trim == frontmatterDelim {
			break
		}
		if !in {
			if strings.HasPrefix(line, key+":") {
				val := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
				if val != "" {
					out = append(out, strings.TrimSpace(strings.Trim(val, `"'`)))
				}
				in = true
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "-") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-")))
		} else if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// a new top-level key ends the list
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ParseFrontmatterMap reads a YAML block-map frontmatter field
// (key:\n  verb:\n    - item). Returns nil when absent; a map when present.
// An inline value on the key line is reported via the bool.
func ParseFrontmatterMap(fm, key string) (map[string][]string, bool) {
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

// ParseContent builds a Page for the given full file content and file id
// (the dir-relative path minus ".md"). path is only used for Page.Path/Base.
func ParseContent(content, path, fileID string) *Page {
	fm, body := SplitFrontmatter(content)
	p := &Page{
		Path:        path,
		Base:        filepath.Base(path),
		FileID:      fileID,
		Frontmatter: fm,
		Body:        body,
		Kind:        FrontmatterField(fm, "kind"),
		ID:          FrontmatterField(fm, "id"),
		Title:       FrontmatterField(fm, "title"),
		Type:        FrontmatterField(fm, "type"),
		Status:      FrontmatterField(fm, "status"),
		Summary:     FrontmatterField(fm, "summary"),
		Verified:    FrontmatterField(fm, "verified"),
		Tags:        FrontmatterList(fm, "tags"),
		Sources:     FrontmatterList(fm, "sources"),
		LineCount:   strings.Count(content, "\n") + 1,
		ClaimCount:  len(ClaimRegexp.FindAllString(body, -1)),
		HasLD:       MarkdownLDPrag.MatchString(body),
	}
	if rels, inline := ParseFrontmatterMap(fm, "relations"); rels != nil || inline {
		p.Relations = rels
	}
	if strings.TrimSpace(p.Title) == "" {
		p.Title = fileID // plain link resolution falls back to the slug
	}
	for _, m := range LinkRegexp.FindAllStringSubmatch(fm+"\n"+body, -1) {
		l := Link{Raw: m[0], Target: strings.TrimSpace(m[1])}
		if i := strings.Index(l.Target, "::"); i >= 0 {
			l.Verb = strings.TrimSpace(l.Target[:i])
			l.Target = strings.TrimSpace(l.Target[i+2:])
		}
		p.Links = append(p.Links, l)
	}
	return p
}

// ParsePage reads and parses a wiki file. dir is the directory the file lives
// in; the FileID is the dir-relative path minus ".md" (flat files keep their
// basename slug, nested pages their relative subpath).
func ParsePage(dir, path string) (*Page, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- corpus path from the walker/caller
	if err != nil {
		return nil, err
	}
	content := string(data)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	fileID := filepath.ToSlash(strings.TrimSuffix(rel, MarkdownExt))
	return ParseContent(content, path, fileID), nil
}

// Builder is the loaded wiki corpus: every page plus id/title indexes used for
// target resolution, inbound computation and cycle detection.
type Builder struct {
	Pages   []*Page
	ByID    map[string]*Page
	ByTitle map[string][]*Page
}

// LoadError reports one unresolvable resource during LoadWiki.
type LoadError struct {
	Path string
	Err  error
}

// LoadWiki walks dir recursively for .md pages, parses each into a Page and
// builds the id/title indexes. A root-level scan error (missing or unreadable
// directory) is returned as walkErr; unreadable page files are reported in
// errs. The Builder is always returned, potentially partial.
func LoadWiki(dir string) (bld *Builder, errs []LoadError, walkErr error) {
	bld = &Builder{ByID: map[string]*Page{}, ByTitle: map[string][]*Page{}}
	var rel []string
	walkErr = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != MarkdownExt {
			return nil
		}
		r, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = append(rel, r)
		return nil
	})
	if walkErr != nil {
		return bld, nil, walkErr
	}
	sort.Strings(rel)
	for _, r := range rel {
		path := filepath.Join(dir, r)
		p, err := ParsePage(dir, path)
		if err != nil {
			errs = append(errs, LoadError{Path: path, Err: err})
			continue
		}
		bld.add(p)
	}
	return bld, errs, nil
}

// CleanTag reports whether a tag matches the strict form.
func CleanTag(t string) bool { return TagRegexp.MatchString(t) }

// Resolve resolves a relation/link target by id (primary), then by a unique
// matching title.
func (b *Builder) Resolve(target string) *Page {
	s := strings.TrimSpace(target)
	if p, ok := b.ByID[s]; ok {
		return p
	}
	if list := b.ByTitle[s]; len(list) == 1 {
		return list[0]
	}
	return nil
}

// Inbound maps each page id to the ids of pages referencing it through
// relations or typed body links.
func (b *Builder) Inbound() map[string][]string {
	inbound := map[string][]string{}
	for _, p := range b.Pages {
		for _, targets := range p.Relations {
			for _, raw := range targets {
				if m := LinkRegexp.FindStringSubmatch(raw); m != nil {
					if t := b.Resolve(strings.TrimSpace(m[1])); t != nil {
						inbound[t.ID] = append(inbound[t.ID], p.ID)
					}
				}
			}
		}
		for _, l := range p.Links {
			if l.Verb == "" {
				continue
			}
			if t := b.Resolve(l.Target); t != nil {
				inbound[t.ID] = append(inbound[t.ID], p.ID)
			}
		}
	}
	return inbound
}

// Cycles detects depends_on dependency loops with a pure-Go DFS. Returns the
// paths of pages participating in a cycle.
func (b *Builder) Cycles() map[string]bool {
	adj := map[string][]string{}
	pathOf := map[string]string{}
	for _, p := range b.Pages {
		pathOf[p.ID] = p.Path
		for _, t := range p.Relations["depends_on"] {
			if m := LinkRegexp.FindStringSubmatch(t); m != nil {
				if tgt := b.ByID[strings.TrimSpace(m[1])]; tgt != nil {
					adj[p.ID] = append(adj[p.ID], tgt.ID)
				}
			}
		}
	}
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
		for _, n := range adj[id] {
			dfs(n)
		}
		onStack[id] = false
	}
	for _, p := range b.Pages {
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

func (b *Builder) add(p *Page) {
	b.Pages = append(b.Pages, p)
	if p.ID != "" {
		b.ByID[p.ID] = p
	}
	b.ByTitle[p.Title] = append(b.ByTitle[p.Title], p)
}
