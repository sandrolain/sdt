package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// Advisory structural checks for `.map.md` documents (analysis 20260925-171754,
// revised by plan 20261002-063031). The rules mirror the dialect contract in
// context/instructions/mindmap-markmap.md — change the two together. Findings
// are WARNING (an inert `markmap:` key, unresolvable links, a malformed
// structure, a marker mistake) or SUGGESTION (budgets), never CRITICAL, so a map
// never hard-fails the lint.

// The wrap kinds and names the marker scan and its findings share.
const (
	ctxMapKindBoundary = "B"
	ctxMapKindSummary  = "S"
	ctxMapNameBoundary = "boundary"
	ctxMapNameSummary  = "summary"
)

const (
	ctxMapBranchesMin = 3
	ctxMapBranchesMax = 7
	ctxMapChildrenMin = 2
	ctxMapChildrenMax = 5
	ctxMapDepthMax    = 3
	ctxMapNodesMax    = 60
)

var (
	// ctxMapBoundaryRe / ctxMapSummaryRe read a membership marker; an unnumbered
	// marker is id "0", as the SPA marker layer records it.
	ctxMapBoundaryRe = regexp.MustCompile(`\[B(\d*)\]`)
	ctxMapSummaryRe  = regexp.MustCompile(`\[S(\d*)\]`)
	// A relationship id is digits only, matching the dialect; `[^n]` is the
	// target end of `[n]`.
	ctxMapRelSourceRe = regexp.MustCompile(`\[(\d+)\]`)
	ctxMapRelTargetRe = regexp.MustCompile(`\[\^(\d+)\]`)
	// A `[B<n>]:`/`[S<n>]:` line declares a title; it is never a topic.
	ctxMapTitleRe = regexp.MustCompile(`^\s*\[([BS])(\d*)\]:\s*(.*)$`)
	ctxMapFenceRe = regexp.MustCompile("^\\s*(```|~~~)")
)

// ctxMapLinkRegexp matches a [[target]] or [[target|label]] wikilink.
var ctxMapLinkRegexp = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// ctxMapAdd appends one map-lint finding at the given severity.
type ctxMapAdd func(sev, msg string)

// lintMapDoc runs the advisory structural checks over a `.map.md` document.
// body is the markdown without frontmatter; content is the full file (used for
// the frontmatter title and the inert `markmap:` options key).
func lintMapDoc(path, content string, body []byte) []ctxLintIssue {
	root := parser.New().Parse(body)
	var docNodes []ast.Node
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		docNodes = append(docNodes, n)
	}
	var issues []ctxLintIssue
	add := func(sev, msg string) {
		issues = append(issues, ctxLintIssue{Path: path, Priority: sev, Message: msg})
	}

	h1, h2, maxLevel := ctxMapHeadings(docNodes)
	ctxMapLintRoot(content, h1, add)
	ctxMapLintBudgets(root, docNodes, h2, maxLevel, add)
	ctxMapLintOptions(content, add)
	ctxMapLintLinks(path, content, add)
	ctxMapCheckMixing(docNodes, add)
	ctxMapLintMarkers(ctxMapScanTopics(docNodes, body), ctxMapScanTitles(body), add)
	return issues
}

// ctxMapHeadings returns the H1 count, the H2 count and the deepest heading
// level among the document-level nodes.
func ctxMapHeadings(docNodes []ast.Node) (h1, h2, maxLevel int) {
	for _, n := range docNodes {
		h, ok := n.(*ast.Heading)
		if !ok {
			continue
		}
		switch h.Level {
		case 1:
			h1++
		case 2:
			h2++
		}
		if h.Level > maxLevel {
			maxLevel = h.Level
		}
	}
	return h1, h2, maxLevel
}

// ctxMapLintRoot checks exactly one root: one H1, or a frontmatter title.
func ctxMapLintRoot(content string, h1 int, add ctxMapAdd) {
	if h1 > 1 {
		add(ctxLintWarning, fmt.Sprintf("map has %d H1 roots; a map has exactly one root", h1))
		return
	}
	if h1 == 0 && contextwiki.FrontmatterField(content, "title") == "" {
		add(ctxLintWarning, "map has no root: add a single `#` heading or a frontmatter `title`")
	}
}

