package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestContextLintHintByClass(t *testing.T) {
	hints := map[string]string{
		"broken link [[missing]]":               "fix the [[target]]",
		"broken source reference: refs/x.md":    "fix the `sources` frontmatter reference",
		"frontmatter missing `kind`":            "add `kind: <type>`",
		"frontmatter missing mandatory `title`": "add the missing mandatory frontmatter key",
		"missing YAML frontmatter":              "start the file with a YAML",
	}
	for msg, wantPrefix := range hints {
		hint := ctxLintHint(msg)
		if hint == "" || !strings.HasPrefix(hint, wantPrefix) {
			t.Errorf("ctxLintHint(%q) = %q, want prefix %q", msg, hint, wantPrefix)
		}
	}
}
func TestContextLintHintTextJSONYAML(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/plan/badlink.md", "---\nkind: plan\nsummary: x\n---\n[[missing-file]]\n")
	idx := "---\nkind: index\nsummary: i\n---\n"
	if err := os.WriteFile("context/index.md", []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}

	text := execute(t, contextLintCmd, nil)
	if !strings.Contains(string(text), "hint: fix the [[target]]") {
		t.Errorf("expected hint in text output:\n%s", text)
	}

	jsonOut := execute(t, contextLintCmd, nil, "--format", "json")
	var issues []ctxLintIssue
	if err := json.Unmarshal(jsonOut, &issues); err != nil {
		t.Fatalf("invalid lint JSON: %v", err)
	}
	found := false
	for _, it := range issues {
		if strings.Contains(it.Message, "broken link") {
			found = true
			if !strings.Contains(it.Hint, "fix the [[target]]") {
				t.Errorf("expected hint on JSON issue, got %q", it.Hint)
			}
		}
	}
	if !found {
		t.Error("expected a broken-link issue in JSON output")
	}

	yamlOut := execute(t, contextLintCmd, nil, "--format", "yaml")
	if !strings.Contains(string(yamlOut), "fix the [[target]]") {
		t.Errorf("expected hint in YAML output:\n%s", yamlOut)
	}
}
