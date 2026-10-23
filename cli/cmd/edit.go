//go:build !wasm

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// corpusDir is the one directory `context edit` owns; this verb refuses it.
const corpusDir = "context"

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Replace one literal occurrence in a file, guarded and atomic",
	Long: `Replace one literal occurrence in a source file.

WHEN — a file needs a surgical change and the exact text is in hand: renaming a
call, fixing a constant, changing one branch. It is the verb a ` + "`python3`" + ` heredoc or
` + "`sed -i`" + ` was standing in for.

WHEN NOT — a corpus document changes: ` + "`sdt context edit`" + ` owns ` + "`context/`" + ` (body,
section, append) and this verb refuses it. A whole file is being written:
` + "`sdt write`" + `. Several hunks in one operation: a patch is the host's job
(` + "`git apply`" + `).

RETURNS — the unified diff between the file as it is and as it would be, then
the write. ` + "`--dry-run`" + ` prints the diff and writes nothing; an ` + "`--old`" + ` equal to
` + "`--new`" + ` reports "no change" and touches nothing.

THE GUARD IS THE POINT — a literal must occur exactly once. Zero occurrences is
an error that names the file; more than one is an error that names the count and
points at ` + "`--all`" + `. Guessing which occurrence was meant is how a literal edit
corrupts a source file, so the verb refuses instead.

IT REFUSES WHAT IT MUST NOT TOUCH — a path under ` + "`context/`" + `, a symlink, a binary
file, a file that is not valid UTF-8, and a path outside the project root when
the project declares one. The write is atomic: a temp file in the same
directory, the original mode, then a rename, so a failed write leaves the file
as it was.

A CRLF FILE STILL MATCHES — the ` + "`\n`" + ` in ` + "`--old`" + ` and ` + "`--new`" + ` is converted to the file's
own newline, so a shell literal does not silently fail to match.

EXAMPLE — ` + "`sdt edit -p cli/cmd/file.go --old 'not exist' --new 'not found'`" + `
          ` + "`sdt edit -p go.mod --old-file old.txt --new-file new.txt --dry-run`" + ``,
	Example: `  sdt edit -p cli/cmd/file.go --old 'old text' --new 'new text'
  sdt edit -p AGENTS.md --old 'x' --new 'y' --all
  sdt edit -p go.mod --old-file old.txt --new-file new.txt --dry-run`,
	Args: cobra.NoArgs,
	Run:  runEdit,
}

// editResult is the machine-readable outcome, shaped like contextEditResult so
// an agent parses one result object for every mutator SDT ships.
type editResult struct {
	Path        string `json:"path" yaml:"path"`
	Changed     bool   `json:"changed" yaml:"changed"`
	Occurrences int    `json:"occurrences" yaml:"occurrences"`
	Replaces    int    `json:"replaces" yaml:"replaces"`
	DryRun      bool   `json:"dry_run" yaml:"dry_run"`
	Diff        string `json:"diff,omitempty" yaml:"diff,omitempty"`
}

