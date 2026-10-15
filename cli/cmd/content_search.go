//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/search"
)

var (
	grepCmd = &cobra.Command{
		Use:   "grep <pattern>",
		Short: "Grep File Content",
		Long: `Search file content with a regular expression over the working tree.

WHEN — you need the lines of the tree that match a pattern and have no host
search tool. The context/ corpus is searched like any other directory, so you can
ask what the project's own records say about a subject.

WHEN NOT — you need a document from the corpus by what it means rather than by
the exact string it contains: ` + "`sdt context search`" + ` ranks documents and filters by
kind, status and date. You want the captured third-party clones under
context/refs (pass ` + "`--refs`" + `); they are pruned by default because they are 9,2 GB
of third-party code. Or you want to prune a large subtree: ` + "`--exclude`" + ` filters
the **results**, so the walk still descends into it — use ` + "`--dir`" + ` to point at
the part you want instead.

RETURNS — one entry per matching line (path, 1-based line number, text) plus the
counts of files searched and matched, and ` + "`truncated`" + ` when a cap stopped the scan
early. Output is bounded by default: 200 matches overall, unlimited per file
unless ` + "`--max-count`" + ` is set.

` + "`-A n`" + `, ` + "`-B n`" + ` and ` + "`-C n`" + ` (GNU-compatible) add the n lines around each match,
so a match is read in its surrounding without a second command. Context lines are
**not** matches: they never satisfy the pattern, never count toward either cap,
and never extend the scan — a match past the cap brings no context with it. In
text they render as ` + "`path-line-text`" + ` against a match's ` + "`path:line:text`" + `, with ` + "`--`" + `
between non-adjacent blocks; a line shared by two matches is printed once and
keeps its match marker. Both sides of an overlap are merged, not concatenated.
` + "`-C n`" + ` is exactly ` + "`-A n -B n`" + `; an explicit side wins over ` + "`-C`" + ` for that side.

` + "`-l`" + ` (` + "`--files-with-matches`" + `) answers "which files" instead of "where": one path
per file carrying a match, sorted, in ` + "`paths`" + ` with ` + "`matches`" + ` left empty. It reads
each file only to its first match and ` + "`--limit`" + ` bounds the list of **paths**. The
flags that describe how to render a match — ` + "`--max-count`" + `, ` + "`-A`" + `, ` + "`-B`" + `, ` + "`-C`" + ` — are
**refused** in this mode rather than ignored, because a caller who sets one believes
it took effect.

` + "`-c`" + ` (` + "`--count`" + `) reports how many lines match instead of which ones — the
` + "`grep -c`" + ` fallback. The count is a fact rather than a rendering, so
` + "`--limit`" + ` is refused with it; ` + "`--max-count`" + ` is honoured and marked ` + "`truncated`" + ` when
it clipped, which is what ` + "`grep -m N -c`" + ` counts. Counting every line of a file is
` + "`sdt read --count`" + `, not a grep with an empty pattern.

The scope, stated plainly: no ignore file is consulted, so a path listed in
.gitignore is searched like any other. The directories pruned are the ones your
project declares as ` + "`skipDirs`" + ` in .sdt.yaml when it declares any, and the shipped
defaults (.git, .hg, .svn, node_modules, .venv) when it does not. A declaration
replaces the tool residues, not the VCS ones: .git, .hg and .svn are pruned
whatever you declare. context/refs and context/tmp are pruned unless ` + "`--refs`" + ` is
given. Binary files are not read.

EXAMPLE — ` + "`sdt grep \"replaceAll\" --include \"**/*.ts\" --limit 40`" + `

Both caps are presentation, never correctness: ` + "`--limit 0`" + ` reports every match.
A path pattern is matched against the whole path, so ` + "`context/analysis/*.md`" + ` does
not reach ` + "`cli/.../context/analysis/*.md`" + `; ` + "`**/`" + ` is how you ask for any depth.`,
		Example: `  sdt grep "errStopScan" --format json
  sdt grep "TODO" --include "**/*.go" --exclude "**/*_test.go"
  sdt grep "cascade rule" --include "context/**/*.md"
  sdt grep "derivedCompletionBlock" --include "**/*.go" -A 12
  sdt grep "DOMPurify" --dir web/src -l --limit 0
  sdt grep "derivedCompletionBlock" --include "**/*.go" -c
  sdt grep "benchmark" --refs --limit 0`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			before, after, err := grepContextFlags(cmd)
			exitWithError(cmd, err)
			filesWithMatches := getBoolFlag(cmd, "files-with-matches", false)
			countOnly := getBoolFlag(cmd, "count", false)
			exitWithError(cmd, grepIncompatibleFlags(cmd, filesWithMatches, countOnly))
			opts := search.GrepOptions{
				Root:             getStringFlag(cmd, "dir", false),
				Pattern:          args[0],
				Include:          getStringFlag(cmd, "include", false),
				Exclude:          getStringFlag(cmd, "exclude", false),
				ProjectRoot:      sdtProjectRoot(),
				SkipDirs:         declaredSkipDirs(),
				SkipPaths:        searchSkipPaths(cmd),
				MaxCountPerFile:  getIntFlag(cmd, "max-count", false),
				Max:              getIntFlag(cmd, "limit", false),
				CaseInsensitive:  getBoolFlag(cmd, "ignore-case", false),
				Before:           before,
				After:            after,
				FilesWithMatches: filesWithMatches,
				CountOnly:        countOnly,
			}
			res, err := search.Grep(opts)
			exitWithError(cmd, err)
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
				switch {
				case countOnly:
					outputLineCount(cmd, *res.Count)
				case filesWithMatches:
					outputGrepLines(cmd, res.Paths)
				default:
					outputGrepLines(cmd, renderGrepMatches(res.Matches, before > 0 || after > 0))
				}
			}
		},
	}

	findCmd = &cobra.Command{
		Use:   "find [glob]",
		Short: "Find Files By Path",
		Long: `List the working-tree paths matching a glob.

WHEN — you need the paths of files you will read, and have no host file-listing
tool. The context/ corpus is listed like any other directory.

WHEN NOT — you need the corpus work files with their status and date:
` + "`sdt context list --type <k>`" + `. You want the captured third-party clones under
context/refs (pass ` + "`--refs`" + `), which are pruned by default for size. Or you are
about to hand this a glob to prune a large subtree — that does not work here,
because the descent happens before the glob is applied; use ` + "`--dir`" + ` to point at
the part you want.

RETURNS — the matching paths, sorted, relative to the search root, each marked
when it is a directory. Omitting the glob lists every file in scope. A pattern
without a slash matches a basename at any depth (` + "`*.md`" + ` finds every markdown
file); a pattern with a slash is matched against the **whole** path, so
` + "`context/analysis/*.md`" + ` reaches only that directory — prefix it with ` + "`**/`" + `
to ask for any depth.

A printed path round-trips: feed it back to reach exactly that file.

The scope, stated plainly: no ignore file is consulted, so a path listed in
.gitignore is listed like any other. The directories pruned are the ones your
project declares as ` + "`skipDirs`" + ` in .sdt.yaml when it declares any, and the shipped
defaults (.git, .hg, .svn, node_modules, .venv) when it does not. A declaration
replaces the tool residues, not the VCS ones: .git, .hg and .svn are pruned
whatever you declare. Patterns are relative to the directory holding .sdt.yaml
and keep that meaning under ` + "`--dir`" + `.
context/refs and context/tmp are pruned unless ` + "`--refs`" + ` is given.

EXAMPLE — ` + "`sdt find \"**/*_test.go\" --format json`" + `
          ` + "`sdt find \"context/decisions/*.md\" --limit 5`" + `
          ` + "`sdt find \"context/refs/**/README.md\" --refs --limit 3`" + `
          ` + "`sdt find context/architecture/stack.md`" + ``,
		Example: `  sdt find "*.tmpl"
  sdt find "internal/**/*.go" --format json
  sdt find --dir web/src --limit 0`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts := search.FindOptions{
				Root:        getStringFlag(cmd, "dir", false),
				ProjectRoot: sdtProjectRoot(),
				SkipDirs:    declaredSkipDirs(),
				SkipPaths:   searchSkipPaths(cmd),
				IncludeDirs: getBoolFlag(cmd, "dirs", false),
				Max:         getIntFlag(cmd, "limit", false),
			}
			if len(args) == 1 {
				opts.Pattern = args[0]
			}
			res, err := search.Find(opts)
			exitWithError(cmd, err)
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
				lines := make([]string, 0, len(res))
				for _, r := range res {
					if r.IsDir {
						lines = append(lines, r.Path+string(filepath.Separator))
						continue
					}
					lines = append(lines, r.Path)
				}
				outputString(cmd, strings.Join(lines, "\n"))
			}
		},
	}
)

