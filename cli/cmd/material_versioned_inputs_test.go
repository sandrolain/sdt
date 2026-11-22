//go:build !wasm

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/metacomp"
)

const profileFixture = `schema_version: "1"
id: software-architect
revision: "1"
title: Software architects
prior_knowledge: [software design]
terminology_familiarity: []
register: Direct and technically precise
example_contexts: [product teams]
accessibility_needs: []
`

const briefFixture = `schema_version: "1"
id: software-architect/architecture/four-key-metrics.it
revision: "1"
title: Four key metrics for a delivery decision
profile:
  id: software-architect
  revision: "1"
outcome: Choose a reporting window and explain its effect on a decision.
content_scope: [four metrics and their reporting windows]
forms: [conceptual-explanation]
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
overrides: []
teaching_examples: []
`

func TestMaterialVersionedProfileAndBriefCommands(t *testing.T) {
	root := setupContextProject(t)
	profileOut := string(execute(t, materialProfileCreateCmd, []byte(profileFixture), "software-architect", "1"))
	if !strings.Contains(profileOut, "created recipient profile software-architect@1") {
		t.Fatalf("profile create output = %q", profileOut)
	}
	profilePath := filepath.Join(root, "context", "learning", "profiles", "software-architect", "1.yaml")
	if _, err := os.Stat(profilePath); err != nil {
		t.Fatalf("profile file missing: %v", err)
	}
	profileBefore, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	shouldExitWithCode(t, 1, func() string {
		_ = execute(t, materialProfileCreateCmd, []byte(strings.Replace(profileFixture, "Software architects", "Changed profile", 1)), "software-architect", "1")
		return ""
	})
	profileAfter, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(profileBefore, profileAfter) {
		t.Fatal("creating an existing profile revision changed its bytes")
	}
	if out := string(execute(t, materialProfileCheckCmd, nil, "software-architect", "1")); !strings.Contains(out, "valid recipient profile software-architect@1") {
		t.Fatalf("profile check output = %q", out)
	}
	briefOut := string(execute(t, materialBriefCreateCmd, []byte(briefFixture), "software-architect/architecture/four-key-metrics.it", "1"))
	if !strings.Contains(briefOut, "created material brief software-architect/architecture/four-key-metrics.it@1") {
		t.Fatalf("brief create output = %q", briefOut)
	}
	briefPath := filepath.Join(root, "context", "learning", "briefs", "software-architect", "architecture", "four-key-metrics.it", "1.yaml")
	if _, err := os.Stat(briefPath); err != nil {
		t.Fatalf("brief file missing: %v", err)
	}
	if out := string(execute(t, materialBriefCheckCmd, nil, "software-architect/architecture/four-key-metrics.it", "1")); !strings.Contains(out, "valid material brief software-architect/architecture/four-key-metrics.it@1") {
		t.Fatalf("brief check output = %q", out)
	}
}

func TestMaterialBriefBindPreservesReviewCompanion(t *testing.T) {
	root := setupContextProject(t)
	writeMaterialPage(t, "architecture/alpha")
	digests := liveDigests(t, "architecture/alpha")
	headerLine := materialHeaderLine("architecture/alpha", [][2]string{{"c-aaaaaaaa", digests["c-aaaaaaaa"]}})
	header, err := parseMaterialHeader(headerLine)
	if err != nil {
		t.Fatal(err)
	}
	guideRel := "context/renders/materials/architect/architecture/alpha.it.md"
	guide := filepath.Join(root, filepath.FromSlash(guideRel))
	writeCtxDoc(t, guideRel, headerLine+"\n\n# Alpha\n\nA guide.\n")
	guideBytes, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	doc := metacomp.New(filepath.ToSlash(guideRel), metacomp.Digest(guideBytes))
	doc["material"] = map[string]any{
		"concept": header.Concept, "syllabus": header.Syllabus, "lang": header.Lang,
		"classification": header.Classification,
		"artifacts":      map[string]any{"guide": map[string]any{"claims": headerClaims(header)}},
	}
	doc["review"] = map[string]any{"cycle": map[string]any{"critic": "critic note", "reader": "reader note"}}
	encoded, err := metacomp.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	writeCtxDoc(t, guideRel+metacomp.CompanionSuffix, string(encoded))
	briefID := "architect/architecture/alpha.it"
	fixture := strings.Replace(briefFixture,
		"id: software-architect/architecture/four-key-metrics.it", "id: "+briefID, 1)
	fixture = strings.Replace(fixture, "profile:\n  id: software-architect\n  revision: \"1\"\n", "", 1)
	fixture = strings.Replace(fixture, "title: Four key metrics for a delivery decision", "title: Alpha guide", 1)
	execute(t, materialBriefCreateCmd, []byte(fixture), briefID, "1")
	if out := string(execute(t, materialBriefBindCmd, nil, briefID, "1")); !strings.Contains(out, "bound material "+briefID+" to brief revision 1") {
		t.Fatalf("bind output = %q", out)
	}
	updated, _, err := metacomp.ReadTarget(root, filepath.ToSlash(guideRel))
	if err != nil {
		t.Fatal(err)
	}
	material := metadataMap(updated["material"])
	brief := metadataMap(material["brief"])
	if metadataString(brief["id"]) != briefID || metadataString(brief["revision"]) != "1" {
		t.Fatalf("brief binding = %#v", brief)
	}
	review := metadataMap(updated["review"])
	cycle := metadataMap(review["cycle"])
	if metadataString(cycle["critic"]) != "critic note" || metadataString(cycle["reader"]) != "reader note" {
		t.Fatalf("review record changed: %#v", review)
	}
}
