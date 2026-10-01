// Package frontmatter reads and writes a document's YAML frontmatter block
// with byte fidelity: a Set or Unset touches only the target key's bytes and
// leaves every other byte — comments, key order, quoting, block scalars, CRLF
// line endings and the body — exactly as it was.
//
// It is the write engine behind `sdt context set`/`unset`. Reads stay in
// internal/contextwiki so the CLI, the index and the viewer cannot drift; this
// package owns only writing. The fidelity contract is why it exists: the
// earlier line patcher could not express a list, a delete or a nested key, and
// could not escape a value.
//
// Only top-level mapping keys are addressable. A dotted/nested key is rejected
// with a named error rather than silently answered by a different node.
package frontmatter

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// ErrNoFrontmatter is returned when the document has no leading frontmatter
// block.
var ErrNoFrontmatter = errors.New("no frontmatter block")

// Block is a parsed frontmatter block: the YAML text between the fences, the
// parsed mapping node, and the byte layout needed to splice one key.
type Block struct {
	// YAML is the text between the opening and closing "---" fences, without
	// the fences themselves.
	YAML string
	// Mapping is the top-level mapping node (Content holds key/value pairs).
	Mapping *yaml.Node
	// lineStart and lineEnd bound the block in the original content (0-based,
	// end exclusive), so the body can be reassembled verbatim.
	lineStart int
	lineEnd   int
	// newline is the line ending the block uses ("\n" or "\r\n").
	newline string
	// lines is the block YAML as EOL-stripped lines for splicing.
	lines []string
}

// Parse extracts the frontmatter block from a full document. The frontmatter
// is the region between the first line that is exactly "---" and the next such
// line.
func Parse(content string) (*Block, error) {
	lines := splitLines(content)
	if len(lines) == 0 || stripEOL(lines[0]) != "---" {
		return nil, ErrNoFrontmatter
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if stripEOL(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, ErrNoFrontmatter
	}

	block := &Block{lineStart: 0, lineEnd: end + 1, newline: "\n"}
	if strings.Contains(lines[0], "\r\n") {
		block.newline = "\r\n"
	}
	block.YAML = joinStripped(lines[1:end])
	// keySpan/insertionLine operate on stripped lines too.
	block.lines = stripAll(lines[1:end])

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(block.YAML), &doc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	block.Mapping = mappingNode(&doc)
	if block.Mapping == nil {
		return nil, fmt.Errorf("parse frontmatter: block is not a mapping")
	}
	return block, nil
}

// mappingNode returns the top-level mapping node from a parsed document node,
// or nil when the document is not a mapping.
func mappingNode(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	return doc
}

// RawValue returns the raw text after "key:" on the key's first line (the
// inline value for a scalar or flow collection; "" for a block value). Unlike
// Key, it does not resolve a sequence/alias node, so it is the right source
// for editing an existing inline list.
func (b *Block) RawValue(key string) string {
	start, _, found := b.keySpan(key)
	if !found {
		return ""
	}
	line := b.lines[start]
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+1:])
}

// Key reports whether the block has a top-level key.
func (b *Block) Key(key string) (value string, ok bool) {
	for i := 0; i+1 < len(b.Mapping.Content); i += 2 {
		if b.Mapping.Content[i].Value == key {
			return b.Mapping.Content[i+1].Value, true
		}
	}
	return "", false
}

// Set writes value for a top-level key, returning the new block YAML. A scalar
// or collection value is written verbatim as its YAML representation; the
// caller is responsible for quoting/escaping (see Scalar). A missing key is
// inserted after its declared neighbour when one exists in after, else
// appended.
func (b *Block) Set(key, value string, after []string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	lines := b.lines

	start, end, found := b.keySpan(key)
	replacement := keyLine(key, value)
	if found {
		lines = splice(lines, start, end, []string{replacement})
	} else {
		insertAt := b.insertionLine(after)
		lines = splice(lines, insertAt, insertAt, []string{replacement})
	}
	return strings.Join(lines, "\n"), nil
}

// Unset removes a top-level key, returning the new block YAML. Removing an
// absent key is a no-op.
func (b *Block) Unset(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	start, end, found := b.keySpan(key)
	if !found {
		return b.YAML, nil
	}
	lines := splice(b.lines, start, end, nil)
	return strings.Join(lines, "\n"), nil
}

// ReplaceBody rebuilds the full document with a new block YAML, keeping the
// body bytes and the fence line endings verbatim.
func (b *Block) ReplaceBody(content, newYAML string) string {
	lines := splitLines(content)
	body := lines[b.lineEnd:]
	head := b.blockHead(newYAML)
	out := append(head, body...)
	return strings.Join(out, "")
}