// searchSkipPaths resolves the corpus scope for one content verb: the default
// prunes context/refs and context/tmp, and --refs reaches the captured clones
// while the scratch directory stays pruned. The distinction matters because
// --exclude cannot do it at all: that filters the results, leaving the descent
// to walk 9,2 GB anyway.
// projectRoot is the directory the declared patterns are relative to, or "" when
// nothing is declared. findProjectConfigAt walks up like .git and degrades to nil
// on a missing or malformed file, so this is silent in every failing case.
func sdtProjectRoot() string {
	_, root, err := findProjectConfigAt()
	if err != nil {
		return ""
	}
	return root
}

// declaredSkipDirs reads the project's `skipDirs` declaration, or nothing. An
// absent key, a value that is not a list of strings and a parse failure all yield
// nil, which is what leaves the shipped defaults in charge.
func declaredSkipDirs() []string {
	cfg, _, err := findProjectConfigAt()
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.SkipDirs
}

// The grep context flags, as constants because the same three names are read
// back in three places: the resolution rule, the refusal rule and the
// registration. A typo in one of them would silently drop the flag.
const (
	flagGrepAfter   = "after"
	flagGrepBefore  = "before"
	flagGrepContext = "context"
)

// grepContextFlags resolves the -A/-B/-C shorthand group into the two counts the
// scanner takes. `-C n` is exactly `-A n -B n`; when an explicit side is also
// given it wins for that side, which is GNU's rule and leaves the caller nothing
// to guess at. A negative count is rejected rather than clamped: a caller asking
// for -3 lines has a bug, and silently reading it as 0 hides it.
func grepContextFlags(cmd *cobra.Command) (before, after int, err error) {
	specs := []struct {
		name, shorthand string
		value           int
	}{
		{flagGrepBefore, "-B", getIntFlag(cmd, flagGrepBefore, false)},
		{flagGrepAfter, "-A", getIntFlag(cmd, flagGrepAfter, false)},
		{flagGrepContext, "-C", getIntFlag(cmd, flagGrepContext, false)},
	}
	for _, s := range specs {
		if s.value < 0 {
			return 0, 0, fmt.Errorf("a context count cannot be negative: %s/--%s is %d", s.shorthand, s.name, s.value)
		}
	}
	ctx := specs[2].value
	if ctx == 0 {
		return specs[0].value, specs[1].value, nil
	}
	before, after = ctx, ctx
	if cmd.Flags().Changed(flagGrepBefore) {
		before = specs[0].value
	}
	if cmd.Flags().Changed(flagGrepAfter) {
		after = specs[1].value
	}
	return before, after, nil
}

