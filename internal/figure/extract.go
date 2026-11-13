// Package figure extracts a book's chapter figures out of an EPUB and pairs
// them with the captions the converted Markdown carries, into a manifest.
//
// The conversion keeps the caption and drops the image; this package puts the
// image back where an author can read it. It is the pure-Go port of
// context/scripts/figures-extract.py: the EPUB is a zip, the spine comes from
// the OPF, every <img> pairs with its alt text, and the alt maps to the
// converted `###### Figure N-M. <caption>` line. The mapping is a string
// match, not a vision call.
//
// An image with no matching caption and a caption with no image are both
// recorded in the manifest, so a silent drop is impossible. A repeated caption
// is a named finding: the second image lands under a suffixed filename
// (figure-8-1.1) and never overwrites the first.
package figure

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// captionRe matches the converted Markdown's figure heading.
var captionRe = regexp.MustCompile(`^###### Figure (\d+-\d+)\. (.+?)\s*$`)

// figureRefRe matches the in-text reference `[Figure N-M](#anchor)` that
// anchors a caption to the prose above it.
var figureRefRe = regexp.MustCompile(`\[Figure (\d+-\d+)\]\(#([^)]+)\)`)

// sourceTailRe strips a trailing "(source: …)" attribution from a caption.
var sourceTailRe = regexp.MustCompile(`\s*\(source:[^)]*\)\s*$`)

// wsRe collapses whitespace runs; Python's \s on str also spans Unicode
// spaces, so the class includes them.
var wsRe = regexp.MustCompile(`[\s\p{Zs}]+`)

var (
	opfPathRe  = regexp.MustCompile(`full-path="([^"]+)"`)
	itemTagRe  = regexp.MustCompile(`<item\b[^>]*>`)
	itemIDRe   = regexp.MustCompile(`id="([^"]+)"`)
	itemHRefRe = regexp.MustCompile(`href="([^"]+)"`)
	itemMedRe  = regexp.MustCompile(`media-type="([^"]+)"`)
	itemRefRe  = regexp.MustCompile(`<itemref\b[^>]*idref="([^"]+)"`)
	chapterRe  = regexp.MustCompile(`^ch\d+$`)
	nonDigitRe = regexp.MustCompile(`\D`)
)

// Options carries one extraction run's inputs. EPUBPath and ConvertedPath are
// the files to read; SourceDisplay and ConvertedDisplay are the paths written
// into the manifest (relative, when the files live under the project root);
// OutDir receives the images and manifest.json.
type Options struct {
	EPUBPath         string
	SourceDisplay    string
	ConvertedPath    string
	ConvertedDisplay string
	OutDir           string
	StrictPosition   bool
}

// Finding is one named anomaly of the run: an image whose alt is empty, a
// caption heading that repeats, or a second image matching a caption that is
// already placed. Findings are reported, never silent, and never enter the
// manifest schema.
type Finding struct {
	Kind     string `json:"kind"` // empty-alt | duplicate-caption | duplicate-image
	FigureID string `json:"figure_id,omitempty"`
	Message  string `json:"message"`
}

// Counts is the found / placed / dropped bookkeeping the counters note
// records.
type Counts struct {
	Found                int `json:"found"`                  // distinct figure captions in the active chapters
	Placed               int `json:"placed"`                 // captions whose image was extracted
	Duplicates           int `json:"duplicates"`             // extra images placed under suffixed names
	Dropped              int `json:"dropped"`                // found minus placed
	UnmappedImages       int `json:"unmapped_images"`        // images no caption matched
	CaptionsWithoutImage int `json:"captions_without_image"` // captions no image matched
}

// UnmappedImage is an image in the EPUB that matched no caption.
type UnmappedImage struct {
	Chapter string `json:"chapter"`
	XHTML   string `json:"xhtml"`
	Src     string `json:"src"`
	Alt     string `json:"alt"`
}

// Result reports one run: where the manifest landed and what it holds.
type Result struct {
	OutDir       string          `json:"out_dir"`
	ManifestPath string          `json:"manifest"`
	Counts       Counts          `json:"counts"`
	Findings     []Finding       `json:"findings"`
	Unmapped     []UnmappedImage `json:"unmapped"`
	Missing      []string        `json:"captions_without_image"`
}

