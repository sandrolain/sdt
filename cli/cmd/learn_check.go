package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// ── sdt learn check (the readiness verdict) ───────────────────────────────────
//
// A concept is teachable when its page and its lesson can carry a complete
// lesson. The six rules below are the whole policy, stated once: three about the
// page, three about the pair. They are about content, never about provenance — a
// missing `verified_at` is reported as a note, and `--verified-only` is the only
// thing that filters on it.

// Readiness verdicts.
const (
	learnVerdictTeachable = "teachable"
	learnVerdictRefused   = "refused"
)

// Provenance notes shown beside a verdict; never part of it.
const (
	learnStampVerified   = "verified"
	learnStampUnverified = "unverified"
)

// ctxLessonMinClaims is the smallest page that can carry a lesson: three items
// need three distinct claims and the `transfer` item a fourth.
const ctxLessonMinClaims = 4

// learnCheckEntry is one concept's readiness report.
type learnCheckEntry struct {
	Concept    string   `json:"concept" yaml:"concept"`
	Title      string   `json:"title,omitempty" yaml:"title,omitempty"`
	Verdict    string   `json:"verdict" yaml:"verdict"`
	Reasons    []string `json:"reasons,omitempty" yaml:"reasons,omitempty"`
	Claims     int      `json:"claims" yaml:"claims"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Maturity   string   `json:"maturity,omitempty" yaml:"maturity,omitempty"`
	Stamp      string   `json:"stamp" yaml:"stamp"`
	Difficulty string   `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Objectives int      `json:"objectives" yaml:"objectives"`
	Items      int      `json:"items" yaml:"items"`
	Uncovered  []int    `json:"uncovered_objectives,omitempty" yaml:"uncovered_objectives,omitempty"`
	Lesson     string   `json:"lesson,omitempty" yaml:"lesson,omitempty"`
}

// learnReadiness is the policy's inputs: a page and the lesson written for it.
type learnReadiness struct {
	Page       *contextwiki.Page
	Lesson     string   // path of the lesson document, "" when absent
	Items      []string // the claim refs of the lesson's item rows
	Objectives []string
}

var learnCheckCmd = &cobra.Command{
	Use:   "check <concept>...",
	Short: "Report whether a concept can be taught, and why not (read-only)",
	Long: `Answer one question per concept: can a lesson be built from this page?

WHEN — before teaching, before composing a lesson, or when a portal decides what
to offer.
WHEN NOT — you want the concepts behind a topic (learn inspect) or the lesson
itself (learn lesson).

RETURNS — one verdict per concept, teachable or refused, with its reasons and the
page's numbers. The six rules are about what the content can support: the page is
` + "`active`" + `, it carries at least four claims, its maturity is not ` + "`wip`" + `, it has
a ` + "`learning:`" + ` block, a lesson document exists, and every objective has an item.
Provenance is a note, never a rule: a page without ` + "`verified_at`" + ` is still
teachable, reported as ` + "`unverified`" + `, and ` + "`--verified-only`" + ` merely narrows the
report to stamped concepts. Exit 1 when a reported concept is refused.

EXAMPLE — sdt learn check architecture/four-key-metrics
          sdt learn check architecture/four-key-metrics --format json
          sdt learn check --verified-only`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			exitWithError(cmd, fmt.Errorf("a concept is required, use sdt learn check <concept>"))
			return
		}
		bld := loadLearnWiki(cmd)
		var out []learnCheckEntry
		for _, arg := range args {
			page := learnResolveConcept(bld, arg)
			if page == nil {
				out = append(out, learnCheckEntry{
					Concept: arg,
					Verdict: learnVerdictRefused,
					Reasons: []string{"no wiki page matches this concept; a lesson is not a substitute for a page"},
					Stamp:   learnStampUnverified,
				})
				continue
			}
			out = append(out, learnReadinessOf(page))
		}
		if getBoolFlag(cmd, "verified-only", false) {
			out = stampedOnly(out)
		}
		refused := 0
		for _, e := range out {
			if e.Verdict == learnVerdictRefused {
				refused++
			}
		}
		learnEmit(cmd, out, func() {
			if len(out) == 0 {
				outputString(cmd, "no concept reported (--verified-only filtered every page out)\n")
				return
			}
			for _, e := range out {
				outputString(cmd, fmt.Sprintf("%-10s %-40s claims=%d objectives=%d items=%d %s\n",
					e.Verdict, e.Concept, e.Claims, e.Objectives, e.Items, e.Stamp))
				for _, r := range e.Reasons {
					outputString(cmd, "           - "+r+"\n")
				}
			}
		})
		if refused > 0 {
			exitWithError(cmd, fmt.Errorf("%d concept(s) refused", refused))
		}
	},
}