// blockHead renders the fence, the block YAML and the closing fence as raw
// lines (line endings included) using the block's own newline.
func (b *Block) blockHead(newYAML string) []string {
	nl := b.newline
	out := []string{"---" + nl}
	if newYAML != "" {
		for _, line := range splitLines(newYAML) {
			out = append(out, stripEOL(line)+nl)
		}
	}
	out = append(out, "---"+nl)
	return out
}

// keySpan returns the [start,end) line range of a key's entry in the block
// YAML, including any nested/indented continuation lines and a leading block
// comment attached to the key. A scalar key is a single line; a key whose value
// is a block map or list spans until the next top-level key.
func (b *Block) keySpan(key string) (start, end int, found bool) {
	lines := b.lines
	for i, line := range lines {
		k, ok := topLevelKey(line)
		if !ok || k != key {
			continue
		}
		start = i
		// Extend over nested (indented) continuation lines. A blank line or a
		// comment-only line ends the entry: a comment that follows belongs to
		// the next key, and a blank line separates entries.
		end = i + 1
		for end < len(lines) {
			if _, isTop := topLevelKey(lines[end]); isTop {
				break
			}
			trimmed := strings.TrimSpace(lines[end])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				break
			}
			end++
		}
		return start, end, true
	}
	return 0, 0, false
}

// insertionLine returns the line index at which a new key should be inserted:
// just after the last key in after that exists, else at the end of the block.
func (b *Block) insertionLine(after []string) int {
	insertAt := len(b.lines)
	for _, k := range after {
		if _, end, found := b.keySpan(k); found && end > 0 && end <= len(b.lines) {
			insertAt = end
		}
	}
	return insertAt
}

// joinStripped joins EOL-stripped lines with "\n".
func joinStripped(lines []string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = stripEOL(l)
	}
	return strings.Join(out, "\n")
}

// stripAll returns EOL-stripped copies of lines.
func stripAll(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = stripEOL(l)
	}
	return out
}

func splice(lines []string, start, end int, replacement []string) []string {
	out := make([]string, 0, len(lines)-(end-start)+len(replacement))
	out = append(out, lines[:start]...)
	out = append(out, replacement...)
	out = append(out, lines[end:]...)
	return out
}

// topLevelKey returns the key of a top-level mapping line ("key: ..."), or
// ("",false) for an indented, comment-only or blank line.
func topLevelKey(line string) (string, bool) {
	trimmed := strings.TrimRight(line, "\r")
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") {
		return "", false
	}
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return "", false
	}
	key := trimmed[:idx]
	// A key containing spaces/quotes is not a plain top-level key we address.
	if strings.ContainsAny(key, "\"'") {
		return "", false
	}
	return key, true
}

func keyLine(key, value string) string {
	if value == "" {
		return key + ":"
	}
	return key + ": " + value
}

func validateKey(key string) error {
	if key == "" {
		return errors.New("empty key")
	}
	if strings.Contains(key, ".") {
		return fmt.Errorf("nested key %q is not supported: only top-level frontmatter keys are addressable", key)
	}
	if strings.ContainsAny(key, ":\n\"'") {
		return fmt.Errorf("invalid key %q", key)
	}
	return nil
}

// Scalar renders a Go string as a YAML scalar, quoting only when needed so
// simple values stay readable and complex values escape correctly.
func Scalar(s string) string {
	return scalarString(s)
}

func scalarString(s string) string {
	if s == "" {
		return `""`
	}
	if isPlainSafe(s) {
		return s
	}
	var buf bytes.Buffer
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

// isPlainSafe reports whether s can be written as an unquoted YAML scalar: no
// leading indicator, no ": " sequence, no newline, no comment introducer.
func isPlainSafe(s string) bool {
	if strings.ContainsAny(s, "\n\r\t") {
		return false
	}
	if strings.Contains(s, ": ") || strings.Contains(s, " #") {
		return false
	}
	if strings.ContainsAny(s, "\"'\\") {
		return false
	}
	switch s[0] {
	case '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
		return false
	}
	// Values that look like other YAML types should be quoted to stay strings.
	switch strings.ToLower(s) {
	case "null", "~", "true", "false", "yes", "no", "on", "off":
		return false
	}
	return true
}

// stripEOL removes a trailing newline (LF or CRLF) from a line.
func stripEOL(line string) string {
	return strings.TrimRight(line, "\r\n")
}

// splitLines splits content into lines, each keeping its own line ending, so a
// re-join is byte-exact. The final element is "" when content ends with a
// newline.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	out := make([]string, 0, strings.Count(content, "\n")+1)
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			out = append(out, content[start:i+1])
			start = i + 1
		}
	}
	if start < len(content) {
		out = append(out, content[start:])
	}
	return out
}
