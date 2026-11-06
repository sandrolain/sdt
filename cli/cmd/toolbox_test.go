package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/templates"
	"github.com/spf13/cobra"
)

// findTopCommand returns the top-level command named name, or nil.
func findTopCommand(name string) *cobra.Command {
	for _, c := range rootCmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// findManifestCommand walks a CommandManifest tree by name path.
func findManifestCommand(m CommandManifest, path ...string) *CommandManifest {
	cur := &m
	for _, name := range path {
		var next *CommandManifest
		for i := range cur.Subcommands {
			if cur.Subcommands[i].Name == name {
				next = &cur.Subcommands[i]
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// commandHasFlag reports whether name is declared on c or anywhere in its
// subtree: a row may name `context edit --append`, where the flag lives one
// level below the command the row points at.
func commandHasFlag(c *cobra.Command, name string) bool {
	if c.LocalFlags().Lookup(name) != nil {
		return true
	}
	for _, sub := range c.Commands() {
		if commandHasFlag(sub, name) {
			return true
		}
	}
	return false
}

// manifestHasFlag is commandHasFlag over a manifest entry.
func manifestHasFlag(m *CommandManifest, name string) bool {
	for _, f := range m.Flags {
		if f.Name == name {
			return true
		}
	}
	for i := range m.Subcommands {
		if manifestHasFlag(&m.Subcommands[i], name) {
			return true
		}
	}
	return false
}

var toolboxFlagRe = regexp.MustCompile(`--([a-z0-9-]+)`)

// checkToolboxPath verifies a nested invocation a row names whole, e.g.
// `sdt context wiki glossary`: the sibling set cannot express it, and an unverified
// path would be the one advertisement the gate does not cover.
func checkToolboxPath(t *testing.T, manifest CommandManifest, row templates.ToolboxRow) {
	t.Helper()
	if len(row.Path) == 0 {
		return
	}
	joined := strings.Join(row.Path, " ")
	if findManifestCommand(manifest, row.Path...) == nil {
		t.Errorf("toolbox row %q: `sdt %s` is not in sdt manifest", row.Operation, joined)
	}
	current := findTopCommand(row.Path[0])
	for i, sub := range row.Path[1:] {
		if current == nil {
			break
		}
		var next *cobra.Command
		for _, c := range current.Commands() {
			if c.Name() == sub {
				next = c
				break
			}
		}
		if next == nil {
			t.Errorf("toolbox row %q: `sdt %s` has no subcommand %q", row.Operation, strings.Join(row.Path[:i+1], " "), sub)
			break
		}
		current = next
	}
	if !strings.Contains(row.Display, "sdt "+joined) {
		t.Errorf("toolbox row %q: display %q does not invoke `sdt %s`", row.Operation, row.Display, joined)
	}
}

// TestToolboxRowsExistInManifest is the gate behind the generated Toolbox
// table: the analysis measured that a rule naming a verb the binary cannot
// perform is dropped wholesale, so a row must be checkable against the binary
// rather than trusted. The row's Command, Subcommands and Flags are the claim;
// the cobra tree and the manifest are the evidence.
func TestToolboxRowsExistInManifest(t *testing.T) {
	rows := templates.ToolboxRows()
	if len(rows) < 8 {
		t.Fatalf("toolbox has %d rows; the shipped surface is at least 8", len(rows))
	}

	manifest := buildCommandManifest(rootCmd)

	for _, row := range rows {
		// The manifest is what an agent discovers the CLI through: a verb
		// missing there is invisible even if it runs.
		m := findManifestCommand(manifest, row.Command)
		if m == nil {
			t.Errorf("toolbox row %q: %q is not in sdt manifest", row.Operation, row.Command)
			continue
		}

		// The cobra tree is what actually parses the invocation.
		cmd := findTopCommand(row.Command)
		if cmd == nil {
			t.Errorf("toolbox row %q: %q is not a command", row.Operation, row.Command)
			continue
		}
		for _, sub := range row.Subcommands {
			found := false
			for _, c := range cmd.Commands() {
				if c.Name() == sub {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("toolbox row %q: %q has no subcommand %q", row.Operation, row.Command, sub)
			}
			if findManifestCommand(manifest, row.Command, sub) == nil {
				t.Errorf("toolbox row %q: %q %s is not in sdt manifest", row.Operation, row.Command, sub)
			}
		}

		checkToolboxPath(t, manifest, row)

		if !strings.Contains(row.Display, "sdt "+row.Command) {
			t.Errorf("toolbox row %q: display %q does not invoke %q", row.Operation, row.Display, row.Command)
		}

		for _, name := range row.Flags {
			if !commandHasFlag(cmd, name) {
				t.Errorf("toolbox row %q: %q has no flag --%s", row.Operation, row.Command, name)
			}
			if !strings.Contains(row.Display, "--"+name) {
				t.Errorf("toolbox row %q: flag --%s is declared but absent from the display", row.Operation, name)
			}
			if !manifestHasFlag(m, name) {
				t.Errorf("toolbox row %q: flag --%s is missing from sdt manifest", row.Operation, name)
			}
		}

		// A row that spans two verbs gates both: Into is the full path of the
		// second invocation the display names.
		if len(row.Into) > 0 {
			into := strings.Join(row.Into, " ")
			if findManifestCommand(manifest, row.Into...) == nil {
				t.Errorf("toolbox row %q: `sdt %s` is not in sdt manifest", row.Operation, into)
			}
			intoCmd := findTopCommand(row.Into[0])
			if intoCmd == nil {
				t.Errorf("toolbox row %q: %q is not a command", row.Operation, row.Into[0])
			} else {
				for _, sub := range row.Into[1:] {
					found := false
					for _, c := range intoCmd.Commands() {
						if c.Name() == sub {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("toolbox row %q: %q has no subcommand %q", row.Operation, row.Into[0], sub)
					}
				}
			}
			if !strings.Contains(row.Display, "sdt "+into) {
				t.Errorf("toolbox row %q: display %q does not invoke `sdt %s`", row.Operation, row.Display, into)
			}
		}

		// Every flag the display advertises must be declared, or the row
		// teaches a flag that does not exist.
		for _, match := range toolboxFlagRe.FindAllStringSubmatch(row.Display, -1) {
			if !commandHasFlag(cmd, match[1]) {
				t.Errorf("toolbox row %q: display advertises --%s, which %q does not declare", row.Operation, match[1], row.Command)
			}
		}
	}
}

// TestToolboxSkillTableMatchesRows keeps the sdt-toolbox skill's copy of the
// table in step with the generated one: the skill is a second surface for the
// same rows, so a row added or reworded in toolbox.go must reach it too.
func TestToolboxSkillTableMatchesRows(t *testing.T) {
	body := templates.Must("skills/sdt-toolbox/SKILL.md.tmpl", nil, nil)
	md := templates.ToolboxMarkdown()
	i := strings.Index(md, "| Need |")
	if i < 0 {
		t.Fatal("toolbox markdown has no table header")
	}
	table := strings.TrimRight(md[i:], "\n")
	if !strings.Contains(body, table) {
		t.Error("sdt-toolbox skill table drifted from ToolboxRows")
	}
}

// TestToolboxMarkdownRendersEveryRow keeps the renderer honest: a row that is
// data but never printed is a silent omission.
func TestToolboxMarkdownRendersEveryRow(t *testing.T) {
	md := templates.ToolboxMarkdown()
	if !strings.HasPrefix(md, "### Toolbox") {
		t.Fatalf("toolbox markdown does not open with its heading:\n%s", md)
	}
	for _, row := range templates.ToolboxRows() {
		if !strings.Contains(md, row.Display) {
			t.Errorf("row %q (%s) missing from the rendered table", row.Operation, row.Display)
		}
		if !strings.Contains(md, row.Replaces) {
			t.Errorf("row %q (%s) missing its host equivalent", row.Operation, row.Replaces)
		}
	}
	if got, want := strings.Count(md, "\n| "), len(templates.ToolboxRows())+1; got != want {
		// +1 for the header row; the separator starts with "|-" and is not counted.
		t.Errorf("toolbox table has %d table lines, want %d", got, want)
	}
}
