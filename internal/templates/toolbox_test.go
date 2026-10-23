package templates

import (
	"strings"
	"testing"
)

// TestToolboxRowsAreCommittedToList proves the table is data with a shape, not
// prose: every row names a command, a display that invokes it, and a host tool
// it replaces.
func TestToolboxRowsAreCommittedToList(t *testing.T) {
	rows := ToolboxRows()
	if len(rows) == 0 {
		t.Fatal("no toolbox rows")
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Operation == "" || row.Display == "" || row.Command == "" || row.Replaces == "" {
			t.Errorf("row %+v has an empty field", row)
		}
		if !strings.Contains(row.Display, "sdt "+row.Command) {
			t.Errorf("row %q: display %q does not invoke %q", row.Operation, row.Display, row.Command)
		}
		if seen[row.Operation] {
			t.Errorf("row %q appears twice", row.Operation)
		}
		seen[row.Operation] = true
	}
}

// TestToolboxMarkdownIsDeterministic keeps the golden guard meaningful: two
// renders are byte-identical, and the section opens with its heading.
func TestToolboxMarkdownIsDeterministic(t *testing.T) {
	first := ToolboxMarkdown()
	if first != ToolboxMarkdown() {
		t.Error("ToolboxMarkdown is not deterministic")
	}
	if !strings.HasPrefix(first, "### Toolbox") {
		t.Errorf("section does not open with its heading:\n%s", first)
	}
	if !strings.Contains(first, "| Need | Verb | Instead of |") {
		t.Errorf("table header missing:\n%s", first)
	}
	if got, want := strings.Count(first, "\n| "), len(ToolboxRows())+1; got != want {
		t.Errorf("table has %d rows, want %d", got, want)
	}
}

// TestListFilesAndDirs covers the two discovery helpers the generated sets are
// built from: one file per template, and a missing directory yielding nothing.
func TestListFilesAndDirs(t *testing.T) {
	if got := ListFiles("skills/sdt-mind-map"); len(got) != 1 || got[0] != "SKILL.md.tmpl" {
		t.Errorf("ListFiles(skills/sdt-mind-map) = %v", got)
	}
	if got := ListFiles("does/not/exist"); len(got) != 0 {
		t.Errorf("ListFiles(missing) = %v", got)
	}
	if got := Dirs("skills"); len(got) < 5 {
		t.Errorf("Dirs(skills) = %v, want the generated skill set", got)
	}
	if got := Dirs("does/not/exist"); len(got) != 0 {
		t.Errorf("Dirs(missing) = %v", got)
	}
}
