package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/frontmatter"
)

// ── technical renders (sdt context wiki render) ──────────────────────────────
//
// A technical render is a dense projection of one wiki page, kept outside the
// index and outside the document kinds (decision 0034): the agent writes the
// prose, this verb owns the one machine fact — whether the render still matches
// the page it was generated from. The header line is the whole contract, and it
// is an HTML comment so a file under context/ with no `kind` cannot be read as a
// malformed document.

const (
	renderDir = "context/renders/technical"
	// RenderStatuses is the closed vocabulary of a render's freshness.
	renderFresh   = "fresh"
	renderStale   = "stale"
	renderMissing = "missing"
	renderOrphan  = "orphan"
)

// renderHeaderRe matches the one line a render carries above its body.
var renderHeaderRe = regexp.MustCompile(`^<!-- generated from wiki/(\S+)@(\S+); regenerate, never edit -->$`)

// renderStatuses returns the closed status vocabulary, freshest first.
func renderStatuses() []string {
	return []string{renderFresh, renderStale, renderMissing, renderOrphan}
}

// renderPath is where a page's render lives: the page id's slashes stay
// directories, so the path is the identity and cannot collide with a wiki id.
func renderPath(pageID string) string {
	return filepath.Join(renderDir, filepath.FromSlash(strings.TrimSpace(pageID))+".md")
}

// renderHeader is the line the verifier reads, and the one an agent must write.
func renderHeader(pageID, updated string) string {
	return fmt.Sprintf("<!-- generated from wiki/%s@%s; regenerate, never edit -->", pageID, updated)
}

// renderRow is one line of the report.
type renderRow struct {
	Page    string `json:"page" yaml:"page"`
	Path    string `json:"path" yaml:"path"`
	Status  string `json:"status" yaml:"status"`
	Stamp   string `json:"stamp,omitempty" yaml:"stamp,omitempty"`
	Updated string `json:"updated,omitempty" yaml:"updated,omitempty"`
}

var contextWikiRenderCmd = &cobra.Command{
	Use:   "render [wiki-id...]",
	Short: "Report the freshness of the technical renders, or gate on it",
	Long: `Compare every technical render (context/renders/technical/<page-id>.md) with
the wiki page it was generated from.

STATUS — fresh (the stamp equals the page's updated), stale (it differs, or the
header cannot be read), missing (a page named on the command line has no render),
orphan (a render whose page does not exist). The walk finds the renders; a page
owes a render only when this command is asked about it.

EXIT — 0 by default: the report is a report. With ` + "`--check`" + ` it exits 1 when
any render is not fresh, so a caller that quotes a render gates on it first.

EXAMPLE — sdt context wiki render
          sdt context wiki render --check
          sdt context wiki render backend/auth --format json`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		bld, _, err := contextwiki.LoadWiki(sdtWikiDir)
		if err != nil {
			exitWithError(cmd, fmt.Errorf("load wiki: %w", err))
			return
		}
		rows, err := renderReport(bld, args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, merr := json.MarshalIndent(rows, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(rows)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if len(rows) == 0 {
				outputString(cmd, fmt.Sprintf("no renders under %s\n", renderDir))
			}
			for _, r := range rows {
				if r.Status == renderFresh {
					outputString(cmd, fmt.Sprintf("%s %s\n", r.Status, r.Page))
					continue
				}
				outputString(cmd, fmt.Sprintf("%s %s (%s → %s)\n",
					r.Status, r.Page, renderStampLabel(r.Stamp), renderStampLabel(r.Updated)))
			}
		}
		if getBoolFlag(cmd, "check", false) && !renderAllFresh(rows) {
			learnExitWith(fmt.Errorf("%s: not every render is fresh", renderDir), 1)
		}
	},
}

// renderStampLabel keeps an absent stamp visible in the text report.
func renderStampLabel(stamp string) string {
	if stamp == "" {
		return "no stamp"
	}
	return stamp
}

// renderAllFresh reports whether every row is fresh, the gate's condition.
func renderAllFresh(rows []renderRow) bool {
	for _, r := range rows {
		if r.Status != renderFresh {
			return false
		}
	}
	return true
}

// renderReport resolves the render set: every render on disk, plus one row for
// each page named on the command line (which is the only way `missing` can be
// reported — the lane never claims a page owes a render).
func renderReport(bld *contextwiki.Builder, pageIDs []string) ([]renderRow, error) {
	updated := pageUpdates(bld)
	rows := map[string]renderRow{}

	walked, err := walkRenders()
	if err != nil {
		return nil, err
	}
	for _, pageID := range walked {
		row := renderRow{Page: pageID, Path: filepath.ToSlash(renderPath(pageID))}
		stamp, headerErr := readRenderStamp(pageID)
		row.Stamp = stamp
		pageUpdated, ok := updated[pageID]
		switch {
		case !ok:
			row.Status = renderOrphan
		case headerErr != nil || stamp != pageUpdated:
			// A render that cannot prove its freshness is not fresh.
			row.Status = renderStale
			if ok {
				row.Updated = pageUpdated
			}
		default:
			row.Status = renderFresh
			row.Updated = pageUpdated
		}
		rows[pageID] = row
	}

	for _, raw := range pageIDs {
		pageID := strings.TrimSpace(raw)
		if pageID == "" {
			continue
		}
		page := bld.Resolve(pageID)
		if page == nil {
			return nil, fmt.Errorf("no wiki page matches %q: a render is a projection of a page, never of a guess", pageID)
		}
		if _, done := rows[page.ID]; done {
			continue
		}
		rows[page.ID] = renderRow{
			Page:    page.ID,
			Path:    filepath.ToSlash(renderPath(page.ID)),
			Status:  renderMissing,
			Updated: updated[page.ID],
		}
	}

	out := make([]renderRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return out, nil
}

// pageUpdates maps every wiki page id to its frontmatter `updated`, the stamp a
// render is compared with. The shared Page carries no such field, and one
// consumer does not earn it a field there.
func pageUpdates(bld *contextwiki.Builder) map[string]string {
	out := make(map[string]string, len(bld.Pages))
	for _, page := range bld.Pages {
		block, err := frontmatter.Parse(page.Frontmatter)
		if err != nil {
			continue
		}
		if value, ok := block.Key("updated"); ok {
			out[page.ID] = value
		}
	}
	return out
}

// walkRenders returns the page id of every render on disk, sorted.
func walkRenders() ([]string, error) {
	var ids []string
	root := filepath.FromSlash(renderDir)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), sdtMarkdownExt) {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		ids = append(ids, strings.TrimSuffix(filepath.ToSlash(rel), sdtMarkdownExt))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", renderDir, err)
	}
	sort.Strings(ids)
	return ids, nil
}

// readRenderStamp reads the stamp out of a render's first line.
func readRenderStamp(pageID string) (string, error) {
	data, err := os.ReadFile(renderPath(pageID)) //#nosec G304 -- derived from a path id, fixed root
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(string(data), "\n")
	match := renderHeaderRe.FindStringSubmatch(strings.TrimSpace(first))
	if match == nil {
		return "", fmt.Errorf("%s: first line is not a render header (%s)",
			renderPath(pageID), renderHeader(pageID, "<page-updated>"))
	}
	if match[1] != pageID {
		return "", fmt.Errorf("%s: header names %q, path names %q",
			renderPath(pageID), match[1], pageID)
	}
	return match[2], nil
}

func init() {
	contextWikiRenderCmd.Flags().Bool("check", false,
		"Exit 1 when any render is not fresh (the gate); without it the report always exits 0")
	contextWikiCmd.AddCommand(contextWikiRenderCmd)
}
