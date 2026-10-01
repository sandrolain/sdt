package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeContextDoc(t *testing.T, rel, content string) {
	t.Helper()
	full := filepath.Join("context", rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const getDoc = "---\nkind: analysis\nuid: 01a0eba1-c6cd-7e9b-8754-a8020ce74450\ntitle: \"My Title\"\nobjective: structured-data-access\ntopics: [cli, docs]\nstatus: active\n---\n\n## Body\n"

func TestContextGetSingleKey(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)

	out := strings.TrimSpace(string(execute(t, contextGetCmd, nil, "analysis/x.md", "status")))
	if out != "active" {
		t.Errorf("status = %q", out)
	}
}

func TestContextGetOrderedKeys(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)

	out := strings.TrimSpace(string(execute(t, contextGetCmd, nil, "analysis/x.md", "objective", "status")))
	if out != "structured-data-access\nactive" {
		t.Errorf("out = %q", out)
	}
}

func TestContextGetRawBlock(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)

	out := string(execute(t, contextGetCmd, nil, "analysis/x.md"))
	if !strings.HasPrefix(out, "---\nkind: analysis\n") || !strings.HasSuffix(out, "---\n") {
		t.Errorf("raw block = %q", out)
	}
}

func TestContextGetMissingKeyExits1(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)
	errs := captureCmdErr(t)

	shouldExitWithCode(t, 1, func() string {
		out := execute(t, contextGetCmd, nil, "analysis/x.md", "nope")
		if strings.TrimSpace(string(out)) != "" {
			t.Errorf("missing key should print an empty line, got %q", out)
		}
		return ""
	})
	if !strings.Contains(errs.String(), "absent") {
		t.Errorf("stderr = %s", errs)
	}
}

func TestContextGetJSONEnvelope(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)
	captureCmdErr(t)

	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, contextGetCmd, nil, "analysis/x.md", "status", "nope", "--format", "json"))
		return ""
	})
	if !strings.Contains(out, `"status": "active"`) {
		t.Errorf("json = %s", out)
	}
	if !strings.Contains(out, `"nope": null`) {
		t.Errorf("missing key should be null in json = %s", out)
	}
}

func TestContextGetJSONAllPresent(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)

	out := execute(t, contextGetCmd, nil, "analysis/x.md", "status", "--format", "json")
	if !strings.Contains(string(out), `"status": "active"`) {
		t.Errorf("json = %s", out)
	}
}

func TestContextGetNestedKeyErrors(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", getDoc)
	captureCmdErr(t)

	shouldExitWithCode(t, 1, func() string {
		execute(t, contextGetCmd, nil, "analysis/x.md", "meta.owner")
		return ""
	})
}

func TestContextGetOutsideContextRejected(t *testing.T) {
	runInTempDir(t)
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextGetCmd, nil, "../outside.md", "kind")
		return ""
	})
}

func TestContextGetCRLFDoc(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\r\nkind: analysis\r\nstatus: active\r\n---\r\n\r\nbody\r\n")

	out := strings.TrimSpace(string(execute(t, contextGetCmd, nil, "analysis/x.md", "status")))
	if out != "active" {
		t.Errorf("crlf status = %q", out)
	}
}

func TestContextGetNoFrontmatter(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "no frontmatter\n")
	captureCmdErr(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, contextGetCmd, nil, "analysis/x.md")
		return ""
	})
}

func TestContextGetRawBlockCRLF(t *testing.T) {
	runInTempDir(t)
	writeContextDoc(t, "analysis/x.md", "---\r\nkind: analysis\r\nstatus: active\r\n---\r\n\r\nbody\r\n")

	out := string(execute(t, contextGetCmd, nil, "analysis/x.md"))
	if !strings.HasPrefix(out, "---\nkind: analysis\n") {
		t.Errorf("raw CRLF block = %q", out)
	}
}