// ctxMapLintBudgets emits SUGGESTIONs when the branch count, depth, per-branch
// children or total node count leave the defaults.
func ctxMapLintBudgets(root ast.Node, docNodes []ast.Node, h2, maxLevel int, add ctxMapAdd) {
	branches := h2
	if branches == 0 {
		for _, n := range docNodes {
			if l, ok := n.(*ast.List); ok {
				branches = listItemCount(l)
				break
			}
		}
	}
	if branches < ctxMapBranchesMin || branches > ctxMapBranchesMax {
		add(ctxLintSuggestion, fmt.Sprintf("%d main branches; the default is %d–%d", branches, ctxMapBranchesMin, ctxMapBranchesMax))
	}
	if depth := maxLevel - 1; depth > ctxMapDepthMax {
		add(ctxLintSuggestion, fmt.Sprintf("heading depth %d exceeds the default %d", depth, ctxMapDepthMax))
	}
	ctxMapCheckChildren(docNodes, add)
	total := 0
	mapWalk(root, func(n ast.Node) {
		switch n.(type) {
		case *ast.Heading, *ast.ListItem:
			total++
		}
	})
	if total > ctxMapNodesMax {
		add(ctxLintSuggestion, fmt.Sprintf("map has %d nodes; the default ceiling is a few dozen (~%d)", total, ctxMapNodesMax))
	}
}

// ctxMapLintOptions reports the removed renderer options channel. The viewer
// parses the document itself and honours no `markmap:` key, so one left in a
// map is inert.
func ctxMapLintOptions(content string, add ctxMapAdd) {
	fm, _ := contextwiki.SplitFrontmatter(content)
	if fm == "" {
		return
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return
	}
	if _, ok := doc["markmap"]; ok {
		add(ctxLintWarning, "the `markmap:` options channel was removed with the markmap renderer; the key is inert (folding is `[F]`, the layout is a viewer choice)")
	}
}

// ctxMapLintLinks flags wikilink targets that do not resolve in the corpus,
// after stripping the `|label` and `#anchor` parts.
func ctxMapLintLinks(path, content string, add ctxMapAdd) {
	dir := filepath.Dir(path)
	for _, m := range ctxMapLinkRegexp.FindAllStringSubmatch(content, -1) {
		target := strings.TrimSpace(m[1])
		if i := strings.Index(target, "#"); i >= 0 {
			target = target[:i]
		}
		if target == "" {
			continue
		}
		abs := filepath.Join(dir, target)
		if _, err := os.Stat(abs); err == nil { //#nosec G703 -- missing file is the finding
			continue
		}
		if filepath.Ext(abs) == "" {
			if _, err := os.Stat(abs + sdtMarkdownExt); err == nil { //#nosec G703 -- missing file is the finding
				continue
			}
		}
		add(ctxLintWarning, "wikilink target does not resolve: "+m[1])
	}
}

// ctxMapCheckChildren reports a SUGGESTION when a branch has fewer than 2 or
// more than 5 direct children. A branch's children are its deeper headings, or
// (when it has none) the items of a top-level list directly under it.
func ctxMapCheckChildren(docNodes []ast.Node, add ctxMapAdd) {
	var starts []int
	for i, n := range docNodes {
		if h, ok := n.(*ast.Heading); ok && h.Level == 2 {
			starts = append(starts, i)
		}
	}
	for b, start := range starts {
		end := len(docNodes)
		if b+1 < len(starts) {
			end = starts[b+1]
		}
		children, sawList := 0, false
		for j := start + 1; j < end; j++ {
			switch n := docNodes[j].(type) {
			case *ast.Heading:
				if n.Level >= 3 {
					children++
				}
			case *ast.List:
				if children == 0 && !sawList {
					children += listItemCount(n)
					sawList = true
				}
			}
		}
		if children < ctxMapChildrenMin || children > ctxMapChildrenMax {
			add(ctxLintSuggestion, fmt.Sprintf("branch %d has %d children; the default is %d–%d", b+1, children, ctxMapChildrenMin, ctxMapChildrenMax))
		}
	}
}

// ctxMapCheckMixing reports a WARNING when a heading has both a list child and a
// deeper heading child (or a document-level list precedes the headings).
func ctxMapCheckMixing(docNodes []ast.Node, add ctxMapAdd) {
	type headingNode struct {
		level                int
		listChild, headChild bool
	}
	var heads []*headingNode
	var stack []int
	rootList := false
	for _, n := range docNodes {
		switch node := n.(type) {
		case *ast.Heading:
			for len(stack) > 0 && heads[stack[len(stack)-1]].level >= node.Level {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				heads[stack[len(stack)-1]].headChild = true
			}
			heads = append(heads, &headingNode{level: node.Level})
			stack = append(stack, len(heads)-1)
		case *ast.List:
			if len(stack) > 0 {
				heads[stack[len(stack)-1]].listChild = true
			} else {
				rootList = true
			}
		}
	}
	for _, h := range heads {
		if h.listChild && h.headChild {
			add(ctxLintWarning, "a parent mixes a loose list with sub-headings; the list lands under the deepest heading")
			return
		}
	}
	if rootList && len(heads) > 0 {
		add(ctxLintWarning, "a document-level list is mixed with headings; the list lands under the deepest heading")
	}
}

