package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func resetMemoryDB(t *testing.T) {
	t.Helper()
	if memoryDB != nil {
		memoryDB.Close()
		memoryDB = nil
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
}

func TestMemorySetGet(t *testing.T) {
	resetMemoryDB(t)
	if err := memorySet("testproj", "testgroup", "mykey", "myval", "a,b"); err != nil {
		t.Fatal(err)
	}
	e, err := memoryGet("testproj", "mykey")
	if err != nil {
		t.Fatal(err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Value != "myval" {
		t.Errorf("expected myval, got %s", e.Value)
	}
	if e.Tags != "a,b" {
		t.Errorf("expected tags a,b, got %s", e.Tags)
	}
}

func TestMemoryList(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "grp", "k1", "v1", "")
	_ = memorySet("proj1", "grp", "k2", "v2", "")
	entries, err := memoryList("proj1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestMemorySearch(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "", "arch", "Using PostgreSQL for relational data", "database,architecture")
	_ = memorySet("proj1", "", "cache", "Using Redis for caching", "cache")
	entries, err := memorySearch("PostgreSQL", "proj1", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected FTS results, got none")
	}
	if entries[0].Key != "arch" {
		t.Errorf("expected arch, got %s", entries[0].Key)
	}
}

func TestMemoryDelete(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "", "k1", "v1", "")
	if err := memoryDelete("proj1", "k1"); err != nil {
		t.Fatal(err)
	}
	e, _ := memoryGet("proj1", "k1")
	if e != nil {
		t.Error("expected entry to be deleted")
	}
}

func TestMemoryDeleteAll(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "", "k1", "v1", "")
	_ = memorySet("proj1", "", "k2", "v2", "")
	if err := memoryDeleteAll("proj1"); err != nil {
		t.Fatal(err)
	}
	entries, _ := memoryList("proj1", "")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after deleteAll, got %d", len(entries))
	}
}

func TestMemoryProjects(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("alpha", "", "k", "v", "")
	_ = memorySet("beta", "", "k", "v", "")
	projects, err := memoryProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) < 2 {
		t.Errorf("expected at least 2 projects, got %d", len(projects))
	}
}

func TestMemoryGroups(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "g1", "k", "v", "")
	_ = memorySet("proj1", "g2", "k2", "v2", "")
	groups, err := memoryGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) < 2 {
		t.Errorf("expected at least 2 groups, got %d", len(groups))
	}
}

func TestMemoryExportImport(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "g", "k1", "v1", "tag1")
	_ = memorySet("proj1", "g", "k2", "v2", "tag2")

	exported, err := memoryExport("proj1")
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) != 2 {
		t.Fatalf("expected 2 exported entries, got %d", len(exported))
	}

	// Reset DB and import
	resetMemoryDB(t)
	if err := memoryImport(exported); err != nil {
		t.Fatal(err)
	}
	list, _ := memoryList("proj1", "")
	if len(list) != 2 {
		t.Errorf("expected 2 re-imported entries, got %d", len(list))
	}
}

func TestMemoryInitCmd(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	os.Chdir(dir)        //nolint:errcheck

	out := execute(t, memoryInitCmd, nil, "--project", "myproj", "--group", "mygrp")
	if string(out) == "" {
		t.Fatal("expected output from memory init")
	}
	if _, err := os.Stat(".sdt.yaml"); err != nil {
		t.Fatal(".sdt.yaml not created")
	}
}

func TestNormalizeTags(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a, b ,c", "a,b,c"},
		{"", ""},
		{" x ", "x"},
	}
	for _, c := range cases {
		got := normalizeTags(c.in)
		if got != c.want {
			t.Errorf("normalizeTags(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMemoryExportJSON(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("proj1", "", "k1", "v1", "")
	out := execute(t, memoryExportCmd, nil, "--project", "proj1", "--format", "json")
	var entries []MemoryEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("export JSON invalid: %v", err)
	}
}

func TestOutputEntries_json(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("p", "g", "k", "v", "tag1")
	out := execute(t, memoryListCmd, nil, "--project", "p", "--format", "json")
	var entries []MemoryEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("expected valid JSON from memory list: %v\nout: %s", err, out)
	}
}

func TestOutputEntries_text(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("p", "g", "mykey", "myvalue", "tagA")
	out := execute(t, memoryListCmd, nil, "--project", "p")
	s := string(out)
	if !strings.Contains(s, "mykey") {
		t.Errorf("expected key in text output, got: %s", s)
	}
}

func TestOutputEntries_yaml(t *testing.T) {
	resetMemoryDB(t)
	_ = memorySet("p", "", "k", "v", "")
	out := execute(t, memoryListCmd, nil, "--project", "p", "--format", "yaml")
	if !strings.Contains(string(out), "key") {
		t.Errorf("expected 'key' in yaml output, got: %s", out)
	}
}
