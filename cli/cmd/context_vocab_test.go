package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The `## Statuses` table in `context/architecture/stack.md` is the only
// reader-facing copy of the per-kind status vocabulary, and it carries a
// hand-written rationale per value that a generated table would lose. So the
// table stays prose, and this test binds the part that must not drift: the
// vocabulary column must equal the registry the validators enforce. The Meaning
// column stays the author's.

// statusTableRow is one parsed row of the status matrix.
type statusTableRow struct {
	kind       string
	vocab      []string
	defaultVal string
}

// parseStatusTable reads the matrix out of the architecture page. The page is
// corpus content and may be absent in a partial checkout, which the callers
// treat as a skip rather than a failure — the same degradation the template
// tests use for `context/`.
func parseStatusTable(t *testing.T) []statusTableRow {
	t.Helper()
	page := filepath.Join(projectRoot(t), "context", "architecture", "stack.md")
	data, err := os.ReadFile(page) //#nosec G304 -- a corpus page in this repository
	if err != nil {
		t.Skipf("the status matrix is not present: %v", err)
	}
	section := statusMatrixSection(string(data))
	if section == "" {
		t.Fatal("the `## Statuses` section was not found in stack.md")
	}
	var rows []statusTableRow
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 3 || cells[0] == "Kind" || strings.HasPrefix(cells[0], "---") {
			continue
		}
		rows = append(rows, statusTableRow{
			kind:       cells[0],
			defaultVal: strings.Trim(cells[1], "` "),
			vocab:      tableVocabulary(cells[2]),
		})
	}
	if len(rows) == 0 {
		t.Fatal("the status matrix parsed no rows")
	}
	return rows
}

// statusMatrixSection returns the body of the `## Statuses` heading.
func statusMatrixSection(doc string) string {
	start := strings.Index(doc, "\n## Statuses\n")
	if start < 0 {
		return ""
	}
	rest := doc[start+len("\n## Statuses\n"):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// splitTableRow splits a markdown table line into trimmed cells.
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// tableVocabulary extracts the backticked values from the vocabulary column.
// The column joins them with a middle dot and may annotate one of them, as the
// tasks row does with `active`(legacy).
var statusValueRe = regexp.MustCompile("`([^`]+)`")

func tableVocabulary(cell string) []string {
	var out []string
	for _, m := range statusValueRe.FindAllStringSubmatch(cell, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// TestStatusMatrixMatchesTheRegistry is the binding: the prose table and the Go
// registry are two copies of one closed register, and the check makes an edit to
// either side visible in the other.
func TestStatusMatrixMatchesTheRegistry(t *testing.T) {
	registry := map[string]ctxDocType{}
	for _, tp := range contextTypeList {
		if len(tp.statuses) > 0 {
			registry[tp.kind] = tp
		}
	}

	covered := map[string]struct{}{}
	for _, row := range parseStatusTable(t) {
		tp, known := registry[row.kind]
		if !known {
			// `reference` is documented as outside the registry and excluded
			// from lint, so the table legitimately has a row with no counterpart.
			if row.kind != "reference" {
				t.Errorf("the matrix documents type %q, which carries no vocabulary in the registry", row.kind)
			}
			continue
		}
		covered[row.kind] = struct{}{}
		if strings.Join(row.vocab, "|") != strings.Join(tp.statuses, "|") {
			t.Errorf("%s vocabulary drifted:\n  matrix:   %v\n  registry: %v", row.kind, row.vocab, tp.statuses)
		}
		if row.defaultVal != tp.defaultStatus {
			t.Errorf("%s default drifted: matrix %q, registry %q", row.kind, row.defaultVal, tp.defaultStatus)
		}
	}
	for kind := range registry {
		if _, ok := covered[kind]; !ok {
			t.Errorf("type %q carries a status vocabulary the matrix does not document", kind)
		}
	}
}

// TestStatusVocabVerbReportsTheRegistry binds the new verb to the same source,
// so the printable copy and the enforced copy cannot part either.
func TestStatusVocabVerbReportsTheRegistry(t *testing.T) {
	rows := ctxVocabRows()
	byKind := map[string]ctxVocabRow{}
	for _, r := range rows {
		byKind[r.Kind] = r
	}
	for _, tp := range contextTypeList {
		if len(tp.statuses) == 0 {
			if _, reported := byKind[tp.kind]; reported {
				t.Errorf("%s carries no vocabulary but the verb reports one", tp.kind)
			}
			continue
		}
		row, ok := byKind[tp.kind]
		if !ok {
			t.Errorf("the verb omits %s, which carries a vocabulary", tp.kind)
			continue
		}
		if strings.Join(row.Statuses, "|") != strings.Join(tp.statuses, "|") {
			t.Errorf("%s: verb %v, registry %v", tp.kind, row.Statuses, tp.statuses)
		}
		if row.Default != tp.defaultStatus {
			t.Errorf("%s default: verb %q, registry %q", tp.kind, row.Default, tp.defaultStatus)
		}
	}
	if len(rows) == 0 {
		t.Fatal("the verb reported no vocabularies at all")
	}
}

// TestStatusVocabCommandFormats pins that the three formats carry the same rows,
// since an agent reading json and a human reading text must not disagree.
func TestStatusVocabCommandFormats(t *testing.T) {
	text := string(execute(t, contextVocabCmd, nil))
	if !strings.Contains(text, "per-kind status vocabulary") {
		t.Errorf("text output has no header:\n%s", text)
	}
	if !strings.Contains(text, "* active") {
		t.Errorf("text output does not mark the default:\n%s", text)
	}
	// The default marker must appear for every reported type, not once.
	if strings.Count(text, "* = default for a new") != len(ctxVocabRows()) {
		t.Errorf("the default marker is not explained per type:\n%s", text)
	}
	if out := string(execute(t, contextVocabCmd, nil, "--format", "json")); !strings.Contains(out, `"statuses"`) {
		t.Errorf("json output carries no statuses:\n%s", out)
	}
	if out := string(execute(t, contextVocabCmd, nil, "--format", "yaml")); !strings.Contains(out, "statuses:") {
		t.Errorf("yaml output carries no statuses:\n%s", out)
	}
}
