package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── wiki frontier score (sdt context wiki frontier) ───────────────────────────
//
// A read-only, model-free report over the wiki graph: for each page,
// score = (out_degree − in_degree) × exp(−days_since_updated/30). A high score
// is a page the corpus points to rarely and that has not moved — the frontier
// where knowledge is thin. It reuses the shared contextwiki builder (ADR-0003)
// and never writes.

// wikiFrontierEntry is one row of the frontier report.
type wikiFrontierEntry struct {
	ID               string  `json:"id" yaml:"id"`
	Title            string  `json:"title" yaml:"title"`
	Path             string  `json:"path" yaml:"path"`
	OutDegree        int     `json:"out_degree" yaml:"out_degree"`
	InDegree         int     `json:"in_degree" yaml:"in_degree"`
	DaysSinceUpdated float64 `json:"days_since_updated" yaml:"days_since_updated"`
	Score            float64 `json:"score" yaml:"score"`
}

// wikiFrontierDecayDays is the e-folding window: a page untouched for this many
// days halves the recency weight, so stale pages surface as the frontier.
const wikiFrontierDecayDays = 30

// wikiOutDegree counts a page's resolved outbound edges (relations + typed
// links), mirroring Builder.Inbound so in- and out-degree are symmetric.
func wikiOutDegree(p *contextwiki.Page, bld *contextwiki.Builder) int {
	n := 0
	for _, targets := range p.Relations {
		for _, raw := range targets {
			if m := contextwiki.LinkRegexp.FindStringSubmatch(raw); m != nil {
				if bld.Resolve(strings.TrimSpace(m[1])) != nil {
					n++
				}
			}
		}
	}
	for _, l := range p.Links {
		if l.Verb == "" {
			continue
		}
		if bld.Resolve(l.Target) != nil {
			n++
		}
	}
	return n
}

// wikiUpdated returns the page's last-updated time: the frontmatter `updated`
// when parseable, else the file mtime, else now.
func wikiUpdated(p *contextwiki.Page, now time.Time) time.Time {
	if raw := strings.TrimSpace(contextwiki.FrontmatterField(p.Frontmatter, "updated")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return t
		}
	}
	if info, err := os.Stat(p.Path); err == nil {
		return info.ModTime()
	}
	return now
}

// wikiFrontier computes the score for every page, sorted descending (tie-break
// by id).
func wikiFrontier(bld *contextwiki.Builder, now time.Time) []wikiFrontierEntry {
	inbound := bld.Inbound()
	out := make([]wikiFrontierEntry, 0, len(bld.Pages))
	for _, p := range bld.Pages {
		outDeg := wikiOutDegree(p, bld)
		inDeg := len(inbound[p.ID])
		days := now.Sub(wikiUpdated(p, now)).Hours() / 24
		if days < 0 {
			days = 0
		}
		out = append(out, wikiFrontierEntry{
			ID:               p.ID,
			Title:            p.Title,
			Path:             p.Path,
			OutDegree:        outDeg,
			InDegree:         inDeg,
			DaysSinceUpdated: days,
			Score:            float64(outDeg-inDeg) * math.Exp(-days/wikiFrontierDecayDays),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}

var contextWikiFrontierCmd = &cobra.Command{
	Use:   "frontier",
	Short: "Report where the wiki is thin (read-only frontier score)",
	Long: `Rank wiki pages by a frontier score: (out_degree − in_degree) ×
exp(−days_since_updated/30). A high score is a page the corpus points to rarely
and that has not moved — where knowledge is thin and deserves more. Read-only,
deterministic, model-free; reuses the shared wiki graph.

Examples:
  sdt context wiki frontier
  sdt context wiki frontier --limit 10 --format json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		bld, errs, werr := contextwiki.LoadWiki(sdtWikiDir)
		if werr != nil {
			exitWithError(cmd, fmt.Errorf("load wiki: %w", werr))
			return
		}
		if len(errs) > 0 {
			// Advisory: unreadable pages are skipped, never fatal here.
			fmt.Fprintf(os.Stderr, "warning: %d unreadable wiki page(s) skipped\n", len(errs))
		}
		entries := wikiFrontier(bld, contextNow())
		if limit := getIntFlag(cmd, "limit", false); limit > 0 && len(entries) > limit {
			entries = entries[:limit]
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(entries, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(entries)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			for _, e := range entries {
				outputString(cmd, fmt.Sprintf("%7.3f  out=%d in=%d  %.0fd  %s\n", e.Score, e.OutDegree, e.InDegree, e.DaysSinceUpdated, e.ID))
			}
		}
	},
}

func init() {
	contextWikiFrontierCmd.Flags().Int("limit", 20, "Maximum rows to report (0 = all)")
	contextWikiCmd.AddCommand(contextWikiFrontierCmd)
}
