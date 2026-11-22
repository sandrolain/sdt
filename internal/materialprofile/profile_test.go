package materialprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validProfile = `schema_version: "1"
id: software-architect
revision: "1"
title: Software architects
prior_knowledge:
  - Basic software design
terminology_familiarity: []
register: Direct and technically precise
example_contexts:
  - Service-oriented product teams
accessibility_needs: []
`

func TestDecodeProfileBindsIdentityAndRequiresExplicitAudienceDimensions(t *testing.T) {
	profile, err := Decode([]byte(validProfile), "software-architect", "1")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != "software-architect" || profile.Revision != "1" {
		t.Fatalf("profile identity = %s@%s", profile.ID, profile.Revision)
	}

	if _, err := Decode([]byte(validProfile), "software-architect", "2"); err == nil || !strings.Contains(err.Error(), "does not match requested revision") {
		t.Fatalf("revision mismatch error = %v", err)
	}
	if _, err := Decode([]byte(strings.Replace(validProfile, "accessibility_needs: []\n", "", 1)), "software-architect", "1"); err == nil || !strings.Contains(err.Error(), "must explicitly declare") {
		t.Fatalf("missing dimension error = %v", err)
	}
	if _, err := Decode([]byte(validProfile+"learner_id: alice\n"), "software-architect", "1"); err == nil {
		t.Fatal("unknown learner field was accepted")
	}
}

func TestReadLoadsOnlyRequestedVersionUnderProfilesRoot(t *testing.T) {
	root := t.TempDir()
	profilePath := filepath.Join(root, "context", "learning", "profiles", "software-architect", "1.yaml")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte(validProfile), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := Read(root, "software-architect", "1")
	if err != nil || profile.Title != "Software architects" {
		t.Fatalf("Read() = %+v, %v", profile, err)
	}
	if _, err := Read(root, "software-architect", "2"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing revision error = %v", err)
	}
	if _, err := Read(root, "../outside", "1"); err == nil {
		t.Fatal("path traversal profile id was accepted")
	}
}

func TestEncodeAndValidateProfileFailures(t *testing.T) {
	profile, err := Decode([]byte(validProfile), "", "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(encoded, profile.ID, profile.Revision); err != nil {
		t.Fatalf("encoded profile does not decode: %v", err)
	}
	if _, err := Encode(Profile{}); err == nil {
		t.Fatal("Encode accepted an invalid profile")
	}
	tests := []struct {
		name string
		edit func(*Profile)
		want string
	}{
		{"schema", func(p *Profile) { p.SchemaVersion = "2" }, "unsupported"},
		{"id", func(p *Profile) { p.ID = "Not Valid" }, "lowercase kebab-case"},
		{"revision", func(p *Profile) { p.Revision = "0" }, "positive integer"},
		{"expected id", func(*Profile) {}, "does not match requested id"},
		{"expected revision", func(*Profile) {}, "does not match requested revision"},
		{"title", func(p *Profile) { p.Title = " " }, "title is required"},
		{"blank list value", func(p *Profile) { p.ExampleContexts = []string{" "} }, "must not be blank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := profile
			tt.edit(&candidate)
			expectedID, expectedRevision := "", ""
			if tt.name == "expected id" {
				expectedID = "different-profile"
			}
			if tt.name == "expected revision" {
				expectedRevision = "2"
			}
			if err := candidate.Validate(expectedID, expectedRevision); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want text %q", err, tt.want)
			}
		})
	}
}
