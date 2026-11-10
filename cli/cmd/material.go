//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/contextwiki"
)

// The material lane is a study artefact, not a corpus record: its authoring source
// carries a header and inline binding markers, and the deliverable must carry none
// of them. `material export` is the boundary; `material check` is its gate.

var materialCmd = &cobra.Command{
	Use:   "material",
	Short: "Export a study guide without its authoring machinery",
}

var materialExportCmd = &cobra.Command{
	Use:   "export <concept>",
	Short: "Write a clean copy of a guide for a reader",
	Long: `Resolve one material by its header and write a clean copy under --out.

The copy has no header, no inline marker, no "Verifica" line naming the item bank
and no backticked page slug: a page slug becomes the page's title. The authoring
source is untouched; the copy is a build artefact, not a corpus document. A
concept with several materials is refused: name --syllabus and --lang.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		out := getStringFlag(cmd, "out", false)
		if out == "" {
			exitWithError(cmd, fmt.Errorf("--out is required: name the directory the clean copy goes to"))
			return
		}
		ref, err := resolveMaterialFile(args[0], getStringFlag(cmd, "syllabus", false), getStringFlag(cmd, "lang", false))
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		result, err := exportMaterialFile(ref, out, loadMaterialWiki())
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			data, _ := json.MarshalIndent(result, "", "  ")
			outputBytes(cmd, data)
		case fmtYAML:
			outputString(cmd, fmt.Sprintf("path: %s\nmarkers_removed: %d\nslugs_resolved: %d\nslugs_unresolved: %d\ncheck_line_removed: %t\n",
				result.Path, result.MarkersRemoved, result.SlugsResolved, result.SlugsUnresolved, result.CheckLineRemoved))
		default:
			outputString(cmd, fmt.Sprintf("exported %s (%d marker(s) removed, %d slug(s) resolved, %d unresolved)\n",
				result.Path, result.MarkersRemoved, result.SlugsResolved, result.SlugsUnresolved))
		}
	},
}

var materialCheckCmd = &cobra.Command{
	Use:   "check <path>",
	Short: "Refuse a package that still carries the authoring machinery",
	Long: `Walk a file or directory and report every authoring residue in the Markdown:
an HTML comment or header, a claim map, a "Verifica" line naming the item bank, or
a backticked page id. Exit 0 when clean, 1 with the findings otherwise.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		findings, err := checkMaterialPath(args[0], loadMaterialWiki())
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			payload := map[string]any{"clean": len(findings) == 0, "findings": findings}
			data, _ := json.MarshalIndent(payload, "", "  ")
			outputBytes(cmd, data)
		default:
			if len(findings) == 0 {
				outputString(cmd, "clean\n")
			} else {
				for _, f := range findings {
					outputString(cmd, fmt.Sprintf("%s:%d: %s\n", f.Path, f.Line, f.Message))
				}
			}
		}
		if len(findings) > 0 {
			exitWithError(cmd, fmt.Errorf("%d authoring residue(s): the package is not a deliverable", len(findings)))
		}
	},
}

func init() {
	materialExportCmd.Flags().String("out", "", "directory the clean copy goes to (required)")
	materialExportCmd.Flags().String("syllabus", "", "syllabus to disambiguate, with or without syllabi/")
	materialExportCmd.Flags().String("lang", "", "language to disambiguate, e.g. it")
	materialCmd.AddCommand(materialExportCmd)
	materialCmd.AddCommand(materialCheckCmd)
	rootCmd.AddCommand(materialCmd)
}

// ── resolution ─────────────────────────────────────────────────────────────────

type materialFile struct {
	Path   string
	Header materialHeader
}

func materialBaseDir() string {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	return filepath.Join(root, filepath.FromSlash(materialDir))
}

