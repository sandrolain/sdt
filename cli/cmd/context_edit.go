package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/mdstruct"
	"github.com/spf13/cobra"
)

var contextEditCmd = &cobra.Command{
	Use:   "edit <doc>",
	Short: "Replace a document body, or one of its sections, from input",
	Long: `Replace the body of a context document, or one of its sections, with text read
from --input, --file, --inb64 or piped stdin.

The frontmatter is never written. A frontmatter key goes through
sdt context set; an input that carries a leading frontmatter block is refused
rather than silently dropped, so an edit can never look like it changed a key
when it did not.

<doc> is resolved in this order: a path under context/ (absolute, context/…, or
kind/name); a bare file name that matches exactly one document; a wiki page id
(for example architecture/deployment-pipeline) or a wiki page file name at any
depth; a document uid. The first form that matches wins, and several matches are
refused with the candidates rather than guessed.

WHEN — the body of an existing document must change and the new text is in hand
(a heredoc, a file, a pipe, the output of an editor).

WHEN NOT — a frontmatter key changes: use sdt context set <doc> <key> <value>.
A whole new document is created: use sdt context new.

RETURNS — the unified diff between the document as it is and as it would be,
then the write. --dry-run prints the diff and writes nothing. When the new text
leaves the body byte-identical the command reports "no change" and touches
nothing; the CLI-owned updated field is refreshed on a real edit, as with
sdt context set.

A SECTION IS THE HEADING AND ITS BODY — --section replaces the whole section, so
the text must begin with that section's heading, at its own level (for example
## Claims). A body without the heading is refused rather than spliced in, which
would delete the anchor a reader navigates by.

--append ADDS INSTEAD OF REPLACING — the text goes after the body's last line,
or after the last line of --section, and carries no heading. It is the shape for
one checklist item, one section, one log line: the worklog and note bodies that
used to be re-sent whole through --file.

AN EMPTY BODY IS A DESTRUCTION — it empties the document, or removes the section
when --section is named. A body source that yields no text is refused unless
--force is given, and the refusal names the document; a caller that pipes the
wrong file cannot wipe a page by accident. --force covers emptiness only: it
never accepts a frontmatter block or a section body without its heading.

EXAMPLE — sdt context edit plan/x --section wave-2 --file wave-2.md
          printf '## Findings\n…\n' | sdt context edit research/x --dry-run`,
	Example: `  sdt context edit plan/x --section wave-2 --file wave-2.md
  printf '## Findings\n' | sdt context edit research/x --dry-run`,
	Args: cobra.MaximumNArgs(1),
	Run:  runContextEdit,
}

