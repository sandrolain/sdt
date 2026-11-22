package cmd

import (
	"strings"
	"testing"
)

func TestLintPrivacyDeclarationRequiresClearanceForSensitiveData(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{"none", "---\nprivacy: none\n---\n", ""},
		{"cleared personal", "---\nprivacy: personal\nprivacy_clearance: {status: cleared, basis: names removed}\n---\n", ""},
		{"missing value", "---\nprivacy_clearance: {status: cleared, basis: checked}\n---\n", "must declare none"},
		{"missing clearance", "---\nprivacy: real\n---\n", "requires `privacy_clearance.status: cleared`"},
		{"wrong clearance", "---\nprivacy: personal\nprivacy_clearance: {status: pending, basis: review}\n---\n", "requires `privacy_clearance.status: cleared`"},
		{"clearance on none", "---\nprivacy: none\nprivacy_clearance: {status: cleared, basis: checked}\n---\n", "not valid when `privacy` is none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := lintPrivacyDeclaration("context/learning/example.md", tc.text)
			if tc.want == "" && len(issues) != 0 {
				t.Fatalf("unexpected privacy finding: %#v", issues)
			}
			if tc.want != "" && (len(issues) != 1 || !strings.Contains(issues[0].Message, tc.want)) {
				t.Fatalf("privacy findings = %#v, want %q", issues, tc.want)
			}
		})
	}
}
