package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaCmd_all(t *testing.T) {
	out := execute(t, schemaCmd, nil, "--format", "json")
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	cmds, ok := result["commands"].([]interface{})
	if !ok || len(cmds) == 0 {
		t.Error("expected non-empty commands array")
	}
}

func TestSchemaCmd_singleCommand(t *testing.T) {
	out := execute(t, schemaCmd, nil, "--command", "tokens", "--format", "json")
	var result CommandSchema
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if !strings.Contains(result.Command, "tokens") {
		t.Errorf("expected command to contain 'tokens', got %q", result.Command)
	}
	if result.Flags == nil {
		t.Error("expected flags schema")
	}
}

func TestSchemaCmd_unknownCommand(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, schemaCmd, nil, "--command", "nonexistent-xyz")
		return ""
	})
}

func TestCobraFlagTypeToJSONType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"int", "number"},
		{"bool", "boolean"},
		{"string", "string"},
		{"stringArray", "array"},
	}
	for _, c := range cases {
		got := cobraFlagTypeToJSONType(c.in)
		if got != c.want {
			t.Errorf("cobraFlagTypeToJSONType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
