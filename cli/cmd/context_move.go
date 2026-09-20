package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// ── context document moves (rename / archive) ───────────────────────────────────
//
// Both commands resolve an existing context/ document (path, as in `status
// get/set`), recompute its path by the kind scheme and rewrite references to it
// across context/ markdown + index.md. --dry-run previews every mutation.

// ctxPlanPrefix is also the leading timestamp of a canonical dated filename
// (YYYYMMDD-HHMMSS-), reused here to split a dated path into prefix + slug.

var ctxDatedPrefix = regexp.MustCompile(`^\d{8}-\d{6}-`)

// ctxMoveExcludedDirs are context/ subtrees that hold dirty clones, imported
// sources or generated content and must never be rewritten.

var ctxMoveExcludedDirs = map[string]bool{
	sdtRefsDir:      true,
	sdtIngestionDir: true,
	sdtInstrDir:     true,
	sdtScriptsDir:   true,
	sdtTmpDir:       true,
	sdtDocsDir:      true,
}

// ctxCleanSlug sanitizes the target slug. Non-wiki kinds keep a single
// kebab-case segment; wiki allows subpaths ("backend/auth").

func ctxCleanSlug(raw string, allowSubpath bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("--slug is required")
	}
	segs := strings.Split(raw, "/")
	if !allowSubpath && len(segs) > 1 {
		return "", fmt.Errorf("slug %q must not contain %q (type wiki accepts subpaths)", raw, string(filepath.Separator))
	}
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		s = sanitizeSlug(s)
		if s == "" {
			return "", fmt.Errorf("invalid slug %q", raw)
		}
		out = append(out, s)
	}
	return strings.Join(out, "/"), nil
}

// contextRenamePath recomputes the document path after a slug change, keeping
// the kind's scheme. Decisions are append-only (immutable filename); task files
// live under the `task` command's pattern.

func contextRenamePath(doc ctxResolvedDoc, newSlug string) (string, error) {
	switch doc.Type.scheme {
	case ctxSchemeDated:
		base := strings.TrimSuffix(filepath.Base(doc.Path), sdtMarkdownExt)
		prefix := ""
		if loc := ctxDatedPrefix.FindStringIndex(base); loc != nil {
			prefix = base[:loc[0]+loc[1]]
		}
		if prefix == "" {
			return "", fmt.Errorf("cannot derive the dated prefix of %s", doc.Path)
		}
		return filepath.Join(filepath.Dir(doc.Path), prefix+newSlug+sdtMarkdownExt), nil
	case ctxSchemeBare:
		return filepath.Join(filepath.Dir(doc.Path), newSlug+sdtMarkdownExt), nil
	case ctxSchemeSubpath:
		return filepath.Join(sdtWikiDir, newSlug+sdtMarkdownExt), nil
	case ctxSchemeTmpBySlug:
		return filepath.Join(filepath.Dir(doc.Path), newSlug), nil
	case ctxSchemeDecision:
		return "", errors.New("decision filenames are immutable; create a new decision with `sdt context new --type decision`")
	case ctxSchemePhase:
		return "", errors.New("task files are managed by `sdt context task`; create the phase list with `sdt context task add --phase <n>`")
	}
	return "", fmt.Errorf("type %s does not support rename", ctxKindLabel(doc.Type))
}

// contextArchivePath builds the destination in context/archive with a fresh
// dated prefix; the slug defaults to the document's current slug.

func contextArchivePath(doc ctxResolvedDoc, slug string) string {
	if slug == "" {
		slug = ctxDocSlug(doc)
	}
	return filepath.Join(sdtArchiveDir, contextTimePrefix("20060102-150405", slug)+sdtMarkdownExt)
}

// ctxDocSlug derives the current slug of a resolved document: the dated
// filename without its timestamp prefix, the bare/subpath name, or — for a
// subpath (wiki) — the whole relative slug with "/" folded to "-".

func ctxDocSlug(doc ctxResolvedDoc) string {
	name := strings.TrimSuffix(filepath.Base(doc.Path), sdtMarkdownExt)
	switch doc.Type.scheme {
	case ctxSchemeDated:
		if loc := ctxDatedPrefix.FindStringIndex(name); loc != nil {
			return name[loc[1]:]
		}
		return name
	case ctxSchemeTmpBySlug:
		return name
	case ctxSchemeSubpath:
		rel, err := filepath.Rel(sdtWikiDir, strings.TrimSuffix(doc.Path, sdtMarkdownExt))
		if err != nil {
			return name
		}
		return sanitizeSlug(rel)
	}
	return name
}

// ── reference rewriting ─────────────────────────────────────────────────────────

var ctxWikilinkRegexp = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// ctxRewriteRef rewrites a single reference (root-relative like "plan/x.md" or
// doc-relative like "../analysis/x") from the old path to the new path,
// preserving the ".md" style and the relative-vs-root form.