// learnReadinessOf applies the six rules to one page and its lesson.
func learnReadinessOf(p *contextwiki.Page) learnCheckEntry {
	lessonPath := learnLessonPath(p.ID)
	objectives := learningObjectivesOf(p.Frontmatter)
	r := learnReadiness{Page: p, Lesson: lessonPath, Objectives: objectives}
	if _, err := os.Stat(lessonPath); err == nil {
		r.Items = learnLessonClaimRefs(lessonPath)
	} else {
		r.Lesson = ""
	}

	e := learnCheckEntry{
		Concept:    p.ID,
		Title:      p.Title,
		Verdict:    learnVerdictTeachable,
		Claims:     p.ClaimCount,
		Status:     p.Status,
		Maturity:   parseFrontmatterField(p.Frontmatter, ctxFrontmatterMaturity),
		Stamp:      learnStampUnverified,
		Objectives: len(objectives),
		Items:      len(r.Items),
		Lesson:     r.Lesson,
	}
	if block, ok := learningBlockOf(p.Frontmatter); ok {
		if d, ok := block["difficulty"].(string); ok {
			e.Difficulty = d
		}
	}
	if parseFrontmatterField(p.Frontmatter, ctxFrontmatterVerifiedAt) != "" {
		e.Stamp = learnStampVerified
	}

	refuse := func(reason string) {
		e.Verdict = learnVerdictRefused
		e.Reasons = append(e.Reasons, reason)
	}
	if p.Status != ctxWikiStatusActive {
		refuse(fmt.Sprintf("page status is %q: a lesson teaches an %q page", p.Status, ctxWikiStatusActive))
	}
	if p.ClaimCount < ctxLessonMinClaims {
		refuse(fmt.Sprintf("page carries %d claim(s), fewer than the %d a complete item set needs", p.ClaimCount, ctxLessonMinClaims))
	}
	if e.Maturity == "wip" {
		refuse("maturity is `wip`: the page is still being written")
	}
	if len(objectives) == 0 {
		refuse("no `learning:` block: the page states no objective, so nothing can be assessed")
	}
	if r.Lesson == "" {
		refuse(fmt.Sprintf("no lesson for this concept (expected %s)", learnLessonPath(p.ID)))
	}
	e.Uncovered = learnUncoveredObjectives(len(objectives), r.Items)
	if len(e.Uncovered) > 0 {
		refuse(fmt.Sprintf("objective(s) %v have no item", e.Uncovered))
	}
	return e
}

// learnUncoveredObjectives returns the 1-based objectives no item points at.
func learnUncoveredObjectives(total int, items []string) []int {
	covered := map[int]bool{}
	for _, raw := range items {
		n := 0
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
			continue
		}
		covered[n] = true
	}
	var out []int
	for i := 1; i <= total; i++ {
		if !covered[i] {
			out = append(out, i)
		}
	}
	return out
}

// learnLessonPath is where a concept's lesson lives: the lesson id mirrors the
// page id, so the path is the mapping.
func learnLessonPath(conceptID string) string {
	return filepath.Join(sdtLearningDir, filepath.FromSlash(conceptID)+sdtMarkdownExt)
}

// learnLessonClaimRefs reads a lesson document and returns the objective index of
// every item row.
func learnLessonClaimRefs(path string) []string {
	data, err := os.ReadFile(path) //#nosec G304 -- path built from the wiki id
	if err != nil {
		return nil
	}
	body := string(data)
	if strings.HasPrefix(body, "---\n") {
		body = string(frontmatterBody(data))
	}
	_, sections := markdownSections(body)
	rows := markdownTableRows(sections["Assess"])
	if len(rows) < 2 {
		return nil
	}
	var out []string
	for _, cells := range rows[1:] {
		if len(cells) != len(ctxLessonItemColumns) {
			continue
		}
		n := 0
		if _, err := fmt.Sscanf(cells[1], "%d", &n); err != nil || n < 1 {
			continue
		}
		out = append(out, cells[1])
	}
	return out
}

// learnResolveConcept resolves one argument to a page: an exact id or title first,
// then the shared topic resolver when it yields exactly one page.
func learnResolveConcept(bld *contextwiki.Builder, arg string) *contextwiki.Page {
	arg = strings.TrimSpace(arg)
	if p := bld.Resolve(arg); p != nil {
		return p
	}
	if pages := learnPagesFor(bld, arg); len(pages) == 1 {
		return pages[0]
	}
	return nil
}

// stampedOnly keeps the concepts that carry a `verified_at` stamp. It changes no
// verdict: a verified page that is refused stays refused.
func stampedOnly(in []learnCheckEntry) []learnCheckEntry {
	out := make([]learnCheckEntry, 0, len(in))
	for _, e := range in {
		if e.Stamp == learnStampVerified {
			out = append(out, e)
		}
	}
	return out
}

func init() {
	learnCheckCmd.Flags().Bool("verified-only", false, "Report only concepts that carry a `verified_at` stamp (a filter, not a rule)")
	learnCmd.AddCommand(learnCheckCmd)
}
