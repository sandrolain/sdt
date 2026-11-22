package materialbrief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validBrief = `schema_version: "1"
id: software-architect/architecture/four-key-metrics.it
revision: "1"
title: Four key metrics for a delivery decision
profile:
  id: software-architect
  revision: "2"
outcome: Choose a reporting window and explain what action it supports.
content_scope:
  - Four key metrics and where they are measured
forms:
  - conceptual-explanation
  - decision-case
length:
  min_words: 500
  max_words: 900
sources:
  - kind: claim
    ref: architecture/four-key-metrics#c-1
    version: 6e03516d0162
  - kind: objective
    ref: architecture/four-key-metrics#objective-1
    version: sha256:abc
  - kind: case
    ref: practice/delivery-window#case-1
    version: sha256:def
overrides: []
teaching_examples:
  - description: A deployment team with a changed reporting window.
    faithful_to: The source's comparison of short and long windows.
`

func TestDecodeBriefBindsProfileSourcesAndInstructionalForm(t *testing.T) {
	brief, err := Decode([]byte(validBrief), "software-architect/architecture/four-key-metrics.it", "1")
	if err != nil {
		t.Fatal(err)
	}
	if brief.Profile == nil || brief.Profile.ID != "software-architect" || brief.Profile.Revision != "2" {
		t.Fatalf("profile reference = %+v", brief.Profile)
	}
	if len(brief.Sources) != 3 || len(brief.Forms) != 2 {
		t.Fatalf("brief = %+v", brief)
	}
}

func TestDecodeBriefRejectsConflictingSourceVersionsAndUnknownFields(t *testing.T) {
	conflicting := strings.Replace(validBrief, "overrides: []", `  - kind: claim
    ref: architecture/four-key-metrics#c-1
    version: changed
overrides: []`, 1)
	if _, err := Decode([]byte(conflicting), "software-architect/architecture/four-key-metrics.it", "1"); err == nil || !strings.Contains(err.Error(), "conflicting versions") {
		t.Fatalf("conflicting source error = %v", err)
	}
	if _, err := Decode([]byte(validBrief+"learner_id: alice\n"), "software-architect/architecture/four-key-metrics.it", "1"); err == nil {
		t.Fatal("unknown learner field was accepted")
	}
}

func TestBriefValidateRejectsInvalidFields(t *testing.T) {
	valid, err := Decode([]byte(validBrief), "", "")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(*Brief)
		want string
	}{
		{"schema", func(b *Brief) { b.SchemaVersion = "2" }, "unsupported"},
		{"material id", func(b *Brief) { b.ID = "../outside" }, "invalid"},
		{"revision", func(b *Brief) { b.Revision = "0" }, "positive integer"},
		{"expected id", func(*Brief) {}, "does not match requested id"},
		{"expected revision", func(*Brief) {}, "does not match requested revision"},
		{"required text", func(b *Brief) { b.Title = " " }, "title and outcome"},
		{"required lists", func(b *Brief) { b.Forms = nil }, "requires content_scope"},
		{"blank scope", func(b *Brief) { b.ContentScope = []string{" "} }, "must not be blank"},
		{"unsupported form", func(b *Brief) { b.Forms = []string{"essay"} }, "unsupported instructional form"},
		{"duplicate form", func(b *Brief) { b.Forms = []string{"decision-case", "decision-case"} }, "duplicate instructional form"},
		{"length", func(b *Brief) { b.Length.MaxWords = 0 }, "brief length"},
		{"source kind", func(b *Brief) { b.Sources[0].Kind = "learner" }, "unsupported kind"},
		{"source identity", func(b *Brief) { b.Sources[0].Version = " " }, "requires ref and version"},
		{"profile ref", func(b *Brief) { b.Profile.Revision = "0" }, "valid profile id and positive revision"},
		{"override", func(b *Brief) { b.Overrides = []Override{{Field: "register"}} }, "requires field, value, and reason"},
		{"teaching example", func(b *Brief) { b.Examples = []TeachingCase{{Description: "example"}} }, "must state description and faithful_to"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brief := valid
			brief.Sources = append([]Source(nil), valid.Sources...)
			if valid.Profile != nil {
				profile := *valid.Profile
				brief.Profile = &profile
			}
			tt.edit(&brief)
			expectedID, expectedRevision := "", ""
			if tt.name == "expected id" {
				expectedID = "different/material"
			}
			if tt.name == "expected revision" {
				expectedRevision = "2"
			}
			if err := brief.Validate(expectedID, expectedRevision); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want text %q", err, tt.want)
			}
		})
	}
}

func TestEncodeAndReadBrief(t *testing.T) {
	brief, err := Decode([]byte(validBrief), "", "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(brief)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(encoded, brief.ID, brief.Revision); err != nil {
		t.Fatalf("encoded brief does not decode: %v", err)
	}
	if _, err := Encode(Brief{}); err == nil {
		t.Fatal("Encode accepted an invalid brief")
	}
	root := t.TempDir()
	if _, err := Read(root, "../outside", "1"); err == nil {
		t.Fatal("Read accepted a path traversal identity")
	}
	if _, err := Read(root, brief.ID, brief.Revision); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Read missing brief error = %v", err)
	}
	path := filepath.Join(root, "context", "learning", "briefs", filepath.FromSlash(brief.ID), brief.Revision+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Read(root, brief.ID, brief.Revision)
	if err != nil || loaded.ID != brief.ID {
		t.Fatalf("Read() = %+v, %v", loaded, err)
	}
}

func TestValidMaterialID(t *testing.T) {
	for _, value := range []string{"software-architect/topic/four-key-metrics.it", "a.b/c-d"} {
		if !ValidMaterialID(value) {
			t.Errorf("ValidMaterialID(%q) = false", value)
		}
	}
	for _, value := range []string{"", "/absolute", "../outside", "a//b", "a\\b", "a/Upper"} {
		if ValidMaterialID(value) {
			t.Errorf("ValidMaterialID(%q) = true", value)
		}
	}
}
