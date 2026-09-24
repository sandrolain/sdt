package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/contextwiki"
	"github.com/sandrolain/sdt/internal/templates"
)

func sectionBlock(name, body string) string {
	body = strings.TrimSpace(body)
	return fmt.Sprintf("<!-- sdt:begin:%s -->\n\n%s\n\n<!-- sdt:end:%s -->\n", name, body, name)
}

func sectionBeginMarker(name string) string {
	return "<!-- sdt:begin:" + name + " -->"
}

func sectionEndMarker(name string) string {
	return "<!-- sdt:end:" + name + " -->"
}

func sectionRegexp(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)` + regexp.QuoteMeta(sectionBeginMarker(name)) + `.*?` + regexp.QuoteMeta(sectionEndMarker(name)))
}

func hasSection(content, name string) bool {
	return sectionRegexp(name).MatchString(content)
}

// agentReplaceSection rebuilds content around the named section after a force
// refresh: the text before and after the markers is preserved (trimmed of stray
// blank lines) and the section is re-inserted separated by a single blank line.
// The output is deterministic and, for files whose only content is the section
// (optionally prefixed by frontmatter), equals agentRenderGenerated.

func agentReplaceSection(content, name, body string) string {
	m := sectionRegexp(name).FindStringIndex(content)
	if m == nil {
		return content
	}
	var b strings.Builder
	before := strings.TrimRight(content[:m[0]], "\n")
	if before != "" {
		b.WriteString(before + "\n\n")
	}
	b.WriteString(sectionBlock(name, body))
	if rest := strings.Trim(content[m[1]:], "\n"); rest != "" {
		b.WriteString("\n" + rest)
	}
	out := b.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

// agentMergeBlock ensures the named block exists in content. With force it is
// refreshed, otherwise left untouched when already present. Returns the new
// content and whether it changed.

func agentMergeBlock(content, name, body string, force bool) (string, bool) {
	if hasSection(content, name) {
		if force {
			return agentReplaceSection(content, name, body), true
		}
		return content, false
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	return content + sectionBlock(name, body), true
}

// agentAppendIfMissing appends the named block only when it is absent. Unlike
// agentMergeBlock it has no force semantics: once present, the block is never
// touched. Used for write-once sections owned by the user/agent.

func agentAppendIfMissing(content, name, body string) string {
	if hasSection(content, name) {
		return content
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	return content + sectionBlock(name, body)
}

// ── generated per-file markers ─────────────────────────────────────────────────

// agentGeneratedMarkerName returns the marker name for a generated file, scoped
// by its owning directory so instructions/analysis.md and commands/analysis.md
// stay distinct.

func agentGeneratedMarkerName(dir, name string) string {
	return dir + "/" + strings.TrimSuffix(name, sdtMarkdownExt)
}

// agentRenderGenerated renders the on-disk form of a generated file from its
// template body: the leading frontmatter block stays at the first line (the
// context tools parse it from there) and the document body is wrapped in an
// sdt:begin/end marker. The output is deterministic so the drift guard can
// compare it byte-for-byte to the committed file.

func agentRenderGenerated(name, templateBody string) string {
	fm, body := contextwiki.SplitFrontmatter(templateBody)
	body = strings.TrimSpace(body)
	if fm = strings.TrimRight(fm, "\n"); fm != "" {
		return fm + "\n\n" + sectionBlock(name, body)
	}
	return sectionBlock(name, body)
}

// agentWriteGeneratedFile creates or refreshes a single generated file whose
// body comes from a template. Without --force an existing file is skipped
// (write-once semantics). With --force a file that already carries the marker
// is refreshed in place — content outside the markers survives; a legacy file
// without the marker has no merge boundary, so it is rewritten wholesale.

func agentWriteGeneratedFile(path, name, templateBody string, force bool) FileResult {
	res := FileResult{Path: path}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir, user-chosen output
	if err != nil && !os.IsNotExist(err) {
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	exists := err == nil
	if exists && !force {
		res.Status = statusSkipped
		res.Reason = "file already exists (use --force to overwrite)"
		return res
	}
	content := string(data)
	if exists && force && hasSection(content, name) {
		_, secBody := contextwiki.SplitFrontmatter(templateBody)
		content = agentReplaceSection(content, name, strings.TrimSpace(secBody))
	} else {
		content = agentRenderGenerated(name, templateBody)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //#nosec G301
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //#nosec G703 G306 -- user-chosen output
		res.Status = statusError
		res.Reason = err.Error()
		return res
	}
	if exists {
		res.Status = statusUpdated
	} else {
		res.Status = statusCreated
	}
	return res
}

// sdtArchiveDeprecatedDir is the subdirectory under context/archive/ where
// --force moves obsolete generated files instead of deleting them.

const sdtArchiveDeprecatedDir = "deprecated"

// agentArchiveGenerated moves an obsolete generated file to
// context/archive/deprecated/<base>-DEPRECATED-<stamp>.md with a loud header,
// preserving history. It reports skipped when the file is already gone or
// already archived, and error when either the archive write or the removal
// fails.

func agentArchiveGenerated(dir, name, reason string) FileResult {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return FileResult{Path: path, Status: statusSkipped, Reason: "file does not exist"}
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	stamp := time.Now().Format("20060102-150405")
	dst := filepath.Join(sdtArchiveDir, sdtArchiveDeprecatedDir, base+"-DEPRECATED-"+stamp+sdtMarkdownExt)
	if _, err := os.Stat(dst); err == nil {
		return FileResult{Path: path, Status: statusSkipped, Reason: "already archived"}
	}
	data, err := os.ReadFile(path) //#nosec G304 -- fixed generated dir, user-chosen output
	if err != nil {
		return FileResult{Path: path, Status: statusError, Reason: err.Error()}
	}
	header := fmt.Sprintf("# DEPRECATED — no longer generated\n\n*sdt agent init --force* archived this file from `%s` on %s: it no longer belongs to the generated set (%s). Keep it while the history matters, then remove it.\n\n---\n\n",
		path, time.Now().Format(time.RFC3339), reason)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil { //#nosec G301
		return FileResult{Path: path, Status: statusError, Reason: err.Error()}
	}
	if err := os.WriteFile(dst, []byte(header+string(data)), 0o644); err != nil { //#nosec G703 G306 -- user-chosen output
		return FileResult{Path: path, Status: statusError, Reason: err.Error()}
	}
	if err := os.Remove(path); err != nil {
		return FileResult{Path: path, Status: statusError, Reason: err.Error()}
	}
	return FileResult{Path: path, Status: statusArchived, Reason: "moved to " + dst}
}

// ── target file helpers ────────────────────────────────────────────────────────

func agentMergeTarget(target, project, group string, force, wantProject bool) (FileResult, string) {
	res := FileResult{Path: target}
	content := ""
	if _, err := os.Stat(target); err == nil {
		data, rerr := os.ReadFile(target) //#nosec G304 -- user-chosen target file
		if rerr != nil {
			res.Status = statusError
			res.Reason = rerr.Error()
			return res, content
		}
		content = string(data)
	}

	// Instructions block: refreshed with --force, else preserved.
	instrContent, instrChanged := agentMergeBlock(content, agentSectionNameInstructions, agentBlockInstructions(project, group), force)

	// Project block: write-once, created only when absent and requested, never
	// by --force. Declining never removes an already-present block.
	projectContent := instrContent
	projectAdded := false
	if wantProject {
		beforeProject := projectContent
		projectContent = agentAppendIfMissing(projectContent, agentSectionNameProject, agentBlockProject(project, group))
		projectAdded = projectContent != beforeProject
	}

	if !instrChanged && !projectAdded {
		res.Status = statusSkipped
		res.Reason = "AGENTS.md already up to date"
	} else if strings.TrimSpace(content) == "" {
		res.Status = statusCreated
	} else {
		res.Status = statusUpdated
	}
	return res, projectContent
}

func agentBlockInstructions(project, group string) string {
	data := struct {
		Project string
		Group   string
	}{Project: project, Group: group}
	return templates.Must("agents/instructions.md.tmpl", data, nil)
}

// agentBlockProject returns the write-once, single generic template placed in
// the `<!-- sdt:begin:project -->` block. The user or agent fills in the
// sections over time; the template does not vary by project type.

func agentBlockProject(project, group string) string {
	_ = project
	_ = group
	return templates.Must("agents/project.md.tmpl", nil, nil)
}

// ── registration ───────────────────────────────────────────────────────────────
