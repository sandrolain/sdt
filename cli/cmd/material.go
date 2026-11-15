//go:build !wasm

package cmd

import (
	"encoding/json"
	"errors"
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
	Use:   materialLane,
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
			data, err := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, data)
		case fmtYAML:
			outputString(cmd, fmt.Sprintf("path: %s\nmarkers_removed: %d\nslugs_resolved: %d\nslugs_unresolved: %d\ncheck_line_removed: %t\n",
				result.Path, result.MarkersRemoved, result.SlugsResolved, result.SlugsUnresolved, result.CheckLineRemoved))
			if result.Deck != "" {
				outputString(cmd, fmt.Sprintf("deck: %s\ndeck_markers_removed: %d\ndeck_notes_kept: %d\n",
					result.Deck, result.DeckMarkers, result.DeckNotes))
			}
		default:
			outputString(cmd, fmt.Sprintf("exported %s (%d marker(s) removed, %d slug(s) resolved, %d unresolved)\n",
				result.Path, result.MarkersRemoved, result.SlugsResolved, result.SlugsUnresolved))
			if result.Deck != "" {
				outputString(cmd, fmt.Sprintf("exported %s (%d marker(s) removed, %d note(s) kept)\n",
					result.Deck, result.DeckMarkers, result.DeckNotes))
			}
		}
	},
}

