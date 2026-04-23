package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromptRender_inline(t *testing.T) {
	out := execute(t, promptRenderCmd, nil, "--template", "Hello, {{.name}}!", "--vars", `{"name":"World"}`)
	if strings.TrimSpace(string(out)) != "Hello, World!" {
		t.Errorf("unexpected render output: %q", out)
	}
}

func TestPromptRender_stdin(t *testing.T) {
	out := execute(t, promptRenderCmd, []byte("Hello, {{.name}}!"), "--vars", `{"name":"Alice"}`)
	if strings.TrimSpace(string(out)) != "Hello, Alice!" {
		t.Errorf("unexpected render output: %q", out)
	}
}

func TestPromptRender_showTokens_json(t *testing.T) {
	out := execute(t, promptRenderCmd, nil,
		"--template", "Hello, World!",
		"--show-tokens",
		"--format", "json",
	)
	var res map[string]interface{}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if _, ok := res["tokens"]; !ok {
		t.Error("expected 'tokens' field in JSON output")
	}
}

func TestPromptValidate_ok(t *testing.T) {
	out := execute(t, promptValidateCmd, nil,
		"--template", "Hello",
		"--max-tokens", "1000",
		"--format", "json",
	)
	var res map[string]interface{}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if res["valid"] != true {
		t.Errorf("expected valid=true, got %v", res["valid"])
	}
}

func TestPromptValidate_exceed(t *testing.T) {
	// 1 token max on a longer text should fail
	shouldExitWithCode(t, 1, func() string {
		execute(t, promptValidateCmd, nil,
			"--template", "This is a very long text that definitely exceeds one token.",
			"--max-tokens", "1",
		)
		return ""
	})
}

func TestPromptRender_missingTemplate(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, promptRenderCmd, nil)
		return ""
	})
}
