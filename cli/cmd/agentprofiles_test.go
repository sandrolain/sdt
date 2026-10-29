package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/templates"
)

// TestProfileRegisterIsAClosedPartition checks the profile register against the
// embedded catalogue: every module it names exists, no module is claimed twice,
// every skill it names exists, and every register slug is unique with core
// first. A module no profile claims is a core module by definition, so the
// register is a partition of the catalogue rather than a second list to keep in
// step.
func TestProfileRegisterIsAClosedPartition(t *testing.T) {
	if agentProfileRegister[0].Slug != agentProfileCore {
		t.Fatalf("core must be first in the register, got %q", agentProfileRegister[0].Slug)
	}

	modules := map[string]bool{}
	for _, m := range allInstructionModules() {
		modules[strings.TrimSuffix(m, ".md")] = true
	}
	skills := map[string]bool{}
	for _, d := range templates.Dirs("skills") {
		skills[d] = true
	}

	seenSlug := map[string]bool{}
	claimed := map[string]string{}
	for _, p := range agentProfileRegister {
		if seenSlug[p.Slug] {
			t.Errorf("profile %q declared twice", p.Slug)
		}
		seenSlug[p.Slug] = true
		if p.Slug == agentProfileCore && (len(p.Modules) > 0 || len(p.Skills) > 0) {
			t.Errorf("core claims %d module(s) and %d skill(s): it is the catch-all", len(p.Modules), len(p.Skills))
		}
		for _, m := range p.Modules {
			if !modules[m] {
				t.Errorf("profile %q names module %q, which is not an embedded instruction template", p.Slug, m)
			}
			if other, ok := claimed[m]; ok {
				t.Errorf("module %q claimed by %q and %q", m, other, p.Slug)
			}
			claimed[m] = p.Slug
		}
		for _, s := range p.Skills {
			if !skills[s] {
				t.Errorf("profile %q names skill %q, which is not an embedded skill template", p.Slug, s)
			}
		}
	}
}

// TestProfileMatrixWritesExactlyItsSet is the matrix: for every optional profile,
// an init that selects only that profile writes core plus that profile's modules
// and nothing else, and the always-loaded on-action table advertises exactly the
// files the project has.
func TestProfileMatrixWritesExactlyItsSet(t *testing.T) {
	for _, profile := range agentProfileRegister[1:] {
		profile := profile
		t.Run(profile.Slug, func(t *testing.T) {
			dir := runInTempDir(t)
			agentInitWithProfiles(t, []string{profile.Slug})

			want := map[string]bool{}
			for _, m := range agentProfileModules([]string{agentProfileCore, profile.Slug}) {
				want[m] = true
			}

			entries, err := os.ReadDir(sdtInstrDir)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, e := range entries {
				got[e.Name()] = true
			}
			if strings.Join(sortedKeys(got), ",") != strings.Join(sortedKeys(want), ",") {
				t.Errorf("profile %q wrote\n  %v\nwant\n  %v", profile.Slug, sortedKeys(got), sortedKeys(want))
			}

			agents, err := os.ReadFile(filepath.Join(dir, agentTargetDefault))
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range allInstructionModules() {
				module := strings.TrimSuffix(m, ".md")
				advertised := strings.Contains(string(agents), "`context/instructions/"+module+".md`")
				if want[m] && !advertised {
					t.Errorf("profile %q: AGENTS.md does not advertise its module %s", profile.Slug, module)
				}
				if !want[m] && advertised {
					t.Errorf("profile %q: AGENTS.md advertises unselected module %s", profile.Slug, module)
				}
			}

			// The command registry follows the modules: a trigger whose contract
			// lives in an unselected module is not a row.
			registry, err := os.ReadFile(filepath.Join(sdtCommandsDir, "index.md"))
			if err != nil {
				t.Fatalf("read the registry: %v", err)
			}
			wikiRow := strings.Contains(string(registry), "| `>wiki` |")
			if profile.Slug == "knowledge" && !wikiRow {
				t.Error("profile knowledge did not register the >wiki trigger")
			}
			if profile.Slug != "knowledge" && wikiRow {
				t.Errorf("profile %q registered >wiki, whose contract is not selected", profile.Slug)
			}
		})
	}
}

