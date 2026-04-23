package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDotEnv(t *testing.T) {
	content := `
# comment
KEY1=value1
export KEY2=value2
KEY3="quoted value"
KEY4='single quoted'
KEY5=value with # inline comment
`
	entries, err := parseDotEnv(content)
	if err != nil {
		t.Fatal(err)
	}
	m := dotEnvToMap(entries)
	cases := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "quoted value",
		"KEY4": "single quoted",
		"KEY5": "value with",
	}
	for k, want := range cases {
		if got := m[k]; got != want {
			t.Errorf("key %s: got %q, want %q", k, got, want)
		}
	}
}

func TestEnvParseJSON(t *testing.T) {
	path := writeTempEnv(t, "FOO=bar\nBAZ=qux\n")
	out := execute(t, envParseCmd, nil, "--file", path)
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["FOO"] != "bar" {
		t.Errorf("expected bar, got %s", result["FOO"])
	}
}

func TestEnvParseShell(t *testing.T) {
	path := writeTempEnv(t, "FOO=bar\n")
	out := execute(t, envParseCmd, nil, "--file", path, "--format", "shell")
	if string(out) != `export FOO="bar"` {
		t.Errorf("unexpected shell output: %q", string(out))
	}
}

func TestEnvGet(t *testing.T) {
	path := writeTempEnv(t, "MY_KEY=hello\n")
	out := execute(t, envGetCmd, nil, "--file", path, "MY_KEY")
	if string(out) != "hello" {
		t.Errorf("expected hello, got %q", string(out))
	}
}

func TestEnvGetMissing(t *testing.T) {
	path := writeTempEnv(t, "OTHER=x\n")
	shouldExitWithCode(t, 1, func() string {
		execute(t, envGetCmd, nil, "--file", path, "MISSING")
		return ""
	})
}

func TestEnvSet(t *testing.T) {
	path := writeTempEnv(t, "A=1\nB=2\n")
	execute(t, envSetCmd, nil, "--file", path, "B", "99")
	out := execute(t, envGetCmd, nil, "--file", path, "B")
	if string(out) != "99" {
		t.Errorf("expected 99, got %q", string(out))
	}
}

func TestEnvSetNew(t *testing.T) {
	path := writeTempEnv(t, "A=1\n")
	execute(t, envSetCmd, nil, "--file", path, "NEW", "added")
	out := execute(t, envGetCmd, nil, "--file", path, "NEW")
	if string(out) != "added" {
		t.Errorf("expected added, got %q", string(out))
	}
}

func TestEnvMerge(t *testing.T) {
	p1 := writeTempEnv(t, "A=1\nB=2\n")
	p2 := writeTempEnv(t, "B=override\nC=3\n")
	out := execute(t, envMergeCmd, nil, "--files", p1+","+p2)
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["A"] != "1" {
		t.Errorf("A: expected 1, got %s", result["A"])
	}
	if result["B"] != "override" {
		t.Errorf("B: expected override, got %s", result["B"])
	}
	if result["C"] != "3" {
		t.Errorf("C: expected 3, got %s", result["C"])
	}
}

func TestEnvMergeToFile(t *testing.T) {
	p1 := writeTempEnv(t, "X=1\n")
	p2 := writeTempEnv(t, "Y=2\n")
	outFile := filepath.Join(t.TempDir(), "merged.env")
	execute(t, envMergeCmd, nil, "--files", p1+","+p2, "--output", outFile)
	entries, err := readDotEnvFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries in merged file, got %d", len(entries))
	}
}

func TestEnvParseStdin(t *testing.T) {
	in := []byte("STDIN_KEY=stdin_val\n")
	out := execute(t, envParseCmd, in)
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["STDIN_KEY"] != "stdin_val" {
		t.Errorf("expected stdin_val, got %s", result["STDIN_KEY"])
	}
}
