package cmd

import (
	"encoding/json"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	in := []byte("Visit https://example.com and http://foo.bar/path?x=1")
	out := execute(t, extractCmd, in, "--type", "urls")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 URLs, got %d: %v", len(result), result)
	}
}

func TestExtractEmails(t *testing.T) {
	in := []byte("Contact alice@example.com or bob@foo.org for info.")
	out := execute(t, extractCmd, in, "--type", "emails")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 emails, got %d: %v", len(result), result)
	}
}

func TestExtractIPs(t *testing.T) {
	in := []byte("Server at 192.168.1.1 and backup at 10.0.0.254.")
	out := execute(t, extractCmd, in, "--type", "ips")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 IPs, got %d: %v", len(result), result)
	}
}

func TestExtractDates(t *testing.T) {
	in := []byte("Created on 2024-01-15, reviewed on January 3, 2025.")
	out := execute(t, extractCmd, in, "--type", "dates")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Errorf("expected dates, got none")
	}
}

func TestExtractNoType(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, extractCmd, nil)
		return ""
	})
}

func TestExtractUnknownType(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, extractCmd, nil, "--type", "unknown")
		return ""
	})
}

func TestExtractEmpty(t *testing.T) {
	in := []byte("no urls here")
	out := execute(t, extractCmd, in, "--type", "urls")
	var result []string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestExtractJSONBlocksReturnsCompleteOuterValues(t *testing.T) {
	in := []byte(`before {"outer":{"inner":[1,{"ok":true}]}} after [1,[2,3]]`)
	matches, err := extractJSONBlocks(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`{"outer":{"inner":[1,{"ok":true}]}}`, `[1,[2,3]]`}
	if len(matches) != len(want) {
		t.Fatalf("expected %d outer JSON values, got %#v", len(want), matches)
	}
	for i := range want {
		if matches[i] != want[i] {
			t.Errorf("match %d = %q, want %q", i, matches[i], want[i])
		}
	}
}

func TestExtractJSONBlocksRejectsMalformedValue(t *testing.T) {
	if _, err := extractJSONBlocks([]byte(`prefix {"broken": } suffix`)); err == nil {
		t.Fatal("expected malformed JSON to return an error")
	}
}

func TestExtractCommandRejectsMalformedJSON(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, extractCmd, []byte(`{"broken": }`), "--type", "json-blocks"))
	})
}

func TestExtractJSONBlocksNoMatchIsEmpty(t *testing.T) {
	matches, err := extractJSONBlocks([]byte("no JSON values here"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %#v", matches)
	}
}

func TestExtractCodeBlocksReturnsCompleteFencedBodies(t *testing.T) {
	in := []byte("before `inline`\n~~~go title\nfunc nested() {\n\treturn `value`\n}\n~~~~\nafter")
	out := execute(t, extractCmd, in, "--type", "code-blocks")
	var matches []string
	if err := json.Unmarshal(out, &matches); err != nil {
		t.Fatal(err)
	}
	want := "func nested() {\n\treturn `value`\n}\n"
	if len(matches) != 1 || matches[0] != want {
		t.Fatalf("matches = %#v, want one body %q", matches, want)
	}
}

func TestExtractCodeBlocksIgnoresUnterminatedFence(t *testing.T) {
	matches, err := extractCodeBlocks([]byte("```go\nunfinished"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected unterminated fence to be ignored, got %#v", matches)
	}
}
