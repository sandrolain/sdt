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
		Long: `Search file content with a regular expression, honouring .gitignore.

WHEN — you need the lines of the working tree that match a pattern and have no
host search tool.

WHEN NOT — you need a document from the corpus by what it says: ` + "`sdt context search`" + ` ranks
documents by meaning and filters by kind, status and date. Use this verb for
source, config and reference files, which that corpus does not index.

RETURNS — one entry per matching line (path, 1-based line number, text) plus the
counts of files searched and matched, and ` + "`truncated`" + ` when a cap stopped the scan
early. Output is bounded by default: 200 matches overall, unlimited per file
unless ` + "`--max-count`" + ` is set, VCS and dependency directories skipped, binary
files not read.

EXAMPLE — ` + "`sdt grep \"replaceAll\" --include \"**/*.ts\" --limit 40`" + `

Both caps are presentation, never correctness: ` + "`--limit 0`" + ` reports every match
and --no-ignore reaches the ignored tree.`,
		Example: `  sdt grep "errStopScan" --format json
  sdt grep "TODO" --include "**/*.go" --exclude "**/*_test.go"
  sdt grep "unique-term-x9y8z7" --no-ignore --limit 0`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts := search.GrepOptions{
				Root:            getStringFlag(cmd, "dir", false),
				Pattern:         args[0],
				Include:         getStringFlag(cmd, "include", false),
				Exclude:         getStringFlag(cmd, "exclude", false),
				NoIgnore:        getBoolFlag(cmd, "no-ignore", false),
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
		Long: `List the working-tree paths matching a glob, honouring .gitignore.

WHEN — you need the paths of files you will read, and have no host file-listing
tool.

WHEN NOT — you need a document from the corpus by what it is: ` + "`sdt context list --type <k>`" + `
lists corpus work files with their status and date. Use this verb for source,
config and reference files, which that corpus does not index.

RETURNS — the matching paths, sorted, relative to the search root, each marked
when it is a directory. Omitting the glob lists every tracked file. A pattern
without a slash matches a basename at any depth (` + "`*.md`" + ` finds every markdown
file); a pattern with a slash is matched against the whole path.

EXAMPLE — ` + "`sdt find \"**/*_test.go\" --format json`" + `

The default view is the tracked one: .gitignore applies, nested files included,
build and dependency trees absent. --no-ignore reaches the raw tree.`,
		Example: `  sdt find "*.tmpl"
  sdt find "internal/**/*.go" --format json
  sdt find --dir web/src --limit 0`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts := search.FindOptions{
				Root:        getStringFlag(cmd, "dir", false),
				NoIgnore:    getBoolFlag(cmd, "no-ignore", false),
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

func init() {
	f := grepCmd.Flags()
	f.String("include", "", "Only search paths matching this glob")
	f.String("exclude", "", "Skip paths matching this glob")
	f.String("dir", "", "Directory to search (default: the working directory)")
	f.Bool("no-ignore", false, "Search the ignored tree as well")
	f.Int("max-count", 0, "Maximum matches reported per file (0: unlimited)")
	f.Int("limit", 0, "Maximum matches reported overall (0: the default cap)")
	f.BoolP("ignore-case", "i", false, "Case-insensitive match")

	f = findCmd.Flags()
	f.String("dir", "", "Directory to search (default: the working directory)")
	f.Bool("no-ignore", false, "List the ignored tree as well")
	f.Bool("dirs", false, "Include directories in the result")
	f.Int("limit", 0, "Maximum paths reported (0: unlimited)")

	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(findCmd)
}
