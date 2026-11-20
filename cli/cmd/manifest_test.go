package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestJSON(t *testing.T) {
	out := execute(t, manifestCmd, nil)
	if len(out) == 0 {
		t.Fatal("manifest produced no output")
	}
	var result struct {
		Commands []CommandManifest `json:"commands"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("manifest output is not valid JSON: %v", err)
	}
	if len(result.Commands) == 0 {
		t.Fatal("manifest 'commands' field is empty or missing")
	}

	got := make(map[string]string, len(result.Commands))
	for _, command := range result.Commands {
		got[command.Name] = command.Category
		if command.Category == "" {
			t.Errorf("root command %q has no discovery category", command.Name)
		}
	}
	for _, command := range rootCmd.Commands() {
		if command.Hidden {
			continue
		}
		category, ok := categoryForCommand(command.Name())
		if !ok {
			t.Errorf("root command %q has no category mapping", command.Name())
			continue
		}
		if got[command.Name()] != category.title {
			t.Errorf("manifest category for %q = %q, want %q", command.Name(), got[command.Name()], category.title)
		}
	}
	for _, removed := range []string{"nslookup", "prompt"} {
		if _, ok := got[removed]; ok {
			t.Errorf("removed command %q remains in the manifest", removed)
		}
	}
	if _, ok := got["lookup"]; ok {
		t.Error("removed lookup alias remains in the manifest")
	}
	if _, ok := got["look"]; ok {
		t.Error("removed look alias remains in the manifest")
	}
	for _, retained := range []string{"dns", "template", "tokens", "truncate", "ipinfo", "json", "q", "conv"} {
		if _, ok := got[retained]; !ok {
			t.Errorf("retained command %q is missing from the manifest", retained)
		}
	}
}

func TestRootHelpCategories(t *testing.T) {
	out, _, err := runRoot(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{
		"Project / agents",
		"Knowledge / learning",
		"Files / data",
		"Development / maintenance",
		"SDT diagnostics",
	} {
		if !strings.Contains(out, title) {
			t.Errorf("root help missing category %q:\n%s", title, out)
		}
	}
}

func TestManifestYAML(t *testing.T) {
	out := execute(t, manifestCmd, nil, "--format", "yaml")
	if len(out) == 0 {
		t.Fatal("manifest yaml produced no output")
	}
	if !strings.Contains(string(out), "commands:") {
		t.Fatal("manifest yaml missing 'commands:' key")
	}
}
