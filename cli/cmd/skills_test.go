package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/templates"
)

// TestAgentInitGeneratesSkills proves sdt agent init renders the Agent Skills in
// the open .agents/skills/<name>/SKILL.md layout, prefixed sdt-.
func TestAgentInitGeneratesSkills(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, agentInitCmd, nil, "--project", "p", "--group", "g", "--yes", "--gitignore", "none")
	for _, name := range []string{"sdt-ui-ux", "sdt-slide-deck", "sdt-mind-map", "sdt-vector-svg", "sdt-browser-verification"} {
		path := filepath.Join(dir, sdtSkillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("skill %s not generated: %v", name, err)
		}
	}
}

// TestWriteSkillFilesIdempotent proves a second init neither rewrites an
// unchanged skill nor drops it.
func TestWriteSkillFilesIdempotent(t *testing.T) {
	runInTempDir(t)
	first := writeSkillFiles(false)
	var updated int
	for _, r := range first {
		if r.Status == statusCreated {
			updated++
		}
	}
	if updated == 0 {
		t.Fatal("first run must create skill files")
	}
	second := writeSkillFiles(false)
	for _, r := range second {
		if r.Status != statusSkipped {
			t.Errorf("second run must skip an existing skill, got %s for %s", r.Status, r.Path)
		}
	}
}

// TestSkillFrontmatterCoherence asserts every skill template carries a
// name == its folder, a trigger description within the 1024-char budget, and an
// H1-free body that routes to generated instruction files without restating
// them.
func TestSkillFrontmatterCoherence(t *testing.T) {
	generated := map[string]bool{}
	for _, f := range instructionFiles("", "") {
		generated[f.name] = true
	}
	refRE := regexp.MustCompile(`context/instructions/([a-z0-9-]+\.md)`)
	names := skillTemplateNames()
	if len(names) == 0 {
		t.Fatal("no skill templates embedded")
	}
	for _, path := range names {
		body := templates.Must(path, nil, nil)
		folder := filepath.Base(filepath.Dir(path))
		fm, _ := contextwiki.SplitFrontmatter(body)
		if !strings.Contains(fm, "name: "+folder) {
			t.Errorf("%s: frontmatter name must equal the folder %q", path, folder)
		}
		if !strings.HasPrefix(folder, "sdt-") {
			t.Errorf("%s: skill folder %q must carry the sdt- prefix", path, folder)
		}
		if d := skillDescription(fm); len(d) == 0 || len(d) > 1024 {
			t.Errorf("%s: description must be 1..1024 chars, got %d", path, len(d))
		}
		if !strings.Contains(fm, "Use this skill") {
			t.Errorf("%s: description must be phrased as a trigger", path)
		}
		refs := refRE.FindAllStringSubmatch(body, -1)
		if len(refs) == 0 {
			t.Errorf("%s: body must reference at least one instruction file", path)
		}
		for _, m := range refs {
			if !generated[m[1]] {
				t.Errorf("%s: references %q, which is not a generated instruction file", path, m[1])
			}
		}
	}
}

// skillDescription extracts the `description:` scalar (possibly a folded block)
// from a frontmatter block and collapses whitespace.
func skillDescription(fm string) string {
	lines := strings.Split(fm, "\n")
	var b strings.Builder
	inDesc := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "description:"):
			inDesc = true
			b.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "description:")))
		case inDesc && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")):
			b.WriteString(" ")
			b.WriteString(strings.TrimSpace(line))
		default:
			inDesc = false
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// TestSkillViewerFree extends the generated-set invariant to the skills surface.
func TestSkillViewerFree(t *testing.T) {
	for _, path := range skillTemplateNames() {
		body := templates.Must(path, nil, nil)
		for _, tok := range []string{"viewer", "sdtviewer"} {
			if regexp.MustCompile(`\b` + tok + `\b`).MatchString(body) {
				t.Errorf("%s names %q; the skills surface must stay viewer-free", path, tok)
			}
		}
	}
}

// TestCommittedSkillsMatchTemplates guards against drift between the skill
// templates and the committed .agents/skills/ (like the AGENTS.md block guard).
// context/ is gitignored, but the skills live under .agents/, which is tracked.
func TestCommittedSkillsMatchTemplates(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, sdtSkillsDir)); err != nil {
		t.Skipf("%s not present: %v", sdtSkillsDir, err)
	}
	for _, f := range skillFiles() {
		path := filepath.Join(root, sdtSkillsDir, f.dir, f.name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read committed skill %s: %v", path, err)
		}
		marker := agentGeneratedMarkerName(filepath.Base(sdtSkillsDir)+"/"+f.dir, f.name)
		if string(data) != agentRenderGenerated(marker, f.body) {
			t.Errorf("drift in %s: run sdt agent init --force", path)
		}
	}
}