func ctxRewriteRef(ref, docDir, oldPath, newPath string) (string, bool) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, "/")
	if ref == "" || ref == sdtWorkDir {
		return ref, false
	}
	hadExt := strings.HasSuffix(ref, sdtMarkdownExt)
	base := strings.TrimSuffix(ref, sdtMarkdownExt)

	oldBase := strings.TrimSuffix(oldPath, sdtMarkdownExt)
	newBase := strings.TrimSuffix(newPath, sdtMarkdownExt)

	root := filepath.Clean(filepath.Join(sdtWorkDir, base))
	docRel := filepath.Clean(filepath.Join(docDir, base))
	if root != oldBase && docRel != oldBase {
		return ref, false
	}

	var repl string
	if docRel == oldBase {
		rel, err := filepath.Rel(docDir, newBase)
		if err != nil {
			return ref, false
		}
		repl = rel
	} else {
		// root-relative: references are stored without the context/ prefix
		rel, err := filepath.Rel(sdtWorkDir, newBase)
		if err != nil {
			return ref, false
		}
		repl = rel
	}
	if hadExt {
		repl += sdtMarkdownExt
	}
	return repl, true
}

// ctxRewriteContent rewrites every occurrence of the old path in content:
// wikilinks ([[...]]) and frontmatter block-list items (links/sources/...).
// It returns the updated content and whether anything changed.

func ctxRewriteContent(content, docDir, oldPath, newPath string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false
	fmWalls := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == ctxFrontmatterDelim {
			fmWalls++
		}
		if fmWalls == 1 && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			trimmed := strings.TrimSpace(line)
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			quoted := strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)
			ref := strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
			if rew, ok := ctxRewriteRef(ref, docDir, oldPath, newPath); ok {
				if quoted {
					rew = `"` + rew + `"`
				}
				lines[i] = trimmed[:len(trimmed)-len(value)] + rew
				changed = true
			}
		}
		if strings.Contains(line, "[[") {
			lines[i] = ctxWikilinkRegexp.ReplaceAllStringFunc(line, func(m string) string {
				inner := m[2 : len(m)-2]
				target, title, hasTitle := strings.Cut(inner, "|")
				quoted := strings.HasPrefix(target, `"`) && strings.HasSuffix(target, `"`)
				ref := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(target), `"`), `"`)
				rew, ok := ctxRewriteRef(ref, docDir, oldPath, newPath)
				if !ok {
					return m
				}
				if quoted {
					rew = `"` + rew + `"`
				}
				if hasTitle {
					rew += "|" + title
				}
				changed = true
				return "[[" + rew + "]]"
			})
		}
	}
	return strings.Join(lines, "\n"), changed
}

// ctxAffectedFiles returns the context/**/*.md files (index.md included, dirty
// clone/import subtrees and the moved document excluded) whose references to
// oldPath must switch to newPath.

func ctxAffectedFiles(docPath, oldPath, newPath string) ([]string, error) {
	var affected []string
	err := filepath.WalkDir(sdtWorkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ctxMoveExcludedDirs[path] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != sdtMarkdownExt || path == docPath || path == oldPath || path == newPath {
			return nil
		}
		data, rerr := os.ReadFile(path) //#nosec G304,G122 -- context work files
		if rerr != nil {
			return rerr
		}
		if _, ok := ctxRewriteContent(string(data), filepath.Dir(path), oldPath, newPath); ok {
			affected = append(affected, path)
		}
		return nil
	})
	return affected, err
}

func ctxApplyRewrite(path, oldPath, newPath string) error {
	data, err := os.ReadFile(path) //#nosec G304 -- context work files
	if err != nil {
		return err
	}
	updated, ok := ctxRewriteContent(string(data), filepath.Dir(path), oldPath, newPath)
	if !ok {
		return nil
	}
	//#nosec G306,G703 -- user work file
	return os.WriteFile(path, []byte(updated), 0o644)
}

// ── sdt context rename ──────────────────────────────────────────────────────────