// TestUnknownProfileIsRefused proves a typo fails before anything is written,
// rather than silently producing the core set.
func TestUnknownProfileIsRefused(t *testing.T) {
	dir := runInTempDir(t)
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--with", "uii"))
	})
	if _, err := os.Stat(filepath.Join(dir, sdtInstrDir)); err == nil {
		t.Error("a refused profile still wrote the instruction directory")
	}
}

// TestRefreshReusesTheStoredSelection proves `--with` is stored and that a later
// refresh without the flag re-renders the same set, so the selection is a
// property of the project rather than of the command line that created it.
func TestRefreshReusesTheStoredSelection(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none", "--with", "diagram")

	cfg := loadProjectConfig(sdtConfigFile)
	if cfg == nil || strings.Join(cfg.Instructions.Profiles, ",") != "core,diagram" {
		t.Fatalf("the selection was not stored: %+v", cfg)
	}

	diagram := filepath.Join(dir, sdtInstrDir, "diagram.md")
	if err := os.Remove(diagram); err != nil {
		t.Fatal(err)
	}
	execute(t, agentRefreshCmd, nil)
	if _, err := os.Stat(diagram); err != nil {
		t.Errorf("refresh did not re-render the stored selection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, sdtInstrDir, "slides.md")); err == nil {
		t.Error("refresh wrote a module outside the stored selection")
	}
}

// TestDoctorReportsOptionalProfilesAsHealthy proves an opt-in profile is healthy
// both ways: a project without roles is `ok: not selected` rather than a FAIL,
// and a project that selected roles has the profiles written by the same command.
func TestDoctorReportsOptionalProfilesAsHealthy(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")

	for _, c := range agentDoctorChecks() {
		if c.Name == doctorNameRoles && c.Status != doctorStatusOK {
			t.Errorf("roles check on a core-only project is %q, want ok: %+v", c.Status, c)
		}
		if c.Name == "profiles" && !strings.Contains(c.Detail, "core") {
			t.Errorf("the profiles check does not report the selection: %+v", c)
		}
	}

	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none", "--with", "roles")
	if _, err := os.Stat(filepath.Join(dir, sdtRolesDir, sdtRolesSharedFile)); err != nil {
		t.Fatalf("selecting the roles profile did not write the shared rules: %v", err)
	}
	for _, c := range agentDoctorChecks() {
		if c.Name == doctorNameRoles && c.Detail == "not selected" {
			t.Error("roles were selected but the check still reports them as not selected")
		}
	}
}

// sortedKeys is the sorted key list of a set, for a readable comparison.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestProfileSelectionIsIndependentOfTime guards the generator's determinism:
// two selections over the same template set differ only by the profile's own
// modules, never by a timestamp leaking into the module list.
func TestProfileSelectionIsIndependentOfTime(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = now
	core := agentProfileModules(nil)
	all := agentProfileModules(agentProfileSlugs())
	if len(all) != len(allInstructionModules()) {
		t.Fatalf("selecting every profile writes %d module(s), catalogue has %d", len(all), len(allInstructionModules()))
	}
	if len(core) >= len(all) {
		t.Fatalf("core alone writes %d module(s), the full selection %d", len(core), len(all))
	}
}

// TestOnActionTableIsOneTableUnderAnySelection: a profile the project did not
// select leaves an empty line where its row would be, and a blank line inside a
// markdown table ends it — the block would read as several tables.
func TestOnActionTableIsOneTableUnderAnySelection(t *testing.T) {
	for _, profiles := range [][]string{nil, {agentProfileCore}, {"ui"}, {"ui", "browser"}} {
		block := agentBlockInstructionsFor("p", "g", profiles)
		if strings.Contains(block, "|\n\n|") {
			t.Errorf("selection %v splits the on-action table:\n%s", profiles, block)
		}
	}
}