// ── manifest schema (the two existing manifests are the contract) ─────────────

type manifestFigure struct {
	ID            string `json:"id"`
	Chapter       string `json:"chapter"`
	File          string `json:"file"`
	Alt           string `json:"alt"`
	XHTML         string `json:"xhtml"`
	ConvertedLine int    `json:"converted_line"`
	Anchor        string `json:"anchor"`
	Span          string `json:"span"`
	PairedBy      string `json:"paired_by,omitempty"`
	Disposition   string `json:"disposition"`
}

type manifest struct {
	Source               string           `json:"source"`
	SourceSHA256         string           `json:"source_sha256"`
	Converted            string           `json:"converted"`
	Chapter              *string          `json:"chapter"`
	Chapters             []int            `json:"chapters"`
	Scope                string           `json:"scope"`
	Figures              []manifestFigure `json:"figures"`
	Unmapped             []UnmappedImage  `json:"unmapped"`
	CaptionsWithoutImage []string         `json:"captions_without_image"`
}

// caption is one `###### Figure N-M.` heading of the converted Markdown.
type caption struct {
	ID      string
	Chapter string
	Text    string
	Line    int
	Anchor  string
	Span    string
}

// spineEntry is one document of the EPUB's reading order.
type spineEntry struct {
	Href  string
	Media string
}

// Extract runs one extraction: walk the EPUB spine, pair every image with its
// caption, write the images and manifest.json into OutDir. Dispositions of a
// previous manifest in OutDir are preserved, so a re-run is idempotent.
func Extract(opts Options) (*Result, error) {
	epubBytes, err := os.ReadFile(opts.EPUBPath) //#nosec G304 -- caller-provided input path
	if err != nil {
		return nil, fmt.Errorf("unreadable input: %s: %w", opts.EPUBPath, err)
	}
	digest := sha256.Sum256(epubBytes)
	sourceSHA := hex.EncodeToString(digest[:])

	convertedBytes, err := os.ReadFile(opts.ConvertedPath) //#nosec G304 -- caller-provided input path
	if err != nil {
		return nil, fmt.Errorf("unreadable input: %s: %w", opts.ConvertedPath, err)
	}
	caps, duplicateCaptions := captions(splitLines(convertedBytes))
	if len(caps) == 0 {
		return nil, fmt.Errorf("no `###### Figure N-M.` headings in %s", opts.ConvertedDisplay)
	}

	if err := os.MkdirAll(opts.OutDir, 0o750); err != nil { //#nosec G301 -- author-facing assets directory
		return nil, fmt.Errorf("cannot create output directory %s: %w", opts.OutDir, err)
	}
	previous := readDispositions(filepath.Join(opts.OutDir, "manifest.json"))

	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		return nil, fmt.Errorf("unreadable EPUB %s: %w", opts.EPUBPath, err)
	}
	spine, err := readSpine(zr)
	if err != nil {
		return nil, fmt.Errorf("unreadable EPUB %s: %w", opts.EPUBPath, err)
	}

	byChapter, fidOrder := captionIndex(caps)
	targets := chapterTargets(spine)
	if len(targets) == 0 {
		return nil, fmt.Errorf("no chapter matched: the spine carries no ch<N> XHTML document")
	}
	chapterNums := make([]int, 0, len(targets))
	for _, t := range targets {
		chapterNums = append(chapterNums, t.num)
	}

	manifest := &manifest{
		Source:               opts.SourceDisplay,
		SourceSHA256:         sourceSHA,
		Converted:            opts.ConvertedDisplay,
		Chapters:             chapterNums,
		Scope:                "all chapters",
		Figures:              []manifestFigure{},
		Unmapped:             []UnmappedImage{},
		CaptionsWithoutImage: []string{},
	}
	var findings []Finding
	repeatedFid := map[string]bool{}
	for _, dup := range duplicateCaptions {
		repeatedFid[dup.ID] = true
		findings = append(findings, Finding{Kind: "duplicate-caption", FigureID: dup.ID,
			Message: fmt.Sprintf("Figure %s: caption repeats at converted lines %s; the last occurrence owns the mapping", dup.ID, dup.Lines)})
	}
	findings = append(findings, sharedCaptionFindings(byChapter, caps)...)

	ex := &extractor{
		opts:        opts,
		zr:          zr,
		caps:        caps,
		byChapter:   byChapter,
		previous:    previous,
		repeatedFid: repeatedFid,
		manifest:    manifest,
		seen:        map[string]bool{},
	}
	ex.findings = findings
	ex.sink = newImageSink(opts.OutDir, &ex.findings)
	for _, t := range targets {
		if err := ex.extractChapter(t); err != nil {
			return nil, err
		}
	}
	findings, seen := ex.findings, ex.seen

	active := map[string]bool{}
	for _, n := range chapterNums {
		active[strconv.Itoa(n)] = true
	}
	for _, fid := range fidOrder {
		c := caps[fid]
		if !active[c.Chapter] || seen[fid] {
			continue
		}
		manifest.CaptionsWithoutImage = append(manifest.CaptionsWithoutImage, fid)
	}
	sort.Strings(manifest.CaptionsWithoutImage)
	sortFigures(manifest.Figures)

	manifestPath := filepath.Join(opts.OutDir, "manifest.json")
	if err := writeManifest(manifestPath, manifest); err != nil {
		return nil, err
	}

	result := &Result{
		OutDir:       opts.OutDir,
		ManifestPath: manifestPath,
		Findings:     findings,
		Unmapped:     manifest.Unmapped,
		Missing:      manifest.CaptionsWithoutImage,
	}
	result.Counts.Found = len(activeCaptions(caps, active))
	result.Counts.Placed = len(seen)
	result.Counts.Duplicates = 0
	for _, f := range manifest.Figures {
		if f.PairedBy == "duplicate" {
			result.Counts.Duplicates++
		}
	}
	result.Counts.Dropped = result.Counts.Found - result.Counts.Placed
	result.Counts.UnmappedImages = len(manifest.Unmapped)
	result.Counts.CaptionsWithoutImage = len(manifest.CaptionsWithoutImage)
	return result, nil
}

