//go:build !wasm

package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

` + "`--batch`" + ` edits several files from one JSON or YAML manifest, so the
N-files × M-pairs fallback (` + "`python3 - <<PY`" + ` with per-pair assertions) is one call:
each entry names a file and its literal pairs, each pair may pin the occurrence
count it expects (` + "`count: 0`" + ` is a guard that refuses when the literal occurs),
every pair asserts before anything is written, and each file is one atomic
transaction — a refused file is left byte-identical while the others apply. The
exit is 0 when every file applied, 3 when some applied and some were refused,
1 when nothing applied. Read the manifest from a path, or pass ` + "`-`" + ` (or nothing)
after ` + "`--batch`" + ` to read it from stdin. Pairs apply in order: a later pair sees
the content its predecessors replaced.

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
          ` + "`sdt edit -p go.mod --old-file old.txt --new-file new.txt --dry-run`" + `
          ` + "`sdt edit --batch batch.yaml`" + ``,
	Example: `  sdt edit -p cli/cmd/file.go --old 'old text' --new 'new text'
  sdt edit -p AGENTS.md --old 'x' --new 'y' --all
  sdt edit -p go.mod --old-file old.txt --new-file new.txt --dry-run`,
	Args: cobra.NoArgs,
	Run:  runEdit,
}

// editBatchManifest is the --batch payload: files, each with its literal pairs
// and the guards that must hold before anything is written.
type editBatchManifest struct {
	Files []editBatchFile `json:"files" yaml:"files"`
}

// editBatchFile is one file of the manifest: its path, the pairs to apply in
// order, and the per-file guards.
type editBatchFile struct {
	File   string          `json:"file" yaml:"file"`
	Pairs  []editBatchPair `json:"pairs" yaml:"pairs"`
	Expect string          `json:"expect,omitempty" yaml:"expect,omitempty"`
	DryRun bool            `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// editBatchPair is one literal replacement. Count is the occurrence count the
// pair expects: absent means exactly once, 0 is a guard that refuses when the
// literal occurs, N replaces all N.
type editBatchPair struct {
	Old   string `json:"old" yaml:"old"`
	New   string `json:"new" yaml:"new"`
	Count *int   `json:"count,omitempty" yaml:"count,omitempty"`
}

// editBatchPairResult is one pair's outcome, the per-pair assertion the python
// fallback reimplemented by hand.
type editBatchPairResult struct {
	Old      string `json:"old" yaml:"old"`
	Found    int    `json:"found" yaml:"found"`
	Replaced int    `json:"replaced" yaml:"replaced"`
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
}

// editBatchFileResult is one file's outcome. Pairs carry the per-pair facts;
// Error names why nothing was written.
type editBatchFileResult struct {
	Path    string                `json:"path" yaml:"path"`
	Changed bool                  `json:"changed" yaml:"changed"`
	DryRun  bool                  `json:"dry_run" yaml:"dry_run"`
	Pairs   []editBatchPairResult `json:"pairs,omitempty" yaml:"pairs,omitempty"`
	Diff    string                `json:"diff,omitempty" yaml:"diff,omitempty"`
	Error   string                `json:"error,omitempty" yaml:"error,omitempty"`
}

// editBatchResult is the whole manifest's outcome.
type editBatchResult struct {
	Files   []editBatchFileResult `json:"files" yaml:"files"`
	Applied int                   `json:"applied" yaml:"applied"`
	Refused int                   `json:"refused" yaml:"refused"`
}

// The addressing modes. The names are the JSON result's `mode` value, so one
// spelling serves the resolver, the dispatcher and the report.
const (
	editModeLiteral = "literal"
	editModeBatch   = "batch"
	editModeRegex   = "regex"
	editModeMarkers = "markers"
	editModeLines   = "lines"
)

