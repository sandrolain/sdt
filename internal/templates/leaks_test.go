package templates

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestGeneratedSetLeaks is the distribution gate: no generated surface may name
// this repository's packages, paths, documents, tools, host or ids.
//
// knownLeaks is the shrinking allowlist of violations still present, keyed by
// `path|pattern` with the expected hit count. An entry that no longer matches
// fails too, so the list cannot rot into a standing exemption.
func TestGeneratedSetLeaks(t *testing.T) {
	got := map[string]int{}
	for _, surface := range leakSurfaces(t) {
		for _, hit := range ScanLeaks(surface.body) {
			got[surface.name+"|"+hit.Pattern]++
			if testing.Verbose() {
				t.Logf("%s:%d [%s] %s", surface.name, hit.Line, hit.Pattern, hit.Text)
			}
		}
	}

	for key, want := range knownLeaks {
		if got[key] != want {
			t.Errorf("allowlist stale: %s declared %d, found %d — remove or fix the entry", key, want, got[key])
		}
	}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := knownLeaks[key]; !ok {
			t.Errorf("leak: %s (x%d) — %s", key, got[key], leakReason(key))
		}
	}
}

// leakReason resolves a `path|pattern` key back to the pattern's reason.
func leakReason(key string) string {
	name := key[strings.IndexByte(key, '|')+1:]
	for _, p := range leakPatterns {
		if p.Name == name {
			return p.Reason
		}
	}
	return "unknown pattern"
}

// leakSurface is one piece of the generated set, as written to a project.
type leakSurface struct {
	name string
	body string
}

// leakSurfaces enumerates the generated set: every embedded template, the
// AGENTS.md instructions block and the roles core layer of this working tree.
// The trigger examples and the compiled constants live in cli/cmd and are
// checked there with the same table.
func leakSurfaces(t *testing.T) []leakSurface {
	t.Helper()
	var surfaces []leakSurface
	for _, dir := range []string{"instructions", "agents", "commands", "workspace", "roles", "skills"} {
		err := fs.WalkDir(templatesFS, dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, rerr := templatesFS.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			surfaces = append(surfaces, leakSurface{name: "internal/templates/" + path, body: string(data)})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}

	root := filepath.Join("..", "..")
	if data, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil {
		re := regexp.MustCompile(`(?ms)^<!-- sdt:begin:instructions -->.*?^<!-- sdt:end:instructions -->`)
		if block := re.FindString(string(data)); block != "" {
			// The identity line is rendered data — this project's own id, exactly
			// as a recipient's would be — not a reference the template chose.
			block = regexp.MustCompile(`(?m)^Project: .*$`).ReplaceAllString(block, "Project: <project>")
			surfaces = append(surfaces, leakSurface{name: "AGENTS.md#instructions", body: block})
		} else {
			t.Error("AGENTS.md: instructions block markers not found")
		}
	}

	rolesDir := filepath.Join(root, "context", "roles")
	if entries, err := os.ReadDir(rolesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
				continue
			}
			data, rerr := os.ReadFile(filepath.Join(rolesDir, e.Name()))
			if rerr != nil {
				t.Fatalf("read roles/%s: %v", e.Name(), rerr)
			}
			slug := strings.TrimSuffix(e.Name(), ".md")
			core, ok := roleCoreRegion(string(data), slug)
			if !ok {
				continue
			}
			surfaces = append(surfaces, leakSurface{name: "context/roles/" + e.Name(), body: core})
		}
	}
	return surfaces
}

// knownLeaks is the audit's remaining backlog (phase 4 of the plan empties it).
var knownLeaks = map[string]int{}

// TestLeakPatternsHaveFixtures proves every row of the table can fail: a gate
// with an untested row is a comment, not a check.
func TestLeakPatternsHaveFixtures(t *testing.T) {
	fixtures := map[string]string{
		"viewer":             "the viewer renders it",
		"repo-package":       "shared with ctxquery",
		"repo-internal-path": "see internal/acpbridge",
		"viewer-path":        "built by web/src/lib/map.ts",
		"repo-dir":           "run ./cli/... first",
		"lint-stack":         "golangci-lint run",
		"host":               "opencode/big-pickle",
		"test-stack":         "its tests (jsdom/vitest)",
		"log-library":        "rendered by lmittmann",
		"project-id":         "project: sdt_01a2b3c4",
		"decision-number":    "see decision 0017",
		"decision-date":      "see decision 2026-10-06",
		"plan-id":            "see plan 20261006-200432",
	}
	names := LeakPatternNames()
	if len(names) != len(fixtures) {
		t.Fatalf("table has %d rows, fixtures cover %d: %v", len(names), len(fixtures), names)
	}
	for _, name := range names {
		body, ok := fixtures[name]
		if !ok {
			t.Errorf("pattern %q has no fixture", name)
			continue
		}
		hits := ScanLeaks(body)
		found := false
		for _, h := range hits {
			if h.Pattern == name {
				found = true
			}
		}
		if !found {
			t.Errorf("pattern %q matched nothing in its fixture %q (hits: %v)", name, body, hits)
		}
	}
}

// TestLeakAllowMarkerExemptsOneLine pins the escape hatch: a legitimate
// illustration is exempted by the line it sits on, and a marker alone on a line
// exempts the following line — never more.
func TestLeakAllowMarkerExemptsOneLine(t *testing.T) {
	inline := "a role reads it (e.g. " + LeakMarker + " two ecosystems) golangci-lint"
	if hits := ScanLeaks(inline); len(hits) != 0 {
		t.Errorf("an inline marker must exempt its line, got %v", hits)
	}
	standalone := "<!-- " + LeakMarker + " example only -->\ngolangci-lint run\ngolangci-lint again"
	hits := ScanLeaks(standalone)
	if len(hits) != 1 || hits[0].Line != 3 {
		t.Errorf("a standalone marker must exempt only the next line, got %v", hits)
	}
	if hits := ScanLeaks("golangci-lint"); len(hits) != 1 {
		t.Errorf("an unmarked line must be reported, got %v", hits)
	}
}
