//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxCleanTarget is one emptied directory and what it held. Files, not
// directories: a directory name on its own is not a removed artifact, and a count
// of them would overstate what the command did.
type ctxCleanTarget struct {
	Dir   string   `json:"dir" yaml:"dir"`
	Files []string `json:"files" yaml:"files"`
	Count int      `json:"count" yaml:"count"`
}

// ctxCleanResult is the reported outcome. Every requested target appears, empty
// included, so "nothing was there" is distinguishable from "the target was not
// reached".
type ctxCleanResult struct {
	Targets []ctxCleanTarget `json:"targets" yaml:"targets"`
	DryRun  bool             `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`
}

// ctxCleanTargets maps the flags to the directories to empty, in a stable order.
// It reads the flags only: whether a target exists is a question for the plan
// below, and answering it here would let an absent directory look like a refused
// invocation.
func ctxCleanTargets(cmd *cobra.Command) []string {
	var dirs []string
	if getBoolFlag(cmd, "trash", false) {
		dirs = append(dirs, sdtTrashDir)
	}
	if getBoolFlag(cmd, "tmp", false) {
		dirs = append(dirs, sdtTmpDir)
	}
	return dirs
}

// ctxCleanPlan lists the files a target holds, sorted, relative to the corpus
// root. A missing directory is an empty plan rather than an error: the caller
// asked for it to be empty, and it already is.
func ctxCleanPlan(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // an unreadable entry still leaves the rest to remove
			return nil
		}
		if !d.IsDir() {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// ctxEmptyDir removes everything inside dir and keeps dir itself, so the next
// write does not have to recreate it and a caller who empties a directory that
// was never created does not get one.
func ctxEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func outputCleanResult(cmd *cobra.Command, res ctxCleanResult) {
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
		var b strings.Builder
		for _, tg := range res.Targets {
			if res.DryRun {
				fmt.Fprintf(&b, "%s: would remove %d file(s)\n", tg.Dir, tg.Count)
			} else {
				fmt.Fprintf(&b, "%s: removed %d file(s)\n", tg.Dir, tg.Count)
			}
			for _, f := range tg.Files {
				fmt.Fprintf(&b, "  %s\n", f)
			}
		}
		outputString(cmd, b.String())
	}
}

var contextCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Empty the discard areas: context/trash and context/tmp",
	Long: `Empty a discard area. ` + "`context rm`" + ` moves a document into ` + "`context/trash`" + ` and
` + "`context/tmp`" + ` collects scratch; nothing else removes either, so they grow until
this command is run.

WHEN — the discard areas have served their purpose: the work you might have
wanted back is behind you, and the scratch from a finished task is noise. Run it
alone or per directory as the argument to ` + "`sdt agent init --force`" + `, whose obsolete
files land in the same trash.

WHEN NOT — something might still be needed: ` + "`--dry-run`" + ` lists exactly what would
go and removes nothing, and a single file can be recovered from
` + "`context/trash`" + ` until then. There is no rotation, no age threshold and no
recycle bin behind the recycle bin.

RETURNS — per target, the files removed (or that would be) relative to the corpus
root, and their count. A directory that does not exist, or is already empty, is
reported as nothing removed rather than as an error, so a repeated run exits 0.

**A target is required.** An invocation with neither ` + "`--trash`" + ` nor ` + "`--tmp`" + ` is
refused before anything is read, and naming the target **is** the confirmation:
there is no second flag to pass, because choosing what to destroy is the whole
decision. Pass both to empty both in one run. Destruction never happens by
omission.

EXAMPLE — ` + "`sdt context clean --trash --dry-run`" + `
          ` + "`sdt context clean --trash --tmp`" + ``,
	Example: `  sdt context clean --trash --dry-run
  sdt context clean --trash
  sdt context clean --trash --tmp --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		dirs := ctxCleanTargets(cmd)
		if len(dirs) == 0 {
			exitWithError(cmd, fmt.Errorf("a target is required: pass --trash, --tmp, or both — this command never empties a directory it was not asked to"))
			return
		}
		dryRun := getBoolFlag(cmd, "dry-run", false)
		res := ctxCleanResult{Targets: make([]ctxCleanTarget, 0, len(dirs)), DryRun: dryRun}
		for _, dir := range dirs {
			files, err := ctxCleanPlan(dir)
			exitWithError(cmd, err)
			if files == nil {
				files = []string{}
			}
			res.Targets = append(res.Targets, ctxCleanTarget{Dir: dir, Files: files, Count: len(files)})
			if dryRun {
				continue
			}
			if err := ctxEmptyDir(dir); err != nil {
				exitWithError(cmd, err)
				return
			}
		}
		outputCleanResult(cmd, res)
	},
}

func init() {
	f := contextCleanCmd.Flags()
	f.Bool("trash", false, "Empty context/trash")
	f.Bool("tmp", false, "Empty context/tmp")
	f.Bool("dry-run", false, "List what would be removed without removing it")
}
