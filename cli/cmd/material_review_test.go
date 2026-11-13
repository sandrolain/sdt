//go:build !wasm

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a ledger fixture in the lane's own shape: two-space indent, cycle last but
// one, critic and reader only.
const materialReviewLedger = `{
  "concept": "architecture/alpha",
  "syllabus": "syllabi/software-architect",
  "lang": "it",
  "files": [
    "alpha.it.md",
    "alpha.it.slide.md"
  ],
  "lesson": "architecture/alpha",
  "claims": [
    {
      "id": "c-aaaaaaaa",
      "digest": "111111111111"
    }
  ],
  "written_at": "2026-11-10T08:00:00Z",
  "cycle": {
    "critic": "cold read, no defect found",
    "reader": "not run"
  },
  "notes": [],
  "binder": {"unbound": 0}
}`

func TestMaterialReviewRecordsVerdict(t *testing.T) {
	writeMaterialStatusFixture(t, "personal", materialReviewLedger)
	ledger := filepath.Join("context", "renders", "materials", "software-architect", "alpha.it.ledger.json")

	out := string(execute(t, materialReviewCmd, nil, "architecture/alpha",
		"--verdict", "teaches the four metrics from the deploy scenario; the exercise lands",
		"--by", "ses_review01", "--model", "opencode/big-pickle"))

	if !strings.Contains(out, "recorded reader verdict") {
		t.Fatalf("the review must report what it recorded, got %q", out)
	}
	var ledgerDoc map[string]any
	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if err := json.Unmarshal(raw, &ledgerDoc); err != nil {
		t.Fatalf("ledger still parses: %v", err)
	}
	cycle := ledgerDoc["cycle"].(map[string]any)
	if cycle["reader"] != "teaches the four metrics from the deploy scenario; the exercise lands" {
		t.Errorf("the verdict is the reader value, got %v", cycle["reader"])
	}
	if cycle["critic"] != "cold read, no defect found" {
		t.Errorf("the critic value is carried unchanged, got %v", cycle["critic"])
	}
	if cycle["reader_by"] != "ses_review01" || cycle["reader_model"] != "opencode/big-pickle" {
		t.Errorf("provenance is recorded, got %v", cycle)
	}
	if _, ok := cycle["reader_at"]; !ok {
		t.Errorf("the date is recorded, got %v", cycle)
	}
	// byte-surgical: the claims block and the binder stay exactly as they were.
	if !strings.Contains(string(raw), "\"written_at\": \"2026-11-10T08:00:00Z\"") {
		t.Errorf("the text outside the cycle object is untouched, got:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"unbound": 0`) {
		t.Errorf("the binder survives the splice, got:\n%s", raw)
	}
}

func TestMaterialReviewMakesStatusRecorded(t *testing.T) {
	writeMaterialStatusFixture(t, "personal", materialReviewLedger)
	execute(t, materialReviewCmd, nil, "architecture/alpha", "--verdict", "cold read, teaches")
	rows := runMaterialStatus(t)
	if rows[0].Review != "recorded" {
		t.Errorf("a recorded verdict is reported as recorded, got %+v", rows[0])
	}
	if !rows[0].Publishable {
		t.Errorf("fresh + recorded + declared classification is publishable, got %+v", rows[0])
	}
}

func TestMaterialReviewRefusesEmptyVerdict(t *testing.T) {
	writeMaterialStatusFixture(t, "personal", materialReviewLedger)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, materialReviewCmd, nil, "architecture/alpha", "--verdict", ""))
	})
}

func TestMaterialReviewRefusesAmbiguousConcept(t *testing.T) {
	writeMaterialStatusFixture(t, "personal", materialReviewLedger)
	// the matcher refuses ambiguity instead of taking the first sorted file.
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, materialReviewCmd, nil, "wiki/nosuchconcept", "--verdict", "x"))
	})
}