// ── converted Markdown: captions ──────────────────────────────────────────────

func splitLines(b []byte) []string {
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// duplicateCaption records one figure id the converted Markdown carries more
// than once, with every line it repeats on.
type duplicateCaption struct {
	ID    string
	Lines string
}

// captions returns every `###### Figure N-M.` heading by id, plus the
// duplicates it found. A repeated id keeps its last occurrence, the way the
// proven Python dict behaves.
func captions(lines []string) (map[string]*caption, []duplicateCaption) {
	out := map[string]*caption{}
	firstLine := map[string]int{}
	var dups []duplicateCaption
	for i, line := range lines {
		m := captionRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fid, text := m[1], m[2]
		anchor, refLine := "", i+1
		for j := i - 1; j >= 0; j-- {
			r := figureRefRe.FindStringSubmatch(lines[j])
			if r != nil && r[1] == fid {
				anchor, refLine = r[2], j+1
				break
			}
		}
		start := refLine
		for j := refLine - 1; j >= 0; j-- {
			if strings.TrimSpace(lines[j]) == "" {
				start = j + 2
				break
			}
		}
		if prev, ok := firstLine[fid]; ok {
			dups = append(dups, duplicateCaption{ID: fid,
				Lines: fmt.Sprintf("%d and %d", prev, i+1)})
		} else {
			firstLine[fid] = i + 1
		}
		out[fid] = &caption{ID: fid, Chapter: chapterOf(fid), Text: text, Line: i + 1, Anchor: anchor, Span: fmt.Sprintf("%d-%d", start, i+1)}
	}
	return out, dups
}

// norm normalises a caption or an alt for matching: typographic quotes become
// ASCII, dashes collapse, emphasis markers vanish, a trailing source note is
// stripped and case and whitespace runs are folded. It mirrors the script.
func norm(text string) string {
	text = strings.NewReplacer(
		"’", "'", "‘", "'",
		"“", `"`, "”", `"`,
		"–", "-", "—", "-",
		"`", "", "*", "",
	).Replace(text)
	text = sourceTailRe.ReplaceAllString(text, "")
	text = wsRe.ReplaceAllString(text, " ")
	return strings.ToLower(strings.TrimSpace(text))
}

// captionIndex groups the captions by chapter and normalised text, in
// Markdown order, the way the script's captions_by_chapter does.
func captionIndex(caps map[string]*caption) (map[string]map[string][]string, []string) {
	order := make([]string, 0, len(caps))
	for _, c := range caps {
		order = append(order, c.ID)
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := caps[order[i]], caps[order[j]]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ID < b.ID
	})
	byChapter := map[string]map[string][]string{}
	seenIn := map[string]map[string]bool{}
	for _, fid := range order {
		c := caps[fid]
		if byChapter[c.Chapter] == nil {
			byChapter[c.Chapter] = map[string][]string{}
			seenIn[c.Chapter] = map[string]bool{}
		}
		key := norm(c.Text)
		if !seenIn[c.Chapter][fid] {
			byChapter[c.Chapter][key] = append(byChapter[c.Chapter][key], fid)
			seenIn[c.Chapter][fid] = true
		}
	}
	return byChapter, order
}

