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
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("manifest output is not valid JSON: %v", err)
	}
	cmds, ok := result["commands"].([]interface{})
	if !ok || len(cmds) == 0 {
		t.Fatal("manifest 'commands' field is empty or missing")
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