// editResult is the machine-readable outcome, shaped like contextEditResult so
// an agent parses one result object for every mutator SDT ships.
type editResult struct {
	Path        string `json:"path" yaml:"path"`
	Mode        string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Changed     bool   `json:"changed" yaml:"changed"`
	Occurrences int    `json:"occurrences" yaml:"occurrences"`
	Replaces    int    `json:"replaces" yaml:"replaces"`
	DryRun      bool   `json:"dry_run" yaml:"dry_run"`
	BlockSHA256 string `json:"block_sha256,omitempty" yaml:"block_sha256,omitempty"`
	Pattern     string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Diff        string `json:"diff,omitempty" yaml:"diff,omitempty"`
}

func runEdit(cmd *cobra.Command, args []string) {
	if cmd.Flags().Changed("batch") {
		runEditBatch(cmd)
		return
	}
	path := getStringFlag(cmd, editFileFlag, true)
	mode, err := editModeOf(cmd)
	exitWithError(cmd, err)
	if mode == "" {
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
	crlf := strings.Contains(content, "\r\n")
	all := getBoolFlag(cmd, "all", false)

	out, err := editApply(cmd, mode, content, crlf, all)
	if err != nil {
		exitWithError(cmd, fmt.Errorf("%s: %w", path, err))
		return
	}
	if err != nil {
		return
	}
	if out.updated == content {
		outputEdit(cmd, editResult{Path: path, Mode: mode, Occurrences: out.occurrences, Replaces: 0, DryRun: getBoolFlag(cmd, "dry-run", false)})
		return
	}
	diff := unifiedDiff(path, path, strings.Split(content, "\n"), strings.Split(out.updated, "\n"), 3)
	dryRun := getBoolFlag(cmd, "dry-run", false)
	outputEdit(cmd, editResult{
		Path:        path,
		Mode:        mode,
		Changed:     true,
		Occurrences: out.occurrences,
		Replaces:    out.replaces,
		DryRun:      dryRun,
		BlockSHA256: out.blockSHA,
		Pattern:     out.pattern,
		Diff:        diff,
	})
	if dryRun {
		return
	}
	exitWithError(cmd, writeFileAtomic(abs, []byte(out.updated), info.Mode().Perm()))
}

// editFileFlag is the target-file flag's name, shared by the single-file form
// and the --batch mixed-modes guard.
const editFileFlag = "file"

// runEditBatch applies a manifest: every pair asserts before anything is
// written, every file is one transaction, and the exit follows decision 0037 —
// 0 all applied, 3 some applied, 1 nothing applied.
func runEditBatch(cmd *cobra.Command) {
	if mode, err := editModeOf(cmd); err == nil && mode != "" {
		exitWithError(cmd, fmt.Errorf("--batch edits from the manifest alone: the %s mode is already addressed, pass the mode flags without --batch", mode))
	}
	if cmd.Flags().Changed(editFileFlag) {
		exitWithError(cmd, fmt.Errorf("--batch edits from the manifest alone: pass --%s without --batch, or the manifest without it", editFileFlag))
	}
	data := editBatchManifestBytes(cmd)
	var m editBatchManifest
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	var err error
	if len(trimmed) > 0 && trimmed[0] == '{' {
		err = json.Unmarshal(data, &m)
	} else {
		err = yaml.Unmarshal(data, &m)
	}
	exitWithError(cmd, err)
	if len(m.Files) == 0 {
		exitWithError(cmd, fmt.Errorf("the manifest names no files"))
	}

	res := editBatchResult{Files: make([]editBatchFileResult, 0, len(m.Files))}
	for _, bf := range m.Files {
		fr := editBatchOne(cmd, bf)
		res.Files = append(res.Files, fr)
		if fr.failed() {
			res.Refused++
		} else {
			res.Applied++
		}
	}
	outputEditBatch(cmd, res)
	switch {
	case res.Refused == 0:
		return
	case res.Applied == 0:
		exitWithError(cmd, fmt.Errorf("no file applied: %d of %d refused", res.Refused, len(res.Files)))
	default:
		exitWithJunk(cmd, fmt.Sprintf("%d of %d files applied", res.Applied, len(res.Files)))
	}
}

// failed says whether the file was written: a file-level error or a refused
// pair means the transaction did not happen.
func (r editBatchFileResult) failed() bool {
	if r.Error != "" {
		return true
	}
	for _, p := range r.Pairs {
		if p.Error != "" {
			return true
		}
	}
	return false
}

// editBatchManifestBytes reads the manifest: the flag's value is a path, "-"
// or empty reads stdin (getInputBytes exits on a read error itself).
func editBatchManifestBytes(cmd *cobra.Command) []byte {
	spec := getStringFlag(cmd, "batch", false)
	if spec == "" || spec == "-" {
		return getInputBytes(cmd, nil)
	}
	data, err := os.ReadFile(spec) //#nosec G304 -- the caller named the manifest path
	exitWithError(cmd, err)
	return data
}

// editBatchOne processes one manifest entry: the file guards first, then every
// pair's assertion, then the single atomic write. A refusal leaves the file
// byte-identical.
func editBatchOne(cmd *cobra.Command, bf editBatchFile) editBatchFileResult {
	res := editBatchFileResult{Path: bf.File, DryRun: bf.DryRun || getBoolFlag(cmd, "dry-run", false)}
	if bf.File == "" {
		res.Error = "the entry names no file"
		return res
	}
	if len(bf.Pairs) == 0 {
		res.Error = "the entry has no pairs"
		return res
	}
	abs, err := filepath.Abs(bf.File)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	info, err := os.Lstat(abs)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if info.Mode()&os.ModeSymlink != 0 {
		res.Error = fmt.Sprintf("%s is a symlink: edit the file it points at, by its own path", bf.File)
		return res
	}
	if !info.Mode().IsRegular() {
		res.Error = fmt.Sprintf("%s is not a regular file", bf.File)
		return res
	}
	if err := editRefuseCorpus(abs, bf.File); err != nil {
		res.Error = err.Error()
		return res
	}
	// #nosec G304 -- the path comes from the manifest the caller supplied
	data, err := os.ReadFile(abs)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		res.Error = fmt.Sprintf("%s is binary or not valid UTF-8: a literal edit would corrupt it", bf.File)
		return res
	}
	content := string(data)
	if bf.Expect != "" {
		if err := editCheckHash(bf.Expect, content); err != nil {
			res.Error = err.Error()
			return res
		}
	}

	crlf := strings.Contains(content, "\r\n")
	updated := content
	for i, p := range bf.Pairs {
		pr := editBatchPairResult{Old: p.Old}
		if p.Old == "" {
			pr.Error = fmt.Sprintf("pair %d: the literal is empty", i+1)
			res.Pairs = append(res.Pairs, pr)
			return res
		}
		want := 1
		if p.Count != nil {
			want = *p.Count
		}
		if want < 0 {
			pr.Error = fmt.Sprintf("pair %d: count %d is negative", i+1, want)
			res.Pairs = append(res.Pairs, pr)
			return res
		}
		pr.Found = strings.Count(updated, editNewlines(p.Old, crlf))
		if pr.Found != want {
			pr.Error = fmt.Sprintf("pair %d: expected %d occurrence(s), found %d", i+1, want, pr.Found)
			res.Pairs = append(res.Pairs, pr)
			return res
		}
		if want > 0 {
			updated = strings.Replace(updated, editNewlines(p.Old, crlf), editNewlines(p.New, crlf), want)
			pr.Replaced = want
		}
		res.Pairs = append(res.Pairs, pr)
	}
	if updated == content {
		return res
	}
	res.Changed = true
	res.Diff = unifiedDiff(bf.File, bf.File, strings.Split(content, "\n"), strings.Split(updated, "\n"), 3)
	if res.DryRun {
		return res
	}
	if err := writeFileAtomic(abs, []byte(updated), info.Mode().Perm()); err != nil {
		res.Error = err.Error()
		res.Changed = false
		res.Diff = ""
	}
	return res
}