// loadMaterialFiles reads every guide's header. A .slide.md or a file without a
// valid header is not a guide and is skipped; the render check reports those.
func loadMaterialFiles() ([]materialFile, error) {
	base := materialBaseDir()
	var files []materialFile
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), sdtMarkdownExt) ||
			strings.HasSuffix(d.Name(), ".slide"+sdtMarkdownExt) {
			return nil
		}
		data, readErr := os.ReadFile(path) //#nosec G304 -- path from the material lane walk
		if readErr != nil {
			return nil
		}
		first, _, _ := strings.Cut(string(data), "\n")
		header, headerErr := parseMaterialHeader(first)
		if headerErr != nil {
			return nil
		}
		files = append(files, materialFile{Path: path, Header: header})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", base, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func resolveMaterialFile(concept, syllabus, lang string) (materialFile, error) {
	concept = strings.TrimPrefix(concept, "wiki/")
	syllabus = strings.TrimPrefix(syllabus, "syllabi/")
	files, err := loadMaterialFiles()
	if err != nil {
		return materialFile{}, err
	}
	var matches []materialFile
	for _, f := range files {
		if f.Header.Concept != concept {
			continue
		}
		if syllabus != "" && strings.TrimPrefix(f.Header.Syllabus, "syllabi/") != syllabus {
			continue
		}
		if lang != "" && f.Header.Lang != lang {
			continue
		}
		matches = append(matches, f)
	}
	switch len(matches) {
	case 0:
		return materialFile{}, fmt.Errorf("no material for %q with syllabus %q and language %q: check the header", concept, syllabus, lang)
	case 1:
		return matches[0], nil
	default:
		paths := make([]string, 0, len(matches))
		for _, m := range matches {
			paths = append(paths, filepath.ToSlash(m.Path))
		}
		return materialFile{}, fmt.Errorf("%d materials match %q: %s — name --syllabus and --lang", len(matches), concept, strings.Join(paths, ", "))
	}
}

func loadMaterialWiki() *contextwiki.Builder {
	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	builder, _, err := contextwiki.LoadWiki(filepath.Join(root, filepath.FromSlash(sdtWikiDir)))
	if err != nil {
		return nil
	}
	return builder
}

// ── export ─────────────────────────────────────────────────────────────────────

var (
	materialCommentRe = regexp.MustCompile(`<!--.*?-->`)
	materialSlugRe    = regexp.MustCompile("`([a-z0-9][a-z0-9-]*(?:/[a-z0-9][a-z0-9-]*)?)`")
)

type materialExportResult struct {
	Path             string `json:"path" yaml:"path"`
	MarkersRemoved   int    `json:"markersRemoved" yaml:"markersRemoved"`
	SlugsResolved    int    `json:"slugsResolved" yaml:"slugsResolved"`
	SlugsUnresolved  int    `json:"slugsUnresolved" yaml:"slugsUnresolved"`
	CheckLineRemoved bool   `json:"checkLineRemoved" yaml:"checkLineRemoved"`
}

func exportMaterialFile(ref materialFile, outDir string, pages *contextwiki.Builder) (materialExportResult, error) {
	data, err := os.ReadFile(ref.Path) //#nosec G304 -- resolved material
	if err != nil {
		return materialExportResult{}, fmt.Errorf("read %s: %w", ref.Path, err)
	}
	clean, result := cleanMaterialText(string(data), pages)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return materialExportResult{}, fmt.Errorf("create %s: %w", outDir, err)
	}
	dst := filepath.Join(outDir, filepath.Base(ref.Path))
	if err := os.WriteFile(dst, []byte(clean), 0o644); err != nil { //#nosec G306 -- a deliverable a reader opens
		return materialExportResult{}, fmt.Errorf("write %s: %w", dst, err)
	}
	result.Path = filepath.ToSlash(dst)
	return result, nil
}

// resolveMaterialSlug accepts a full page id or the bare last segment a guide uses
// in prose. A bare segment is resolved only when exactly one page carries it.
func resolveMaterialSlug(pages *contextwiki.Builder, slug string) *contextwiki.Page {
	if pages == nil {
		return nil
	}
	if strings.Contains(slug, "/") {
		return pages.Resolve(slug)
	}
	var found *contextwiki.Page
	for id, page := range pages.ByID {
		if filepath.Base(id) == slug {
			if found != nil {
				return nil
			}
			found = page
		}
	}
	return found
}

