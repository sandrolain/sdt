package cmd

import (
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

func TestReadinessCandidatesRequireAnExactChoiceForPartialMatches(t *testing.T) {
	one := &contextwiki.Page{ID: "architecture/business-domain", Title: "Business domain"}
	two := &contextwiki.Page{ID: "architecture/business-domains", Title: "Business domains"}
	bld := &contextwiki.Builder{Pages: []*contextwiki.Page{one, two}, ByID: map[string]*contextwiki.Page{one.ID: one, two.ID: two}}

	if got := readinessCandidates(bld, "architecture/business-domain"); len(got) != 1 || got[0] != one {
		t.Fatalf("exact ID resolution = %v, want business-domain only", got)
	}
	if got := readinessCandidates(bld, "business domain"); len(got) != 2 {
		t.Fatalf("partial candidates = %v, want both matches", got)
	}
}

func TestTopicReadinessMarksDownstreamTargetsNotPlanned(t *testing.T) {
	setupContextProject(t)
	p := &contextwiki.Page{ID: "architecture/x", Title: "X", Path: "context/wiki/architecture/x.md", Status: ctxWikiStatusActive, Sources: []string{"refs/x.md"}}
	writeReadinessSource(t, "refs/x.md", "used")
	report := deriveTopicReadiness("x", "wiki", p)
	if len(report.Stages) != 4 {
		t.Fatalf("stages = %d, want source/wiki/learning/materials", len(report.Stages))
	}
	if report.Stages[0].Status != "recorded" || report.Stages[1].Status != "ready" {
		t.Fatalf("planned source/wiki stages = %+v, want recorded/ready", report.Stages[:2])
	}
	if report.Stages[2].Status != "not_planned" || report.Stages[3].Status != "not_planned" {
		t.Fatalf("downstream stages = %+v, want not_planned", report.Stages[2:])
	}
	if report.NextAction != "" {
		t.Fatalf("next action = %q, want none", report.NextAction)
	}
}

func TestTopicReadinessChoosesTheFirstUnreadyStage(t *testing.T) {
	setupContextProject(t)
	p := &contextwiki.Page{ID: "architecture/x", Title: "X", Path: "context/wiki/architecture/x.md", Status: ctxWikiStatusActive, Sources: []string{"refs/x.md"}}
	writeReadinessSource(t, "refs/x.md", "used")
	writeMaterialPage(t, p.ID)
	report := deriveTopicReadiness("x", "materials", p)
	if report.NextAction != "learning readiness for context/learning/architecture/x.md" {
		t.Fatalf("next action = %q, want the first unmet learning stage", report.NextAction)
	}
	if report.Stages[3].Status != "absent" {
		t.Fatalf("materials stage = %+v, want absent with no guide", report.Stages[3])
	}
}

func TestTopicReadinessChoosesUnreviewedSourceBeforeLearning(t *testing.T) {
	setupContextProject(t)
	writeReadinessSource(t, "refs/x.md", ctxSourceStatusCandidate)
	p := &contextwiki.Page{ID: "architecture/x", Title: "X", Path: "context/wiki/architecture/x.md", Status: ctxWikiStatusActive, Sources: []string{"refs/x.md"}}
	report := deriveTopicReadiness("x", ctxFrontmatterLearning, p)
	if report.Stages[0].Status != readinessStatusUnreviewed {
		t.Fatalf("source stage = %+v, want unreviewed", report.Stages[0])
	}
	if report.NextAction != "sources readiness for context/sources/source.md (custody: candidate)" {
		t.Fatalf("next action = %q, want the candidate source sheet", report.NextAction)
	}
}

func TestTopicReadinessDoesNotCallAnUnassessedConceptTeachable(t *testing.T) {
	p := lessonFixture(t, checkPageFM, "## Prerequisites\n\n## Worked example\n\nNo assessment items.\n", true)
	p.Path = "context/wiki/architecture/x.md"
	p.Sources = []string{"refs/archived-source.md"}
	writeReadinessSource(t, "refs/archived-source.md", "retired")
	report := deriveTopicReadiness("architecture/x", "learning", p)
	if report.Stages[0].Status != "recorded" || !strings.Contains(strings.Join(report.Stages[0].Reasons, " "), "retired") {
		t.Fatalf("source stage = %+v, want source reference reported separately", report.Stages[0])
	}
	if report.Stages[2].Status != "blocked" || len(report.Stages[2].Reasons) == 0 {
		t.Fatalf("learning stage = %+v, want blocked for missing assessment coverage", report.Stages[2])
	}
	if report.NextAction != "learning readiness for "+learnLessonPath(p.ID) {
		t.Fatalf("next action = %q, want the missing learning artifact", report.NextAction)
	}
}

func writeReadinessSource(t *testing.T, extract, status string) {
	t.Helper()
	writeCtxDoc(t, "context/sources/source.md", "---\nkind: source\nstatus: "+status+"\nextract: "+extract+"\n---\n")
}

func TestReadinessMaterialStatusKeepsDimensionsDistinct(t *testing.T) {
	base := materialStatusRow{Freshness: renderFresh, Deck: renderFresh, Review: materialReviewRecorded, Technical: "source-check"}
	cases := []struct {
		name string
		row  materialStatusRow
		want string
	}{
		{name: "ready", row: base, want: "ready"},
		{name: "stale deck", row: materialStatusRow{Freshness: renderFresh, Deck: renderStale, Review: materialReviewRecorded, Technical: "source-check"}, want: "stale"},
		{name: "unreviewed", row: materialStatusRow{Freshness: renderFresh, Deck: renderFresh, Review: materialReviewNotRun, Technical: "source-check"}, want: "unreviewed"},
		{name: "technical issue", row: materialStatusRow{Freshness: renderFresh, Deck: renderFresh, Review: materialReviewRecorded, Technical: "issues"}, want: "blocked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := readinessMaterialStatus(tc.row); got != tc.want {
				t.Errorf("status = %q, want %q", got, tc.want)
			}
		})
	}
}