// outputEditBatch renders the whole manifest's outcome: text prints one line
// per refused file or pair plus each diff, json/yaml carry the facts.
func outputEditBatch(cmd *cobra.Command, res editBatchResult) {
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
		var b strings.Builder
		for _, fr := range res.Files {
			if fr.Error != "" {
				fmt.Fprintf(&b, "%s: %s\n", fr.Path, fr.Error)
				continue
			}
			for _, p := range fr.Pairs {
				if p.Error != "" {
					fmt.Fprintf(&b, "%s: %s\n", fr.Path, p.Error)
				}
			}
			if fr.failed() {
				continue
			}
			if !fr.Changed {
				fmt.Fprintf(&b, "%s: no change\n", fr.Path)
				continue
			}
			b.WriteString(fr.Diff)
		}
		outputString(cmd, b.String())
	}
}

// editOutcome is what one addressing mode produced: the new content, the counts
// the result reports and, for a block, the hash of the region before the write.
type editOutcome struct {
	updated     string
	occurrences int
	replaces    int
	blockSHA    string
	pattern     string
}

// editModeOf resolves exactly one addressing mode from the flags. Two modes are
// a caller bug, not a precedence rule to guess at. --old belongs to both the
// literal pair and --regex, so it is not counted as a mode of its own.
func editModeOf(cmd *cobra.Command) (string, error) {
	f := cmd.Flags()
	hasRegex := f.Changed("regex")
	hasMarkers := f.Changed("from-marker") || f.Changed("to-marker")
	hasLines := f.Changed("lines")
	hasOld := f.Changed("old") || f.Changed("old-file")

	var chosen []string
	if hasRegex {
		chosen = append(chosen, "--regex")
	}
	if hasMarkers {
		chosen = append(chosen, "--from-marker/--to-marker")
	}
	if hasLines {
		chosen = append(chosen, "--lines")
	}
	if len(chosen) > 1 {
		return "", fmt.Errorf("%s and %s address different things: pass one mode", chosen[0], chosen[1])
	}
	if len(chosen) == 0 {
		if !hasOld {
			return "", fmt.Errorf("nothing to replace: pass --old, --regex, --from-marker/--to-marker or --lines")
		}
		olds, _, err := editValues(cmd, "old")
		if err != nil {
			return "", err
		}
		if len(olds) > 1 {
			return editModeBatch, nil
		}
		return editModeLiteral, nil
	}
	mode := chosen[0]
	if mode == "--from-marker/--to-marker" {
		if !f.Changed("from-marker") || !f.Changed("to-marker") {
			return "", fmt.Errorf("--from-marker and --to-marker must be passed together: they name the block's two boundaries")
		}
		mode = editModeMarkers
	}
	if mode == "--lines" {
		mode = editModeLines
	}
	if mode == "--regex" {
		if !hasOld {
			return "", fmt.Errorf("--regex takes the pattern as --old")
		}
		mode = editModeRegex
	}
	if hasOld && mode != editModeRegex {
		return "", fmt.Errorf("--old and %s address different things: pass one mode", chosen[0])
	}
	if f.Changed("expect") && mode != editModeMarkers && mode != editModeLines {
		return "", fmt.Errorf("--expect pins a block addressed by --from-marker/--to-marker or --lines, not a %s replacement", mode)
	}
	return mode, nil
}