// runContextEdit replaces the body (or one section) and reports the change. It
// only ever writes the body: lifecycle state stays owned by the CLI.
func runContextEdit(cmd *cobra.Command, args []string) {
	doc, err := resolveContextDoc(cmd, args)
	if err != nil {
		exitWithError(cmd, err)
		return
	}

	//#nosec G304 -- resolved context document
	data, err := os.ReadFile(doc.Path)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	content := string(data)

	newText := getContextBody(cmd, nil)
	if newText == "" && !contextBodyFlagSet(cmd) {
		exitWithError(cmd, fmt.Errorf("no replacement body: pass --input, --file or --inb64, or pipe it on stdin"))
		return
	}
	if newText == "" && !getBoolFlag(cmd, "force", false) {
		target := "the document"
		switch {
		case getStringFlag(cmd, "section", false) != "":
			target = fmt.Sprintf("section %q", getStringFlag(cmd, "section", false))
		case getStringFlag(cmd, "row", false) != "":
			target = fmt.Sprintf("row %q", getStringFlag(cmd, "row", false))
		case getStringFlag(cmd, "fence", false) != "":
			target = fmt.Sprintf("fence %q", getStringFlag(cmd, "fence", false))
		}
		exitWithError(cmd, fmt.Errorf("refusing an empty body for %s: an empty replacement removes %s; pass --force to do it", doc.Path, target))
		return
	}
	if fm, _ := contextwiki.SplitFrontmatter(newText); fm != "" {
		exitWithError(cmd, fmt.Errorf("the input carries a frontmatter block; context edit writes the body only: use sdt context set <doc> <key> <value> for frontmatter keys"))
		return
	}
	text := ensureTrailingNewline(newText)
	section := getStringFlag(cmd, "section", false)
	row := getStringFlag(cmd, "row", false)
	fence := getStringFlag(cmd, "fence", false)
	if row != "" && fence != "" {
		exitWithError(cmd, fmt.Errorf("--row and --fence address different structures: pass one"))
		return
	}
	if getBoolFlag(cmd, "append", false) && (row != "" || fence != "") {
		exitWithError(cmd, fmt.Errorf("--append adds text; it cannot be combined with --row or --fence, which replace one structure"))
		return
	}
	if fence != "" && section != "" {
		exitWithError(cmd, fmt.Errorf("--fence addresses the whole body: drop --section, or use --row for a table inside a section"))
		return
	}

	fm, body := contextwiki.SplitFrontmatter(content)
	var newBody string
	switch {
	case row != "":
		newBody, err = replaceContextRow(body, section, row, text)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("%s: %w", doc.Path, err))
			return
		}
	case fence != "":
		newBody, err = replaceContextFence(body, fence, text)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("%s: %w", doc.Path, err))
			return
		}
	case getBoolFlag(cmd, "append", false):
		if strings.TrimSpace(newText) == "" {
			exitWithError(cmd, fmt.Errorf("nothing to append to %s: pass the text through --input, --file or stdin", doc.Path))
			return
		}
		newBody, err = appendContextBody(body, section, text)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("%s: %w", doc.Path, err))
			return
		}
	case section != "":
		newBody, err = replaceContextSection(body, section, text)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("%s: %w", doc.Path, err))
			return
		}
	default:
		newBody = text
	}
	if newBody == body {
		outputContextEdit(cmd, contextEditResult{Path: doc.Path, Section: getStringFlag(cmd, "section", false), DryRun: getBoolFlag(cmd, "dry-run", false)})
		return
	}

	newContent := finalizeDocWrite(cmd, doc, "", fm+newBody)
	diff := unifiedDiff(doc.Path, doc.Path, strings.Split(content, "\n"), strings.Split(newContent, "\n"), 3)
	dryRun := getBoolFlag(cmd, "dry-run", false)
	outputContextEdit(cmd, contextEditResult{
		Path:    doc.Path,
		Section: getStringFlag(cmd, "section", false),
		Changed: true,
		DryRun:  dryRun,
		Diff:    diff,
	})
	if dryRun {
		return
	}
	if err := writeContextDocFile(doc.Path, newContent); err != nil {
		exitWithError(cmd, err)
		return
	}
}

// contextEditResult is the machine-readable outcome of an edit: the text form
// prints the diff alone, while json/yaml carry the same diff plus the facts an
// agent checks — whether anything changed and whether it was written.
type contextEditResult struct {
	Path    string `json:"path" yaml:"path"`
	Section string `json:"section,omitempty" yaml:"section,omitempty"`
	Changed bool   `json:"changed" yaml:"changed"`
	DryRun  bool   `json:"dry_run" yaml:"dry_run"`
	Diff    string `json:"diff,omitempty" yaml:"diff,omitempty"`
}

// outputContextEdit renders the edit outcome in the selected format.
func outputContextEdit(cmd *cobra.Command, res contextEditResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		if !res.Changed {
			outputString(cmd, "no change\n")
			return
		}
		outputString(cmd, res.Diff)
	}
}

// contextBodyFlagSet reports whether a body source was named explicitly. An
// empty replacement is accepted only when the caller said so (--input "" or an
// empty --file); otherwise the command refuses rather than blanking a body.
func contextBodyFlagSet(cmd *cobra.Command) bool {
	f := cmd.Flags()
	return f.Lookup("input").Changed || f.Lookup("file").Changed || f.Lookup("inb64").Changed
}