// chapterOf is the part of "8-14" before the dash.
func chapterOf(fid string) string {
	return fid[:strings.Index(fid, "-")]
}

// ── EPUB: spine and images ────────────────────────────────────────────────────

// readSpine returns the reading order from the OPF spine.
func readSpine(zr *zip.Reader) ([]spineEntry, error) {
	container, err := zipRead(zr, "META-INF/container.xml")
	if err != nil {
		return nil, err
	}
	m := opfPathRe.FindSubmatch(container)
	if m == nil {
		return nil, fmt.Errorf("container.xml carries no full-path")
	}
	opfPath := string(m[1])
	opf, err := zipRead(zr, opfPath)
	if err != nil {
		return nil, err
	}
	base := path.Dir(opfPath)
	manifestItems := map[string]spineEntry{}
	for _, tag := range itemTagRe.FindAllString(string(opf), -1) {
		id, href := itemIDRe.FindStringSubmatch(tag), itemHRefRe.FindStringSubmatch(tag)
		if id == nil || href == nil {
			continue
		}
		media := ""
		if mm := itemMedRe.FindStringSubmatch(tag); mm != nil {
			media = mm[1]
		}
		manifestItems[id[1]] = spineEntry{Href: posixJoin(base, href[1]), Media: media}
	}
	var refs []string
	for _, m := range itemRefRe.FindAllStringSubmatch(string(opf), -1) {
		refs = append(refs, m[1])
	}
	spine := make([]spineEntry, 0, len(refs))
	for _, ref := range refs {
		if e, ok := manifestItems[ref]; ok {
			spine = append(spine, e)
		}
	}
	return spine, nil
}

// posixJoin joins an OPF-relative base with an item href the way posixpath
// does: an absolute href replaces the base.
func posixJoin(base, href string) string {
	if strings.HasPrefix(href, "/") {
		return path.Clean(href)
	}
	return path.Join(base, href)
}

type chapterTarget struct {
	href string
	num  int
}

// chapterTargets picks the spine's ch<N> XHTML documents in reading order.
func chapterTargets(spine []spineEntry) []chapterTarget {
	var targets []chapterTarget
	for _, e := range spine {
		if !strings.HasPrefix(e.Media, "application/xhtml") {
			continue
		}
		stem := strings.TrimSuffix(path.Base(e.Href), path.Ext(e.Href))
		if !chapterRe.MatchString(stem) {
			continue
		}
		n, err := strconv.Atoi(nonDigitRe.ReplaceAllString(stem, ""))
		if err != nil {
			continue
		}
		targets = append(targets, chapterTarget{href: e.Href, num: n})
	}
	return targets
}

type image struct {
	src string
	alt string
}

