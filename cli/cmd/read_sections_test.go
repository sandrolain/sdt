package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// sectionsFixture is a body with a preamble, a section, its subsection and a
// sibling: the shape whose ids a reader needs before addressing one.
func sectionsFixture(t *testing.T) string {
	t.Helper()
	dir := runInTempDir(t)
	doc := "---\nkind: notes\nsummary: s\n---\n\nintro\n\n## Alpha\n\nalpha body\n\n### Alpha child\n\nchild body\n\n## Beta\n\nbeta body\n"
	writeTestFile(t, filepath.Join(dir, "sections.md"), doc)
	return "sections.md"
}

// TestReadSectionsListsIdsLevelsAndHeadings is the discoverability contract: the
// listing is the round trip `read --section <id>` needs.
func TestReadSectionsListsIdsLevelsAndHeadings(t *testing.T) {
	file := sectionsFixture(t)
	out := string(execute(t, fileReadCmd, nil, "-p", file, "--sections"))
	want := []string{
		"section\tL0\t(preamble)",
		"alpha\tL2\tAlpha",
		"alpha-child\tL3\tAlpha child",
		"beta\tL2\tBeta",
	}
	for _, line := range want {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("listing misses %q:\n%s", line, out)
		}
	}
}

// TestReadSectionsRoundTrips proves the printed id addresses the same section.
func TestReadSectionsRoundTrips(t *testing.T) {
	file := sectionsFixture(t)
	out := string(execute(t, fileReadCmd, nil, "-p", file, "--section", "alpha-child"))
	if !strings.Contains(out, "### Alpha child") || !strings.Contains(out, "child body") {
		t.Errorf("id did not address the section: %q", out)
	}
	if strings.Contains(out, "beta body") {
		t.Errorf("id addressed past the section: %q", out)
	}
}

// TestReadSectionsJSON is the machine shape: an id, a level and a heading per
// section, under the path that was read.
func TestReadSectionsJSON(t *testing.T) {
	file := sectionsFixture(t)
	out := execute(t, fileReadCmd, nil, "-p", file, "--sections", "--format", "json")
	var payload struct {
		Path     string `json:"path"`
		Sections []struct {
			ID      string `json:"id"`
			Level   int    `json:"level"`
			Heading string `json:"heading"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json answer: %v\n%s", err, out)
	}
	if payload.Path != file {
		t.Errorf("path = %q, want %q", payload.Path, file)
	}
	if len(payload.Sections) != 4 {
		t.Fatalf("sections = %d, want 4: %+v", len(payload.Sections), payload.Sections)
	}
	if got := payload.Sections[2]; got.ID != "alpha-child" || got.Level != 3 || got.Heading != "Alpha child" {
		t.Errorf("third section = %+v", got)
	}
}

// TestReadSectionsRefusesContradictoryFlags keeps one answer per invocation:
// listing and selecting cannot both be the request.
func TestReadSectionsRefusesContradictoryFlags(t *testing.T) {
	file := sectionsFixture(t)
	for _, extra := range [][]string{
		{"--section", "alpha"},
		{"--lines", "1:2"},
		{"--count"},
		{"--numbered"},
	} {
		args := append([]string{"-p", file, "--sections"}, extra...)
		shouldExitWithCode(t, 1, func() string {
			return string(execute(t, fileReadCmd, nil, args...))
		})
	}
}
