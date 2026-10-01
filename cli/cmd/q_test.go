package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const qFixture = `{
  "documents": [
    {"path": "a.md", "kind": "analysis", "n": 1},
    {"path": "b.md", "kind": "plan", "n": 2}
  ],
  "meta": {"count": 2, "ratio": 1.50}
}`

func TestQGetScalar(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)
	out := execute(t, qGetCmd, nil, "data.json", "meta.count")
	if strings.TrimSpace(string(out)) != "2" {
		t.Errorf("out = %q", out)
	}
}

func TestQGetRawJSONForNestedResult(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)
	out := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", "meta")))
	if !strings.HasPrefix(out, "{") || !json.Valid([]byte(out)) {
		t.Errorf("expected raw JSON object, got %q", out)
	}
}

func TestQGetQueriesProjectionsAndModifiers(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)
	cases := []struct {
		path string
		want string
	}{
		{"documents.#", "2"},
		{"documents.#.path", `["a.md","b.md"]`},
		{`documents.#(kind=="plan").path`, "b.md"},
		{`documents.#(kind=="plan")#.path`, `["b.md"]`},
		{"documents.0.path", "a.md"},
		{"@keys", `["documents","meta"]`},
	}
	for _, tc := range cases {
		out := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", tc.path)))
		if out != tc.want {
			t.Errorf("get %q = %q, want %q", tc.path, out, tc.want)
		}
	}
}

func TestQGetEnvelope(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)
	out := execute(t, qGetCmd, nil, "data.json", "meta.count", "--format", "json")
	for _, want := range []string{`"path": "meta.count"`, `"exists": true`, `"type": "Number"`, `"value": 2`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("envelope missing %s: %s", want, out)
		}
	}
	yamlOut := execute(t, qGetCmd, nil, "data.json", "meta.count", "--format", "yaml")
	if !strings.Contains(string(yamlOut), "path: meta.count") {
		t.Errorf("yaml envelope = %s", yamlOut)
	}
}

func TestQGetNullIsFound(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", `{"a": null}`)
	out := execute(t, qGetCmd, nil, "data.json", "a")
	if strings.TrimSpace(string(out)) != "null" {
		t.Errorf("out = %q", out)
	}
}

func TestQGetMissingPathExits1(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, qGetCmd, nil, "data.json", "meta.missing")
		return ""
	})
}

func TestQGetDefault(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)
	out := execute(t, qGetCmd, nil, "data.json", "meta.missing", "--default", "fallback")
	if strings.TrimSpace(string(out)) != "fallback" {
		t.Errorf("out = %q", out)
	}
}

func TestQGetFromStdin(t *testing.T) {
	runInTempDir(t)
	captureCmdIn(t, []byte(`{"a":{"b":7}}`))
	out := execute(t, qGetCmd, []byte(`{"a":{"b":7}}`), "-", "a.b")
	if strings.TrimSpace(string(out)) != "7" {
		t.Errorf("out = %q", out)
	}
}

func TestQGetMissingFileExits1(t *testing.T) {
	runInTempDir(t)
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, qGetCmd, nil, "nope.json", "a")
		return ""
	})
}

func TestQYAMLRejected(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.yaml", "a: 1\n")
	errs := captureCmdErr(t)
	shouldExitWithCode(t, 2, func() string {
		execute(t, qGetCmd, nil, "data.yaml", "a")
		return ""
	})
	if !strings.Contains(errs.String(), "sdt conv --in yaml --out json") {
		t.Errorf("stderr = %s", errs)
	}
}

func TestQMalformedJSONExits2(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "bad.json", "{not json")
	captureCmdErr(t)
	shouldExitWithCode(t, 2, func() string {
		execute(t, qGetCmd, nil, "bad.json", "a")
		return ""
	})
}

func TestQSetTypedValues(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", qFixture)

	execute(t, qSetCmd, nil, "data.json", "meta.count", "3")
	if got := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", "meta.count"))); got != "3" {
		t.Errorf("count = %q", got)
	}

	execute(t, qSetCmd, nil, "data.json", "meta.enabled", "true")
	if got := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", "meta.enabled"))); got != "true" {
		t.Errorf("enabled = %q", got)
	}

	execute(t, qSetCmd, nil, "data.json", "meta.tags", `["a","b"]`)
	if got := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", "meta.tags"))); got != `["a","b"]` {
		t.Errorf("tags = %q", got)
	}

	execute(t, qSetCmd, nil, "data.json", "meta.note", "hello world")
	data, err := os.ReadFile("data.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"note":"hello world"`) {
		t.Errorf("note not stored as a string:\n%s", data)
	}
}

func TestQSetPreservesUntouchedBytes(t *testing.T) {
	runInTempDir(t)
	content := "{\n  \"a\": 1,\n  \"b\": {\n    \"c\": 1.50\n  }\n}\n"
	writeJSON(t, "data.json", content)
	execute(t, qSetCmd, nil, "data.json", "a", "2")
	data, err := os.ReadFile("data.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"c": 1.50`) {
		t.Errorf("number literal rewritten:\n%s", data)
	}
	if !strings.Contains(string(data), "  \"a\": 2,") {
		t.Errorf("target not rewritten in place:\n%s", data)
	}
}

func TestQSetNewPath(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", `{"a":1}`)
	execute(t, qSetCmd, nil, "data.json", "b.c", "2")
	if got := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", "b.c"))); got != "2" {
		t.Errorf("b.c = %q", got)
	}
}

func TestQSetValueFromInput(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", `{"a":1}`)
	execute(t, qSetCmd, nil, "data.json", "a", "--input", "42")
	if got := strings.TrimSpace(string(execute(t, qGetCmd, nil, "data.json", "a"))); got != "42" {
		t.Errorf("a = %q", got)
	}
}

func TestQSetStdinPrintsResult(t *testing.T) {
	runInTempDir(t)
	captureCmdIn(t, []byte(`{"a":1}`))
	out := execute(t, qSetCmd, []byte(`{"a":1}`), "-", "a", "2")
	if strings.TrimSpace(string(out)) != `{"a":2}` {
		t.Errorf("out = %q", out)
	}
}

func TestQDelStdinPrintsResult(t *testing.T) {
	runInTempDir(t)
	captureCmdIn(t, []byte(`{"a":1,"b":2}`))
	out := execute(t, qDelCmd, []byte(`{"a":1,"b":2}`), "-", "a")
	if strings.TrimSpace(string(out)) != `{"b":2}` {
		t.Errorf("out = %q", out)
	}
}

func TestQDel(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", `{"a":1,"b":2}`)
	execute(t, qDelCmd, nil, "data.json", "a")
	data, err := os.ReadFile("data.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"a"`) || !strings.Contains(string(data), `"b":2`) {
		t.Errorf("del result:\n%s", data)
	}
}

func TestQDelMissingPathExits1(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", `{"a":1}`)
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, qDelCmd, nil, "data.json", "zzz")
		return ""
	})
}

func TestQDelArrayElement(t *testing.T) {
	runInTempDir(t)
	writeJSON(t, "data.json", `{"items":[{"id":1},{"id":2}]}`)
	execute(t, qDelCmd, nil, "data.json", "items.0")
	data, err := os.ReadFile("data.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `{"id":2}`) || strings.Contains(string(data), `{"id":1}`) {
		t.Errorf("array del result:\n%s", data)
	}
}
