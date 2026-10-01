package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestBuildIndexJSONValid(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nstatus: active\ntitle: T\nobjective: obj-a\ntopics: [cli, docs]\ncategories: [new-feature]\nsummary: S\n---\nbody\n")
	writeContextDoc(t, "plan/y.md", "---\nkind: plan\nstatus: active\nobjective: obj-a\nsummary: P\n---\nbody\n")

	out, err := buildIndexJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.Valid(out) {
		t.Fatalf("invalid JSON:\n%s", out)
	}

	got := gjson.Get(out, `documents.#(objective=="obj-a")#.path`).Array()
	if len(got) != 2 {
		t.Errorf("objective query = %v, want 2 docs", got)
	}
	topics := gjson.Get(out, `documents.#(path=="analysis/x.md").topics`).Array()
	if len(topics) != 2 || topics[0].String() != "cli" {
		t.Errorf("topics = %v", topics)
	}
	cats := gjson.Get(out, `documents.#(path=="analysis/x.md").categories`).Array()
	if len(cats) != 1 || cats[0].String() != "new-feature" {
		t.Errorf("categories = %v", cats)
	}
}

func TestReindexWritesBothAndLeavesMarkdownIdentical(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nstatus: active\nsummary: S\n---\nbody\n")

	mdBefore, err := buildIndex()
	if err != nil {
		t.Fatal(err)
	}
	execute(t, contextReindexCmd, nil)

	md, err := os.ReadFile(sdtContextIndex)
	if err != nil {
		t.Fatal(err)
	}
	if string(md) != mdBefore {
		t.Error("context/index.md changed beyond the JSON addition")
	}
	if _, err := os.Stat(ctxIndexJSONPath); err != nil {
		t.Fatalf("context/index.json not written: %v", err)
	}
}

func TestReindexJSONIfChangedGuard(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nstatus: active\nsummary: S\n---\nbody\n")

	first := string(execute(t, contextReindexCmd, nil, "--format", "json"))
	if !strings.Contains(first, `"json": "created"`) {
		t.Errorf("first reindex = %s", first)
	}
	second := string(execute(t, contextReindexCmd, nil, "--format", "json"))
	if !strings.Contains(second, `"json": "unchanged"`) {
		t.Errorf("second reindex = %s", second)
	}
}

func TestReindexJSONSchema(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nstatus: draft\ntitle: T\nobjective: o\ntopics: [a]\ncategories: [research]\nsummary: S\n---\nbody\n")
	execute(t, contextReindexCmd, nil)

	data, err := os.ReadFile(ctxIndexJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &struct {
		Documents []json.RawMessage `json:"documents"`
	}{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"path": "analysis/x.md"`) {
		t.Errorf("schema missing path:\n%s", data)
	}
	for _, field := range []string{`"kind": "analysis"`, `"status": "draft"`, `"title": "T"`, `"objective": "o"`, `"summary": "S"`, `"topics"`, `"categories"`} {
		if !strings.Contains(string(data), field) {
			t.Errorf("schema missing %s", field)
		}
	}
	_ = doc
}

func TestReindexJSONSortsByPath(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/b.md", "---\nkind: analysis\nstatus: active\nsummary: B\n---\nbody\n")
	writeContextDoc(t, "analysis/a.md", "---\nkind: analysis\nstatus: active\nsummary: A\n---\nbody\n")

	out, err := buildIndexJSON()
	if err != nil {
		t.Fatal(err)
	}
	ia := strings.Index(out, `"analysis/a.md"`)
	ib := strings.Index(out, `"analysis/b.md"`)
	if ia < 0 || ib < 0 || ia > ib {
		t.Errorf("documents not sorted by path:\n%s", out)
	}
}

func TestReindexIncludesMultipleDocuments(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\nkind: analysis\nstatus: active\nsummary: S\n---\nbody\n")
	writeContextDoc(t, "plan/y.md", "---\nkind: plan\nstatus: active\nsummary: P\n---\nbody\n")

	out, err := buildIndexJSON()
	if err != nil {
		t.Fatal(err)
	}
	if n := gjson.Get(out, "documents.#").Int(); n != 2 {
		t.Errorf("documents = %d, want 2", n)
	}
}