// collectImages returns the document's <img src alt> in document order.
func collectImages(doc []byte) ([]image, error) {
	root, err := html.Parse(bytes.NewReader(doc))
	if err != nil {
		return nil, err
	}
	var images []image
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "img") {
			img := image{}
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "src":
					img.src = a.Val
				case "alt":
					img.alt = a.Val
				}
			}
			images = append(images, img)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return images, nil
}

func zipRead(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("cannot read %s in the EPUB: %w", name, err)
			}
			defer func() { _ = rc.Close() }() //nolint:errcheck // read-only handle
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("the EPUB carries no %s", name)
}

// extractor carries one run's state across the per-chapter steps.
type extractor struct {
	opts        Options
	zr          *zip.Reader
	caps        map[string]*caption
	byChapter   map[string]map[string][]string
	previous    map[string]string
	repeatedFid map[string]bool
	sink        *imageSink
	manifest    *manifest
	findings    []Finding
	seen        map[string]bool
}

// extractChapter places every image of one spine chapter, then pairs what
// remains by position when the counts agree.
func (ex *extractor) extractChapter(t chapterTarget) error {
	chapter := strconv.Itoa(t.num)
	byCaption := ex.byChapter[chapter]
	doc, err := zipRead(ex.zr, t.href)
	if err != nil {
		return err
	}
	images, err := collectImages(doc)
	if err != nil {
		return fmt.Errorf("chapter %s (%s): %w", chapter, t.href, err)
	}
	for _, img := range images {
		if err := ex.placeImage(t, chapter, byCaption, img); err != nil {
			return err
		}
	}
	return ex.pairByPosition(t, chapter, byCaption)
}

// placeImage writes one chapter image under its caption's figure id. An alt
// that matched caption text whose every figure is placed is the second image
// of that caption: a caption heading the Markdown repeats owns it under a
// suffixed file, named and never an overwrite; any other second image stays
// in the unmapped pool, where the positional pairing disposes of it exactly
// as the proven script does.
func (ex *extractor) placeImage(t chapterTarget, chapter string, byCaption map[string][]string, img image) error {
	alt := norm(img.alt)
	if fid := firstUnseen(byCaption[alt], ex.seen); fid != "" {
		return ex.place(t, img, fid, "", "")
	}
	fids := byCaption[alt]
	if alt != "" && len(fids) > 0 && ex.repeatedFid[fids[0]] {
		fid := fids[0]
		name := fmt.Sprintf("figure-%s.1%s", fid, imageExt(t.href, img.src))
		return ex.place(t, img, fid, name, "duplicate")
	}
	ex.manifest.Unmapped = append(ex.manifest.Unmapped, UnmappedImage{
		Chapter: chapter, XHTML: t.href, Src: img.src, Alt: img.alt,
	})
	return nil
}

// pairByPosition pairs the chapter's captionless images with its imageless
// captions only when the counts agree, so a decorative image can never be
// assigned a caption it does not belong to. Strict mode refuses a guess when
// several candidates could pair, naming the captions and the candidate
// positions instead.
func (ex *extractor) pairByPosition(t chapterTarget, chapter string, byCaption map[string][]string) error {
	chapterUnmapped := unmappedOf(ex.manifest.Unmapped, chapter)
	chapterMissing := missingOf(byCaption, ex.seen, ex.caps)
	if len(chapterUnmapped) == 0 || len(chapterUnmapped) != len(chapterMissing) {
		return nil
	}
	if ex.opts.StrictPosition && len(chapterUnmapped) > 1 {
		return fmt.Errorf("ambiguous positional pairing in chapter %s: %d images without caption and %d captions without image; candidates: %s; re-run without --strict-position to pair by position",
			chapter, len(chapterUnmapped), len(chapterMissing),
			ambiguityDetail(chapterMissing, ex.caps, chapterUnmapped, t.href))
	}
	for i, u := range chapterUnmapped {
		fid := chapterMissing[i]
		name := fmt.Sprintf("figure-%s%s", fid, imageExt(t.href, u.Src))
		if err := ex.place(t, image{src: u.Src, alt: u.Alt}, fid, name, "position"); err != nil {
			return err
		}
	}
	ex.manifest.Unmapped = dropUnmapped(ex.manifest.Unmapped, chapterUnmapped)
	return nil
}

