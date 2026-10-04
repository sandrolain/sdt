package templates

import (
	"strings"
	"testing"
)

// TestResearchTemplateEditorialContract pins the populate-wiki correction: the
// generated contract names the gated archive verb, makes populate-wiki
// read-only and states that the CLI never writes a wiki page.
func TestResearchTemplateEditorialContract(t *testing.T) {
	data, err := templatesFS.ReadFile("instructions/research.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		"sdt research archive",
		"read-only",
		"The CLI never writes a `context/wiki/` page",
		"Wiki population is agent editorial work",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("research.md.tmpl missing %q", want)
		}
	}
	for _, removed := range []string{
		"populate-wiki --apply",
		"one page per verified",
		"Promoted from research",
	} {
		if strings.Contains(s, removed) {
			t.Errorf("research.md.tmpl still carries the removed auto-write text %q", removed)
		}
	}
}