func runEdit(cmd *cobra.Command, args []string) {
	path := getStringFlag(cmd, "file", true)
	oldText, err := editText(cmd, "old")
	exitWithError(cmd, err)
	if oldText == "" {
		exitWithError(cmd, fmt.Errorf("--old is empty: an empty literal matches everywhere, so there is no safe replacement to make"))
		return
	}
	newText, err := editText(cmd, "new")
	exitWithError(cmd, err)
	if !cmd.Flags().Changed("new") && !cmd.Flags().Changed("new-file") {
		exitWithError(cmd, fmt.Errorf("no replacement: pass --new (empty to delete the literal) or --new-file"))
		return
	}

	abs, err := filepath.Abs(path)
	exitWithError(cmd, err)
	info, err := os.Lstat(abs)
	exitWithError(cmd, err)
	if info.Mode()&os.ModeSymlink != 0 {
		exitWithError(cmd, fmt.Errorf("%s is a symlink: edit the file it points at, by its own path", path))
		return
	}
	if !info.Mode().IsRegular() {
		exitWithError(cmd, fmt.Errorf("%s is not a regular file", path))
		return
	}
	exitWithError(cmd, editRefuseCorpus(abs, path))

	// #nosec G304 -- the path is the command's subject, validated above
	data, err := os.ReadFile(abs)
	exitWithError(cmd, err)
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		exitWithError(cmd, fmt.Errorf("%s is binary or not valid UTF-8: a literal edit would corrupt it", path))
		return
	}
	content := string(data)

	// A CRLF file matches neither a shell literal's `\n` nor its own bytes; the
	// conversion makes the literal mean what the caller typed.
	crlf := strings.Contains(content, "\r\n")
	oldText = editNewlines(oldText, crlf)
	newText = editNewlines(newText, crlf)

	count := strings.Count(content, oldText)
	if count == 0 {
		exitWithError(cmd, fmt.Errorf("the literal does not occur in %s", path))
		return
	}
	all := getBoolFlag(cmd, "all", false)
	if count > 1 && !all {
		exitWithError(cmd, fmt.Errorf("the literal occurs %d times in %s: make it unique, or pass --all to replace every occurrence", count, path))
		return
	}

	replaces := 1
	if all {
		replaces = count
	}
	if oldText == newText {
		outputEdit(cmd, editResult{Path: path, Occurrences: count, Replaces: 0, DryRun: getBoolFlag(cmd, "dry-run", false)})
		return
	}
	updated := strings.Replace(content, oldText, newText, replaces)
	diff := unifiedDiff(path, path, strings.Split(content, "\n"), strings.Split(updated, "\n"), 3)
	dryRun := getBoolFlag(cmd, "dry-run", false)
	outputEdit(cmd, editResult{
		Path:        path,
		Changed:     true,
		Occurrences: count,
		Replaces:    replaces,
		DryRun:      dryRun,
		Diff:        diff,
	})
	if dryRun {
		return
	}
	exitWithError(cmd, writeFileAtomic(abs, []byte(updated), info.Mode().Perm()))
}

// editText reads one side of the replacement from its flag or its --*-file
// twin. The pair is mutually exclusive: two sources for one side is a caller
// bug, not a precedence rule to guess at.
func editText(cmd *cobra.Command, name string) (string, error) {
	file := name + "-file"
	hasFlag := cmd.Flags().Changed(name)
	hasFile := cmd.Flags().Changed(file)
	if hasFlag && hasFile {
		return "", fmt.Errorf("--%s and --%s both name the %s: pass one", name, file, name)
	}
	if !hasFile {
		return getStringFlag(cmd, name, false), nil
	}
	path := getStringFlag(cmd, file, false)
	data, err := os.ReadFile(path) //#nosec G304 -- the caller named the file
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// editRefuseCorpus keeps the corpus to one mutator: `context edit` owns it, and
// a second writer for the same document is the duplication the doctrine forbids.
// A path outside the project root is refused for the mirror reason: the verb's
// subject is the project.
func editRefuseCorpus(abs, given string) error {
	root := sdtProjectRoot()
	if root == "" {
		return nil
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil
	}
	slash := filepath.ToSlash(rel)
	if slash == ".." || strings.HasPrefix(slash, "../") {
		return fmt.Errorf("%s is outside the project root %s", given, root)
	}
	if slash == corpusDir || strings.HasPrefix(slash, corpusDir+"/") {
		return fmt.Errorf("%s is a corpus document: use sdt context edit (body, section or --append) for context/", given)
	}
	return nil
}

// editNewlines adapts a literal to a CRLF file: normalise first, so a literal
// that already carries \r\n is not doubled, then apply the file's newline.
func editNewlines(text string, crlf bool) string {
	if !crlf {
		return text
	}
	normalised := strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(normalised, "\n", "\r\n")
}

// writeFileAtomic replaces path with data through a same-directory temp file:
// a failed write leaves the original untouched, and the rename keeps the mode
// the caller passed.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sdt-edit-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		// No-op once the rename succeeded; nothing to report either way.
		_ = os.Remove(name) //nolint:errcheck
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck // the write error is the one that matters
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck // the sync error is the one that matters
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// outputEdit renders the outcome like every other mutator: text prints the diff
// alone, json/yaml carry the facts an agent checks.
func outputEdit(cmd *cobra.Command, res editResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, append(out, '\n'))
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

func init() {
	f := editCmd.Flags()
	f.StringP("file", "p", "", "File to edit")
	f.String("old", "", "Literal text to replace (must occur exactly once)")
	f.String("new", "", "Replacement text (empty deletes the literal)")
	f.String("old-file", "", "Read the literal to replace from this file")
	f.String("new-file", "", "Read the replacement text from this file")
	f.Bool("all", false, "Replace every occurrence instead of requiring exactly one")
	f.Bool("dry-run", false, "Print the diff without writing")
	rootCmd.AddCommand(editCmd)
}