// editApply dispatches to one mode. Every guard runs before anything is
// returned, so a refusal leaves the caller's content untouched.
func editApply(cmd *cobra.Command, mode, content string, crlf, all bool) (editOutcome, error) {
	switch mode {
	case editModeLiteral:
		return editApplyLiteral(cmd, content, crlf, all)
	case editModeBatch:
		return editApplyBatch(cmd, content, crlf, all)
	case editModeRegex:
		return editApplyRegex(cmd, content, crlf, all)
	case editModeMarkers:
		return editApplyMarkers(cmd, content, crlf)
	case editModeLines:
		return editApplyLines(cmd, content, crlf)
	}
	return editOutcome{}, fmt.Errorf("unknown edit mode %q", mode)
}

func editApplyLiteral(cmd *cobra.Command, content string, crlf, all bool) (editOutcome, error) {
	olds, _, err := editValues(cmd, "old")
	if err != nil {
		return editOutcome{}, err
	}
	if len(olds) == 0 {
		return editOutcome{}, fmt.Errorf("--old is empty: an empty literal matches everywhere, so there is no safe replacement to make")
	}
	if len(olds) != 1 {
		return editOutcome{}, fmt.Errorf("one literal per invocation: pass --old once")
	}
	newText, err := editNewValue(cmd, 1)
	if err != nil {
		return editOutcome{}, err
	}
	if olds[0] == "" {
		return editOutcome{}, fmt.Errorf("--old is empty: an empty literal matches everywhere, so there is no safe replacement to make")
	}
	oldText := editNewlines(olds[0], crlf)
	newText = editNewlines(newText, crlf)

	count := strings.Count(content, oldText)
	if count == 0 {
		return editOutcome{}, fmt.Errorf("the literal does not occur")
	}
	if count > 1 && !all {
		return editOutcome{}, fmt.Errorf("the literal occurs %d times: make it unique, or pass --all to replace every occurrence", count)
	}
	replaces := 1
	if all {
		replaces = count
	}
	return editOutcome{
		updated:     strings.Replace(content, oldText, newText, replaces),
		occurrences: count,
		replaces:    replaces,
	}, nil
}

