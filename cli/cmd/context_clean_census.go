//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/templates"
)

// The residue census. `context clean --trash/--tmp` empties a discard area the
// caller named; the census answers the other question — which files are residue
// at all — and applies the rule the project decided (2026-10-29): a residue is a
// file under context/tmp, a task file whose plan no longer resolves, or a
// generated file whose template or command is gone. A record (analysis, plan,
// worklog, note, question, research, decision, wiki) is never a candidate,
// however old: history is the corpus's product, not its debt.

// cleanClass is one kind of residue and the files that fall in it.
type cleanClass struct {
	Label string   `json:"class" yaml:"class"`
	Paths []string `json:"paths" yaml:"paths"`
}

// ctxCleanCutoff parses --older-than: a duration (30d, 12h, 3w, 90m) or an
// RFC3339 timestamp. Empty means no cutoff, which keeps the whole-directory
// sweep the verb always had.
func ctxCleanCutoff(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if len(value) < 2 {
		return time.Time{}, fmt.Errorf("--older-than %q is not a duration (30d, 12h, 3w) or an RFC3339 timestamp", value)
	}
	n, err := strconv.Atoi(value[:len(value)-1])
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("--older-than %q is not a duration (30d, 12h, 3w) or an RFC3339 timestamp", value)
	}
	var d time.Duration
	switch value[len(value)-1] {
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	case 'm':
		d = time.Duration(n) * time.Minute
	default:
		return time.Time{}, fmt.Errorf("--older-than %q has an unknown unit (use d, h, w or m)", value)
	}
	return contextNow().Add(-d), nil
}

// runCleanCensus reports the three residue classes and, with --apply, removes
// exactly them. Without --apply nothing is written: a census that deletes by
// default is not a census.
func runCleanCensus(cmd *cobra.Command) {
	cutoff, err := ctxCleanCutoff(getStringFlag(cmd, "older-than", false))
	exitWithError(cmd, err)
	if cutoff.IsZero() {
		cutoff = contextNow().Add(-30 * 24 * time.Hour)
	}
	classes := cleanCensus(cutoff)
	total := 0
	for _, c := range classes {
		total += len(c.Paths)
	}
	apply := getBoolFlag(cmd, "apply", false)
	switch getFormat(cmd) {
	case fmtJSON, fmtYAML:
		outputCleanCensus(cmd, classes, apply)
	default:
		for _, c := range classes {
			if len(c.Paths) == 0 {
				continue
			}
			outputString(cmd, fmt.Sprintf("%s (%d):\n", c.Label, len(c.Paths)))
			for _, path := range c.Paths {
				outputString(cmd, "  "+path+"\n")
			}
		}
		if total == 0 {
			outputString(cmd, "no residue: nothing older than "+cutoff.Format(time.RFC3339)+" and no unaligned generated file\n")
		}
	}
	if total == 0 {
		return
	}
	if !apply {
		outputString(cmd, fmt.Sprintf("%d file(s) would be removed; pass --apply to remove them\n", total))
		return
	}
	removed := 0
	for _, c := range classes {
		for _, path := range c.Paths {
			if err := cleanRemove(path); err != nil {
				outputString(cmd, "skipped "+path+": "+err.Error()+"\n")
				continue
			}
			removed++
		}
	}
	outputString(cmd, fmt.Sprintf("removed %d file(s)\n", removed))
}

// cleanCensus walks the three classes. A class with no candidate is an empty
// slice, so the answer's shape does not depend on the corpus.
func cleanCensus(cutoff time.Time) []cleanClass {
	return []cleanClass{
		{Label: "tmp", Paths: cleanTmp(cutoff)},
		{Label: "orphan tasks", Paths: cleanOrphanTasks(cutoff)},
		{Label: "stale generated", Paths: cleanStaleGenerated()},
	}
}

// cleanTmp lists files under context/tmp older than the cutoff.
func cleanTmp(cutoff time.Time) []string {
	var out []string
	//nolint:errcheck // a missing tmp directory is an empty class, not an error
	filepath.WalkDir(sdtTmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || !info.ModTime().Before(cutoff) {
			return nil
		}
		out = append(out, filepath.ToSlash(path))
		return nil
	})
	sort.Strings(out)
	return out
}

