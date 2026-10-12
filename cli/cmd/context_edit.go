package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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

WHEN — the body of an existing document must change and the new text is in hand
(a heredoc, a file, a pipe, the output of an editor).

WHEN NOT — a frontmatter key changes: use sdt context set <doc> <key> <value>.
A whole new document is created: use sdt context new.

RETURNS — the unified diff between the document as it is and as it would be,
then the write. --dry-run prints the diff and writes nothing. When the new text
leaves the body byte-identical the command reports "no change" and touches
nothing; the CLI-owned updated field is refreshed on a real edit, as with
sdt context set.

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
	if fm, _ := contextwiki.SplitFrontmatter(newText); fm != "" {
		exitWithError(cmd, fmt.Errorf("the input carries a frontmatter block; context edit writes the body only: use sdt context set <doc> <key> <value> for frontmatter keys"))
		return
	}
	text := ensureTrailingNewline(newText)

	fm, body := contextwiki.SplitFrontmatter(content)
	newBody := text
	if section := getStringFlag(cmd, "section", false); section != "" {
		newBody, err = replaceContextSection(body, section, text)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
	}
	if newBody == body {
		outputContextEdit(cmd, contextEditResult{Path: doc.Path, Section: getStringFlag(cmd, "section", false), DryRun: getBoolFlag(cmd, "dry-run", false)})
		return
	}

	newContent := refreshUpdated(cmd, doc, "", fm+newBody)
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

func init() {
	addContextStatusRefFlags(contextEditCmd)
	contextEditCmd.Flags().String("section", "", "Edit only this section (its stable id or its heading)")
	contextEditCmd.Flags().Bool("dry-run", false, "Print the diff without writing")
	contextCmd.AddCommand(contextEditCmd)
}
