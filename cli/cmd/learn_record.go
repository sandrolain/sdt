package cmd

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

// ── sdt learn record (the only writer) ────────────────────────────────────────
//
// Everything else in the lane is a report. This verb writes the learner state and
// only when it is called explicitly: a checked answer, a taught lesson or a
// review never writes as a side effect. It also owns the ceiling that keeps the
// state honest — a first assessment cannot claim the top level, so one good answer
// never becomes mastery.

// learnRecordResult is what one explicit call wrote.
type learnRecordResult struct {
	Concept       string `json:"concept,omitempty" yaml:"concept,omitempty"`
	Understanding string `json:"understanding,omitempty" yaml:"understanding,omitempty"`
	Basis         string `json:"basis,omitempty" yaml:"basis,omitempty"`
	Created       bool   `json:"created,omitempty" yaml:"created,omitempty"`
	Profile       bool   `json:"profile,omitempty" yaml:"profile,omitempty"`
	Goals         int    `json:"goals,omitempty" yaml:"goals,omitempty"`
	Session       string `json:"session,omitempty" yaml:"session,omitempty"`
	Path          string `json:"path" yaml:"path"`
}

var learnRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record an assessment, a goal or a session in the learner state (the only writer)",
	Long: `Append or update the learner state — on an explicit call, never as a side effect.

WHEN — the learner demonstrated, applied or recalled a concept; a goal was declared;
a session ended.
WHEN NOT — you are about to check an answer: the check is read-only, and only what
you decide afterwards is recorded here.

RETURNS — what was written to context/learning/learner.yaml. Provenance is explicit:
` + "`--basis`" + ` defaults to inferred (the weaker claim). A first assessment is
capped at ` + "`partial`" + ` — reaching ` + "`solid`" + ` needs a second
demonstration, so one answer never implies mastery. An unknown concept fails.

EXAMPLE — sdt learn record --concept backend/auth --understanding partial --basis observed
          sdt learn record --profile "a learner" --since 2026-10-13
          sdt learn record --concept backend/auth --session --covered backend-auth`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		wikiID := strings.TrimSpace(getStringFlag(cmd, "concept", false))
		level := strings.TrimSpace(getStringFlag(cmd, "understanding", false))
		goalArgs := getStringArrayFlag(cmd, "goal", false)
		profile := strings.TrimSpace(getStringFlag(cmd, "profile", false))
		session := getBoolFlag(cmd, "session", false)

		if wikiID == "" && len(goalArgs) == 0 && profile == "" && !session {
			exitWithError(cmd, fmt.Errorf("nothing to record: pass --concept, --goal, --profile or --session"))
			return
		}
		if level != "" && wikiID == "" {
			exitWithError(cmd, fmt.Errorf("--understanding belongs to a concept: pass --concept <wiki-id>"))
			return
		}
		if err := learnRecordCheckLevel(level); err != nil {
			exitWithError(cmd, err)
			return
		}

		st := loadLearnState(cmd)
		bld := loadLearnWiki(cmd)
		now := contextNow().UTC()
		result := learnRecordResult{Path: learn.Path(".")}

		if profile != "" {
			st.Profile.Name = profile
			if since := strings.TrimSpace(getStringFlag(cmd, "since", false)); since != "" {
				st.Profile.Since = since
			}
			if st.Profile.Since == "" {
				st.Profile.Since = now.Format(learn.DateLayout)
			}
			result.Profile = true
		}

		for _, g := range goalArgs {
			if err := learnRecordAddGoal(st, g, now); err != nil {
				exitWithError(cmd, err)
				return
			}
			result.Goals++
		}

		if wikiID != "" {
			page := bld.Resolve(wikiID)
			if page == nil {
				exitWithError(cmd, fmt.Errorf("no wiki page matches the concept %q: "+
					"a learner record anchors to a page, never to recall", wikiID))
				return
			}
			rec, created, err := learnRecordAssess(st, bld, page, level, cmd, now)
			if err != nil {
				exitWithError(cmd, err)
				return
			}
			result.Concept, result.Understanding, result.Basis, result.Created =
				rec.ID, rec.LastAssessed.Understanding, rec.LastAssessed.Basis, created
		}

		if session {
			entry, err := learnRecordSession(st, cmd, now)
			if err != nil {
				exitWithError(cmd, err)
				return
			}
			result.Session = entry.At
		}

		if st.Version == 0 {
			st.Version = learn.Version
		}
		if err := learn.Save(".", st); err != nil {
			exitWithError(cmd, err)
			return
		}
		learnEmit(cmd, result, func() {
			if result.Profile {
				outputString(cmd, fmt.Sprintf("profile: %s (since %s)\n", st.Profile.Name, st.Profile.Since))
			}
			for range result.Goals {
				outputString(cmd, "goal: recorded\n")
			}
			if result.Concept != "" {
				verb := "updated"
				if result.Created {
					verb = "created"
				}
				outputString(cmd, fmt.Sprintf("concept: %s (%s, %s, basis %s)\n",
					result.Concept, verb, result.Understanding, result.Basis))
			}
			if result.Session != "" {
				outputString(cmd, "session: appended at "+result.Session+"\n")
			}
			outputString(cmd, "written: "+result.Path+"\n")
		})
	},
}