// cleanMaterialText removes the header, every inline comment, the check line that
// names the bank, and resolves a backticked page id to the page's title. It is
// line-based so a removed marker leaves no hole: a line that held only a comment
// disappears, and a comment at the end of prose is stripped from the prose.
func cleanMaterialText(text string, pages *contextwiki.Builder) (string, materialExportResult) {
	result := materialExportResult{}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(strings.TrimSpace(line), "<!-- material:") {
			result.MarkersRemoved++
			continue
		}
		if strings.Contains(line, "assess=") {
			result.MarkersRemoved++
			// Remove the "Verifica" line above, skipping the blank lines a guide
			// leaves between the exercise and the check.
			j := len(kept) - 1
			for j >= 0 && strings.TrimSpace(kept[j]) == "" {
				j--
			}
			if j >= 0 && materialCheckRe.MatchString(kept[j]) {
				kept = append(kept[:j], kept[j+1:]...)
				result.CheckLineRemoved = true
			}
			continue
		}
		if materialCheckRe.MatchString(line) {
			result.MarkersRemoved++
			result.CheckLineRemoved = true
			continue
		}
		cleaned := materialCommentRe.ReplaceAllString(line, "")
		if cleaned != line {
			result.MarkersRemoved++
		}
		if strings.TrimSpace(line) != "" && strings.TrimSpace(cleaned) == "" {
			continue
		}
		kept = append(kept, strings.TrimRight(cleaned, " \t"))
	}
	body := strings.Join(kept, "\n")
	body = materialSlugRe.ReplaceAllStringFunc(body, func(token string) string {
		match := materialSlugRe.FindStringSubmatch(token)
		if page := resolveMaterialSlug(pages, match[1]); page != nil && strings.TrimSpace(page.Title) != "" {
			result.SlugsResolved++
			return page.Title
		}
		result.SlugsUnresolved++
		return token
	})
	// Collapse the blank runs the removals leave, and keep one trailing newline.
	blank := regexp.MustCompile(`\n{3,}`)
	body = blank.ReplaceAllString(body, "\n\n")
	body = strings.Trim(body, "\n")
	if body != "" {
		body += "\n"
	}
	return body, result
}

// ── check ──────────────────────────────────────────────────────────────────────

type materialFinding struct {
	Path    string `json:"path" yaml:"path"`
	Line    int    `json:"line" yaml:"line"`
	Message string `json:"message" yaml:"message"`
}

func checkMaterialPath(path string, pages *contextwiki.Builder) ([]materialFinding, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var files []string
	if info.IsDir() {
		walkErr := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), sdtMarkdownExt) {
				files = append(files, p)
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	} else {
		files = []string{path}
	}
	sort.Strings(files)
	var findings []materialFinding
	for _, file := range files {
		data, readErr := os.ReadFile(file) //#nosec G304 -- the path the caller asked to check
		if readErr != nil {
			return nil, readErr
		}
		for i, line := range strings.Split(string(data), "\n") {
			add := func(message string) {
				findings = append(findings, materialFinding{Path: filepath.ToSlash(file), Line: i + 1, Message: message})
			}
			switch {
			case strings.Contains(line, "<!--"):
				add("authoring comment or header")
			case strings.Contains(line, "claims="):
				add("claim map")
			case materialCheckRe.MatchString(line):
				add("check line naming the item bank")
			case strings.Contains(strings.ToLower(line), "verifica del corso"):
				add("check line naming the item bank")
			}
			for _, match := range materialSlugRe.FindAllStringSubmatch(line, -1) {
				if resolveMaterialSlug(pages, match[1]) != nil {
					add("page slug `" + match[1] + "`")
				}
			}
		}
	}
	return findings, nil
}
