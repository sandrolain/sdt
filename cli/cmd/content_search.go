//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
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
  sdt grep "benchmark" --refs --limit 0`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts := search.GrepOptions{
				Root:            getStringFlag(cmd, "dir", false),
				Pattern:         args[0],
				Include:         getStringFlag(cmd, "include", false),
				Exclude:         getStringFlag(cmd, "exclude", false),
				ProjectRoot:     sdtProjectRoot(),
				SkipDirs:        declaredSkipDirs(),
				SkipPaths:       searchSkipPaths(cmd),
				MaxCountPerFile: getIntFlag(cmd, "max-count", false),
				Max:             getIntFlag(cmd, "limit", false),
				CaseInsensitive: getBoolFlag(cmd, "ignore-case", false),
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
				lines := make([]string, 0, len(res.Matches))
				for _, m := range res.Matches {
					lines = append(lines, fmt.Sprintf("%s:%d:%s", m.Path, m.Line, m.Text))
				}
				outputString(cmd, strings.Join(lines, "\n"))
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

	f = findCmd.Flags()
	f.String("dir", "", "Directory to search (default: the working directory)")
	f.Bool("no-ignore", false, "No longer needed: the raw tree is already the default (accepted for compatibility)")
	f.Bool("refs", false, "Also list the captured clones under context/refs (pruned by default)")
	f.Bool("dirs", false, "Include directories in the result")
	f.Int("limit", 0, "Maximum paths reported (0: unlimited)")

	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(findCmd)
}