var materialCheckCmd = &cobra.Command{
	Use:   "check <path>",
	Short: "Refuse a package that still carries the authoring machinery",
	Long: `Walk a file or directory and report every authoring residue in the Markdown:
an HTML comment or header, a claim map, a "Verifica" line naming the item bank,
a backticked page id, or a curator-register phrase addressing the corpus instead
of the reader. Exit 0 when clean, 1 with the findings otherwise.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		findings, err := checkMaterialPath(args[0], loadMaterialWiki())
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			payload := map[string]any{materialStatusClean: len(findings) == 0, "findings": findings}
			data, err := json.MarshalIndent(payload, "", "  ")
			exitWithError(cmd, err)
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
	var paths []string
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
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", base, err)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	var files []materialFile
	for _, path := range paths {
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			continue
		}
		data, readErr := root.ReadFile(rel)
		if readErr != nil {
			continue
		}
		first, _, _ := strings.Cut(string(data), "\n")
		header, headerErr := parseMaterialHeader(first)
		if headerErr == nil {
			files = append(files, materialFile{Path: path, Header: header})
		}
	}
	if err := root.Close(); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func resolveMaterialFile(concept, syllabus, lang string) (materialFile, error) {
	matches, err := matchMaterialFiles(concept, syllabus, lang)
	if err != nil {
		return materialFile{}, err
	}
	switch len(matches) {
	case 0:
		return materialFile{}, fmt.Errorf("no material for %q with syllabus %q and language %q: check the header", strings.TrimPrefix(concept, "wiki/"), syllabus, lang)
	case 1:
		return matches[0], nil
	default:
		return materialFile{}, fmt.Errorf("%d materials match %q: %s — name --syllabus and --lang", len(matches), strings.TrimPrefix(concept, "wiki/"), materialCandidates(matches))
	}
}

// matchMaterialFiles is the shared material matcher (the export resolver's
// matching, decision D7): the concept id with or without `wiki/`, the syllabus
// with or without `syllabi/`, the language exact. An empty syllabus or language
// matches across all of them; zero matches is an empty slice, not an error, so
// each caller decides whether absence is fatal.
func matchMaterialFiles(concept, syllabus, lang string) ([]materialFile, error) {
	concept = strings.TrimPrefix(concept, "wiki/")
	syllabus = strings.TrimPrefix(syllabus, "syllabi/")
	files, err := loadMaterialFiles()
	if err != nil {
		return nil, err
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
	return matches, nil
}

// materialCandidates renders the paths a refusal names, so an ambiguous concept
// is refused with its candidates and never resolved by sort order.
func materialCandidates(matches []materialFile) string {
	paths := make([]string, 0, len(matches))
	for _, m := range matches {
		paths = append(paths, filepath.ToSlash(m.Path))
	}
	return strings.Join(paths, ", ")
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
	Deck             string `json:"deck,omitempty" yaml:"deck,omitempty"`
	DeckMarkers      int    `json:"deckMarkers,omitempty" yaml:"deckMarkers,omitempty"`
	DeckNotes        int    `json:"deckNotes,omitempty" yaml:"deckNotes,omitempty"`
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
	outputRoot, err := os.OpenRoot(outDir)
	if err != nil {
		return materialExportResult{}, fmt.Errorf("open %s: %w", outDir, err)
	}
	if err := outputRoot.WriteFile(filepath.Base(dst), []byte(clean), 0o644); err != nil { //#nosec G306 -- a deliverable a reader opens
		return materialExportResult{}, fmt.Errorf("write %s: %w", dst, errors.Join(err, outputRoot.Close()))
	}
	if err := outputRoot.Close(); err != nil {
		return materialExportResult{}, err
	}
	result.Path = filepath.ToSlash(dst)

	// The deck beside the guide travels with it: the same boundary, one file.
	deckPath := strings.TrimSuffix(ref.Path, sdtMarkdownExt) + ".slide" + sdtMarkdownExt
	if deckData, readErr := os.ReadFile(deckPath); readErr == nil { //#nosec G304 -- the deck beside the resolved guide
		deckClean, deckMarkers, deckNotes := cleanDeckText(string(deckData), pages)
		deckDst := filepath.Join(outDir, filepath.Base(deckPath))
		deckRoot, rootErr := os.OpenRoot(outDir)
		if rootErr != nil {
			return materialExportResult{}, rootErr
		}
		if writeErr := deckRoot.WriteFile(filepath.Base(deckDst), []byte(deckClean), 0o644); writeErr != nil { //#nosec G306 -- a deliverable a reader opens
			return materialExportResult{}, fmt.Errorf("write %s: %w", deckDst, errors.Join(writeErr, deckRoot.Close()))
		}
		if closeErr := deckRoot.Close(); closeErr != nil {
			return materialExportResult{}, closeErr
		}
		result.Deck = filepath.ToSlash(deckDst)
		result.DeckMarkers = deckMarkers
		result.DeckNotes = deckNotes
	}
	return result, nil
}

// cleanDeckText prepares a deck for a reader who does not have the corpus. The
// material header moves out of Marp's way (it must precede the frontmatter), the
// binding markers are dropped, and the speaker notes go with them: the notes are
// the presenter's, and they cite claim ids. A Marp directive (a comment that is
// neither a marker nor a "Note:") stays, because dropping it changes the deck.
func cleanDeckText(text string, pages *contextwiki.Builder) (string, int, int) {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "<!-- material:") {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	kept := make([]string, 0, len(lines))
	markers, notes := 0, 0
	skipping, keepingNote := false, false
	for _, line := range lines {
		cleaned, keep, marker, note, nextSkipping, nextNote := cleanDeckLine(line, skipping, keepingNote)
		skipping, keepingNote = nextSkipping, nextNote
		if keep {
			kept = append(kept, cleaned)
		}
		markers += marker
		notes += note
	}
	body := strings.Join(kept, "\n")
	body = materialSlugRe.ReplaceAllStringFunc(body, func(token string) string {
		match := materialSlugRe.FindStringSubmatch(token)
		if page := resolveMaterialSlug(pages, match[1]); page != nil && strings.TrimSpace(page.Title) != "" {
			return page.Title
		}
		return token
	})
	body = regexp.MustCompile(`\n{3,}`).ReplaceAllString(body, "\n\n")
	body = strings.Trim(body, "\n")
	var dropped int
	body, dropped = dropBankSlides(body)
	markers += dropped
	if body != "" {
		body += "\n"
	}
	return body, markers, notes
}

func cleanDeckLine(line string, skipping, keepingNote bool) (string, bool, int, int, bool, bool) {
	trimmed := strings.TrimSpace(line)
	if skipping {
		return "", false, 0, 0, !strings.Contains(trimmed, "-->"), false
	}
	if keepingNote {
		return cleanSpeakerNote(line), true, 0, 0, false, !strings.Contains(trimmed, "-->")
	}
	if materialCheckRe.MatchString(line) {
		return "", false, 1, 0, false, false
	}
	if strings.HasPrefix(trimmed, "<!--") {
		return cleanDeckComment(line, trimmed)
	}
	cleaned := materialCommentRe.ReplaceAllString(line, "")
	markerCount := 0
	if cleaned != line {
		if materialMarkerLine(line) {
			markerCount = 1
		}
		if speakerNoteLine(line) {
			cleaned = cleanSpeakerNote(cleaned)
		}
	}
	return strings.TrimRight(cleaned, " \t"), true, markerCount, 0, false, false
}

func cleanDeckComment(line, trimmed string) (string, bool, int, int, bool, bool) {
	closed := strings.Contains(trimmed, "-->")
	if materialMarkerLine(trimmed) {
		return "", false, 1, 0, !closed, false
	}
	if speakerNoteLine(trimmed) {
		cleaned := cleanSpeakerNote(line)
		if speakerNoteText(cleaned) != "" {
			return cleaned, true, 0, 1, false, !closed
		}
		return "", false, 0, 0, !closed, false
	}
	// A Marp directive (fit, _class, _paginate …) shapes the slide and stays.
	return strings.TrimRight(line, " \t"), true, 0, 0, false, false
}

var materialClaimRefRe = regexp.MustCompile(`(?i)\bclaim\s+\d+(?:\s*(?:,|e|and)\s*\d+)*\.?`)

// cleanSpeakerNote keeps the presenter's note and removes the claim ids it used to
// cite: the ids are the corpus's, not the presenter's.
func cleanSpeakerNote(line string) string {
	out := materialClaimRefRe.ReplaceAllString(line, "")
	out = regexp.MustCompile(`\s{2,}`).ReplaceAllString(out, " ")
	return strings.TrimRight(out, " \t")
}

func speakerNoteText(line string) string {
	text := strings.TrimSpace(line)
	text = strings.TrimPrefix(text, "<!--")
	text = strings.TrimSuffix(text, "-->")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "Note:")
	return strings.TrimSpace(text)
}

// dropBankSlides removes the slide that promises the course's item bank. A guide
// closes its exercise with one line the cleaner drops; a deck carries a whole
// slide ("Verifica: …", "sono nella verifica del corso"), and the delivered deck
// does not carry the bank. The course position ("Dove sei") stays: the presenter
// has the course.
func dropBankSlides(body string) (string, int) {
	lines := strings.Split(body, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim {
		return body, 0
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == ctxFrontmatterDelim {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return body, 0
	}
	front := lines[:closeIdx+1]
	rest := lines[closeIdx+1:]
	var slides [][]string
	current := []string{}
	for _, line := range rest {
		if strings.TrimSpace(line) == ctxFrontmatterDelim {
			slides = append(slides, current)
			current = nil
			continue
		}
		current = append(current, line)
	}
	slides = append(slides, current)
	kept := make([][]string, 0, len(slides))
	dropped := 0
	for _, slide := range slides {
		text := strings.ToLower(strings.Join(slide, "\n"))
		first := ""
		for _, line := range slide {
			if strings.TrimSpace(line) != "" {
				first = strings.TrimSpace(line)
				break
			}
		}
		if strings.Contains(text, "verifica del corso") || strings.HasPrefix(first, "# Verifica") {
			dropped++
			continue
		}
		kept = append(kept, slide)
	}
	out := strings.Join(front, "\n")
	for i, slide := range kept {
		block := strings.Trim(strings.Join(slide, "\n"), "\n")
		if i == 0 {
			out += "\n\n" + block
			continue
		}
		out += "\n\n---\n\n" + block
	}
	return out, dropped
}

func materialMarkerLine(line string) bool {
	for _, marker := range []string{"claim=", "lesson=", "objective=", "assess=", "gloss=", "connective"} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func speakerNoteLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "<!--")
	trimmed = strings.TrimSpace(trimmed)
	return strings.HasPrefix(strings.ToLower(trimmed), "note:")
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

// materialRegisterPhrases are the curator-register phrases the export must not
// carry: text addressing the corpus curator instead of the standalone reader.
// Exact phrases, matched case-insensitively — no fuzzy matching; a future
// pattern is one line here. "Dopo questa guida" is deliberately absent: the
// course hand-off is an accepted register (user decision 2026-11-07).
var materialRegisterPhrases = []string{
	"come nel corpus",
	"come nella guida precedente",
	"come vedrai",
	"come vedremo",
	"come nel resto del corso",
}

func checkMaterialPath(path string, pages *contextwiki.Builder) ([]materialFinding, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	rootPath, files, err := materialCheckFiles(path, info.IsDir())
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var findings []materialFinding
	for _, file := range files {
		data, readErr := root.ReadFile(file)
		if readErr != nil {
			return nil, errors.Join(readErr, root.Close())
		}
		displayPath := filepath.Join(rootPath, file)
		findings = append(findings, checkMaterialContents(displayPath, string(data), pages)...)
	}
	if err := root.Close(); err != nil {
		return nil, err
	}
	return findings, nil
}

func materialCheckFiles(path string, isDir bool) (string, []string, error) {
	rootPath := path
	var files []string
	if isDir {
		walkErr := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), sdtMarkdownExt) {
				rel, relErr := filepath.Rel(path, p)
				if relErr != nil {
					return relErr
				}
				files = append(files, rel)
			}
			return nil
		})
		if walkErr != nil {
			return "", nil, walkErr
		}
	} else {
		rootPath = filepath.Dir(path)
		files = []string{filepath.Base(path)}
	}
	return rootPath, files, nil
}

func checkMaterialContents(path, content string, pages *contextwiki.Builder) []materialFinding {
	isDeck := strings.HasSuffix(path, ".slide"+sdtMarkdownExt)
	lines := strings.Split(content, "\n")
	var findings []materialFinding
	if isDeck && (len(lines) == 0 || strings.TrimSpace(lines[0]) != ctxFrontmatterDelim) {
		findings = append(findings, materialFinding{Path: filepath.ToSlash(path), Line: 1,
			Message: "frontmatter is not at the start: the material header or a blank line displaces it, so Marp renders an empty first slide"})
	}
	for i, line := range lines {
		for _, message := range materialLineFindings(line, isDeck) {
			findings = append(findings, materialFinding{Path: filepath.ToSlash(path), Line: i + 1, Message: message})
		}
		for _, match := range materialSlugRe.FindAllStringSubmatch(line, -1) {
			if resolveMaterialSlug(pages, match[1]) != nil {
				findings = append(findings, materialFinding{Path: filepath.ToSlash(path), Line: i + 1, Message: "page slug `" + match[1] + "`"})
			}
		}
		lower := strings.ToLower(line)
		for _, phrase := range materialRegisterPhrases {
			if strings.Contains(lower, phrase) {
				findings = append(findings, materialFinding{Path: filepath.ToSlash(path), Line: i + 1, Message: "curator register: \"" + phrase + "\""})
				break
			}
		}
	}
	return findings
}

func materialLineFindings(line string, isDeck bool) []string {
	if isDeck {
		switch {
		case materialMarkerLine(line):
			return []string{"authoring marker"}
		case strings.Contains(line, "claims="):
			return []string{"claim map"}
		case strings.Contains(strings.ToLower(line), "verifica del corso"):
			return []string{"check line naming the item bank"}
		case strings.Contains(line, "claim "):
			return []string{"internal claim reference"}
		}
		return nil
	}
	switch {
	case strings.Contains(line, "<!--"):
		return []string{"authoring comment or header"}
	case strings.Contains(line, "claims="):
		return []string{"claim map"}
	case materialCheckRe.MatchString(line), strings.Contains(strings.ToLower(line), "verifica del corso"):
		return []string{"check line naming the item bank"}
	default:
		return nil
	}
}