// ensureTrailingNewline keeps a non-empty body ending in exactly one newline, so
// a piped or --input value cannot leave the document without its final newline.
func ensureTrailingNewline(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

// replaceContextSection returns body with the named section's bytes replaced by
// text. The section is matched by its stable id or its heading text — the same
// selection `context show --section` makes — so a show → edit round trip names
// the same section. An empty text removes the section.
func replaceContextSection(body, sectionRef, text string) (string, error) {
	lines := strings.Split(body, "\n")
	// mdstruct drops the single empty element a terminating newline produces, so
	// its 1-based line numbers index this slice directly.
	trailing := false
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
		trailing = true
	}
	for _, s := range mdstruct.SplitSections(body) {
		if !strings.EqualFold(s.ID, sectionRef) && !strings.EqualFold(s.Heading, sectionRef) {
			continue
		}
		var repl []string
		if text != "" {
			if err := checkSectionReplacement(text, s); err != nil {
				return "", err
			}
			repl = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		}
		out := make([]string, 0, len(lines)+len(repl))
		out = append(out, lines[:s.StartLine-1]...)
		out = append(out, repl...)
		out = append(out, lines[s.EndLine:]...)
		res := strings.Join(out, "\n")
		if trailing {
			res += "\n"
		}
		return res, nil
	}
	return "", fmt.Errorf("section %q not found", sectionRef)
}

// appendContextBody returns body with text appended after its last line, or
// after the last line of the named section. Unlike a replacement, the text does
// not carry the heading: append adds a line, a checklist item or a block to
// what is already there, which is the case that used to mean re-sending a whole
// body through --file.
func appendContextBody(body, sectionRef, text string) (string, error) {
	lines := strings.Split(body, "\n")
	trailing := false
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
		trailing = true
	}
	// insertAt is the 0-based index just past the last line being appended to.
	insertAt := len(lines)
	if sectionRef != "" {
		found := false
		for _, s := range mdstruct.SplitSections(body) {
			if !strings.EqualFold(s.ID, sectionRef) && !strings.EqualFold(s.Heading, sectionRef) {
				continue
			}
			// mdstruct reports EndLine 1-based and inclusive; the index after it
			// is the same number.
			insertAt = s.EndLine
			found = true
			break
		}
		if !found {
			return "", fmt.Errorf("section %q not found", sectionRef)
		}
		// The section's range includes the blank line before the next heading;
		// appending before it keeps the block separated from that heading.
		for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
	}
	repl := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	out := make([]string, 0, len(lines)+len(repl))
	out = append(out, lines[:insertAt]...)
	out = append(out, repl...)
	out = append(out, lines[insertAt:]...)
	res := strings.Join(out, "\n")
	if trailing {
		res += "\n"
	}
	return res, nil
}

// checkSectionReplacement refuses a non-empty body that does not begin with the
// heading it is replacing. mdstruct's section range is heading-inclusive by
// design, so splicing a body without the heading would delete it — the anchor a
// reader navigates by. Refusing is honest; auto-prepending would guess.
func checkSectionReplacement(text string, s mdstruct.Section) error {
	first := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	level, heading, ok := splitATXHeading(first)
	want := headingLine(s)
	if !ok || level != s.Level || !strings.EqualFold(heading, s.Heading) {
		return fmt.Errorf("a section replacement must begin with its heading %q, got %q", want, first)
	}
	return nil
}

// headingLine renders a section's heading as it is written in the document.
func headingLine(s mdstruct.Section) string {
	if s.Level == 0 {
		return ""
	}
	return strings.Repeat("#", s.Level) + " " + s.Heading
}

// splitATXHeading parses one ATX heading line into its level and text, dropping
// an optional closing hash run the way mdstruct does.
func splitATXHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	rest := line[level:]
	if rest == "" || rest[0] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(strings.TrimRight(rest, "#")), true
}

func init() {
	addContextStatusRefFlags(contextEditCmd)
	contextEditCmd.Flags().String("section", "", "Edit only this section (its stable id or its heading)")
	contextEditCmd.Flags().String("row", "", "Replace the table row whose first cell is this literal")
	contextEditCmd.Flags().String("fence", "", "Replace the fenced block by 1-based index or by its opening info string")
	contextEditCmd.Flags().Bool("append", false, "Append the input after the body, or after --section, instead of replacing")
	contextEditCmd.Flags().Bool("dry-run", false, "Print the diff without writing")
	contextEditCmd.Flags().Bool("force", false, "Allow an empty body (empties the document, or removes the section)")
	addProvenanceFlags(contextEditCmd)
	contextCmd.AddCommand(contextEditCmd)
}

// splitBodyLines splits a body into lines, dropping the empty element a
// terminating newline produces and reporting whether it was there.
func splitBodyLines(body string) ([]string, bool) {
	lines := strings.Split(body, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		return lines[:n-1], true
	}
	return lines, false
}