func editApplyBatch(cmd *cobra.Command, content string, crlf, all bool) (editOutcome, error) {
	olds, oldFromFile, err := editValues(cmd, "old")
	if err != nil {
		return editOutcome{}, err
	}
	news, newFromFile, err := editValues(cmd, "new")
	if err != nil {
		return editOutcome{}, err
	}
	if oldFromFile || newFromFile {
		return editOutcome{}, fmt.Errorf("--old-file/--new-file replace one value: a batch passes --old and --new once per pair")
	}
	if len(olds) != len(news) {
		return editOutcome{}, fmt.Errorf("--old and --new must pair up: %d literal(s) against %d replacement(s)", len(olds), len(news))
	}
	updated := content
	totalFound, totalReplaces := 0, 0
	for i := range olds {
		oldText := editNewlines(olds[i], crlf)
		if oldText == "" {
			return editOutcome{}, fmt.Errorf("pair %d: --old is empty", i+1)
		}
		count := strings.Count(updated, oldText)
		if count == 0 {
			return editOutcome{}, fmt.Errorf("pair %d (%q): the literal does not occur", i+1, olds[i])
		}
		if count > 1 && !all {
			return editOutcome{}, fmt.Errorf("pair %d (%q): the literal occurs %d times: make it unique, or pass --all", i+1, olds[i], count)
		}
		replaces := 1
		if all {
			replaces = count
		}
		updated = strings.Replace(updated, oldText, editNewlines(news[i], crlf), replaces)
		totalFound += count
		totalReplaces += replaces
	}
	return editOutcome{updated: updated, occurrences: totalFound, replaces: totalReplaces}, nil
}

func editApplyRegex(cmd *cobra.Command, content string, crlf, all bool) (editOutcome, error) {
	olds, oldFromFile, err := editValues(cmd, "old")
	if err != nil {
		return editOutcome{}, err
	}
	if oldFromFile {
		return editOutcome{}, fmt.Errorf("--regex takes the pattern as --old, not --old-file")
	}
	if len(olds) != 1 || olds[0] == "" {
		return editOutcome{}, fmt.Errorf("--regex takes exactly one non-empty --old pattern")
	}
	newText, err := editNewValue(cmd, 1)
	if err != nil {
		return editOutcome{}, err
	}
	pattern := editNewlines(olds[0], crlf)
	newText = editNewlines(newText, crlf)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return editOutcome{}, fmt.Errorf("the pattern does not compile: %w", err)
	}
	if re.MatchString("") {
		return editOutcome{}, fmt.Errorf("the pattern matches the empty string: it would insert between every byte; make it require a character")
	}
	matches := re.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return editOutcome{}, fmt.Errorf("the pattern does not match")
	}
	if len(matches) > 1 && !all {
		return editOutcome{}, fmt.Errorf("the pattern matches %d times: make it unique, or pass --all to replace every occurrence", len(matches))
	}
	replaces := 1
	if all {
		replaces = len(matches)
	}
	var updated string
	if all {
		updated = re.ReplaceAllString(content, newText)
	} else {
		m := re.FindStringSubmatchIndex(content)
		expanded := re.ExpandString(nil, newText, content, m)
		updated = content[:m[0]] + string(expanded) + content[m[1]:]
	}
	return editOutcome{updated: updated, occurrences: len(matches), replaces: replaces, pattern: pattern}, nil
}

