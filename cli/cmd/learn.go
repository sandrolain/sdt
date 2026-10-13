package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// ── sdt learn (the learning lane) ─────────────────────────────────────────────
//
// A read-only-by-default surface over the local wiki and the project-scoped
// learner state (context/learning/learner.yaml). Only `learn record` writes, and
// only when it is invoked explicitly; every other verb is a report. Learner state
// is evidence, never wiki knowledge: no verb here writes a wiki page.

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Learn, review and record concepts against the local wiki",
	Long: `Resolve a topic to the wiki concepts that cover it, report what is due or
weak, and record what the learner demonstrated.

WHEN — you need to learn, review, quiz or practise a concept, or to explain one
from the corpus.
WHEN NOT — the corpus holds no page for the topic: this surface reports what the
corpus has, it never teaches from recall. Ingesting a source is a different job.

RETURNS — reports over the wiki graph and the learner state. Only ` + "`learn record`" + `
writes, and only on an explicit call.

EXAMPLE — sdt learn inspect "resolution order"
          sdt learn review --limit 5`,
}

// learnView pairs one learner-state concept record with the wiki page it maps to.
type learnView struct {
	ID            string `json:"id" yaml:"id"`
	Wiki          string `json:"wiki" yaml:"wiki"`
	Title         string `json:"title,omitempty" yaml:"title,omitempty"`
	Recorded      bool   `json:"recorded" yaml:"recorded"`
	Understanding string `json:"understanding,omitempty" yaml:"understanding,omitempty"`
	Basis         string `json:"basis,omitempty" yaml:"basis,omitempty"`
	Misconception string `json:"misconception,omitempty" yaml:"misconception,omitempty"`
	NextReview    string `json:"next_review,omitempty" yaml:"next_review,omitempty"`
	LastAssessed  string `json:"last_assessed,omitempty" yaml:"last_assessed,omitempty"`
}

// loadLearnState loads the learner state; an absent state is empty, not an error.
func loadLearnState(cmd *cobra.Command) *learn.State {
	st, err := learn.Load(".")
	if err != nil {
		exitWithError(cmd, err)
		return &learn.State{}
	}
	return st
}

// loadLearnWiki loads the wiki graph; unreadable pages are advisory, never fatal.
func loadLearnWiki(cmd *cobra.Command) *contextwiki.Builder {
	bld, errs, werr := contextwiki.LoadWiki(sdtWikiDir)
	if werr != nil {
		exitWithError(cmd, fmt.Errorf("load wiki: %w", werr))
		return &contextwiki.Builder{}
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d unreadable wiki page(s) skipped\n", len(errs))
	}
	return bld
}

// learnRecord returns the state record mapped to a wiki page id, or nil.
func learnRecord(st *learn.State, wikiID string) *learn.Concept {
	for i := range st.Concepts {
		if st.Concepts[i].Wiki == wikiID {
			return &st.Concepts[i]
		}
	}
	return nil
}

// learnViewOf builds the reported view of one state record, keeping the
// assessment provenance (basis and stamp) beside the standing.
func learnViewOf(c *learn.Concept) learnView {
	v := learnView{
		ID: c.ID, Wiki: c.Wiki, Recorded: true,
		Misconception: c.Misconception, NextReview: c.NextReview,
	}
	if a := c.LastAssessed; a != nil {
		v.Understanding = a.Understanding
		v.Basis = a.Basis
		v.LastAssessed = a.At
	}
	return v
}

// learnViewFor pairs a page with its record, when the learner has one.
func learnViewFor(p *contextwiki.Page, st *learn.State) learnView {
	rec := learnRecord(st, p.ID)
	if rec == nil {
		return learnView{ID: learn.ConceptID(p.ID), Wiki: p.ID, Title: p.Title}
	}
	v := learnViewOf(rec)
	v.Title = p.Title
	return v
}

// learnPrerequisites returns the resolved depends_on/part_of ids of a page, sorted.
func learnPrerequisites(bld *contextwiki.Builder, p *contextwiki.Page) []string {
	seen := map[string]bool{}
	var out []string
	for _, verb := range []string{"depends_on", "part_of"} {
		for _, raw := range p.Relations[verb] {
			m := contextwiki.LinkRegexp.FindStringSubmatch(raw)
			if m == nil {
				continue
			}
			id := strings.TrimSpace(m[1])
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if bld.Resolve(id) != nil {
				out = append(out, id)
			}
		}
	}
	sort.Strings(out)
	return out
}

// learnPagesFor resolves a topic to the wiki pages that cover it: an exact id or
// title match first, then a case-insensitive substring over id, title, summary
// and tags. The result is ordered by id, so a report is stable.
func learnPagesFor(bld *contextwiki.Builder, topic string) []*contextwiki.Page {
	topic = strings.TrimSpace(topic)
	var exact, partial []*contextwiki.Page
	for _, p := range bld.Pages {
		switch {
		case p.ID == topic || strings.EqualFold(p.Title, topic):
			exact = append(exact, p)
		case learnPageMatches(p, topic):
			partial = append(partial, p)
		}
	}
	out := append(exact, partial...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// learnPageMatches reports whether a page carries the topic in id, title, summary
// or tags.
func learnPageMatches(p *contextwiki.Page, topic string) bool {
	needle := strings.ToLower(topic)
	for _, field := range append([]string{p.ID, p.Title, p.Summary}, p.Tags...) {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

// learnStatusWord is the one-word standing of a concept in a report.
func learnStatusWord(v learnView) string {
	if !v.Recorded {
		return "new"
	}
	if v.Understanding == "" {
		return "recorded"
	}
	return v.Understanding
}

// learnDue reports whether a review date has arrived. An empty or unparseable
// date means "not scheduled", never "due".
func learnDue(nextReview string, now time.Time) bool {
	if nextReview == "" {
		return false
	}
	due, err := time.Parse(learn.DateLayout, nextReview)
	if err != nil {
		return false
	}
	return !due.After(now)
}

// learnEmit prints a report in the selected format: the machine formats carry the
// payload, text renders it through text().
func learnEmit(cmd *cobra.Command, payload any, text func()) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(payload, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(payload)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		text()
	}
}

func init() {
	rootCmd.AddCommand(learnCmd)
}