// replaceContextRow replaces the one table row whose first cell equals the
// literal, inside the named section when one is given. The row is one line: a
// multi-line replacement would make the table unreadable, so it is refused.
func replaceContextRow(body, sectionRef, firstCell, text string) (string, error) {
	lines, trailing := splitBodyLines(body)
	lo, hi := 1, len(lines)
	if sectionRef != "" {
		found := false
		for _, s := range mdstruct.SplitSections(body) {
			if !strings.EqualFold(s.ID, sectionRef) && !strings.EqualFold(s.Heading, sectionRef) {
				continue
			}
			lo, hi, found = s.StartLine, s.EndLine, true
			break
		}
		if !found {
			return "", fmt.Errorf("section %q not found", sectionRef)
		}
	}
	var rows []int
	for i := lo - 1; i < hi && i < len(lines); i++ {
		cells, ok := tableRowCells(lines[i])
		if !ok {
			continue
		}
		if strings.TrimSpace(cells[0]) == firstCell {
			rows = append(rows, i)
		}
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no table row starts with %q", firstCell)
	}
	if len(rows) > 1 {
		return "", fmt.Errorf("%d table rows start with %q: make the first cell unique", len(rows), firstCell)
	}
	replacement := strings.TrimSuffix(text, "\n")
	if strings.Contains(replacement, "\n") {
		return "", fmt.Errorf("a table row is one line: the replacement carries %d", strings.Count(replacement, "\n")+1)
	}
	lines[rows[0]] = replacement
	return joinBodyLines(lines, trailing), nil
}

// tableRowCells splits a `| a | b |` row into its trimmed cells. A separator
// row (all dashes and colons) is not a row.
func tableRowCells(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil, false
	}
	parts := strings.Split(strings.Trim(trimmed, "|"), "|")
	if len(parts) == 0 {
		return nil, false
	}
	separator := true
	for _, p := range parts {
		cell := strings.TrimSpace(p)
		if cell == "" || strings.Trim(cell, "-:") != "" {
			separator = false
			break
		}
	}
	if separator {
		return nil, false
	}
	return parts, true
}

// replaceContextFence replaces the inner content of one fenced block, keeping
// its opening and closing fences. The selector is a 1-based index or the info
// string on the opening fence.
func replaceContextFence(body, selector, text string) (string, error) {
	lines, trailing := splitBodyLines(body)
	type fence struct {
		open, close int
		info        string
	}
	var fences []fence
	for i := 0; i < len(lines); i++ {
		marker, info, ok := fenceOpen(lines[i])
		if !ok {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if isFenceClose(lines[j], marker) {
				fences = append(fences, fence{open: i, close: j, info: info})
				i = j
				break
			}
		}
	}
	var matches []int
	if n, err := strconv.Atoi(selector); err == nil {
		if n < 1 || n > len(fences) {
			return "", fmt.Errorf("fence %d does not exist: the body has %d", n, len(fences))
		}
		matches = append(matches, n-1)
	} else {
		for i, f := range fences {
			first := ""
			if fields := strings.Fields(f.info); len(fields) > 0 {
				first = fields[0]
			}
			if f.info == selector || first == selector {
				matches = append(matches, i)
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no fenced block matches %q", selector)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%d fenced blocks match %q: address one by its 1-based index", len(matches), selector)
	}
	f := fences[matches[0]]
	repl := []string{}
	if text != "" {
		repl = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	out := make([]string, 0, len(lines)+len(repl))
	out = append(out, lines[:f.open+1]...)
	out = append(out, repl...)
	out = append(out, lines[f.close:]...)
	return joinBodyLines(out, trailing), nil
}

// fenceOpen reads an opening fence and its info string.
func fenceOpen(line string) (marker, info string, ok bool) {
	trimmed := strings.TrimSpace(line)
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, m) {
			run := 0
			for run < len(trimmed) && trimmed[run] == m[0] {
				run++
			}
			return trimmed[:run], strings.TrimSpace(trimmed[run:]), true
		}
	}
	return "", "", false
}

// isFenceClose reports whether line closes the fence opened by marker.
func isFenceClose(line, marker string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, marker) && strings.Trim(trimmed[len(marker):], " ") == ""
}

func joinBodyLines(lines []string, trailing bool) string {
	res := strings.Join(lines, "\n")
	if trailing {
		res += "\n"
	}
	return res
}
