package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandrolain/sdt/internal/research"
	"github.com/spf13/cobra"
)

// reindexAndLint regenerates the context index and runs the wiki lint after a
// promotion, so a written page is immediately consistent with the corpus. It
// logs a problem rather than aborting the apply, which already happened.
func reindexAndLint(_ *cobra.Command) {
	if idx, err := buildIndex(); err == nil {
		if _, werr := writeIndexIfChanged(idx); werr != nil {
			slog.Warn("reindex after promotion failed", "err", werr)
		}
	} else {
		slog.Warn("build index after promotion failed", "err", err)
	}
	for _, issue := range lintWikiLint() {
		if issue.Priority == ctxLintCritical {
			slog.Warn("wiki lint after promotion", "issue", issue.String())
		}
	}
}

var researchPopulateWikiCmd = &cobra.Command{
	Use:   "populate-wiki",
	Short: "Propose (and, on approval, apply) promoting verified findings to the wiki",
	Long: `Propose wiki pages from a run's verified sources: one page per verified
source, deduplicated against the existing wiki ids, with a page budget. The
default is a PREVIEW that writes nothing.

--apply performs the promotion, but only when you also pass --confirm: it writes
the pages, reindexes and runs the wiki lint. A source SDT could not verify never
becomes a page; refused sources are listed with the reason.

Examples:
  sdt research populate-wiki --brief "embedded vector stores"
  sdt research populate-wiki --brief "…" --max-pages 3 --format json
  sdt research populate-wiki --brief "…" --apply --confirm`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r, root, err := resolveRun(cmd)
		exitWithError(cmd, err)

		brief := getStringFlag(cmd, "brief", false)
		maxPages := getIntFlag(cmd, "max-pages", false)
		apply := getBoolFlag(cmd, "apply", false)

		existing := wikiPageIDs(root)
		plan := r.PlanWiki(research.RunDir(root, r.RunID), brief, existing, maxPages)

		if !apply {
			outputResearch(cmd, plan, func() string { return plan.RenderPlan() })
			return
		}

		if !getBoolFlag(cmd, "confirm", false) {
			exitWithError(cmd, fmt.Errorf("--apply needs --confirm: the promotion is a write into the wiki"))
			return
		}
		if !plan.Establishable() {
			exitWithError(cmd, fmt.Errorf("nothing to promote: no verified source to turn into a page"))
			return
		}

		byID := map[string]research.Source{}
		for _, s := range r.Sources {
			if s.Status == research.StatusVerified {
				byID[research.SourceSlug(s.CanonicalURL)] = s
			}
		}
		written := 0
		for _, page := range plan.Pages {
			src, ok := byID[page.ID]
			if !ok {
				continue
			}
			// Archive the raw capture into the run's dated directory under
			// context/refs/ and cite it: the wiki contract requires every claim
			// to cite a refs/ file.
			refRel, aerr := r.ArchiveSource(root, src)
			if aerr != nil {
				exitWithError(cmd, aerr)
				return
			}
			path := research.WikiPagePath(root, page.ID)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				exitWithError(cmd, err)
			}
			//#nosec G306 -- a wiki corpus document
			if err := os.WriteFile(path, []byte(research.RenderPage(page, src, r.RunID, refRel)), 0o644); err != nil {
				exitWithError(cmd, err)
			}
			written++
		}

		reindexAndLint(cmd)
		r.MarkCheckpoint("populate-wiki")
		if err := r.Save(root); err != nil {
			exitWithError(cmd, err)
		}
		outputString(cmd, fmt.Sprintf("applied: %d page(s) written; reindex + wiki lint run\n", written))
	},
}

// wikiPageIDs lists the existing wiki page ids (relative subpath minus .md).
func wikiPageIDs(root string) []string {
	dir := filepath.Join(root, "context", "wiki")
	var ids []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // a walk error just yields a partial id list
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		ids = append(ids, strings.TrimSuffix(filepath.ToSlash(rel), ".md"))
		return nil
	})
	sort.Strings(ids)
	return ids
}

func init() {
	researchPopulateWikiCmd.Flags().String("brief", "", "Why the knowledge is useful to the project")
	researchPopulateWikiCmd.Flags().Int("max-pages", 0, "Cap the pages proposed (0 = no cap)")
	researchPopulateWikiCmd.Flags().Bool("apply", false, "Apply the plan (default: preview only)")
	researchPopulateWikiCmd.Flags().Bool("confirm", false, "Confirm the write into the wiki (required with --apply)")
	researchPopulateWikiCmd.Flags().String("run", "", "Run id (default: the latest run)")
	researchCmd.AddCommand(researchPopulateWikiCmd)
}