// mapWalk calls fn for every descendant of n in document order.
func mapWalk(n ast.Node, fn func(ast.Node)) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		fn(c)
		mapWalk(c, fn)
	}
}

// listItemCount counts the direct ListItem children of a list.
func listItemCount(l *ast.List) int {
	c := 0
	for n := l.FirstChild(); n != nil; n = n.NextSibling() {
		if _, ok := n.(*ast.ListItem); ok {
			c++
		}
	}
	return c
}

// ctxMapTopic is one structural topic of a map — a heading, a list item or a
// standalone paragraph — carrying the markers its own text declares and the
// parent it hangs from, which is what the marker checks read.
type ctxMapTopic struct {
	line     int
	boundary string
	summary  string
	relSrc   string
	relTgt   string
	parent   *ctxMapTopic
}

// ctxMapTitle is a `[B<n>]: title` / `[S<n>]: title` declaration line.
type ctxMapTitle struct {
	kind  string
	id    string
	title string
	line  int
}

// ctxMapNodeText returns the block's own source text.
func ctxMapNodeText(n ast.Node, body []byte) string {
	type sourced interface{ Source() []text.Segment }
	s, ok := n.(sourced)
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, seg := range s.Source() {
		if seg.Start >= 0 && seg.Stop <= len(body) && seg.Start < seg.Stop {
			sb.Write(body[seg.Start:seg.Stop])
			sb.WriteString(" ")
		}
	}
	return strings.TrimSpace(sb.String())
}

// ctxMapItemParts splits a list item into its own label paragraph and the
// blocks nested under it: goldmark puts the label in the first `*ast.Paragraph`
// and any further paragraph or list after it.
func ctxMapItemParts(item ast.Node) (label ast.Node, extra []ast.Node) {
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		p, ok := c.(*ast.Paragraph)
		if !ok {
			extra = append(extra, c)
			continue
		}
		if label == nil {
			label = p
			continue
		}
		extra = append(extra, p)
	}
	return label, extra
}

// ctxMapMarkerID normalises an unnumbered boundary/summary marker to id "0".
func ctxMapMarkerID(m []string) string {
	return ctxMapOrZero(m[1])
}

// ctxMapOrZero maps the empty digit group of an unnumbered marker to id "0".
func ctxMapOrZero(digits string) string {
	if digits == "" {
		return "0"
	}
	return digits
}

// ctxMapNewTopic records one topic and the markers its own text declares.
func ctxMapNewTopic(text string, n ast.Node, body []byte, parent *ctxMapTopic) *ctxMapTopic {
	t := &ctxMapTopic{line: markdownLineNumber(body, n.Pos()), parent: parent}
	if m := ctxMapBoundaryRe.FindStringSubmatch(text); m != nil {
		t.boundary = ctxMapMarkerID(m)
	}
	if m := ctxMapSummaryRe.FindStringSubmatch(text); m != nil {
		t.summary = ctxMapMarkerID(m)
	}
	if m := ctxMapRelSourceRe.FindStringSubmatch(text); m != nil {
		t.relSrc = m[1]
	}
	if m := ctxMapRelTargetRe.FindStringSubmatch(text); m != nil {
		t.relTgt = m[1]
	}
	return t
}

// ctxMapScanTopics walks the map structure once: headings nest by level and a
// list attaches to the innermost open heading, exactly as the SPA parser builds
// the tree, so a check here describes what the viewer will render.
func ctxMapScanTopics(docNodes []ast.Node, body []byte) []*ctxMapTopic {
	var topics []*ctxMapTopic
	type heading struct {
		level int
		topic *ctxMapTopic
	}
	var stack []heading
	current := func() *ctxMapTopic {
		if len(stack) > 0 {
			return stack[len(stack)-1].topic
		}
		return nil
	}
	var walkBlocks func(nodes []ast.Node, parent *ctxMapTopic)
	walkBlocks = func(nodes []ast.Node, parent *ctxMapTopic) {
		for _, n := range nodes {
			switch node := n.(type) {
			case *ast.Heading:
				for len(stack) > 0 && stack[len(stack)-1].level >= node.Level {
					stack = stack[:len(stack)-1]
				}
				t := ctxMapNewTopic(ctxMapNodeText(node, body), node, body, current())
				topics = append(topics, t)
				stack = append(stack, heading{level: node.Level, topic: t})
			case *ast.List:
				for c := node.FirstChild(); c != nil; c = c.NextSibling() {
					label, inner := ctxMapItemParts(c)
					if label == nil {
						walkBlocks(inner, current())
						continue
					}
					t := ctxMapNewTopic(ctxMapNodeText(label, body), label, body, current())
					topics = append(topics, t)
					walkBlocks(inner, t)
				}
			case *ast.Paragraph:
				topics = append(topics, ctxMapNewTopic(ctxMapNodeText(node, body), node, body, current()))
			}
		}
	}
	walkBlocks(docNodes, nil)
	return topics
}

