package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/learn"
)

func TestLearnAttemptHistoryKeepsOldVersionsAndUsesLatestPerItem(t *testing.T) {
	old := learn.Attempt{Item: "a-1", At: "2026-10-01T00:00:00Z", KeyHash: "old"}
	concept := &learn.Concept{Wiki: "architecture/x", LastAttempt: &old}
	current := learn.Attempt{Item: "a-1", At: "2026-10-02T00:00:00Z", KeyHash: "current"}
	learnAppendAttempt(concept, current)
	if len(concept.Attempts) != 2 || concept.Attempts[0].KeyHash != "old" || concept.LastAttempt.KeyHash != "current" {
		t.Fatalf("attempt history lost its previous version: %+v", concept)
	}
}

func TestLearnAttemptRevisionTracksKeyAndObjective(t *testing.T) {
	writeLesson(t, checkLessonBody)
	item, ok := learnLessonItemByID("architecture/x", "a-1")
	if !ok {
		t.Fatal("fixture item missing")
	}
	metadata := learningMetadataForConcept("architecture/x")
	concept := &learn.Concept{Wiki: "architecture/x", LastAttempt: &learn.Attempt{
		Item: item.ID, KeyHash: item.KeyDigest, ClaimHash: item.Hash,
		ObjectiveHash: learnObjectiveHash(metadata.Objectives, item.Objective),
	}}
	if reason := learnAttemptRevisionReason(concept, nil); reason != "" {
		t.Fatalf("unchanged evidence needs review: %s", reason)
	}
	lessonPath := learnLessonPath("architecture/x")
	lesson, err := os.ReadFile(lessonPath)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(lesson), "name the parts", "name the components", 1)
	if err := os.WriteFile(lessonPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if reason := learnAttemptRevisionReason(concept, nil); !strings.Contains(reason, "objective changed") {
		t.Fatalf("objective revision was not targeted: %q", reason)
	}
	concept.LastAttempt.ObjectiveHash = learnObjectiveHash(learningMetadataForConcept("architecture/x").Objectives, item.Objective)
	assessmentPath := learnAssessmentPath("architecture/x")
	assessment, err := os.ReadFile(assessmentPath)
	if err != nil {
		t.Fatal(err)
	}
	changedAssessment := strings.Replace(string(assessment), "| the parts |", "| the components |", 1)
	if err := os.WriteFile(assessmentPath, []byte(changedAssessment), 0o644); err != nil {
		t.Fatal(err)
	}
	if reason := learnAttemptRevisionReason(concept, nil); !strings.Contains(reason, "key changed") {
		t.Fatalf("assessment revision was not targeted: %q", reason)
	}
}

func TestLearnAttemptRevisionTargetsChangedClaimAndClearsAfterReassessment(t *testing.T) {
	writeLesson(t, checkLessonBody)
	items, ok := learnAssessmentItems("architecture/x")
	if !ok || len(items) != 2 {
		t.Fatalf("lesson items = %d, ok=%t", len(items), ok)
	}
	meta := learningMetadataForConcept("architecture/x")
	first := learn.Attempt{Item: items[0].ID, At: "2026-10-01T00:00:00Z", KeyHash: items[0].KeyDigest, ClaimHash: items[0].Hash, ObjectiveHash: learnObjectiveHash(meta.Objectives, items[0].Objective)}
	second := learn.Attempt{Item: items[1].ID, At: "2026-10-01T00:01:00Z", KeyHash: items[1].KeyDigest, ClaimHash: items[1].Hash, ObjectiveHash: learnObjectiveHash(meta.Objectives, items[1].Objective)}
	concept := &learn.Concept{Wiki: "architecture/x", Attempts: []learn.Attempt{first, second}, LastAttempt: &second}
	bld := tempWiki()
	if reason := learnAttemptRevisionReason(concept, bld); reason != "" {
		t.Fatalf("unchanged claims need no review: %s", reason)
	}
	page := *bld.ByID["architecture/x"]
	page.Body = strings.Replace(page.Body, "<!-- c-aaaaaaaa --> a", "<!-- c-aaaaaaaa --> changed", 1)
	bld.ByID[page.ID] = &page
	if reason := learnAttemptRevisionReason(concept, bld); !strings.Contains(reason, "supporting claim changed") {
		t.Fatalf("claim change was not targeted: %q", reason)
	}
	first.At = "2026-10-02T00:00:00Z"
	first.ClaimHash = learnItemClaimDigest(bld, items[0])
	assessmentPath := learnAssessmentPath("architecture/x")
	assessment, err := os.ReadFile(assessmentPath)
	if err != nil {
		t.Fatal(err)
	}
	refrozen := strings.Replace(string(assessment), items[0].Hash, first.ClaimHash, 1)
	if err := os.WriteFile(assessmentPath, []byte(refrozen), 0o644); err != nil {
		t.Fatal(err)
	}
	concept.Attempts = []learn.Attempt{first, second}
	concept.LastAttempt = &concept.Attempts[1]
	learnAppendAttempt(concept, first)
	if reason := learnAttemptRevisionReason(concept, bld); reason != "" {
		t.Fatalf("reassessment should supersede stale evidence for that item: %s", reason)
	}
}

func learnItemClaimDigest(bld *contextwiki.Builder, item learnLessonItem) string {
	_, digest := learnItemClaimVerdict(bld, "architecture/x", item)
	return digest
}
