package cmd

import (
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/templates"
)

// ── generated Agent Skills (.agents/skills/) ──────────────────────────────────

// sdtSkillsDir is the on-disk root of the generated Agent Skills, in the open
// agentskills.io layout (one folder per skill, each holding a SKILL.md).

const sdtSkillsDir = ".agents/skills"

// skillFile is one generated skill file: the skill directory (relative to
// sdtSkillsDir) and the file it contains.

type skillFile struct {
	dir  string
	name string
	body string
}

// skillFiles returns the generated skills: one per embedded
// internal/templates/skills/<dir>/*.tmpl, in sorted order. The embedded template
// directories are the single source — adding a skill template adds the generated
// skill, with no second list to keep in step. Every skill is prefixed `sdt-` so
// it never collides with a user-chosen skill.

func skillFiles() []skillFile {
	var out []skillFile
	for _, dir := range templates.Dirs("skills") {
		for _, tmpl := range templates.ListFiles("skills/" + dir) {
			out = append(out, skillFile{
				dir:  dir,
				name: strings.TrimSuffix(tmpl, ".tmpl"),
				body: templates.Must("skills/"+dir+"/"+tmpl, nil, nil),
			})
		}
	}
	return out
}

// writeSkillFiles renders the skills under .agents/skills/. It is
// non-destructive: without --force an existing file is skipped; with --force a
// file that carries the generated marker is refreshed in place (content outside
// the markers survives).

func writeSkillFiles(force bool) []FileResult {
	scope := filepath.Base(sdtSkillsDir)
	var results []FileResult
	for _, f := range skillFiles() {
		path := filepath.Join(sdtSkillsDir, f.dir, f.name)
		marker := agentGeneratedMarkerName(scope+"/"+f.dir, f.name)
		results = append(results, agentWriteGeneratedFile(path, marker, f.body, force))
	}
	return results
}

// skillTemplateNames lists the embedded skill template paths ("skills/<dir>/<file>"),
// for the viewer-free and coherence guards.

func skillTemplateNames() []string {
	var out []string
	for _, dir := range templates.Dirs("skills") {
		for _, tmpl := range templates.ListFiles("skills/" + dir) {
			out = append(out, strings.Join([]string{"skills", dir, tmpl}, "/"))
		}
	}
	return out
}