// grepIncompatibleFlags refuses the flag combinations whose effect cannot be
// honoured. Flags describe how to render a *match* — its neighbours, how many
// per file, where it is — and each mode reports something else, so honouring
// them is impossible and ignoring them would leave the caller believing a flag
// took effect. The refusal names the flag and the reason, which is the only
// useful outcome.
//
// `--max-count` is deliberately not refused with `--count`: it stops the scan of
// a file, so `grep -m N -c` counts the same way in GNU grep, and the answer is
// marked `truncated` when it clipped.
func grepIncompatibleFlags(cmd *cobra.Command, filesWithMatches, countOnly bool) error {
	if filesWithMatches && countOnly {
		return fmt.Errorf("--files-with-matches and --count answer different questions (which files, how many lines): ask one of them")
	}
	switch {
	case countOnly:
		if getIntFlag(cmd, "limit", false) > 0 {
			return fmt.Errorf("--limit has no meaning with --count: a count bounded by a display cap would be a lie rather than a sample")
		}
		for _, name := range []string{flagGrepAfter, flagGrepBefore, flagGrepContext} {
			if getIntFlag(cmd, name, false) > 0 {
				return fmt.Errorf("--%s has no meaning with --count, which reports a number rather than lines: drop it", name)
			}
		}
	case filesWithMatches:
		if getIntFlag(cmd, "max-count", false) > 0 {
			return fmt.Errorf("--max-count has no meaning with --files-with-matches, which reports one entry per file rather than per match: drop it")
		}
		for _, name := range []string{flagGrepAfter, flagGrepBefore, flagGrepContext} {
			if getIntFlag(cmd, name, false) > 0 {
				return fmt.Errorf("--%s has no meaning with --files-with-matches, which reports paths rather than lines: drop it", name)
			}
		}
	}
	return nil
}

// grepRenderLine is one line of the text rendering: its number, its text and
// whether it is a match (the marker) or context.
type grepRenderLine struct {
	number  int
	text    string
	isMatch bool
}

// outputGrepLines prints one entry per line, terminated. The trailing newline is
// not cosmetic: without it the output is not line-oriented, so a pipe into
// `wc -l` under-reports by one and a reader expecting a final newline sees a
// truncated line.
func outputGrepLines(cmd *cobra.Command, lines []string) {
	if len(lines) == 0 {
		return
	}
	outputString(cmd, strings.Join(lines, "\n")+"\n")
}

