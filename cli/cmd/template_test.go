package cmd

import (
	"testing"
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

func TestTemplateStdinTemplate(t *testing.T) {
	in := []byte("Value is {{.x}}")
	out := execute(t, templateCmd, in,
		"--data", `{"x":42}`,
	)
	if string(out) != "Value is 42" {
		t.Errorf("unexpected output: %q", string(out))
	}
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