// place writes one image and records its manifest entry. An empty name means
// the caption's own filename; an empty alt is a named finding on every path
// that places its image.
func (ex *extractor) place(t chapterTarget, img image, fid, name, pairedBy string) error {
	if name == "" {
		name = fmt.Sprintf("figure-%s%s", fid, imageExt(t.href, img.src))
	}
	name, err := ex.sink.write(ex.zr, t.href, img.src, fid, name)
	if err != nil {
		return err
	}
	ex.seen[fid] = true
	ex.manifest.Figures = append(ex.manifest.Figures, figureEntry(fid, t, img, name, ex.caps[fid], ex.previous, pairedBy))
	if img.alt == "" {
		ex.findings = append(ex.findings, Finding{Kind: "empty-alt", FigureID: fid,
			Message: fmt.Sprintf("Figure %s: empty alt; image paired by position (converted line %d)", fid, ex.caps[fid].Line)})
	}
	if pairedBy == "duplicate" {
		ex.findings = append(ex.findings, Finding{Kind: "duplicate-image", FigureID: fid,
			Message: fmt.Sprintf("Figure %s: second image for the repeated caption, placed as %s", fid, name)})
	}
	return nil
}

// ── placement helpers ─────────────────────────────────────────────────────────

func firstUnseen(fids []string, seen map[string]bool) string {
	for _, fid := range fids {
		if !seen[fid] {
			return fid
		}
	}
	return ""
}

// imageExt resolves an image's extension from its zip path, defaulting to
// .png like the script.
func imageExt(chapterHref, src string) string {
	resolved := resolveImage(chapterHref, src)
	if ext := path.Ext(resolved); ext != "" {
		return strings.ToLower(ext)
	}
	return ".png"
}

// resolveImage resolves a document-relative image path against the chapter's
// directory inside the zip.
func resolveImage(chapterHref, src string) string {
	return path.Clean(posixJoin(path.Dir(chapterHref), src))
}

// sharedCaptionFindings names the captions two figure ids carry in the same
// chapter (Figure 8-1 and Figure 8-5 are both "Layered architecture" in the
// pilot): each keeps its own figure id, so the files never collide.
func sharedCaptionFindings(byChapter map[string]map[string][]string, caps map[string]*caption) []Finding {
	chapters := make([]string, 0, len(byChapter))
	for chapter := range byChapter {
		chapters = append(chapters, chapter)
	}
	sort.Strings(chapters)
	var out []Finding
	for _, chapter := range chapters {
		keys := make([]string, 0, len(byChapter[chapter]))
		for key := range byChapter[chapter] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fids := byChapter[chapter][key]
			if len(fids) < 2 {
				continue
			}
			out = append(out, Finding{Kind: "shared-caption-text", FigureID: fids[0],
				Message: fmt.Sprintf("chapter %s: Figures %s share the caption %q; each keeps its own figure id", chapter, strings.Join(fids, ", "), caps[fids[0]].Text)})
		}
	}
	return out
}

// imageSink writes the extracted images and guards every name: a name already
// written this run is never overwritten — the second image keeps a suffixed
// file and the collision is named.
type imageSink struct {
	out      string
	written  map[string]int
	findings *[]Finding
}

func newImageSink(out string, findings *[]Finding) *imageSink {
	return &imageSink{out: out, written: map[string]int{}, findings: findings}
}

// write stores one image under name; on a collision it returns the suffixed
// name it actually used.
func (s *imageSink) write(zr *zip.Reader, chapterHref, src, fid, name string) (string, error) {
	if n, ok := s.written[name]; ok {
		n++
		s.written[name] = n
		ext := path.Ext(name)
		original := name
		name = fmt.Sprintf("%s.%d%s", strings.TrimSuffix(original, ext), n, ext)
		*s.findings = append(*s.findings, Finding{Kind: "duplicate-image", FigureID: fid,
			Message: fmt.Sprintf("Figure %s: %s would be overwritten; the second image kept as %s", fid, original, name)})
	} else {
		s.written[name] = 0
	}
	data, err := zipRead(zr, resolveImage(chapterHref, src))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.out, name), data, 0o644); err != nil { //#nosec G304 G306 -- author-facing asset under the caller's OutDir
		return "", fmt.Errorf("cannot write %s: %w", name, err)
	}
	return name, nil
}

