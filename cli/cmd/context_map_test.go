package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// writeDoc writes one fixture corpus document, creating its directory.
func writeDoc(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// fixtureCorpus builds a small corpus covering one document per kind the map
// describes, plus the three awkward facts the Gotchas section must surface.
func fixtureCorpus(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".sdt.yaml", []byte("project: fixture\ngroup: fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, "context/plan/20261001-000000-fixture-plan.md", `---
kind: plan
uid: 01a1fixtureplan0000000000000
analysis_id: 01a1fixtureanalysis000000000
title: Fixture plan
summary: A plan in the fixture corpus.
status: active
objective: obj-a
created: 2026-10-01T00:00:00Z
sources:
  - analysis/20261001-000000-fixture-live.md
links:
  - analysis/20261001-000000-fixture-live.md
---

## Objective

Fixture.
`)
	writeDoc(t, "context/analysis/20261001-000000-fixture-live.md", `---
kind: analysis
uid: 01a1fixtureanalysis000000000
title: Fixture analysis
summary: A live analysis in the fixture corpus.
status: active
objective: obj-a
created: 2026-10-01T00:00:00Z
links:
  - plan/20261001-000000-fixture-plan.md
---

## Context

Fixture.
`)
	writeDoc(t, "context/analysis/20261002-000000-fixture-postponed.md", `---
kind: analysis
title: Postponed fixture analysis
summary: A postponed analysis with a revival condition.
status: postponed
objective: obj-b
created: 2026-10-02T00:00:00Z
---

## Context

Fixture.

**Revival condition.** A real deck exists in the corpus.
`)
	writeDoc(t, "context/notes/20261001-000000-fixture-dead-end.md", `---
kind: notes
title: A rejected approach
summary: A dead end kept so nobody reopens it.
note_type: dead-end
objective: obj-a
created: 2026-10-01T00:00:00Z
---

## Context

Fixture.
`)
	writeDoc(t, "context/questions/20261001-000000-fixture-question.md", `---
kind: questions
title: An open question
summary: A question still awaiting an answer.
status: active
created: 2026-10-01T00:00:00Z
---

## Question

Fixture.
`)
	writeDoc(t, "context/architecture/fixture-architecture.md", `---
kind: architecture
title: Fixture architecture
summary: An architecture page.
status: current
created: 2026-10-03T00:00:00Z
---

## Overview

Fixture.
`)
}

func TestContextMapDocuments(t *testing.T) {
	fixtureCorpus(t)
	rep := ctxMapBuild("")

	if rep.Scope != ctxMapScope {
		t.Fatalf("scope = %q", rep.Scope)
	}
	if rep.Total != 6 {
		t.Fatalf("total = %d, want 6", rep.Total)
	}

	counts := map[string]int{}
	for _, row := range rep.Documents {
		counts[row.Kind] = row.Count
	}
	for kind, want := range map[string]int{
		ctxTypePlan: 1, ctxTypeAnalysis: 2, ctxTypeNotes: 1, ctxTypeQuestions: 1, ctxTypeArchitecture: 1,
	} {
		if counts[kind] != want {
			t.Fatalf("%s count = %d, want %d", kind, counts[kind], want)
		}
	}
	for _, row := range rep.Documents {
		if row.Kind != ctxTypeAnalysis {
			continue
		}
		if len(row.Statuses) != 2 {
			t.Fatalf("analysis statuses = %+v, want active and postponed", row.Statuses)
		}
		if row.Newest != "2026-10-02" {
			t.Fatalf("analysis newest = %q", row.Newest)
		}
	}
}

func TestContextMapFrontmatter(t *testing.T) {
	fixtureCorpus(t)
	rep := ctxMapBuild("")
	if len(rep.Frontmatter) == 0 {
		t.Fatal("frontmatter section is empty")
	}
	var kindKey *ctxMapKey
	for i, key := range rep.Frontmatter {
		if key.Key == "kind" {
			kindKey = &rep.Frontmatter[i]
			break
		}
	}
	if kindKey == nil || kindKey.Coverage != "6/6 (100%)" {
		t.Fatalf("kind row = %+v, want full coverage", kindKey)
	}
}

func TestContextMapObjectivesAndRelations(t *testing.T) {
	fixtureCorpus(t)
	rep := ctxMapBuild("")
	if len(rep.Objectives) != 2 || rep.Objectives[0].Objective != "obj-a" {
		t.Fatalf("objectives = %+v", rep.Objectives)
	}
	if rep.Objectives[0].Analyses != 1 || rep.Objectives[0].Plans != 1 || rep.Objectives[0].Active != 2 {
		t.Fatalf("obj-a roll-up = %+v", rep.Objectives[0])
	}
	if rep.Relations.WithSources != 1 || rep.Relations.WithLinks != 2 {
		t.Fatalf("relations = %+v", rep.Relations)
	}
	if rep.Relations.ParentEdges == 0 {
		t.Fatalf("the plan's analysis_id names the analysis: relations = %+v", rep.Relations)
	}
}

func TestContextMapGotchas(t *testing.T) {
	fixtureCorpus(t)
	names := map[string]ctxMapGotcha{}
	for _, gotcha := range ctxMapBuild("").Gotchas {
		names[gotcha.Name] = gotcha
	}
	for _, want := range []string{"dead-ends", "postponed", "open-questions"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("gotchas = %+v, want %s", names, want)
		}
	}
	if !strings.Contains(names["postponed"].Detail, "A real deck exists") {
		t.Fatalf("postponed detail must carry the revival condition: %q", names["postponed"].Detail)
	}
	if !strings.Contains(names["open-questions"].Detail, "fixture-question.md") {
		t.Fatalf("open questions must name the document: %q", names["open-questions"].Detail)
	}
}

func TestContextMapTrySection(t *testing.T) {
	fixtureCorpus(t)
	rep := ctxMapBuild("")
	if len(rep.Try) != 3 {
		t.Fatalf("try = %+v, want three queries", rep.Try)
	}
	for _, row := range rep.Try {
		if !strings.HasPrefix(row.Query, "sdt ") || row.Why == "" {
			t.Fatalf("try row = %+v", row)
		}
	}
}

func TestContextMapObjectiveFilter(t *testing.T) {
	fixtureCorpus(t)
	rep := ctxMapBuild("obj-b")
	if rep.Total != 1 {
		t.Fatalf("total = %d, want the one obj-b document", rep.Total)
	}
	if len(rep.Objectives) != 1 || rep.Objectives[0].Objective != "obj-b" {
		t.Fatalf("objectives = %+v", rep.Objectives)
	}
}

// TestContextMapCaveatInEveryFormat is the map's central promise: whatever the
// format, the scope statement comes with the data.
func TestContextMapCaveatInEveryFormat(t *testing.T) {
	fixtureCorpus(t)

	text, _, err := runRoot(t, "context", "map")
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if !strings.Contains(text, ctxMapScope) {
		t.Fatalf("text output lacks the caveat:\n%s", text)
	}

	jsonOut, _, err := runRoot(t, "context", "map", "--format", "json")
	if err != nil {
		t.Fatalf("map json: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &decoded); err != nil {
		t.Fatalf("map --format json is not JSON: %v", err)
	}
	if decoded["scope"] != ctxMapScope {
		t.Fatalf("json scope = %v", decoded["scope"])
	}

	yamlOut, _, err := runRoot(t, "context", "map", "--format", "yaml")
	if err != nil {
		t.Fatalf("map yaml: %v", err)
	}
	var fromYAML map[string]any
	if err := yaml.Unmarshal([]byte(yamlOut), &fromYAML); err != nil {
		t.Fatalf("map --format yaml is not YAML: %v", err)
	}
	if fromYAML["scope"] != ctxMapScope {
		t.Fatalf("yaml scope = %v", fromYAML["scope"])
	}
}

func TestContextMapEmptyCorpus(t *testing.T) {
	t.Chdir(t.TempDir())
	rep := ctxMapBuild("")
	if rep.Total != 0 {
		t.Fatalf("total = %d on an empty corpus", rep.Total)
	}
	text := ctxMapText(rep)
	if !strings.Contains(text, ctxMapScope) {
		t.Fatalf("an empty corpus must still state the scope:\n%s", text)
	}
	if !strings.Contains(text, "(no corpus documents)") {
		t.Fatalf("an empty corpus must say so:\n%s", text)
	}
}

// TestContextMapCountsAgreeWithStatus is the cross-check the plan asks for: a
// reader who compares the map with `context status` must see the same numbers.
// The questions row is excluded because it counts open questions, not documents,
// and so are the two synthetic rows (dead-ends, postponed) that the map reports
// under Gotchas.
func TestContextMapCountsAgreeWithStatus(t *testing.T) {
	t.Chdir(projectRoot(t))

	labelToKind := map[string]string{}
	for _, t2 := range contextTypeList {
		labelToKind[ctxKindLabel(t2)] = t2.kind
	}
	counts := map[string]int{}
	for _, row := range ctxMapBuild("").Documents {
		counts[row.Kind] = row.Count
	}
	compared := 0
	for _, row := range ctxStatusRows() {
		kind, ok := labelToKind[row.Type]
		if !ok || kind == ctxTypeQuestions {
			continue
		}
		compared++
		if counts[kind] != row.Count {
			t.Fatalf("%s: map %d, status %d", kind, counts[kind], row.Count)
		}
	}
	if compared < 10 {
		t.Fatalf("compared only %d rows; the cross-check is not exercising the corpus", compared)
	}
}

// TestContextMapGotchaCountsAgreeWithStatus pins the derived counts to the ones
// `context status` already prints, so the map cannot drift into a second truth.
func TestContextMapGotchaCountsAgreeWithStatus(t *testing.T) {
	t.Chdir(projectRoot(t))
	gotchas := map[string]int{}
	for _, gotcha := range ctxMapBuild("").Gotchas {
		gotchas[gotcha.Name] = gotcha.Count
	}
	if row, ok := ctxStatusDeadEndRow(); ok && gotchas["dead-ends"] != row.Count {
		t.Fatalf("dead-ends: map %d, status %d", gotchas["dead-ends"], row.Count)
	}
	if row, ok := ctxStatusPostponedRow(); ok && gotchas["postponed"] != row.Count {
		t.Fatalf("postponed: map %d, status %d", gotchas["postponed"], row.Count)
	}
	files, err := dirFiles(sdtQuestionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := ctxQuestionsUnresolved(files); gotchas["open-questions"] != want && want > 0 {
		t.Fatalf("open-questions: map %d, status %d", gotchas["open-questions"], want)
	}
}
