//nolint:goconst // the keyword and intent-key sets are inherently tables of strings
package cmd

import (
	"fmt"
	"strings"
)

// Advisory structural checks for fenced `mermaid` blocks and the optional intent
// line above one. The rules mirror the dialect contract in
// context/instructions/diagram-mermaid.md — change the two together.
//
// The keyword set is the renderer's, not Mermaid's whole language: the first
// tokens the pinned Pele detects (web/package.json, and its dist/diagrams/
// parsers). The set has a second copy in the dialect module's table, and two
// tests bind the copies: TestDiagramKeywordsMatchDialect reads the generated
// module, and web/src/lib/diagramKeywords.test.ts reads this map and asks the
// renderer's own detector.
//
// Findings are SUGGESTION only. A diagram never hard-fails the lint, and the
// check reads the type keyword — it never parses the block, so a green lint is
// never "this diagram draws". Every finding says so.
const ctxDiagramKeywordSource = "Pele 1.13 (web/package.json)"

// ctxDiagramKeywords is one copy of a value that has two: see the file comment.
var ctxDiagramKeywords = map[string]bool{
	"flowchart": true, "graph": true,
	"sequenceDiagram": true, "classDiagram": true,
	"stateDiagram": true, "stateDiagram-v2": true,
	"erDiagram": true, "journey": true, "gantt": true, "pie": true,
	"quadrantChart": true, "requirementDiagram": true, "gitGraph": true,
	"C4Context": true, "C4Container": true, "C4Component": true,
	"C4Dynamic": true, "C4Deployment": true,
	"mindmap": true, "timeline": true, "sankey-beta": true, "xychart-beta": true,
	"block-beta": true, "packet-beta": true, "architecture-beta": true,
	"radar-beta": true, "treemap-beta": true, "kanban": true, "ishikawa": true,
	"eventmodeling": true, "info": true, "railroad-beta": true,
	"treeView-beta": true, "wardley-beta": true,
}

// ctxDiagramIntentKeys is the closed key set of the optional intent line:
// <!-- diagram: question=…; source=…; form=… -->
var ctxDiagramIntentKeys = map[string]bool{
	"question":    true,
	ctxTypeSource: true,
	"form":        true,
}

// ctxDiagramBlock is one fenced `mermaid` block: the line of its opening fence,
// its inner lines, and the intent comment that directly precedes it, if any.
type ctxDiagramBlock struct {
	Line       int
	Source     []string
	Intent     string
	IntentLine int
}

// ctxDiagramBlocks returns every fenced `mermaid` block of a document body, in
// order. Fences of other info strings are skipped, and so is the content of a
// block whose marker never closes — the fence check owns that finding.
func ctxDiagramBlocks(lines []string) []ctxDiagramBlock {
	var blocks []ctxDiagramBlock
	for i := 0; i < len(lines); i++ {
		marker, info, ok := fenceOpen(lines[i])
		if !ok {
			continue
		}
		closeAt := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if isFenceClose(lines[j], marker) {
				closeAt = j
				break
			}
		}
		if fields := strings.Fields(info); len(fields) > 0 && strings.EqualFold(fields[0], "mermaid") {
			block := ctxDiagramBlock{Line: i + 1, Source: lines[i+1 : closeAt]}
			// The intent line is the line directly above the fence: a blank line
			// between the comment and the block detaches it.
			if i > 0 {
				if line := strings.TrimSpace(lines[i-1]); strings.HasPrefix(line, "<!-- diagram:") {
					block.Intent = line
					block.IntentLine = i
				}
			}
			blocks = append(blocks, block)
		}
		i = closeAt
	}
	return blocks
}