func figureEntry(fid string, t chapterTarget, img image, name string, c *caption, previous map[string]string, pairedBy string) manifestFigure {
	disposition := previous[fid]
	if disposition == "" {
		disposition = "unclassified"
	}
	f := manifestFigure{
		ID: fid, Chapter: strconv.Itoa(t.num), File: name,
		Alt: img.alt, XHTML: resolveImage(t.href, img.src),
		ConvertedLine: c.Line, Anchor: c.Anchor, Span: c.Span,
		Disposition: disposition,
	}
	if pairedBy != "" {
		f.PairedBy = pairedBy
	}
	return f
}

func unmappedOf(unmapped []UnmappedImage, chapter string) []*UnmappedImage {
	var out []*UnmappedImage
	for i := range unmapped {
		if unmapped[i].Chapter == chapter {
			out = append(out, &unmapped[i])
		}
	}
	return out
}

func dropUnmapped(unmapped []UnmappedImage, dropped []*UnmappedImage) []UnmappedImage {
	set := map[*UnmappedImage]bool{}
	for _, d := range dropped {
		set[d] = true
	}
	out := make([]UnmappedImage, 0, len(unmapped))
	for i := range unmapped {
		if !set[&unmapped[i]] {
			out = append(out, unmapped[i])
		}
	}
	return out
}

func missingOf(byCaption map[string][]string, seen map[string]bool, caps map[string]*caption) []string {
	var missing []string
	for _, fids := range byCaption {
		for _, fid := range fids {
			if !seen[fid] {
				missing = append(missing, fid)
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		return caps[missing[i]].Line < caps[missing[j]].Line
	})
	return missing
}

func ambiguityDetail(missing []string, caps map[string]*caption, images []*UnmappedImage, chapterHref string) string {
	var capsPart, imgPart []string
	for _, fid := range missing {
		capsPart = append(capsPart, fmt.Sprintf("Figure %s (converted line %d)", fid, caps[fid].Line))
	}
	for _, u := range images {
		imgPart = append(imgPart, resolveImage(chapterHref, u.Src))
	}
	return fmt.Sprintf("%s ← images %s", strings.Join(capsPart, ", "), strings.Join(imgPart, ", "))
}

// activeCaptions counts the distinct captions of the active chapters.
func activeCaptions(caps map[string]*caption, active map[string]bool) []string {
	var out []string
	for fid, c := range caps {
		if active[c.Chapter] {
			out = append(out, fid)
		}
	}
	return out
}

func sortFigures(figures []manifestFigure) {
	sort.Slice(figures, func(i, j int) bool {
		return figureLess(figures[i].ID, figures[j].ID)
	})
}

func figureLess(a, b string) bool {
	ab, bb := strings.SplitN(a, "-", 2), strings.SplitN(b, "-", 2)
	if an, bn := partNum(ab[0]), partNum(bb[0]); an != bn {
		return an < bn
	}
	return partNum(ab[1]) < partNum(bb[1])
}

// partNum parses one numeric part of a figure id; a non-numeric part sorts
// before every number.
func partNum(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

// readDispositions preserves a previous manifest's dispositions, so a re-run
// is idempotent. An unreadable manifest simply carries none.
func readDispositions(manifestPath string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(manifestPath) //#nosec G304 -- path inside the caller's OutDir
	if err != nil {
		return out
	}
	var prev struct {
		Figures []struct {
			ID          string `json:"id"`
			Disposition string `json:"disposition"`
		} `json:"figures"`
	}
	if json.Unmarshal(data, &prev) != nil {
		return out
	}
	for _, f := range prev.Figures {
		out[f.ID] = f.Disposition
	}
	return out
}

func writeManifest(manifestPath string, m *manifest) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return os.WriteFile(manifestPath, buf.Bytes(), 0o644) //#nosec G304 G306 -- author-facing manifest inside the caller's OutDir
}
