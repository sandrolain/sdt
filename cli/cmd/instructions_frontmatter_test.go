package cmd

import (
	"os"
	"strings"
	"testing"
)

// TestFrontmatterFamiliesAreOptIn proves the default posture: a project that
// declared no optional family sees no finding for one, while the base keys stay
// mandatory. This is the whole point of the declaration — a check nobody can act
// on is noise, and "0 findings" has to mean something.
func TestFrontmatterFamiliesAreOptIn(t *testing.T) {
	setupContextProject(t)
	writeCtxDoc(t, "context/analysis/ax.md",
		"---\nkind: analysis\nsummary: s\nlinks: none\nmaturity: stable\nevidence_class: nonsense\nrigor: P9\n---\nbody\n")

	out := string(execute(t, contextLintCmd, nil, "--format", "json"))
	for _, unwanted := range []string{"`maturity`", "`evidence_class`", "`rigor`", "missing `objective`"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("an undeclared family produced a finding (%q):\n%s", unwanted, out)
		}
	}

	// The base keys are not a family: they stay mandatory either way.
	writeCtxDoc(t, "context/notes/base.md", "---\nkind: notes\n---\nbody\n")
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, contextLintCmd, nil, "--format", "json"))
		return out
	})
	if !strings.Contains(out, "mandatory `summary`") {
		t.Errorf("a base key stopped being mandatory:\n%s", out)
	}
}

// TestFrontmatterFamilyEnabledReports proves the other half: declaring a family
// turns its checks back on, including the vocabulary check a typo trips.
func TestFrontmatterFamilyEnabledReports(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity", "evidence", "rigor")
	writeCtxDoc(t, "context/analysis/ax.md",
		"---\nkind: analysis\nsummary: s\nlinks: none\nmaturity: stable\nevidence_class: nonsense\nrigor: P9\n---\nbody\n")

	out := string(execute(t, contextLintCmd, nil, "--format", "json"))
	for _, want := range []string{"`maturity` \\\"stable\\\" is outside the vocabulary", "`evidence_class` \\\"nonsense\\\" is outside the vocabulary", "`rigor` \\\"P9\\\" is outside the dial"} {
		if !strings.Contains(out, want) {
			t.Errorf("an enabled family produced no finding (%q):\n%s", want, out)
		}
	}
}

// TestWarrantFamilyIsSeparate pins the split inside the maturity axis: the
// promotion ladder can be on while the warrant advice stays off.
func TestWarrantFamilyIsSeparate(t *testing.T) {
	setupContextProject(t)
	enableFrontmatterFamilies(t, "maturity")
	writeCtxDoc(t, "context/notes/w.md",
		"---\nkind: notes\nsummary: s\nagent: a\nmaturity: established\n---\nbody\n")
	out := string(execute(t, contextLintCmd, nil, "--format", "json"))
	if strings.Contains(out, "without `warrant`") {
		t.Errorf("the warrant advice ran without its family:\n%s", out)
	}

	enableFrontmatterFamilies(t, "maturity", "warrant")
	out = string(execute(t, contextLintCmd, nil, "--format", "json"))
	if !strings.Contains(out, "without `warrant`") {
		t.Errorf("the warrant family was declared but produced no finding:\n%s", out)
	}
}

// TestUnknownFrontmatterFamilyIsRefused proves a typo is refused where it is
// written, rather than silently enabling nothing.
func TestUnknownFrontmatterFamilyIsRefused(t *testing.T) {
	runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--yes")
	cfg := loadProjectConfig(sdtConfigFile)
	if cfg == nil {
		t.Fatal("agent init wrote no config")
	}
	cfg.Instructions.Frontmatter = []string{"maturty"}
	if err := os.WriteFile(sdtConfigFile, []byte(buildProjectConfigContent(cfg, "")), 0o600); err != nil { //#nosec G306 -- test fixture
		t.Fatal(err)
	}

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentInitCmd, nil, "--project", "p", "--yes", "--force"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, agentRefreshCmd, nil))
	})

	// Lint reports the typo once, as a configuration CRITICAL, instead of
	// quietly ignoring the family the author meant to turn on.
	writeCtxDoc(t, "context/notes/n.md", "---\nkind: notes\nsummary: s\nagent: a\n---\nbody\n")
	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, contextLintCmd, nil, "--format", "json"))
		return out
	})
	if !strings.Contains(out, "unknown frontmatter family") || !strings.Contains(out, "maturty") {
		t.Errorf("lint did not report the unknown family:\n%s", out)
	}
}

// TestVocabPrintsFrontmatterFamilies proves the discovery surface: the vocabulary
// verb reports which optional families are on and which keys each covers.
func TestVocabPrintsFrontmatterFamilies(t *testing.T) {
	setupContextProject(t)
	out := string(execute(t, contextVocabCmd, nil))
	if !strings.Contains(out, "optional frontmatter families") {
		t.Fatalf("vocab does not mention the families:\n%s", out)
	}
	if !strings.Contains(out, "maturity [off]") {
		t.Errorf("vocab does not report a family as off by default:\n%s", out)
	}

	enableFrontmatterFamilies(t, "maturity")
	out = string(execute(t, contextVocabCmd, nil))
	if !strings.Contains(out, "maturity [on]") || !strings.Contains(out, "`maturity`, `superseded_by`") {
		t.Errorf("vocab does not report the enabled family and its keys:\n%s", out)
	}
}