// renderGrepMatches renders the matches as contiguous blocks of lines, the shape
// a reader recognises from grep: `path:line:text` for a match,
// `path-line-text` for a context line, and `--` between non-adjacent blocks.
//
// The blocks are merged before they are rendered rather than concatenated: the
// lines of every match in one file are collected into one set keyed by line
// number, a match marker overriding a context entry for the same line, and the
// set is then printed in line order. Concatenating the blocks instead is what
// prints an overlapping line twice — once as context for one match and once as
// the next match's own line — which is the defect this shape exists to avoid.
func renderGrepMatches(matches []search.GrepMatch, withContext bool) []string {
	var out []string
	for i := 0; i < len(matches); {
		path := matches[i].Path
		byLine, next := grepLinesForPath(matches, i)
		i = next
		if len(out) > 0 && withContext {
			out = append(out, "--")
		}
		out = append(out, renderGrepPathLines(path, byLine, withContext)...)
	}
	return out
}

// grepLinesForPath collects the lines of every match in one file into a set keyed
// by line number and returns the index of the first match of the next path. A
// match marker overrides a context entry for the same line, while a context entry
// never overrides one, which is what keeps a line shared by two matches marked
// as a match.
func grepLinesForPath(matches []search.GrepMatch, start int) (map[int]grepRenderLine, int) {
	path := matches[start].Path
	byLine := map[int]grepRenderLine{}
	i := start
	for i < len(matches) && matches[i].Path == path {
		m := matches[i]
		for _, c := range m.Before {
			if _, seen := byLine[c.Line]; !seen {
				byLine[c.Line] = grepRenderLine{number: c.Line, text: c.Text}
			}
		}
		byLine[m.Line] = grepRenderLine{number: m.Line, text: m.Text, isMatch: true}
		for _, c := range m.After {
			if _, seen := byLine[c.Line]; !seen {
				byLine[c.Line] = grepRenderLine{number: c.Line, text: c.Text}
			}
		}
		i++
	}
	return byLine, i
}

// renderGrepPathLines prints one file's merged lines in order, marking the gaps
// between non-adjacent blocks.
func renderGrepPathLines(path string, byLine map[int]grepRenderLine, withContext bool) []string {
	numbers := make([]int, 0, len(byLine))
	for n := range byLine {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	out := make([]string, 0, len(numbers))
	prev := 0
	for k, n := range numbers {
		if k > 0 && withContext && n > prev+1 {
			out = append(out, "--")
		}
		out = append(out, formatGrepLine(path, byLine[n]))
		prev = n
	}
	return out
}

// formatGrepLine renders one line: grep's own convention, `path:line:text` for a
// match and `path-line-text` for context.
func formatGrepLine(path string, l grepRenderLine) string {
	sep := "-"
	if l.isMatch {
		sep = ":"
	}
	return fmt.Sprintf("%s%s%d%s%s", path, sep, l.number, sep, l.text)
}

func searchSkipPaths(cmd *cobra.Command) []string {
	if getBoolFlag(cmd, "refs", false) {
		return search.DefaultSkipPathsNoRefs
	}
	return nil
}

func init() {
	f := grepCmd.Flags()
	f.String("include", "", "Only search paths matching this glob")
	f.String("exclude", "", "Skip paths matching this glob")
	f.String("dir", "", "Directory to search (default: the working directory)")
	f.Bool("no-ignore", false, "No longer needed: the raw tree is already the default (accepted for compatibility)")
	f.Bool("refs", false, "Also search the captured clones under context/refs (pruned by default)")
	f.Int("max-count", 0, "Maximum matches reported per file (0: unlimited)")
	f.Int("limit", 0, "Maximum matches reported overall (0: the default cap)")
	f.BoolP("ignore-case", "i", false, "Case-insensitive match")
	f.IntP(flagGrepAfter, "A", 0, "Print the N lines after each match")
	f.IntP(flagGrepBefore, "B", 0, "Print the N lines before each match")
	f.IntP(flagGrepContext, "C", 0, "Print the N lines before and after each match")
	f.BoolP("files-with-matches", "l", false, "Print only the paths of files carrying a match")
	f.BoolP("count", "c", false, "Print the number of matching lines instead of them")

	f = findCmd.Flags()
	f.String("dir", "", "Directory to search (default: the working directory)")
	f.Bool("no-ignore", false, "No longer needed: the raw tree is already the default (accepted for compatibility)")
	f.Bool("refs", false, "Also list the captured clones under context/refs (pruned by default)")
	f.Bool("dirs", false, "Include directories in the result")
	f.Int("limit", 0, "Maximum paths reported (0: unlimited)")

	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(findCmd)
}