var contextRenameCmd = &cobra.Command{
	Use:   "rename <ref> --slug <new>",
	Short: "Rename a context/ document (slug change)",
	Long: `Rename an existing context/ document: recompute its path by the kind
scheme (dated kinds keep their creation-time prefix, bare/architecture and tmp
keep their dir, wiki accepts subpath slugs) and rewrite every reference to it
across context/ markdown and index.md.

Decisions are append-only (immutable filename) and task files are managed by
` + "`sdt context task`" + ` — both are rejected.

Examples:
  sdt context rename context/analysis/20260920-130000-x.md --slug backend
  sdt context rename context/wiki/auth.md --slug backend/session
  sdt context rename context/plan/20260920-131900-plan-x.md --slug plan-y --dry-run`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		newSlug, err := ctxCleanSlug(getStringFlag(cmd, "slug", false), doc.Type.scheme == ctxSchemeSubpath)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		newPath, err := contextRenamePath(doc, newSlug)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		if newPath == doc.Path {
			exitWithError(cmd, fmt.Errorf("new slug equals the current name: %s", doc.Path))
			return
		}
		if _, err := os.Stat(newPath); err == nil {
			exitWithError(cmd, fmt.Errorf("target already exists: %s", newPath))
			return
		}
		affected, err := ctxAffectedFiles(doc.Path, doc.Path, newPath)
		if err != nil {
			exitWithError(cmd, err)
			return
		}

		if getBoolFlag(cmd, "dry-run", false) {
			outputString(cmd, fmt.Sprintf("would rename %s -> %s\n", doc.Path, newPath))
			for _, p := range affected {
				outputString(cmd, fmt.Sprintf("would rewrite %s\n", p))
			}
			return
		}
		if err := os.Rename(doc.Path, newPath); err != nil { //#nosec G306 -- user work file
			exitWithError(cmd, err)
			return
		}
		for _, p := range affected {
			if err := ctxApplyRewrite(p, doc.Path, newPath); err != nil {
				exitWithError(cmd, err)
				return
			}
		}
		outputString(cmd, newPath+"\n")
		for _, p := range affected {
			outputString(cmd, "rewrote "+p+"\n")
		}
	},
}

// ── sdt context archive ─────────────────────────────────────────────────────────

var contextArchiveCmd = &cobra.Command{
	Use:   "archive <ref> [--slug <new>]",
	Short: "Archive a context/ document to context/archive/",
	Long: `Move an existing context/ document to context/archive/ with a fresh
dated name and set ` + "`status: archived`" + ` when the kind's vocabulary has it.
Task files go through ` + "`sdt context task archive`" + ` and decisions are
append-only — both are rejected here.

Examples:
  sdt context archive context/analysis/20260920-130000-x.md
  sdt context archive context/analysis/20260920-130000-x.md --slug audit-2026
  sdt context archive context/wiki/backend/auth.md --dry-run`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doc, err := resolveContextDoc(cmd, args)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		if doc.Type.scheme == ctxSchemePhase {
			exitWithError(cmd, errors.New("archive task files with `sdt context task archive --phase <n>` (the generic archive does not touch them)"))
			return
		}
		if doc.Type.scheme == ctxSchemeDecision {
			exitWithError(cmd, errors.New("decision records are append-only; impossible to archive them"))
			return
		}
		if doc.Type.scheme == ctxSchemeTmpBySlug {
			exitWithError(cmd, errors.New("tmp is scratch space, not a work document; move it manually"))
			return
		}
		slug := sanitizeSlug(getStringFlag(cmd, "slug", false))
		newPath := contextArchivePath(doc, slug)
		if _, err := os.Stat(newPath); err == nil {
			exitWithError(cmd, fmt.Errorf("target already exists: %s", newPath))
			return
		}
		affected, err := ctxAffectedFiles(doc.Path, doc.Path, newPath)
		if err != nil {
			exitWithError(cmd, err)
			return
		}

		content := ""
		if getBoolFlag(cmd, "dry-run", false) {
			outputString(cmd, fmt.Sprintf("would archive %s -> %s\n", doc.Path, newPath))
			for _, p := range affected {
				outputString(cmd, fmt.Sprintf("would rewrite %s\n", p))
			}
			return
		}

		data, err := os.ReadFile(doc.Path) //#nosec G304 -- user work file
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		content = string(data)
		if ctxStatusInVocab(doc.Type, statusArchived) {
			if updated, changed := setFrontmatterFields(content, ctxStatusSetPatches(doc.Type, statusArchived)); changed {
				content = updated
			}
		}
		if err := os.MkdirAll(sdtArchiveDir, 0o750); err != nil { //#nosec G301 -- user work dir
			if err != nil {
				exitWithError(cmd, err)
				return
			}
		}
		//#nosec G306 -- user work file
		if err := os.WriteFile(newPath, []byte(content), 0o644); err != nil {
			exitWithError(cmd, err)
			return
		}
		if err := os.Remove(doc.Path); err != nil {
			exitWithError(cmd, err)
			return
		}
		for _, p := range affected {
			if err := ctxApplyRewrite(p, doc.Path, newPath); err != nil {
				exitWithError(cmd, err)
				return
			}
		}
		outputString(cmd, newPath+"\n")
		for _, p := range affected {
			outputString(cmd, "rewrote "+p+"\n")
		}
	},
}

func init() {
	contextRenameCmd.Flags().String("slug", "", "New slug (target name)")
	contextRenameCmd.Flags().Bool("dry-run", false, "Preview without changing anything")
	contextArchiveCmd.Flags().String("slug", "", "Archive slug (default: derived from the document)")
	contextArchiveCmd.Flags().Bool("dry-run", false, "Preview without changing anything")
	contextCmd.AddCommand(contextRenameCmd, contextArchiveCmd)
}
