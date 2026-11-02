package cmd

import (
	"strings"
	"testing"
)

// TestLintSourceSheet pins the three shaped keys of a source sheet and the one
// advisory: a good sheet is silent, each defect produces exactly its line, and
// the check never fires outside `kind: source`.
func TestLintSourceSheet(t *testing.T) {
	sheet := func(kind string, fields map[string]string) string {
		var b strings.Builder
		b.WriteString("---\nkind: " + kind + "\ntitle: vectorize-io/hindsight\n")
		for k, v := range fields {
			b.WriteString(k + ": " + v + "\n")
		}
		b.WriteString("---\n\n## Why it matters\n")
		return b.String()
	}
	good := map[string]string{
		"url":      "https://github.com/vectorize-io/hindsight",
		"sha256":   strings.Repeat("a", 64),
		"license":  "Apache-2.0",
		"status":   "gone",
		"restore":  "git clone https://github.com/vectorize-io/hindsight && git checkout ec39e10",
		"resource": "repo",
	}
	cases := []struct {
		name    string
		kind    string
		fields  map[string]string
		want    int
		wantMsg string
	}{
		{name: "a complete sheet is silent", kind: ctxTypeSource, fields: good},
		{
			name: "no url", kind: ctxTypeSource,
			fields: map[string]string{"resource": "repo"},
			want:   1, wantMsg: "missing `url`",
		},
		{
			name: "relative url", kind: ctxTypeSource,
			fields: map[string]string{"url": "/docs/spec.md"},
			want:   1, wantMsg: "not an absolute http(s) URL",
		},
		{
			name: "non-http scheme", kind: ctxTypeSource,
			fields: map[string]string{"url": "ftp://example.org/x"},
			want:   1, wantMsg: "not an absolute http(s) URL",
		},
		{
			name: "short digest", kind: ctxTypeSource,
			fields: map[string]string{"url": "https://example.org", "sha256": "6b1043a1"},
			want:   1, wantMsg: "not a lowercase hex SHA-256",
		},
		{
			name: "licence in prose", kind: ctxTypeSource,
			fields: map[string]string{"url": "https://example.org", "license": "permissive MIT"},
			want:   1, wantMsg: "not an SPDX identifier",
		},
		{
			name: "restore on a live source", kind: ctxTypeSource,
			fields: map[string]string{"url": "https://example.org", "status": "used", "restore": "git clone x"},
			want:   1, wantMsg: "while status is not `gone`",
		},
		{name: "another kind is never checked", kind: ctxTypeNotes, fields: map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := lintSourceSheet("context/sources/x.md", sheet(tc.kind, tc.fields), tc.kind)
			if len(issues) != tc.want {
				t.Fatalf("got %d issue(s), want %d: %+v", len(issues), tc.want, issues)
			}
			if tc.wantMsg != "" && !strings.Contains(issues[0].Message, tc.wantMsg) {
				t.Errorf("message %q does not carry %q", issues[0].Message, tc.wantMsg)
			}
		})
	}
}