// learnLevels is the vocabulary a rating must come from, weakest first.
var learnLevels = []string{
	learn.UnderstandingUnknown, learn.UnderstandingWeak,
	learn.UnderstandingPartial, learn.UnderstandingSolid,
}

// learnRecordCheckLevel refuses an unknown level, and the empty one.
func learnRecordCheckLevel(level string) error {
	if level == "" {
		return nil
	}
	for _, known := range learnLevels {
		if level == known {
			return nil
		}
	}
	return fmt.Errorf("--understanding %q is not a known level (%s)",
		level, strings.Join(learnLevels, ", "))
}

// learnRecordAssess upserts the concept record and stamps the assessment. A first
// assessment is capped at partial: reaching solid needs a prior demonstration, so
// one answer never implies mastery.
func learnRecordAssess(st *learn.State, bld *contextwiki.Builder, p *contextwiki.Page,
	level string, cmd *cobra.Command, now time.Time) (*learn.Concept, bool, error) {

	rec := learnRecord(st, p.ID)
	created := rec == nil
	if created {
		rec = &learn.Concept{ID: learn.ConceptID(p.ID), Wiki: p.ID}
		st.Concepts = append(st.Concepts, *rec)
	}
	if level != "" {
		if created && level == learn.UnderstandingSolid {
			return nil, false, fmt.Errorf(
				"concept %q has no earlier demonstration: one answer never implies mastery — "+
					"record %q now, %q after a second demonstration",
				rec.ID, learn.UnderstandingPartial, learn.UnderstandingSolid)
		}
		basis := getStringFlag(cmd, "basis", false)
		if basis != learn.BasisObserved {
			basis = learn.BasisInferred // the weaker claim is the default
		}
		rec.LastAssessed = &learn.Assessment{
			At:            now.Format(learn.TimeLayout),
			Basis:         basis,
			Understanding: level,
		}
	}
	if prereq := getStringArrayFlag(cmd, "prerequisite", false); len(prereq) > 0 {
		rec.Prerequisites = append([]string(nil), prereq...)
	} else if len(rec.Prerequisites) == 0 {
		rec.Prerequisites = learnPrerequisiteIDs(bld, p)
	}
	for _, loc := range getStringArrayFlag(cmd, "evidence", false) {
		rec.Evidence = learnEvidenceAdd(rec.Evidence, loc)
	}
	// The page's own sources are the record's anchor; explicit locators extend them.
	for _, src := range p.Sources {
		rec.Evidence = learnEvidenceAdd(rec.Evidence, src)
	}
	if m := strings.TrimSpace(getStringFlag(cmd, "misconception", false)); m != "" {
		rec.Misconception = m
	}
	if nr := strings.TrimSpace(getStringFlag(cmd, "next-review", false)); nr != "" {
		rec.NextReview = nr
	}
	// Keep the slice element in sync with the upserted record.
	for i := range st.Concepts {
		if st.Concepts[i].ID == rec.ID {
			st.Concepts[i] = *rec
		}
	}
	return rec, created, nil
}