// ctxMapScanTitles collects the title declaration lines. They are read line-wise
// rather than from the AST: `[B1]: text` parses as a link reference definition,
// and a title must be found whatever goldmark made of it. Fenced code is
// skipped so a `[B1]:` line inside a block is not read as a declaration.
func ctxMapScanTitles(body []byte) []ctxMapTitle {
	var titles []ctxMapTitle
	fence := ""
	for i, line := range strings.Split(string(body), "\n") {
		if marker := ctxMapFenceRe.FindStringSubmatch(line); marker != nil {
			// A fence opens a block, and closes it only when it matches the
			// marker that opened it (``` and ~~~ do not close each other).
			switch marker[1] {
			case fence:
				fence = ""
			default:
				if fence == "" {
					fence = marker[1]
				}
			}
			continue
		}
		if fence != "" {
			continue
		}
		m := ctxMapTitleRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		kind := m[1]
		if kind != ctxMapKindBoundary {
			kind = ctxMapKindSummary
		}
		titles = append(titles, ctxMapTitle{kind: kind, id: ctxMapOrZero(m[2]), title: m[3], line: i + 1})
	}
	return titles
}

// ctxMapTitleHasMarker reports whether a title line carries a marker. The
// dialect ignores those (a relationship cannot target a wrap title), so the
// author has to know the marker was dropped.
func ctxMapTitleHasMarker(title string) bool {
	return ctxMapRelTargetRe.MatchString(title) || ctxMapRelSourceRe.MatchString(title) ||
		ctxMapBoundaryRe.MatchString(title) || ctxMapSummaryRe.MatchString(title) ||
		strings.Contains(title, "[F]") || strings.Contains(title, "[N:") ||
		strings.Contains(title, "[L:") || strings.Contains(title, "[!")
}

// ctxMapLintMarkers checks the XMindMark markers the dialect now understands:
// a relationship needs both of its ends and exactly one source, a wrap title
// needs members, a summary wraps siblings under one parent, and a marker on a
// title line is reported as dropped.
func ctxMapLintMarkers(topics []*ctxMapTopic, titles []ctxMapTitle, add ctxMapAdd) {
	sources := map[string]int{}
	targets := map[string]int{}
	members := map[string]int{}
	summaryParents := map[string]map[*ctxMapTopic]bool{}
	for _, t := range topics {
		if t.relSrc != "" {
			sources[t.relSrc]++
		}
		if t.relTgt != "" {
			targets[t.relTgt]++
		}
		if t.boundary != "" {
			members[ctxMapKindBoundary+t.boundary]++
		}
		if t.summary != "" {
			key := ctxMapKindSummary + t.summary
			members[key]++
			if summaryParents[key] == nil {
				summaryParents[key] = map[*ctxMapTopic]bool{}
			}
			summaryParents[key][t.parent] = true
		}
	}
	for _, id := range ctxMapSortedKeys(sources) {
		if targets[id] == 0 {
			add(ctxLintWarning, fmt.Sprintf("relationship [%s] has no [^%s] target in this map, so no edge is drawn", id, id))
			continue
		}
		if sources[id] > 1 {
			add(ctxLintWarning, fmt.Sprintf("relationship id [%s] is declared by %d topics; a relationship has exactly one source", id, sources[id]))
		}
	}
	for _, id := range ctxMapSortedKeys(targets) {
		if sources[id] == 0 {
			add(ctxLintWarning, fmt.Sprintf("relationship target [^%s] has no [%s] source in this map, so no edge is drawn", id, id))
		}
	}
	for _, key := range ctxMapSortedKeys(summaryParents) {
		if parents := len(summaryParents[key]); parents > 1 {
			add(ctxLintWarning, fmt.Sprintf("summary [%s] wraps topics under %d different parents; a summary wraps siblings", key, parents))
		}
	}
	for _, t := range titles {
		if members[t.kind+t.id] == 0 {
			add(ctxLintWarning, fmt.Sprintf("%s on line %d has no member topic: nothing declares [%s%s]", strings.ToLower(ctxMapKindName(t.kind))+" title", t.line, t.kind, t.id))
		}
		if ctxMapTitleHasMarker(t.title) {
			add(ctxLintWarning, fmt.Sprintf("a marker on the title line %d is dropped: a relationship cannot target a boundary or summary title", t.line))
		}
	}
}

// ctxMapKindName names a wrap kind for a finding message.
func ctxMapKindName(kind string) string {
	if kind == ctxMapKindSummary {
		return ctxMapNameSummary
	}
	return ctxMapNameBoundary
}

// ctxMapSortedKeys returns a map's string keys in order, so findings come out
// in a stable order.
func ctxMapSortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
