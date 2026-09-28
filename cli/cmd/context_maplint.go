package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// Advisory structural checks for `.map.md` documents (analysis 20260925-171754).
// The rules mirror the format contract in
// context/instructions/mindmap-markmap.md — change the two together. Findings
// are WARNING (content the parser drops, unresolvable links, unknown options) or
// SUGGESTION (budgets), never CRITICAL, so a map never hard-fails the lint.

const (
	ctxMapBranchesMin = 3
	ctxMapBranchesMax = 7
	ctxMapChildrenMin = 2
	ctxMapChildrenMax = 5
	ctxMapDepthMax    = 3
	ctxMapNodesMax    = 60
)

// ctxMapKnownOptions is the markmap frontmatter options channel; an unknown key
// is silently inert.
var ctxMapKnownOptions = map[string]bool{
	"title": true, "color": true, "colorFreezeLevel": true, "duration": true,
	"maxWidth": true, "initialExpandLevel": true, "extraJs": true,
	"extraCss": true, "htmlParser": true,
}

// ctxMapLinkRegexp matches a [[target]] or [[target|label]] wikilink.
var ctxMapLinkRegexp = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// ctxMapAdd appends one map-lint finding at the given severity.
type ctxMapAdd func(sev, msg string)

// lintMapDoc runs the seven advisory structural checks over a `.map.md`
// document. body is the markdown without frontmatter; content is the full file
// (used for the frontmatter title and the `markmap:` options channel).
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
	ctxMapLintDropped(root, body, add)
	ctxMapLintBlockquotes(root, add)
	ctxMapLintBudgets(root, docNodes, h2, maxLevel, add)
	ctxMapLintOptions(content, add)
	ctxMapLintLinks(path, content, add)
	ctxMapCheckMixing(docNodes, add)
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

// ctxMapLintDropped flags prose paragraphs the parser discards. An image-only
// paragraph is a leaf and is allowed; a paragraph inside a list item is a node.
func ctxMapLintDropped(root ast.Node, body []byte, add ctxMapAdd) {
	mapWalk(root, func(n ast.Node) {
		p, ok := n.(*ast.Paragraph)
		if !ok {
			return
		}
		if _, ok := p.Parent().(*ast.ListItem); ok {
			return
		}
		if insideBlockquote(p) || paragraphIsImageOnly(p) {
			return
		}
		add(ctxLintWarning, fmt.Sprintf("paragraph at line %d is dropped by the parser (maps accept headings, lists, tables, fenced code and image-only paragraphs only)", markdownLineNumber(body, p.Pos())))
	})
}

// ctxMapLintBlockquotes flags blockquotes, which the parser drops.
func ctxMapLintBlockquotes(root ast.Node, add ctxMapAdd) {
	mapWalk(root, func(n ast.Node) {
		if _, ok := n.(*ast.Blockquote); ok {
			add(ctxLintWarning, "blockquote is dropped by the parser")
		}
	})
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

// ctxMapLintOptions checks that only known `markmap:` frontmatter keys are used.
func ctxMapLintOptions(content string, add ctxMapAdd) {
	fm, _ := contextwiki.SplitFrontmatter(content)
	if fm == "" {
		return
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return
	}
	mk, ok := doc["markmap"].(map[string]any)
	if !ok {
		return
	}
	for key := range mk {
		if !ctxMapKnownOptions[key] {
			add(ctxLintWarning, fmt.Sprintf("unknown `markmap:` option %q (silently inert)", key))
		}
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
			add(ctxLintWarning, "a parent mixes a loose list with sub-headings; the parser resets its children and drops content")
			return
		}
	}
	if rootList && len(heads) > 0 {
		add(ctxLintWarning, "a document-level list is mixed with headings; the parser resets the tree and drops content")
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

// paragraphIsImageOnly reports whether a paragraph is a single image (a leaf
// node the parser keeps).
func paragraphIsImageOnly(p *ast.Paragraph) bool {
	child := p.FirstChild()
	if child == nil || child != p.LastChild() {
		return false
	}
	_, ok := child.(*ast.Image)
	return ok
}

// insideBlockquote reports whether a node has a blockquote ancestor.
func insideBlockquote(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.Blockquote); ok {
			return true
		}
	}
	return false
}
