package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestTemplateFlag(t *testing.T) {
	out := execute(t, templateCmd, nil,
		"--tmpl", "Hello, {{.name}}!",
		"--data", `{"name":"World"}`,
	)
	if string(out) != "Hello, World!" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestTemplateYAMLData(t *testing.T) {
	out := execute(t, templateCmd, nil,
		"--tmpl", "{{.greeting}} {{.target}}",
		"--data", "greeting: Hi\ntarget: Alice",
	)
	if string(out) != "Hi Alice" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestTemplateNoData(t *testing.T) {
	out := execute(t, templateCmd, nil,
		"--tmpl", "static text",
	)
	if string(out) != "static text" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestTemplateValidateJSONReportsEstimate(t *testing.T) {
	out := execute(t, templateValidateCmd, nil, "--tmpl", "Hello, {{.name}}", "--data", `{"name":"Ada"}`, "--max-tokens", "20", "--format", "json")
	var result TemplateValidationResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !result.Valid || result.EstimatedTokens == 0 || result.Model != defaultModel || result.MaxTokens != 20 {
		t.Fatalf("unexpected validation result: %+v", result)
	}
	if !strings.Contains(result.Note, "not provider-exact") {
		t.Fatalf("note does not identify the estimate: %q", result.Note)
	}
}

func TestTemplateValidateYAMLReportsEstimate(t *testing.T) {
	out := execute(t, templateValidateCmd, nil, "--tmpl", "{{.name}}", "--data", `{"name":"Ada"}`, "--max-tokens", "20", "--format", "yaml")
	var result TemplateValidationResult
	if err := yaml.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	if !result.Valid || result.EstimatedTokens == 0 || result.MaxTokens != 20 {
		t.Fatalf("unexpected validation result: %+v", result)
	}
}

func TestTemplateValidateFailsWhenEstimateExceedsLimit(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, templateValidateCmd, nil, "--tmpl", strings.Repeat("word ", 100), "--max-tokens", "1"))
	})
}

func TestTemplateStdinTemplate(t *testing.T) {
	in := []byte("Value is {{.x}}")
	out := execute(t, templateCmd, in,
		"--data", `{"x":42}`,
	)
	if string(out) != "Value is 42" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestTemplateFileInputs(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "greeting.tmpl")
	dataPath := filepath.Join(dir, "vars.yaml")
	if err := os.WriteFile(templatePath, []byte("Hello, {{.name}}!"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dataPath, []byte("name: Ada\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := execute(t, templateCmd, nil, "--template-file", templatePath, "--file-data", dataPath)
	if string(out) != "Hello, Ada!" {
		t.Fatalf("output = %q, want Hello, Ada!", out)
	}
}

func TestTemplateGenericFileInput(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "greeting.tmpl")
	if err := os.WriteFile(templatePath, []byte("Hello, {{.name}}!"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := execute(t, templateCmd, nil, "--file", templatePath, "--data", `{"name":"Ada"}`)
	if string(out) != "Hello, Ada!" {
		t.Fatalf("output = %q, want Hello, Ada!", out)
	}
}

func TestTemplateGlobalInputModes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "input", args: []string{"--input", "Hello, {{.name}}!"}, want: "Hello, Ada!"},
		{name: "base64", args: []string{"--inb64", "SGk="}, want: "Hi"},
		{name: "positional", args: []string{"Hello, {{.name}}!"}, want: "Hello, Ada!"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(test.args, "--data", `{"name":"Ada"}`)
			out := execute(t, templateCmd, nil, args...)
			if string(out) != test.want {
				t.Fatalf("output = %q, want %q", out, test.want)
			}
		})
	}
}

func TestTemplateRejectsMultipleTemplateSources(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, templateCmd, nil, "--tmpl", "inline", "--input", "also inline")
		return ""
	})
}

func TestTemplateRejectsStdinAndExplicitTemplate(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, templateCmd, []byte("from stdin"), "--tmpl", "inline")
		return ""
	})
}

func TestTemplateRejectsMultipleVariableSources(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "vars.json")
	if err := os.WriteFile(dataPath, []byte(`{"name":"Ada"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	shouldExitWithCode(t, 1, func() string {
		execute(t, templateCmd, nil, "--tmpl", "{{.name}}", "--data", `{"name":"Ada"}`, "--file-data", dataPath)
		return ""
	})
}

func TestTemplateRejectsEmptyDataFilePath(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, templateCmd, nil, "--tmpl", "static", "--file-data", "")
		return ""
	})
}

func TestTemplateInvalidData(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, templateCmd, nil,
			"--tmpl", "{{.x}}",
			"--data", "not json or yaml: {{{",
		)
		return ""
	})
}

func TestTemplateNoInput(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, templateCmd, nil)
		return ""
	})
}

func TestParseDataInput(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		wantKey string
		wantVal interface{}
	}{
		{"empty", "", false, "", nil},
		{"json", `{"k":"v"}`, false, "k", "v"},
		{"yaml", "k: v", false, "k", "v"},
		{"invalid", "not: {{{ invalid", true, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := parseDataInput(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantKey != "" {
				if d[tc.wantKey] != tc.wantVal {
					t.Errorf("got %v, want %v", d[tc.wantKey], tc.wantVal)
				}
			}
		})
	}
}
