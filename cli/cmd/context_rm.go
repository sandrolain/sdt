//go:build !wasm

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// ctxRmResult is the reported outcome of one discard. Referrers is always
// emitted, empty included, so "nothing named it" stays distinguishable from "the
// report did not run" — the same reason the search verbs carry their counters.
type ctxRmResult struct {
	Path      string   `json:"path" yaml:"path"`
	Kind      string   `json:"kind" yaml:"kind"`
	Moved     string   `json:"moved" yaml:"moved"`
	Referrers []string `json:"referrers" yaml:"referrers"`
	DryRun    bool     `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`
}

// ctxRefTokenRegexp matches any of refs as a whole token, so a reference is not
// reported for a path that merely begins the same way (`...-old.md.bak` is not a
// reference to `...-old.md`).
func ctxRefTokenRegexp(refs []string) *regexp.Regexp {
	quoted := make([]string, 0, len(refs))
	for _, r := range refs {
		if r = strings.TrimSpace(r); r != "" {
			quoted = append(quoted, regexp.QuoteMeta(r))
		}
	}
	return regexp.MustCompile(`(^|[^A-Za-z0-9_./-])(` + strings.Join(quoted, "|") + `)($|[^A-Za-z0-9_./-])`)
}

// ctxReferrers lists the corpus documents that name path, tried in both spellings
// the corpus uses: the stored corpus-relative form (`notes/x.md`) and the on-disk
// one (`context/notes/x.md`).
//
// The match is deliberately shape-agnostic and is NOT the rewriter's. A rewriter
// only needs to see what it can edit — frontmatter block-list items and
// double-bracket wiki links — so it does not see the inline list
// `sdt context set links "['a.md']"` writes, which is the spelling the CLI itself
// produces. A report built on the rewriter's discovery would be silent about
// exactly the most common kind of reference.
func ctxReferrers(path string) ([]string, error) {
	pat := ctxRefTokenRegexp([]string{
		strings.TrimPrefix(filepath.ToSlash(path), "./"),
		ctxCorpusRef(path),
	})
	skip := map[string]bool{path: true}
	hits, err := ctxScanCorpusRefs(skip, func(_, content string) bool {
		return pat.MatchString(content)
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(hits)
	return hits, nil
}

// ctxTrashDestination returns where a discarded document goes: the same relative
// path under the trash, so which kind it was stays visible, with a timestamp
// inserted on a collision rather than an overwrite. Losing a file to a name clash
// inside a discard area would be the silent data loss the soft delete exists to
// avoid.
func ctxTrashDestination(path string) (string, error) {
	rel := filepath.ToSlash(path)
	if i := strings.LastIndex(rel, "/"+sdtWorkDir+"/"); i >= 0 {
		rel = rel[i+len(sdtWorkDir)+2:]
	} else {
		rel = strings.TrimPrefix(rel, sdtWorkDir+"/")
	}
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == path || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%s is not inside %s", path, sdtWorkDir)
	}
	dst := filepath.Join(sdtTrashDir, rel)
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(dst)
		dst = strings.TrimSuffix(dst, ext) + "-" + contextNow().Format("20060102-150405") + ext
	}
	return dst, nil
}

// ctxMoveFile moves src to dst, creating the parent directory. A rename fails
// with EXDEV when the destination is another filesystem — possible when a project
// mounts the trash elsewhere — and only that case falls back to a copy and
// remove; any other failure is reported as it is, rather than retried as a
// different operation.
func ctxMoveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil { //#nosec G301 -- user work dir
		return err
	}
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	data, rerr := os.ReadFile(src) //#nosec G304 -- user work file
	if rerr != nil {
		return rerr
	}
	if werr := os.WriteFile(dst, data, 0o644); werr != nil { //#nosec G306,G703 -- a path under context/trash
		return werr
	}
	return os.Remove(src)
}

func outputRmResult(cmd *cobra.Command, res ctxRmResult) {
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
		verb := "moved"
		if res.DryRun {
			verb = "would move"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s %s → %s\n", verb, res.Path, res.Moved)
		switch len(res.Referrers) {
		case 0:
			b.WriteString("  no document references it\n")
		default:
			fmt.Fprintf(&b, "  %d document(s) still reference it:\n", len(res.Referrers))
			for _, r := range res.Referrers {
				fmt.Fprintf(&b, "    %s\n", r)
			}
		}
		outputString(cmd, b.String())
	}
}

var contextRmCmd = &cobra.Command{
	Use:   "rm <ref>",
	Short: "Discard a document by moving it into context/trash",
	Long: `Discard a document: move the file into ` + "`context/trash`" + `, leaving its bytes
untouched.

WHEN — the file itself must go: a wrong capture, a scratch document, something the
session created and should not have. This is an **editing** command, not a
management one; ` + "`sdt context deprecate`" + ` is the command for "keep it, stop
trusting it".

WHEN NOT — the document is still worth reading: ` + "`deprecate`" + ` retires it in place
and no reference breaks. You want the corpus left consistent and are willing to
spend the round trips: this command moves the file and **reports** the documents
that referenced it rather than repairing them.

RETURNS — the path, the type, where the file went, and the documents that named
it, so the caller can decide what to do about them. The report is the point: a
reference into the trash would survive until the first ` + "`clean --trash`" + ` and then
break with nothing to notice it, so the repair is left where the decision is.

The move is recoverable by construction — the file is one directory away — which
is why there is no confirmation flag to pass. A name already present in the trash
is not overwritten: the file lands with a timestamp instead.

EXAMPLE — ` + "`sdt context rm notes/20260101-000000-scratch`" + `
          ` + "`sdt context rm --type notes --slug scratch --dry-run`" + ``,
	Example: `  sdt context rm notes/20260101-000000-scratch
  sdt context rm analysis/20260101-000000-abandoned --dry-run
  sdt context rm --type notes --slug scratch --format json`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		exitWithError(cmd, err)
		dst, err := ctxTrashDestination(doc.Path)
		exitWithError(cmd, err)
		referrers, err := ctxReferrers(doc.Path)
		exitWithError(cmd, err)
		if referrers == nil {
			referrers = []string{}
		}

		res := ctxRmResult{Path: doc.Path, Kind: doc.Type.kind, Moved: dst, Referrers: referrers}
		if getBoolFlag(cmd, "dry-run", false) {
			res.DryRun = true
			outputRmResult(cmd, res)
			return
		}
		if err := ctxMoveFile(doc.Path, dst); err != nil {
			exitWithError(cmd, err)
			return
		}
		// The document set changed, so the generated index is regenerated: an
		// entry that outlived its file would be a broken link nobody authored.
		if _, err := ctxRegenerateIndex(); err != nil {
			exitWithError(cmd, err)
		}
		outputRmResult(cmd, res)
	},
}

func init() {
	contextRmCmd.Flags().Bool("dry-run", false, "Report the move and the referrers without moving anything")
	addContextStatusRefFlags(contextRmCmd)
}
