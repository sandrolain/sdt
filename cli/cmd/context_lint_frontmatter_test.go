package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "valid mapping",
			content: "---\nkind: tasks\nsummary: valid\n---\nbody\n",
		},
		{
			name:    "nested mapping and block list",
			content: "---\nkind: tasks\nmetadata:\n  owner: team\nsources:\n  - plan/example.md\n---\nbody\n",
		},
		{
			name:    "multiline scalar",
			content: "---\nkind: notes\nsummary: |\n  first line\n  second line\n---\nbody\n",
		},
		{
			name:    "status-like body prose is outside the block",
			content: "---\nkind: tasks\nsummary: valid\nstatus: pending\n---\nstatus completed\nstatus active\nstatus draft\n",
		},
		{
			name:    "CRLF delimiters",
			content: "---\r\nkind: tasks\r\nsummary: valid\r\n---\r\nbody\r\n",
		},
		{
			name:    "missing frontmatter",
			content: "body without metadata\n",
			wantErr: "missing YAML frontmatter",
		},
		{
			name:    "unterminated block",
			content: "---\nkind: tasks\nsummary: missing closer\n",
			wantErr: "unterminated YAML frontmatter",
		},
		{
			name:    "invalid YAML",
			content: "---\nkind: tasks\nstatus completed\n---\nbody\n",
			wantErr: "invalid YAML frontmatter:",
		},
		{
			name:    "scalar root",
			content: "---\njust a scalar\n---\nbody\n",
			wantErr: "YAML frontmatter must be a mapping",
		},
		{
			name:    "sequence root",
			content: "---\n- item\n---\nbody\n",
			wantErr: "YAML frontmatter must be a mapping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFrontmatter([]byte(tt.content))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateFrontmatter() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateFrontmatter() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFrontmatterIgnoresStatusLikeBodyProse(t *testing.T) {
	paths := []string{
		"../../context/analysis/20260925-171816-slides-corpus-lane-implementation.md",
		"../../context/plan/20260920-131900-plan-cli-doc-management-homogeneity.md",
		"../../context/worklog/20260920-163722-cli-doc-management-homogeneity-closeout.md",
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateFrontmatter(data); err != nil {
				t.Fatalf("status-like body prose must not invalidate frontmatter: %v", err)
			}
		})
	}
}

func TestLintFrontmatterSyntaxSeverity(t *testing.T) {
	missing := lintFrontmatterSyntax("missing.md", []byte("plain legacy document\n"))
	if missing == nil || missing.Priority != ctxLintWarning || missing.Message != "missing YAML frontmatter" {
		t.Fatalf("missing frontmatter issue = %#v, want distinct WARNING", missing)
	}

	malformed := lintFrontmatterSyntax("malformed.md", []byte("---\nstatus completed\n---\n"))
	if malformed == nil || malformed.Priority != ctxLintCritical || !strings.Contains(malformed.Message, "frontmatter") {
		t.Fatalf("malformed frontmatter issue = %#v, want CRITICAL", malformed)
	}

	if got := lintFrontmatterSyntax("valid.md", []byte("---\nkind: notes\n---\n")); got != nil {
		t.Fatalf("valid frontmatter issue = %#v, want nil", got)
	}
}

func TestLintDocMalformedFrontmatterIsCritical(t *testing.T) {
	dir := runInTempDir(t)
	path := filepath.Join("context", "tmp", "malformed-frontmatter.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nkind: tasks\nstatus completed\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	issues := lintDoc(path)
	if len(issues) != 1 || issues[0].Priority != ctxLintCritical || !strings.Contains(issues[0].Message, "invalid YAML frontmatter") {
		t.Fatalf("lintDoc() issues = %#v, want one CRITICAL malformed-frontmatter issue (cwd %s)", issues, dir)
	}
}
