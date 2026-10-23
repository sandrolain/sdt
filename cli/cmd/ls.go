//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/search"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List a directory, one or more levels deep",
	Long: `List the entries of a directory.

WHEN — you need to know what is in a directory, and how large the entries are:
the first step of nearly every session on an unfamiliar tree.

WHEN NOT — you know the shape of the path you want: ` + "`sdt find <glob>`" + ` walks by
pattern and prints paths only. You want one file's contents: ` + "`sdt read -p`" + `.

RETURNS — one line per entry, directories first then by path, as ` + "`d`" + ` or ` + "`f`" + `
then the path. ` + "`--stat`" + ` adds a size and a modification time; ` + "`--depth`" + ` descends
that many levels (1 by default, the directory itself); ` + "`--limit`" + ` caps the rows
and marks the answer truncated.

The walk prunes what ` + "`find`" + ` prunes: the project's declared ` + "`skipDirs`" + ` and the
shipped defaults, plus ` + "`context/refs`" + ` and ` + "`context/tmp`" + ` — a listing that walked
9,2 GB of captured clones would not be a listing.

EXAMPLE — ` + "`sdt ls --stat --depth 2`" + `
          ` + "`sdt ls --dir cli/cmd --limit 40`" + ``,
	Example: `  sdt ls
  sdt ls --stat
  sdt ls --dir context/analysis --limit 20`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		opts := search.ListOptions{
			Root:        getStringFlag(cmd, "dir", false),
			Depth:       getIntFlag(cmd, "depth", false),
			ProjectRoot: sdtProjectRoot(),
			SkipDirs:    declaredSkipDirs(),
			Max:         getIntFlag(cmd, "limit", false),
			Stat:        getBoolFlag(cmd, "stat", false),
		}
		res, err := search.List(opts)
		exitWithError(cmd, err)
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
			lines := make([]string, 0, len(res.Entries))
			for _, e := range res.Entries {
				kind := "f"
				if e.Dir {
					kind = "d"
				}
				if opts.Stat {
					lines = append(lines, fmt.Sprintf("%s %8s  %s  %s",
						kind, humanSize(e.Size), e.Modified.Local().Format("2006-01-02 15:04"), e.Path))
					continue
				}
				lines = append(lines, kind+"  "+e.Path)
			}
			if len(lines) > 0 {
				outputString(cmd, strings.Join(lines, "\n")+"\n")
			}
		}
	},
}

// humanSize renders a byte count for a listing column: exact bytes below a
// kibibyte, then one decimal of the largest unit that fits. It is presentation
// only — --format json carries the exact number.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(n)/float64(div), "KMGTPE"[exp])
}

func init() {
	f := lsCmd.Flags()
	f.String("dir", "", "Directory to list (default: the working directory)")
	f.Int("depth", 1, "How many levels below --dir to list")
	f.Bool("stat", false, "Add size and modification time")
	f.Int("limit", 0, "Maximum entries reported (0: unlimited)")
	rootCmd.AddCommand(lsCmd)
}