// cleanOrphanTasks lists task files older than the cutoff whose plan reference
// resolves to nothing: the phase they executed is gone.
func cleanOrphanTasks(cutoff time.Time) []string {
	files, err := dirFiles(sdtTasksDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, path := range files {
		info, ierr := os.Stat(path)
		if ierr != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		//#nosec G304 -- corpus listing
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		if taskPlanResolves(string(data)) {
			continue
		}
		out = append(out, filepath.ToSlash(path))
	}
	sort.Strings(out)
	return out
}

// taskPlanResolves reports whether a task file names a plan that exists.
func taskPlanResolves(content string) bool {
	for _, ref := range parseFrontmatterList(content, ctxFrontmatterSources) {
		if !strings.HasPrefix(ref, "plan/") {
			continue
		}
		if path := closeoutAbsRef(ref); path != "" {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
	return false
}

// cleanStaleGenerated lists generated files with no template or command behind
// them: an instruction module, a role profile, or a CLI page for a command that
// no longer exists. A regeneration rewrites the aligned ones and cannot remove
// these.
func cleanStaleGenerated() []string {
	var out []string
	out = append(out, staleAgainstTemplates(sdtInstrDir, "instructions")...)
	roleTemplates := map[string]bool{}
	for _, name := range templates.List("roles/core") {
		roleTemplates[strings.TrimSuffix(name, ".md.tmpl")+sdtMarkdownExt] = true
	}
	for _, name := range templates.ListFiles("roles") {
		roleTemplates[strings.TrimSuffix(name, ".tmpl")] = true
	}
	out = append(out, staleAgainstKnown(sdtRolesDir, roleTemplates)...)
	out = append(out, staleCommandDocs()...)
	sort.Strings(out)
	return out
}

// staleAgainstTemplates lists the markdown files in dir whose `.md.tmpl` sibling
// no longer ships.
func staleAgainstTemplates(dir, templateDir string) []string {
	known := map[string]bool{}
	for _, name := range templates.List(templateDir) {
		known[strings.TrimSuffix(name, ".md.tmpl")+sdtMarkdownExt] = true
	}
	return staleAgainstKnown(dir, known)
}

// staleAgainstKnown lists the markdown files in dir whose base name is not in
// the known set.
func staleAgainstKnown(dir string, known map[string]bool) []string {
	files, err := dirFiles(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, path := range files {
		if filepath.Base(path) == "README.md" {
			continue
		}
		if !known[filepath.Base(path)] {
			out = append(out, filepath.ToSlash(path))
		}
	}
	return out
}

// staleCommandDocs lists context/sdtdocs pages whose command path no longer
// exists on the root command.
func staleCommandDocs() []string {
	files, err := dirFiles(sdtDocsDir)
	if err != nil {
		return nil
	}
	known := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			path := strings.ReplaceAll(strings.TrimPrefix(child.CommandPath(), rootCmd.Name()+" "), " ", "-")
			if path != "" && path != rootCmd.Name() {
				known[path+sdtMarkdownExt] = true
			}
			walk(child)
		}
	}
	walk(rootCmd)
	var out []string
	for _, path := range files {
		if !known[filepath.Base(path)] {
			out = append(out, filepath.ToSlash(path))
		}
	}
	return out
}

// cleanRemove deletes one census path, refusing anything outside context/, a
// directory, or a symlink.
func cleanRemove(path string) error {
	clean := filepath.Clean(path)
	if !strings.HasPrefix(filepath.ToSlash(clean), sdtWorkDir+"/") {
		return fmt.Errorf("outside %s/", sdtWorkDir)
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("a directory, not a file")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("a symlink")
	}
	return os.Remove(clean)
}

// outputCleanCensus prints the census as the caller asked for it.
func outputCleanCensus(cmd *cobra.Command, classes []cleanClass, apply bool) {
	rows := make([]cleanClass, 0, len(classes))
	for _, c := range classes {
		if c.Paths == nil {
			c.Paths = []string{}
		}
		rows = append(rows, c)
	}
	payload := struct {
		Apply   bool         `json:"apply" yaml:"apply"`
		Classes []cleanClass `json:"classes" yaml:"classes"`
	}{Apply: apply, Classes: rows}
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(payload, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(payload)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	}
}