// learnEvidenceAdd appends locators, skipping blanks and repeats, so a repeated
// record extends the evidence instead of growing a duplicate list.
func learnEvidenceAdd(dst []string, add ...string) []string {
	for _, raw := range add {
		e := strings.TrimSpace(raw)
		if e == "" || slices.Contains(dst, e) {
			continue
		}
		dst = append(dst, e)
	}
	return dst
}

// learnPrerequisiteIDs returns the resolved depends_on/part_of targets of a page as
// stable record ids, sorted.
func learnPrerequisiteIDs(bld *contextwiki.Builder, p *contextwiki.Page) []string {
	ids := learnPrerequisites(bld, p)
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, learn.ConceptID(id))
	}
	sort.Strings(out)
	return out
}

// learnRecordAddGoal upserts one goal from a "<wiki-id>=<outcome>" argument.
func learnRecordAddGoal(st *learn.State, arg string, now time.Time) error {
	wikiID, outcome, ok := strings.Cut(strings.TrimSpace(arg), "=")
	if !ok || strings.TrimSpace(wikiID) == "" || strings.TrimSpace(outcome) == "" {
		return fmt.Errorf("--goal %q must be \"<wiki-id>=<outcome>\"", arg)
	}
	id := "g" + learn.ConceptID(wikiID)
	created := now.Format(learn.DateLayout)
	for i := range st.Goals {
		if st.Goals[i].ID == id || st.Goals[i].Concept == wikiID {
			st.Goals[i].Outcome = strings.TrimSpace(outcome)
			return nil
		}
	}
	st.Goals = append(st.Goals, learn.Goal{
		ID: id, Concept: strings.TrimSpace(wikiID),
		Outcome: strings.TrimSpace(outcome), Created: created,
	})
	return nil
}

// learnRecordSession appends one session-log entry, defaulting the covered and
// weak lists to the concept just recorded.
func learnRecordSession(st *learn.State, cmd *cobra.Command, now time.Time) (learn.Session, error) {
	entry := learn.Session{At: now.Format(learn.TimeLayout)}
	entry.Covered = getStringArrayFlag(cmd, "covered", false)
	entry.Weak = getStringArrayFlag(cmd, "weak", false)
	entry.Due = getStringArrayFlag(cmd, "due", false)
	if len(entry.Covered) == 0 {
		for i := range st.Concepts {
			if a := st.Concepts[i].LastAssessed; a != nil && a.At == entry.At {
				entry.Covered = append(entry.Covered, st.Concepts[i].ID)
			}
		}
	}
	if len(entry.Weak) == 0 {
		for i := range st.Concepts {
			c := &st.Concepts[i]
			switch {
			case strings.TrimSpace(c.Misconception) != "":
				entry.Weak = append(entry.Weak, c.ID)
			case c.LastAssessed != nil && (c.LastAssessed.Understanding == learn.UnderstandingWeak ||
				c.LastAssessed.Understanding == learn.UnderstandingPartial):
				entry.Weak = append(entry.Weak, c.ID)
			}
		}
	}
	st.Sessions = append(st.Sessions, entry)
	return entry, nil
}

func init() {
	f := learnRecordCmd.Flags()
	f.String("concept", "", "Wiki page id of the concept being assessed")
	f.String("understanding", "", "Level demonstrated: "+strings.Join(learnLevels, ", "))
	f.String("basis", "", "Provenance of the rating: observed or inferred (default inferred)")
	f.String("misconception", "", "Open misconception to record")
	f.StringArray("evidence", nil, "Evidence locator backing the record (repeatable)")
	f.StringArray("prerequisite", nil, "Prerequisite record id (repeatable; defaults to the page's relations)")
	f.String("next-review", "", "Next review date (YYYY-MM-DD)")
	f.String("profile", "", "Learner name to record")
	f.String("since", "", "Learning start date (YYYY-MM-DD)")
	f.StringArray("goal", nil, "Goal as \"<wiki-id>=<outcome>\" (repeatable)")
	f.Bool("session", false, "Append a session-log entry")
	f.StringArray("covered", nil, "Concept record id covered in the session (repeatable)")
	f.StringArray("weak", nil, "Concept record id that stayed weak (repeatable)")
	f.StringArray("due", nil, "Concept record id that became due (repeatable)")
	learnCmd.AddCommand(learnRecordCmd)
}
