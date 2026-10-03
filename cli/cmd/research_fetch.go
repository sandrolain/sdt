package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/sandrolain/sdt/cli/utils/converter"
	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// fetchTargets acquires each target into the run, recording provenance on
// success and a retry/error on failure. It returns how many succeeded and how
// many failed. It never aborts on a single failure. Callers persist the run.
func fetchTargets(r *research.Run, root string, targets []string, userAgent string, timeout int) (fetched, failed int) {
	conv, err := converter.NewConverter(converter.Options{
		BulletListMarker: "-",
		CodeBlockStyle:   "fenced",
		EmDelimiter:      "*",
		StrongDelimiter:  "**",
		LinkStyle:        "inlined",
	})
	if err != nil {
		return 0, len(targets)
	}
	rawDir := filepath.Join(research.RunDir(root, r.RunID), research.RawDirName)

	for _, target := range targets {
		captured, ferr := fetchSinglePage(target, userAgent, timeout, false, false, false, conv)
		if ferr != nil {
			r.RecordFailure(target, ferr)
			failed++
			continue
		}
		page := captured.Page
		src := research.Source{
			CanonicalURL: page.URL,
			Title:        page.Title,
			Author:       page.Author,
			Date:         page.Date,
			ContentType:  "text/markdown",
		}
		if src.CanonicalURL == "" {
			src.CanonicalURL = target
		}
		if err := r.RecordFetch(rawDir, src, []byte(captured.Markdown)); err != nil {
			r.RecordFailure(target, err)
			failed++
			continue
		}
		fetched++
		r.MarkCheckpoint("fetch")
	}
	return fetched, failed
}

// saveOrLog persists the run, logging a save error rather than masking the
// caller's primary error.
func saveOrLog(r *research.Run, root string) {
	if err := r.Save(root); err != nil {
		slog.Error("save run", "err", err)
	}
}

var researchFetchCmd = &cobra.Command{
	Use:   "fetch <url>...",
	Short: "Acquire named URLs into the run with provenance",
	Long: `Fetch explicitly named URLs (or every 'discovered' source when no URL is
given) into the run with 'sdt crawldown's pipeline: each page is converted to
Markdown, written under the run's raw/ directory, and recorded in the manifest
with its title, date, content type, size, SHA-256 and a checkpoint.

Only the URLs you name (or the discovered sources) are fetched — this is not a
recursive crawler. A failed URL is recorded with its error and a retry count,
and does not abort the others; the command exits non-zero when any failed.

Examples:
  sdt research fetch https://example.com/a https://example.com/b
  sdt research fetch            # fetch every 'discovered' source recorded by search`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		userAgent := getStringFlag(cmd, "user-agent", false)
		timeout := getIntFlag(cmd, "timeout", false)

		targets := args
		if len(targets) == 0 {
			targets = r.PendingFetch()
		}
		if len(targets) == 0 {
			exitWithError(cmd, errors.New("nothing to fetch: name URLs or run `sdt research search` first"))
			return
		}

		fetched, failed := fetchTargets(r, root, targets, userAgent, timeout)
		if terr := requireNoError(cmd, failed, fetched); terr != nil {
			saveOrLog(r, root)
			exitWithError(cmd, terr)
			return
		}
		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
		}
		outputResearch(cmd, r, func() string {
			return fmt.Sprintf("run %s: fetched %d, failed %d\n", r.RunID, fetched, failed)
		})
	},
}

// requireNoError returns an error when some targets failed, so the command exits
// non-zero while the successful fetches stay recorded.
func requireNoError(_ *cobra.Command, failed, fetched int) error {
	if failed > 0 {
		return fmt.Errorf("%d of %d fetch(es) failed", failed, failed+fetched)
	}
	return nil
}

func init() {
	researchFetchCmd.Flags().Int("timeout", 60, "Request timeout in seconds")
	researchFetchCmd.Flags().Int("delay", 0, "Delay in seconds between fetches")
	researchFetchCmd.Flags().String("user-agent", "sdt/1.0", "HTTP user agent for requests")
	researchFetchCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchFetchCmd)
}
