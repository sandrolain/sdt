package search

import (
	"os"
	"path/filepath"
	"testing"
)

// handleCorpus creates a corpus with one dated work file and one document
// addressed by a uid, plus a document that shares a term with neither.
func handleCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("context/analysis/20260911-084500-sample-pipeline-study.md", `---
kind: analysis
title: Sample pipeline study
summary: A study of the rollout
created: 2026-09-11
---

The rollout body mentions handleword-x1.
`)
	write("context/notes/wave-ledger.md", `---
kind: notes
uid: 01a12e77-0eb7-7d4b-ae9c-9943cad648b6
title: Wave ledger
summary: Counters per wave
created: 2026-09-12
---

Ledger body mentions handleword-x1 too.
`)
	write("context/wiki/unrelated.md", `---
kind: wiki
title: Unrelated
summary: No handle here
created: 2026-09-13
---

Nothing to resolve, but the ledger run is filed under
context/analysis/20260911-084500-sample-pipeline-study.md.
`)
	return root
}

func TestSearchDateTimeHandleRanksItsDocumentFirst(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	res, err := ix.Search("20260911-084500", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "context/analysis/20260911-084500-sample-pipeline-study.md"
	if len(res.Results) == 0 || res.Results[0].Path != want {
		t.Fatalf("handle not ranked first: %+v", res.Results)
	}
	if res.Results[0].Score != handleScore {
		t.Errorf("resolved score = %v, want %v", res.Results[0].Score, handleScore)
	}
	// The lexical hits the query shape would otherwise produce stay below it.
	if len(res.Results) < 2 {
		t.Fatalf("want lexical hits below the resolved handle, got %d results", len(res.Results))
	}
	if res.Results[0].Path == res.Results[1].Path {
		t.Error("the resolved document is duplicated in the result set")
	}
}

func TestSearchUIDHandleResolvesItsDocument(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	res, err := ix.Search("01a12e77-0eb7-7d4b-ae9c-9943cad648b6", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "context/notes/wave-ledger.md"
	if len(res.Results) == 0 || res.Results[0].Path != want {
		t.Fatalf("uid not resolved first: %+v", res.Results)
	}
}

func TestSearchUIDHandleIsCaseInsensitive(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	res, err := ix.Search("01A12E77-0EB7-7D4B-AE9C-9943CAD648B6", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) == 0 || res.Results[0].Path != "context/notes/wave-ledger.md" {
		t.Fatalf("uppercase uid not resolved: %+v", res.Results)
	}
}

func TestSearchNonHandleQueryFallsThroughToLexical(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	res, err := ix.Search("handleword-x1", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 2 {
		t.Fatalf("want the two lexical hits, got %d: %+v", len(res.Results), res.Results)
	}
	for _, r := range res.Results {
		if r.Score == handleScore {
			t.Errorf("%s must not carry a resolved-handle score", r.Path)
		}
	}
}

func TestSearchHandleRespectsFilter(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	res, err := ix.Search("20260911-084500", "notes", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if hasPath(res.Results, "context/analysis/20260911-084500-sample-pipeline-study.md") {
		t.Errorf("the resolved document ignores the kind filter: %+v", res.Results)
	}
	for _, r := range res.Results {
		if r.Score == handleScore {
			t.Errorf("a filtered-out handle must not resolve: %s", r.Path)
		}
	}
}

func TestSearchUnknownHandleFallsBackToSubstring(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	// A handle shape matching nothing must behave exactly as before: the
	// lexical path answers, and the substring fallback still fills a gap.
	res, err := ix.Search("20200101-000000", "", "", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 0 {
		t.Fatalf("want no resolved hit for an unknown handle: %+v", res.Results)
	}
	if !res.Partial {
		t.Error("an unmatched handle must keep the substring fallback's partial flag")
	}
}

func TestSearchHandleHonoursMax(t *testing.T) {
	ix := newIndex(t, handleCorpus(t))
	defer ix.Close()

	res, err := ix.Search("20260911-084500", "", "", "", "", "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("max not honoured: got %d results", len(res.Results))
	}
	if res.Results[0].Path != "context/analysis/20260911-084500-sample-pipeline-study.md" {
		t.Errorf("max dropped the resolved handle: %+v", res.Results)
	}
}

func TestClassifyHandleShapes(t *testing.T) {
	cases := map[string]handleShape{
		"20260911-084500":                      handleDateTime,
		"01a12e77-0eb7-7d4b-ae9c-9943cad648b6": handleUID,
		"01A12E77-0EB7-7D4B-AE9C-9943CAD648B6": handleUID,
		"20260911":                             handleNone, // date only
		"084500":                               handleNone, // time only
		"20260911-0845":                        handleNone, // short timestamp
		"pipeline":                             handleNone, // free text
		"01a12e77-0eb7-7d4b-ae9c":              handleNone, // truncated uid
	}
	for q, want := range cases {
		if got := classifyHandle(q); got != want {
			t.Errorf("classifyHandle(%q) = %v, want %v", q, got, want)
		}
	}
}
