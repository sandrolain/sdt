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

// TestSchemaCmd_positionalCommand proves the natural spelling works: the command
// named as arguments answers for that command, and naming it twice is refused
// rather than silently resolved one way.
func TestSchemaCmd_positionalCommand(t *testing.T) {
	out := execute(t, schemaCmd, nil, "tokens", "--format", "json")
	var result CommandSchema
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if !strings.Contains(result.Command, "tokens") {
		t.Errorf("positional command resolved to %q", result.Command)
	}

	multi := execute(t, schemaCmd, nil, "jwt", "parse", "--format", "json")
	if err := json.Unmarshal(multi, &result); err != nil {
		t.Fatalf("invalid JSON for a two-word command: %v\nout: %s", err, multi)
	}
	if result.Command != "jwt parse" {
		t.Errorf("two-word positional resolved to %q, want \"jwt parse\"", result.Command)
	}

	shouldExitWithCode(t, 1, func() string {
		execute(t, schemaCmd, nil, "tokens", "--command", "tokens")
		return ""
	})
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