func editApplyMarkers(cmd *cobra.Command, content string, crlf bool) (editOutcome, error) {
	fromText := getStringFlag(cmd, "from-marker", false)
	toText := getStringFlag(cmd, "to-marker", false)
	if fromText == "" || toText == "" {
		return editOutcome{}, fmt.Errorf("--from-marker and --to-marker must both be non-empty")
	}
	fromText = editNewlines(fromText, crlf)
	toText = editNewlines(toText, crlf)
	if strings.Count(content, fromText) != 1 {
		return editOutcome{}, fmt.Errorf("the from-marker occurs %d times: it must occur exactly once", strings.Count(content, fromText))
	}
	if strings.Count(content, toText) != 1 {
		return editOutcome{}, fmt.Errorf("the to-marker occurs %d times: it must occur exactly once", strings.Count(content, toText))
	}
	fromAt := strings.Index(content, fromText)
	toAt := strings.Index(content, toText)
	if toAt < fromAt+len(fromText) {
		return editOutcome{}, fmt.Errorf("the to-marker precedes the from-marker: name the boundaries in file order")
	}
	region := content[fromAt+len(fromText) : toAt]
	if err := editCheckExpect(cmd, region); err != nil {
		return editOutcome{}, err
	}
	newText, err := editNewValue(cmd, 1)
	if err != nil {
		return editOutcome{}, err
	}
	updated := content[:fromAt+len(fromText)] + editNewlines(newText, crlf) + content[toAt:]
	return editOutcome{updated: updated, occurrences: 1, replaces: 1, blockSHA: editBlockHash(region)}, nil
}

func editApplyLines(cmd *cobra.Command, content string, crlf bool) (editOutcome, error) {
	spec := getStringFlag(cmd, "lines", false)
	a, b, err := editParseLineRange(spec)
	if err != nil {
		return editOutcome{}, err
	}
	nl := "\n"
	if crlf {
		nl = "\r\n"
	}
	lines := strings.SplitAfter(content, nl)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if b > len(lines) {
		return editOutcome{}, fmt.Errorf("--lines %d:%d is past the file's %d line(s)", a, b, len(lines))
	}
	start, end := 0, 0
	for i := 0; i < a-1; i++ {
		start += len(lines[i])
	}
	for i := 0; i < b; i++ {
		end += len(lines[i])
	}
	region := content[start:end]
	if err := editCheckExpect(cmd, region); err != nil {
		return editOutcome{}, err
	}
	newText, err := editNewValue(cmd, 1)
	if err != nil {
		return editOutcome{}, err
	}
	repl := editNewlines(newText, crlf)
	if repl != "" && !strings.HasSuffix(repl, "\n") {
		repl += nl
	}
	updated := content[:start] + repl + content[end:]
	return editOutcome{updated: updated, occurrences: 1, replaces: 1, blockSHA: editBlockHash(region)}, nil
}

// editParseLineRange reads "a:b", 1-based and inclusive.
func editParseLineRange(spec string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(spec), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("--lines wants a:b (1-based, inclusive), got %q", spec)
	}
	a, errA := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, errB := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errA != nil || errB != nil || a < 1 || b < a {
		return 0, 0, fmt.Errorf("--lines wants a:b with 1 <= a <= b, got %q", spec)
	}
	return a, b, nil
}