// ctxDiagramTypeKeyword returns the type keyword that opens a block's source,
// skipping blank lines, `%%` comments and an init directive, and a leading YAML
// frontmatter block (Pele accepts one, so two of Mermaid's demos open with it).
func ctxDiagramTypeKeyword(source []string) string {
	i := 0
	for i < len(source) && strings.TrimSpace(source[i]) == "" {
		i++
	}
	if i < len(source) && strings.TrimSpace(source[i]) == "---" {
		for i++; i < len(source); i++ {
			if strings.TrimSpace(source[i]) == "---" {
				i++
				break
			}
		}
	}
	for ; i < len(source); i++ {
		line := strings.TrimSpace(source[i])
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		return strings.Fields(line)[0]
	}
	return ""
}

// lintDiagram reports a `mermaid` block whose type keyword the renderer does not
// know, and an intent line that is malformed. Advisory only, and the message
// states the check's own limit.
func lintDiagram(path, body string) []ctxLintIssue {
	var issues []ctxLintIssue
	for _, block := range ctxDiagramBlocks(strings.Split(body, "\n")) {
		keyword := ctxDiagramTypeKeyword(block.Source)
		switch {
		case keyword == "":
			issues = append(issues, ctxLintIssue{
				Path:     path,
				Priority: ctxLintSuggestion,
				Message:  fmt.Sprintf("line %d: `mermaid` block opens with no type keyword — fix the block; this check reads the keyword only, not the diagram's syntax", block.Line),
			})
		case !ctxDiagramKeywords[keyword]:
			issues = append(issues, ctxLintIssue{
				Path:     path,
				Priority: ctxLintSuggestion,
				Message:  fmt.Sprintf("line %d: `%s` is not a type keyword %s draws — a block that does not parse reaches the reader as source; this check reads the keyword only, not the diagram's syntax", block.Line, keyword, ctxDiagramKeywordSource),
			})
		}
		issues = append(issues, lintDiagramIntent(path, block)...)
	}
	return issues
}

// lintDiagramIntent validates the optional intent line above a block. Absence is
// never a finding: the line records a provenance when there is one to record.
func lintDiagramIntent(path string, block ctxDiagramBlock) []ctxLintIssue {
	if block.Intent == "" {
		return nil
	}
	issue := func(format string, args ...any) []ctxLintIssue {
		return []ctxLintIssue{{
			Path:     path,
			Priority: ctxLintSuggestion,
			Message:  fmt.Sprintf("line %d: ", block.IntentLine) + fmt.Sprintf(format, args...),
		}}
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(block.Intent, "<!--"), "-->"))
	inner = strings.TrimSpace(strings.TrimPrefix(inner, "diagram:"))

	var issues []ctxLintIssue
	form := ""
	for _, pair := range strings.Split(inner, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, found := strings.Cut(pair, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !found || key == "" {
			issues = append(issues, issue("intent line entry %q is not `key=value`", pair)...)
			continue
		}
		if !ctxDiagramIntentKeys[key] {
			issues = append(issues, issue("intent line key `%s` is outside the set (question | source | form)", key)...)
			continue
		}
		if value == "" {
			issues = append(issues, issue("intent line key `%s` has no value", key)...)
			continue
		}
		if key == "form" {
			form = value
			if !ctxDiagramKeywords[value] {
				issues = append(issues, issue("intent line `form=%s` is not a type keyword %s draws", value, ctxDiagramKeywordSource)...)
			}
		}
	}
	if form != "" && ctxDiagramKeywords[form] {
		if keyword := ctxDiagramTypeKeyword(block.Source); keyword != "" && keyword != form {
			issues = append(issues, issue("intent line `form=%s` disagrees with the block's `%s`", form, keyword)...)
		}
	}
	return issues
}

// diagramWriteWarnings returns the messages the write path prints for the block
// it is about to write. It never blocks: a corpus edit is refused for a broken
// structure, never for an unconventional diagram.
func diagramWriteWarnings(path, body string) []string {
	var out []string
	for _, issue := range lintDiagram(path, body) {
		out = append(out, issue.Message)
	}
	return out
}
