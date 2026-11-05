//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/ctxrel"
	"github.com/sandrolain/sdt/internal/frontmatter"
	"github.com/spf13/cobra"
	yamlv3 "go.yaml.in/yaml/v3"
)

// ctxMapScope is the caveat the map prints before anything else, in every
// format. A map that lists the corpus is not a map of everything the project
// knows: reading an empty section as "this does not exist" is the failure it
// exists to prevent.
const ctxMapScope = "This map lists corpus documents. An empty section does not establish that a source, decision or reference is absent; context/refs, context/tmp, context/trash and context/sdtdocs are outside it."

// ctxMapCount is one labelled count in a stable order.
type ctxMapCount struct {
	Name  string `json:"name"  yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

// ctxMapDocument is one kind's row in the Documents section.
type ctxMapDocument struct {
	Kind     string        `json:"kind"     yaml:"kind"`
	Dir      string        `json:"dir"      yaml:"dir"`
	Count    int           `json:"count"    yaml:"count"`
	Statuses []ctxMapCount `json:"statuses" yaml:"statuses"`
	Newest   string        `json:"newest"   yaml:"newest"`
}

// ctxMapKey is one frontmatter key's inventory row.
type ctxMapKey struct {
	Key      string `json:"key"      yaml:"key"`
	Type     string `json:"type"     yaml:"type"`
	Coverage string `json:"coverage" yaml:"coverage"`
	Example  string `json:"example"  yaml:"example"`
}

// ctxMapObjective is one objective's roll-up.
type ctxMapObjective struct {
	Objective string `json:"objective" yaml:"objective"`
	Documents int    `json:"documents" yaml:"documents"`
	Analyses  int    `json:"analyses"  yaml:"analyses"`
	Plans     int    `json:"plans"     yaml:"plans"`
	Active    int    `json:"active"    yaml:"active"`
}

// ctxMapRelations is the traceability slice: how many documents declare their
// sources and links, and which declare neither.
type ctxMapRelations struct {
	WithSources int      `json:"with_sources" yaml:"with_sources"`
	WithLinks   int      `json:"with_links"   yaml:"with_links"`
	Orphans     int      `json:"orphans"      yaml:"orphans"`
	OrphanRefs  []string `json:"orphan_refs"  yaml:"orphan_refs"`
	ParentEdges int      `json:"parent_edges" yaml:"parent_edges"`
	// Lifecycle is the denominator of ParentEdges: the analysis, plan and task
	// documents a parent edge can belong to.
	Lifecycle int `json:"lifecycle" yaml:"lifecycle"`
}

// ctxMapGotcha is one awkward fact the map surfaces.
type ctxMapGotcha struct {
	Name   string `json:"name"   yaml:"name"`
	Count  int    `json:"count"  yaml:"count"`
	Detail string `json:"detail" yaml:"detail"`
}

// ctxMapTry is one query derived from the map itself.
type ctxMapTry struct {
	Query string `json:"query" yaml:"query"`
	Why   string `json:"why"   yaml:"why"`
}

// ctxMapReport is the whole map, in the shape every output format renders.
type ctxMapReport struct {
	Scope       string            `json:"scope"       yaml:"scope"`
	Corpus      string            `json:"corpus"      yaml:"corpus"`
	Total       int               `json:"total"       yaml:"total"`
	Documents   []ctxMapDocument  `json:"documents"   yaml:"documents"`
	Frontmatter []ctxMapKey       `json:"frontmatter" yaml:"frontmatter"`
	Objectives  []ctxMapObjective `json:"objectives"  yaml:"objectives"`
	Relations   ctxMapRelations   `json:"relations"   yaml:"relations"`
	Gotchas     []ctxMapGotcha    `json:"gotchas"     yaml:"gotchas"`
	Try         []ctxMapTry       `json:"try"         yaml:"try"`
}

// ctxMapDoc is one document as the map scans it: enough to aggregate every
// section without holding the document bodies.
type ctxMapDoc struct {
	Ref       string
	Kind      string
	Status    string
	Objective string
	Newest    string
	Sources   int
	Links     int
	NoteType  string
	Revival   string
	// Unparsed marks a document whose frontmatter the map could not read: still
	// counted, so a section cannot silently shrink, but surfaced as a gotcha.
	Unparsed bool
	Fields   map[string]ctxMapField
}

// ctxMapField is one frontmatter key's value shape on one document.
type ctxMapField struct {
	Type    string
	Example string
}

// The value shapes the Frontmatter section names, as constants because the same
// three words are written in the scan and asserted in the tests.
const (
	ctxMapTypeScalar = "scalar"
	ctxMapTypeList   = "list"
	ctxMapTypeMap    = "map"
)

// ctxMapBuild scans the corpus and assembles the map. objective narrows the
// scan; an empty objective covers every corpus document.
func ctxMapBuild(objective string) ctxMapReport {
	docs := ctxMapScan()
	if objective != "" {
		docs = ctxMapFilterObjective(docs, objective)
	}
	corpus, err := filepath.Abs(sdtWorkDir)
	if err != nil {
		corpus = sdtWorkDir
	}
	return ctxMapReport{
		Scope:       ctxMapScope,
		Corpus:      corpus,
		Total:       len(docs),
		Documents:   ctxMapDocuments(docs),
		Frontmatter: ctxMapFrontmatter(docs),
		Objectives:  ctxMapObjectives(docs),
		Relations:   ctxMapRelationsSection(docs),
		Gotchas:     ctxMapGotchas(docs),
		Try:         ctxMapTrySection(docs),
	}
}

// ctxMapScan reads every corpus document the index walks, classified by the
// type registry so the counts match `sdt context status` row for row. Each
// directory is walked once even when several kinds share it.
func ctxMapScan() []ctxMapDoc {
	cache := map[string][]string{}
	seen := map[string]bool{}
	var docs []ctxMapDoc
	for _, t := range contextTypeList {
		if ctxIndexRouteForType(t) != ctxIndexRouteCorpus {
			continue
		}
		files, ok := cache[t.dir]
		if !ok {
			var err error
			files, err = dirFilesFor(t.dir)
			if err != nil {
				files = nil // a missing directory is an empty one
			}
			cache[t.dir] = files
		}
		for _, f := range files {
			if seen[f] || !ctxFileMatchesKind(t, f) {
				continue
			}
			seen[f] = true
			docs = append(docs, ctxMapReadDoc(t.kind, f))
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Ref < docs[j].Ref })
	return docs
}

// ctxMapReadDoc reads one document's frontmatter into its aggregate row. A
// document the map cannot parse is still a document, so it is counted with no
// metadata and reported as a gotcha instead of silently shrinking a section.
func ctxMapReadDoc(kind, path string) ctxMapDoc {
	data, err := os.ReadFile(path) // #nosec G304 -- a corpus document the index already walks
	if err != nil {
		return ctxMapDoc{Ref: path, Kind: kind, Unparsed: true}
	}
	block, err := frontmatter.Parse(string(data))
	if err != nil {
		return ctxMapDoc{Ref: path, Kind: kind, Unparsed: true}
	}
	doc := ctxMapDoc{Ref: path, Kind: kind, Fields: ctxMapFields(block)}
	doc.Status, _ = block.Key("status")
	doc.Objective, _ = block.Key("objective")
	doc.NoteType, _ = block.Key("note_type")
	doc.Newest, _ = block.Key("created")
	if doc.Newest == "" {
		doc.Newest, _ = block.Key("updated")
	}
	if items, _, err := block.ListItems("sources"); err == nil {
		doc.Sources = len(items)
	}
	if items, _, err := block.ListItems("links"); err == nil {
		doc.Links = len(items)
	}
	if kind == ctxTypeAnalysis && strings.TrimSpace(doc.Status) == statusPostponed {
		doc.Revival = ctxMapRevival(string(data))
	}
	return doc
}

// ctxMapFields inventories the frontmatter keys of one document.
func ctxMapFields(block *frontmatter.Block) map[string]ctxMapField {
	fields := map[string]ctxMapField{}
	if block.Mapping == nil {
		return fields
	}
	content := block.Mapping.Content
	for i := 0; i+1 < len(content); i += 2 {
		key := content[i].Value
		value := content[i+1]
		field := ctxMapField{Type: ctxMapTypeScalar, Example: value.Value}
		switch value.Kind {
		case yamlv3.SequenceNode:
			field.Type = ctxMapTypeList
			if len(value.Content) > 0 {
				field.Example = value.Content[0].Value
			}
		case yamlv3.MappingNode:
			field.Type = ctxMapTypeMap
			field.Example = ""
		}
		fields[key] = field
	}
	return fields
}

// ctxMapRevival extracts a postponed analysis's revival condition — the line a
// reader needs to know when the decision should be reopened. Empty means the
// document states none, which the map reports rather than hides.
func ctxMapRevival(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(trimmed), "**revival condition") {
			continue
		}
		parts := []string{ctxMapPlain(trimmed)}
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				break
			}
			parts = append(parts, ctxMapPlain(strings.TrimSpace(next)))
		}
		return ctxMapTruncate(strings.Join(parts, " "), 160)
	}
	return ""
}

// ctxMapPlain strips the markdown emphasis a frontmatter-adjacent line carries.
func ctxMapPlain(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "**", ""))
}

// ctxMapFilterObjective keeps the documents of one objective.
func ctxMapFilterObjective(docs []ctxMapDoc, objective string) []ctxMapDoc {
	var out []ctxMapDoc
	for _, doc := range docs {
		if doc.Objective == objective {
			out = append(out, doc)
		}
	}
	return out
}

// ctxMapDocuments groups the documents by kind: count, status distribution and
// the newest creation date.
func ctxMapDocuments(docs []ctxMapDoc) []ctxMapDocument {
	rows := map[string]*ctxMapDocument{}
	statuses := map[string]map[string]int{}
	for _, doc := range docs {
		row, ok := rows[doc.Kind]
		if !ok {
			row = &ctxMapDocument{Kind: doc.Kind, Dir: ctxMapKindDir(doc.Kind)}
			rows[doc.Kind] = row
			statuses[doc.Kind] = map[string]int{}
		}
		row.Count++
		if doc.Status != "" {
			statuses[doc.Kind][doc.Status]++
		}
		if doc.Newest > row.Newest {
			row.Newest = doc.Newest
		}
	}
	out := make([]ctxMapDocument, 0, len(rows))
	for kind, row := range rows {
		row.Statuses = ctxMapStatusBreakdown(statuses[kind], row.Count)
		row.Newest = ctxMapDate(row.Newest)
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

// ctxMapStatusBreakdown renders the statuses and names the remainder: a
// breakdown that does not add up to the row's count invites the reader to
// assume the missing documents have no status, which they do — silently.
func ctxMapStatusBreakdown(counts map[string]int, total int) []ctxMapCount {
	out := ctxMapSortedCounts(counts)
	sum := 0
	for _, c := range out {
		sum += c.Count
	}
	if missing := total - sum; missing > 0 {
		out = append(out, ctxMapCount{Name: "no status", Count: missing})
	}
	return out
}

// ctxMapKindDir is where a kind lives, so the map says where to look next
// instead of leaving the reader to derive the path from the type registry.
func ctxMapKindDir(kind string) string {
	if t, ok := ctxTypeLookup(kind); ok {
		return t.dir
	}
	return ""
}

// ctxMapFrontmatter inventories the frontmatter keys by coverage, most used
// first, with one real value so a reader can see the shape instead of guessing.
func ctxMapFrontmatter(docs []ctxMapDoc) []ctxMapKey {
	if len(docs) == 0 {
		return nil
	}
	present := map[string]int{}
	example := map[string]string{}
	kindOf := map[string]string{}
	for _, doc := range docs {
		for key, field := range doc.Fields {
			present[key]++
			if kindOf[key] == "" {
				kindOf[key] = field.Type
			}
			if example[key] == "" && field.Example != "" {
				example[key] = ctxMapTruncate(ctxMapPlain(field.Example), 48)
			}
		}
	}
	out := make([]ctxMapKey, 0, len(present))
	for key, n := range present {
		out = append(out, ctxMapKey{
			Key:      key,
			Type:     kindOf[key],
			Coverage: fmt.Sprintf("%d/%d (%d%%)", n, len(docs), n*100/len(docs)),
			Example:  example[key],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if present[out[i].Key] != present[out[j].Key] {
			return present[out[i].Key] > present[out[j].Key]
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// ctxMapObjectives rolls the documents up by objective, most populated first.
func ctxMapObjectives(docs []ctxMapDoc) []ctxMapObjective {
	rows := map[string]*ctxMapObjective{}
	for _, doc := range docs {
		if doc.Objective == "" {
			continue
		}
		row, ok := rows[doc.Objective]
		if !ok {
			row = &ctxMapObjective{Objective: doc.Objective}
			rows[doc.Objective] = row
		}
		row.Documents++
		switch doc.Kind {
		case ctxTypeAnalysis:
			row.Analyses++
		case ctxTypePlan:
			row.Plans++
		}
		if strings.EqualFold(strings.TrimSpace(doc.Status), ctxWikiStatusActive) {
			row.Active++
		}
	}
	out := make([]ctxMapObjective, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Documents != out[j].Documents {
			return out[i].Documents > out[j].Documents
		}
		return out[i].Objective < out[j].Objective
	})
	return out
}

// ctxMapRelationsSection counts the traceability edges, naming the documents
// that declare neither a source nor a link — the ones nobody can follow.
func ctxMapRelationsSection(docs []ctxMapDoc) ctxMapRelations {
	var rel ctxMapRelations
	for _, doc := range docs {
		if doc.Sources > 0 {
			rel.WithSources++
		}
		if doc.Links > 0 {
			rel.WithLinks++
		}
		if doc.Sources == 0 && doc.Links == 0 {
			rel.Orphans++
			if len(rel.OrphanRefs) < 5 {
				rel.OrphanRefs = append(rel.OrphanRefs, doc.Ref)
			}
		}
	}
	for _, doc := range docs {
		switch doc.Kind {
		case ctxTypeAnalysis, ctxTypePlan, ctxTypeTasks:
			rel.Lifecycle++
		}
	}
	if edges, err := ctxrel.Load(sdtWorkDir); err == nil {
		for _, doc := range docs {
			if edges.ParentOf(doc.Ref) != "" {
				rel.ParentEdges++
			}
		}
	}
	return rel
}

// ctxMapGotchas collects the awkward facts: rejected approaches, postponed
// analyses with their revival condition, unanswered questions and the lint
// verdict. Each count is derived from the same documents the rest of the map
// scans, so the sections cannot disagree with each other.
func ctxMapGotchas(docs []ctxMapDoc) []ctxMapGotcha {
	var out []ctxMapGotcha

	deadEnds := ctxMapByObjective(docs, func(d ctxMapDoc) bool {
		return d.Kind == ctxTypeNotes && d.NoteType == ctxNoteTypeDeadEnd
	})
	if deadEnds.count > 0 {
		out = append(out, ctxMapGotcha{
			Name:   "dead-ends",
			Count:  deadEnds.count,
			Detail: "rejected approaches, read before reopening: " + deadEnds.summary(),
		})
	}

	postponed := ctxMapByObjective(docs, func(d ctxMapDoc) bool {
		return d.Kind == ctxTypeAnalysis && strings.TrimSpace(d.Status) == statusPostponed
	})
	if postponed.count > 0 {
		detail := "revisit when the revival condition holds: " + postponed.summary()
		detail += "; " + ctxMapRevivalDetail(docs)
		out = append(out, ctxMapGotcha{Name: "postponed", Count: postponed.count, Detail: detail})
	}

	questions := ctxMapByObjective(docs, func(d ctxMapDoc) bool {
		return d.Kind == ctxTypeQuestions && !strings.EqualFold(strings.TrimSpace(d.Status), questionStatusResolved)
	})
	if questions.count > 0 {
		out = append(out, ctxMapGotcha{
			Name:   "open-questions",
			Count:  questions.count,
			Detail: "awaiting an answer: " + questions.firstRefs(3),
		})
	}

	v := storeCompletenessVerdict()
	out = append(out, ctxMapGotcha{
		Name:  "store-verdict",
		Count: v.Critical + v.Warnings + v.Suggestions,
		Detail: fmt.Sprintf("%s (as `context status` computes it): %d critical, %d warnings, %d suggestions",
			v.Verdict, v.Critical, v.Warnings, v.Suggestions),
	})

	unparsed := ctxMapByObjective(docs, func(d ctxMapDoc) bool { return d.Unparsed })
	if unparsed.count > 0 {
		out = append(out, ctxMapGotcha{
			Name:   "unreadable",
			Count:  unparsed.count,
			Detail: "no parseable frontmatter, counted but not described: " + unparsed.firstRefs(3),
		})
	}
	return out
}

// ctxMapGroup is a count plus the objectives and references behind it.
type ctxMapGroup struct {
	count      int
	objectives map[string]int
	refs       []string
}

// summary renders "objective (n), …" in a stable order, and names the
// documents carrying no objective: a breakdown that silently drops them reads
// as if every document were accounted for.
func (g ctxMapGroup) summary() string {
	if len(g.objectives) == 0 {
		return "no objective declared"
	}
	names := make([]string, 0, len(g.objectives))
	for name := range g.objectives {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names)+1)
	grouped := 0
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s (%d)", name, g.objectives[name]))
		grouped += g.objectives[name]
	}
	if missing := g.count - grouped; missing > 0 {
		parts = append(parts, fmt.Sprintf("%d with no objective", missing))
	}
	return strings.Join(parts, ", ")
}

// refs renders the first n references.
func (g ctxMapGroup) firstRefs(n int) string {
	if len(g.refs) <= n {
		return strings.Join(g.refs, ", ")
	}
	return strings.Join(g.refs[:n], ", ") + fmt.Sprintf(" (+%d more)", len(g.refs)-n)
}

// ctxMapByObjective selects the documents matching keep and counts them by
// objective.
func ctxMapByObjective(docs []ctxMapDoc, keep func(ctxMapDoc) bool) ctxMapGroup {
	group := ctxMapGroup{objectives: map[string]int{}}
	for _, doc := range docs {
		if !keep(doc) {
			continue
		}
		group.count++
		group.refs = append(group.refs, doc.Ref)
		if doc.Objective != "" {
			group.objectives[doc.Objective]++
		}
	}
	sort.Strings(group.refs)
	return group
}

// ctxMapRevivalDetail names the revival condition of up to two postponed
// analyses: a postponed decision without a stated condition is a dead end the
// map should not hide.
func ctxMapRevivalDetail(docs []ctxMapDoc) string {
	var parts []string
	for _, doc := range docs {
		if doc.Kind != ctxTypeAnalysis || strings.TrimSpace(doc.Status) != statusPostponed {
			continue
		}
		condition := doc.Revival
		if condition == "" {
			condition = "no revival condition stated"
		}
		parts = append(parts, doc.Ref+": "+condition)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		return "no revival condition stated"
	}
	return strings.Join(parts, " | ")
}

// ctxMapTrySection derives three first moves from the map itself, so the map
// ends with what to run instead of with a wall of counts.
func ctxMapTrySection(docs []ctxMapDoc) []ctxMapTry {
	var tries []ctxMapTry
	objectives := ctxMapObjectives(docs)
	if len(objectives) > 0 {
		tries = append(tries, ctxMapTry{
			Query: fmt.Sprintf("sdt context search %q", objectives[0].Objective),
			Why:   "the objective with the most documents",
		})
	}
	kind := ""
	most := 0
	for _, row := range ctxMapDocuments(docs) {
		if row.Count > most {
			kind, most = row.Kind, row.Count
		}
	}
	if kind != "" {
		tries = append(tries, ctxMapTry{
			Query: "sdt context list --type " + kind,
			Why:   "where most documents are",
		})
	}
	newest := ""
	for _, doc := range docs {
		if doc.Newest > newest {
			newest = doc.Newest
		}
	}
	if newest != "" {
		for _, doc := range docs {
			if doc.Newest == newest {
				tries = append(tries, ctxMapTry{
					Query: "sdt read -p " + doc.Ref,
					Why:   "the newest document",
				})
				break
			}
		}
	}
	return tries
}

// ctxMapSortedCounts renders a name→count map as a stable slice.
func ctxMapSortedCounts(counts map[string]int) []ctxMapCount {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ctxMapCount, 0, len(names))
	for _, name := range names {
		out = append(out, ctxMapCount{Name: name, Count: counts[name]})
	}
	return out
}

// ctxMapDate keeps the calendar day of a timestamp: the map answers "how recent
// is this kind", not "at what second".
func ctxMapDate(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// ctxMapTruncate shortens a value to width runes with an ellipsis.
func ctxMapTruncate(s string, width int) string {
	runes := []rune(strings.Join(strings.Fields(s), " "))
	if len(runes) <= width {
		return string(runes)
	}
	return string(runes[:width-1]) + "…"
}

var contextMapCmd = &cobra.Command{
	Use:   "map",
	Short: "Map the corpus: what it holds, what is awkward, and what it cannot see",
	Long: `Map the corpus and state the map's own scope.

WHEN — you meet a corpus cold: this is the one page that says which kinds exist,
which frontmatter keys they share, which objectives are alive, which documents
nobody can trace, and what is known to be awkward (dead ends, postponed
analyses with their revival condition, open questions, lint counts).

WHEN NOT — you know what you are looking for: map/search the specific thing.

The first line is the scope, in every format. ` + "`context/refs`, `context/tmp`," + `
` + "`context/trash` and `context/sdtdocs` are outside the map, so an empty section never" + `
means the thing is absent — it means the thing is not a corpus document.

Reads the documents, never ` + "`context/index.json`" + `: a stale index must not produce a
confident wrong answer. No cache, no write.

Example: ` + "`sdt context map`" + ` · ` + "`sdt context map --objective agent-refs --format json`" + ``,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		report := ctxMapBuild(getStringFlag(cmd, "objective", false))
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(report, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, append(out, '\n'))
		case fmtYAML:
			out, err := yaml.Marshal(report)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, ctxMapText(report))
		}
	},
}

// ctxMapText renders the map for a reader: scope first, then the six sections.
func ctxMapText(rep ctxMapReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "scope  %s\n", rep.Scope)
	fmt.Fprintf(&b, "corpus %s (%d documents)\n", rep.Corpus, rep.Total)

	b.WriteString("\n## Documents\n")
	if len(rep.Documents) == 0 {
		b.WriteString("  (no corpus documents)\n")
	}
	for _, row := range rep.Documents {
		fmt.Fprintf(&b, "  %-14s %-22s %5d  %-40s newest %s\n", row.Kind, row.Dir, row.Count, ctxMapStatusLine(row.Statuses), row.Newest)
	}

	b.WriteString("\n## Frontmatter\n")
	if len(rep.Frontmatter) == 0 {
		b.WriteString("  (no documents)\n")
	}
	for _, key := range rep.Frontmatter {
		fmt.Fprintf(&b, "  %-18s %-6s %-14s %s\n", key.Key, key.Type, key.Coverage, key.Example)
	}

	b.WriteString("\n## Objectives\n")
	if len(rep.Objectives) == 0 {
		b.WriteString("  (no objective declared)\n")
	}
	for _, row := range rep.Objectives {
		fmt.Fprintf(&b, "  %-28s %4d documents  analyses %d  plans %d  active docs %d\n",
			row.Objective, row.Documents, row.Analyses, row.Plans, row.Active)
	}

	b.WriteString("\n## Relations\n")
	fmt.Fprintf(&b, "  sources declared %d, links declared %d\n",
		rep.Relations.WithSources, rep.Relations.WithLinks)
	fmt.Fprintf(&b, "  resolved parent edges %d of %d lifecycle documents (analysis, plan, tasks)\n",
		rep.Relations.ParentEdges, rep.Relations.Lifecycle)
	fmt.Fprintf(&b, "  orphans (no sources, no links) %d", rep.Relations.Orphans)
	if len(rep.Relations.OrphanRefs) > 0 {
		fmt.Fprintf(&b, ": %s", strings.Join(rep.Relations.OrphanRefs, ", "))
	}
	b.WriteString("\n")

	b.WriteString("\n## Gotchas\n")
	if len(rep.Gotchas) == 0 {
		b.WriteString("  (nothing recorded)\n")
	}
	for _, row := range rep.Gotchas {
		fmt.Fprintf(&b, "  %-15s %-5d %s\n", row.Name, row.Count, row.Detail)
	}

	b.WriteString("\n## Try\n")
	if len(rep.Try) == 0 {
		b.WriteString("  (nothing to try on an empty corpus)\n")
	}
	for _, row := range rep.Try {
		fmt.Fprintf(&b, "  %-52s %s\n", row.Query, row.Why)
	}
	b.WriteString("\nThis is a map, not an index: per-document rows are `sdt context list` and `sdt context search`.\n")
	return b.String()
}

// ctxMapStatusLine renders a kind's status distribution as "name n, name n".
func ctxMapStatusLine(counts []ctxMapCount) string {
	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		parts = append(parts, fmt.Sprintf("%s %d", c.Name, c.Count))
	}
	return strings.Join(parts, ", ")
}

func init() {
	contextMapCmd.Flags().String("objective", "", "Limit the map to one objective slug")
	contextCmd.AddCommand(contextMapCmd)
}