// editCheckExpect compares the pinned hash with the block's own, before any
// substitution. The prefix must be at least 8 hex characters.
func editCheckExpect(cmd *cobra.Command, region string) error {
	if !cmd.Flags().Changed("expect") {
		return nil
	}
	return editCheckHash(getStringFlag(cmd, "expect", false), region)
}

// editCheckHash compares a hex-prefix pin with a region's sha256. It is shared
// by the --expect flag and the manifest's per-file expect.
func editCheckHash(want, region string) error {
	want = strings.ToLower(strings.TrimSpace(want))
	if len(want) < 8 {
		return fmt.Errorf("--expect needs at least 8 hex characters of the block's sha256")
	}
	got := editBlockHash(region)
	if !strings.HasPrefix(got, want) {
		return fmt.Errorf("block hash mismatch: expected %s…, the block is %s", want, got)
	}
	return nil
}

func editBlockHash(region string) string {
	sum := sha256.Sum256([]byte(region))
	return hex.EncodeToString(sum[:])
}

// editValues reads one side of a replacement: the flag (repeatable) or its
// --*-file twin, never both.
func editValues(cmd *cobra.Command, name string) ([]string, bool, error) {
	f := cmd.Flags()
	hasFlag := f.Changed(name)
	hasFile := f.Changed(name + "-file")
	if hasFlag && hasFile {
		return nil, false, fmt.Errorf("--%s and --%s both name the %s: pass one", name, name+"-file", name)
	}
	if hasFile {
		data, err := os.ReadFile(getStringFlag(cmd, name+"-file", false)) //#nosec G304 -- the caller named the file
		if err != nil {
			return nil, false, err
		}
		return []string{string(data)}, true, nil
	}
	vals, err := f.GetStringArray(name)
	if err != nil {
		return nil, false, err
	}
	return vals, false, nil
}

// editNewValue reads the replacement for a single-pair mode: --new (even empty)
// or --new-file, and requires one of them.
func editNewValue(cmd *cobra.Command, want int) (string, error) {
	vals, fromFile, err := editValues(cmd, "new")
	if err != nil {
		return "", err
	}
	if !fromFile && len(vals) == 0 {
		// pflag drops an empty string from a string array, so `--new ""` reads
		// as no value at all; Changed is what says the deletion was asked for.
		if cmd.Flags().Changed("new") {
			return "", nil
		}
		return "", fmt.Errorf("no replacement: pass --new (empty to delete) or --new-file")
	}
	if len(vals) != want {
		return "", fmt.Errorf("this mode takes one replacement, got %d", len(vals))
	}
	if fromFile && len(vals) != 1 {
		return "", fmt.Errorf("--new-file names one replacement")
	}
	return vals[0], nil
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
	f.StringArray("old", nil, "Literal text to replace (repeat for a batch; must occur exactly once)")
	f.StringArray("new", nil, "Replacement text (empty deletes; repeat for a batch)")
	f.String("old-file", "", "Read the literal to replace from this file")
	f.String("new-file", "", "Read the replacement text from this file")
	f.Bool("regex", false, "Treat --old as an RE2 pattern and expand $1 in --new")
	f.String("from-marker", "", "Literal that opens the block to replace (with --to-marker)")
	f.String("to-marker", "", "Literal that closes the block to replace (with --from-marker)")
	f.String("lines", "", "Replace the line range a:b (1-based, inclusive)")
	f.String("expect", "", "Refuse unless the block's sha256 starts with this hex prefix (at least 8)")
	f.String("batch", "", "Edit N files from a JSON/YAML manifest of file entries with literal pairs (path, or \"-\"/empty reads stdin)")
	f.Bool("all", false, "Replace every occurrence instead of requiring exactly one")
	f.Bool("dry-run", false, "Print the diff without writing")
	rootCmd.AddCommand(editCmd)
}
