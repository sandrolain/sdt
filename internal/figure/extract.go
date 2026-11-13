// Package figure extracts a book's chapter figures out of a source (an EPUB
// or a PDF) and pairs them with the captions the converted Markdown carries,
// into a manifest.
//
// The conversion keeps the caption and drops the image; this package puts the
// image back where an author can read it. It is the pure-Go port of
// context/scripts/figures-extract.py: the EPUB is a zip, the spine comes from
// the OPF, every <img> pairs with its alt text, and the alt maps to the
// converted `###### Figure N-M. <caption>` line. The mapping is a string
// match, not a vision call.
//
// A PDF source carries no alt text and no file paths: its pairing is purely
// positional — the converted captions in reading order against the extracted
// images in page order (source.go's seam). A count mismatch is never guessed:
// the default records the unpaired ones as dropped, --strict-position refuses
// the ambiguity naming the counts and the first divergence point.
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

// Options carries one extraction run's inputs. EPUBPath (an .epub or a .pdf —
// the name is the pilot's) and ConvertedPath are the files to read;
// SourceDisplay and ConvertedDisplay are the paths written into the manifest
// (relative, when the files live under the project root); OutDir receives the
// images and manifest.json.
type Options struct {
	EPUBPath         string
	SourceDisplay    string
	ConvertedPath    string
	ConvertedDisplay string
	OutDir           string
	StrictPosition   bool
}

// Finding is one named anomaly of the run: an image whose alt is empty, a
// caption heading that repeats, a second image matching a caption that is
// already placed, a positional count mismatch, a caption no PDF page locates,
// or a page whose caption/image ambiguity cannot be resolved. Findings are
// reported, never silent, and never enter the manifest schema.
type Finding struct {
	Kind     string `json:"kind"` // empty-alt | duplicate-caption | duplicate-image | shared-caption-text | count-mismatch | no-label-page | ambiguous-page | unclaimed-images
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

// UnmappedImage is an image in the source that matched no caption.
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

// ── manifest schema (the existing manifests are the contract) ─────────────────

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

// Extract runs one extraction: read the source's images through the seam
// (source.go), pair every image with its caption, write the images and
// manifest.json into OutDir. Dispositions of a previous manifest in OutDir are
// preserved, so a re-run is idempotent.
func Extract(opts Options) (*Result, error) {
	sourceBytes, err := os.ReadFile(opts.EPUBPath) //#nosec G304 -- caller-provided input path
	if err != nil {
		return nil, fmt.Errorf("unreadable input: %s: %w", opts.EPUBPath, err)
	}
	digest := sha256.Sum256(sourceBytes)
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

	imgs, err := sourceImages(opts.EPUBPath)
	if err != nil {
		return nil, err
	}

	byChapter, fidOrder := captionIndex(caps)
	pdf := strings.EqualFold(filepath.Ext(opts.EPUBPath), ".pdf")

	man := &manifest{
		Source:               opts.SourceDisplay,
		SourceSHA256:         sourceSHA,
		Converted:            opts.ConvertedDisplay,
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
		caps:        caps,
		byChapter:   byChapter,
		previous:    previous,
		repeatedFid: repeatedFid,
		manifest:    man,
		seen:        map[string]bool{},
	}
	ex.findings = findings
	ex.sink = newImageSink(opts.OutDir, &ex.findings)

	if pdf {
		man.Chapters = captionChapters(fidOrder, caps)
		if err := ex.extractPDF(imgs, fidOrder); err != nil {
			return nil, err
		}
	} else {
		if man.Chapters, err = epubChapters(opts.EPUBPath); err != nil {
			return nil, err
		}
		if err := ex.extractEPUB(imgs); err != nil {
			return nil, err
		}
	}

	active := map[string]bool{}
	for _, n := range man.Chapters {
		active[strconv.Itoa(n)] = true
	}
	for _, fid := range fidOrder {
		c := caps[fid]
		if !active[c.Chapter] || ex.seen[fid] {
			continue
		}
		man.CaptionsWithoutImage = append(man.CaptionsWithoutImage, fid)
	}
	sort.Strings(man.CaptionsWithoutImage)
	sortFigures(man.Figures)

	manifestPath := filepath.Join(opts.OutDir, "manifest.json")
	if err := writeManifest(manifestPath, man); err != nil {
		return nil, err
	}

	result := &Result{
		OutDir:       opts.OutDir,
		ManifestPath: manifestPath,
		Findings:     ex.findings,
		Unmapped:     man.Unmapped,
		Missing:      man.CaptionsWithoutImage,
	}
	result.Counts.Found = len(activeCaptions(caps, active))
	result.Counts.Placed = len(ex.seen)
	result.Counts.Duplicates = 0
	for _, f := range man.Figures {
		if f.PairedBy == "duplicate" {
			result.Counts.Duplicates++
		}
	}
	result.Counts.Dropped = result.Counts.Found - result.Counts.Placed
	result.Counts.UnmappedImages = len(man.Unmapped)
	result.Counts.CaptionsWithoutImage = len(man.CaptionsWithoutImage)
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

// captionChapters lists the distinct chapters the captions name, sorted
// numerically — the chapters a PDF run is active over.
func captionChapters(fidOrder []string, caps map[string]*caption) []int {
	seen := map[int]bool{}
	var out []int
	for _, fid := range fidOrder {
		n, err := strconv.Atoi(caps[fid].Chapter)
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Ints(out)
	return out
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

type htmlImage struct {
	src string
	alt string
}

// collectImages returns the document's <img src alt> in document order.
func collectImages(doc []byte) ([]htmlImage, error) {
	root, err := html.Parse(bytes.NewReader(doc))
	if err != nil {
		return nil, err
	}
	var images []htmlImage
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "img") {
			img := htmlImage{}
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

// ── pairing ───────────────────────────────────────────────────────────────────

// extractor carries one run's state across the per-chapter steps.
type extractor struct {
	opts        Options
	caps        map[string]*caption
	byChapter   map[string]map[string][]string
	previous    map[string]string
	repeatedFid map[string]bool
	sink        *imageSink
	manifest    *manifest
	findings    []Finding
	seen        map[string]bool
}

// extractEPUB walks the collected images in spine order: the images of one
// chapter run are placed by alt, then the chapter's captionless remainder is
// paired by position when the counts agree — the proven script's order.
func (ex *extractor) extractEPUB(imgs []rawImage) error {
	for i := 0; i < len(imgs); {
		j := i
		for j < len(imgs) && imgs[j].chapter == imgs[i].chapter {
			j++
		}
		chapter := imgs[i].chapter
		byCaption := ex.byChapter[chapter]
		base := len(ex.manifest.Unmapped)
		var pool []rawImage
		for _, img := range imgs[i:j] {
			if err := ex.placeImage(chapter, byCaption, img, &pool); err != nil {
				return err
			}
		}
		if err := ex.pairByPosition(chapter, byCaption, pool, base); err != nil {
			return err
		}
		i = j
	}
	return nil
}

// placeImage writes one chapter image under its caption's figure id. An alt
// that matched caption text whose every figure is placed is the second image
// of that caption: a caption heading the Markdown repeats owns it under a
// suffixed file, named and never an overwrite; any other second image stays
// in the unmapped pool, where the positional pairing disposes of it exactly
// as the proven script does.
func (ex *extractor) placeImage(chapter string, byCaption map[string][]string, img rawImage, pool *[]rawImage) error {
	alt := norm(img.alt)
	if fid := firstUnseen(byCaption[alt], ex.seen); fid != "" {
		return ex.place(chapter, img, fid, "", "")
	}
	fids := byCaption[alt]
	if alt != "" && len(fids) > 0 && ex.repeatedFid[fids[0]] {
		fid := fids[0]
		name := fmt.Sprintf("figure-%s.1%s", fid, img.ext)
		if err := ex.place(chapter, img, fid, name, "duplicate"); err != nil {
			return err
		}
		ex.findings = append(ex.findings, Finding{Kind: "duplicate-image", FigureID: fid,
			Message: fmt.Sprintf("Figure %s: second image for the repeated caption, placed as %s", fid, name)})
		return nil
	}
	*pool = append(*pool, img)
	ex.manifest.Unmapped = append(ex.manifest.Unmapped, UnmappedImage{
		Chapter: chapter, XHTML: img.href, Src: img.src, Alt: img.alt,
	})
	return nil
}

// pairByPosition pairs the chapter's captionless images with its imageless
// captions only when the counts agree, so a decorative image can never be
// assigned a caption it does not belong to. Strict mode refuses a guess when
// several candidates could pair, naming the captions and the candidate
// positions instead.
func (ex *extractor) pairByPosition(chapter string, byCaption map[string][]string, pool []rawImage, base int) error {
	chapterMissing := missingOf(byCaption, ex.seen, ex.caps)
	if len(pool) == 0 || len(pool) != len(chapterMissing) {
		return nil
	}
	if ex.opts.StrictPosition && len(pool) > 1 {
		return fmt.Errorf("ambiguous positional pairing in chapter %s: %d images without caption and %d captions without image; candidates: %s; re-run without --strict-position to pair by position",
			chapter, len(pool), len(chapterMissing),
			ambiguityDetail(chapterMissing, ex.caps, pool))
	}
	for i, img := range pool {
		fid := chapterMissing[i]
		name := fmt.Sprintf("figure-%s%s", fid, img.ext)
		if err := ex.place(chapter, img, fid, name, "position"); err != nil {
			return err
		}
		if img.alt == "" {
			ex.findings = append(ex.findings, Finding{Kind: "empty-alt", FigureID: fid,
				Message: fmt.Sprintf("Figure %s: empty alt; image paired by position (converted line %d)", fid, ex.caps[fid].Line)})
		}
	}
	ex.manifest.Unmapped = ex.manifest.Unmapped[:base]
	return nil
}

// extractPDF pairs the PDF's images with the converted captions by page text:
// a page whose text carries the reference "Figure N-M" is the caption's own
// page, and the image on it is the figure — a PDF carries no alt text, and
// books caption their figures under or near the image. When several pages
// mention the label, the caption's text confirms the page (preferring pages
// that carry an image). A page matched by more than one caption, or a caption
// whose matched page carries more than one image, is an ambiguity, never a
// guess: strict mode refuses it naming the counts and the first divergence
// point, the default records every unpaired image and caption as dropped.
// What the pass leaves unpaired — covers, logos, decorations, and the captions
// no page locates — is recorded as unpaired; the positional stream is
// structurally wrong for PDFs, so no positional fallback runs here.
func (ex *extractor) extractPDF(imgs []rawImage, fidOrder []string) error {
	pages, err := pdfPageTexts(ex.opts.EPUBPath)
	if err != nil {
		return err
	}
	imgsByPage := pdfImagesByPage(imgs)
	resolution := ex.pdfResolveCaptions(fidOrder, pages, imgsByPage)
	if err := ex.pdfStrictGate(fidOrder, resolution, imgsByPage); err != nil {
		return err
	}
	placedPages, err := ex.pdfPlacePlaced(resolution, imgsByPage)
	if err != nil {
		return err
	}
	ex.pdfRecordDrops(resolution, imgs, imgsByPage, placedPages)
	return nil
}

// pdfResolution carries one PDF pass's caption-to-page resolution.
type pdfResolution struct {
	pageCaptions map[int][]string // the caption's own page → its captions, reading order
	resolvedTo   map[string]int   // caption → its page
	candidates   map[string][]int // caption → the pages mentioning its label
	noLabelPage  []string         // captions no page's text locates
	unresolved   []string         // captions several pages locate, none singled out
}

// pdfImagesByPage groups the PDF's images by page, pdfcpu's order.
func pdfImagesByPage(imgs []rawImage) map[int][]rawImage {
	byPage := map[int][]rawImage{}
	for _, img := range imgs {
		byPage[img.page] = append(byPage[img.page], img)
	}
	return byPage
}

// pdfResolveCaptions locates every caption's own page: by its label, confirmed
// by the caption's text when several pages mention the label. A caption no
// page locates, or several pages locate without a winner, is recorded.
func (ex *extractor) pdfResolveCaptions(fidOrder []string, pages map[int]string, imgsByPage map[int][]rawImage) *pdfResolution {
	normPage := map[int]string{}
	labelPages := map[string][]int{}
	seenPage := map[string]map[int]bool{}
	for pg, text := range pages {
		normPage[pg] = pdfNorm(text)
		for _, m := range pdfLabelRe.FindAllStringSubmatch(strings.ToLower(text), -1) {
			label := m[1] + "-" + m[2]
			if seenPage[label] == nil {
				seenPage[label] = map[int]bool{}
			}
			if !seenPage[label][pg] {
				seenPage[label][pg] = true
				labelPages[label] = append(labelPages[label], pg)
			}
		}
	}
	res := &pdfResolution{
		pageCaptions: map[int][]string{},
		resolvedTo:   map[string]int{},
		candidates:   map[string][]int{},
	}
	for _, fid := range fidOrder {
		cand := labelPages[fid]
		res.candidates[fid] = cand
		switch {
		case len(cand) == 0:
			res.noLabelPage = append(res.noLabelPage, fid)
		case len(cand) == 1:
			res.pageCaptions[cand[0]] = append(res.pageCaptions[cand[0]], fid)
			res.resolvedTo[fid] = cand[0]
		default:
			if page, ok := pdfConfirmPage(fid, cand, ex.caps[fid].Text, normPage, imgsByPage); ok {
				res.pageCaptions[page] = append(res.pageCaptions[page], fid)
				res.resolvedTo[fid] = page
			} else {
				res.unresolved = append(res.unresolved, fid)
			}
		}
	}
	return res
}

// pdfStrictGate refuses the first ambiguity in reading order: a caption whose
// label several pages carry without its text singling one out, or a caption
// whose page cannot host its image alone.
func (ex *extractor) pdfStrictGate(fidOrder []string, res *pdfResolution, imgsByPage map[int][]rawImage) error {
	if !ex.opts.StrictPosition {
		return nil
	}
	for _, fid := range fidOrder {
		if containsString(res.unresolved, fid) {
			return fmt.Errorf("ambiguous page-text pairing in %s: Figure %s is mentioned on pages %v and its caption text does not single one out (all captions: %d); re-run without --strict-position to record the ambiguity and drop the unpaired ones",
				ex.opts.EPUBPath, fid, res.candidates[fid], len(fidOrder))
		}
		if pg, ok := res.resolvedTo[fid]; ok && (len(res.pageCaptions[pg]) > 1 || len(imgsByPage[pg]) > 1) {
			return fmt.Errorf("ambiguous page-text pairing in %s: %s; re-run without --strict-position to record the ambiguity and drop the unpaired ones",
				ex.opts.EPUBPath, pdfStrictDetail(pg, res.pageCaptions[pg], imgsByPage[pg], len(fidOrder)))
		}
	}
	return nil
}

// pdfPlacePlaced places every unambiguous caption on its page's single image
// and reports the pages it placed.
func (ex *extractor) pdfPlacePlaced(res *pdfResolution, imgsByPage map[int][]rawImage) (map[int]bool, error) {
	placedPages := map[int]bool{}
	for _, pg := range sortedIntKeys(res.pageCaptions) {
		fids := res.pageCaptions[pg]
		images := imgsByPage[pg]
		if len(fids) == 1 && len(images) == 1 {
			if err := ex.placePDF(images[0], fids[0]); err != nil {
				return placedPages, err
			}
			placedPages[pg] = true
			continue
		}
		if len(fids) == 1 && len(images) == 0 {
			continue // the page confirms the caption but carries no image: caption_without_image
		}
		// >1 caption or >1 image on the page: the ambiguity is recorded
		ex.findings = append(ex.findings, Finding{Kind: "ambiguous-page", FigureID: fids[0],
			Message: fmt.Sprintf("page %d carries %d image(s) for %s; the ambiguity is recorded, not guessed", pg, len(images), figureList(fids))})
	}
	return placedPages, nil
}

// pdfRecordDrops records what the pass left unpaired: the captions no page
// locates or no page confirms, and the images no placed figure claims —
// covers, logos, decorations — with one finding each.
func (ex *extractor) pdfRecordDrops(res *pdfResolution, imgs []rawImage, imgsByPage map[int][]rawImage, placedPages map[int]bool) {
	if len(res.unresolved) > 0 {
		for _, fid := range res.unresolved {
			ex.findings = append(ex.findings, Finding{Kind: "ambiguous-page", FigureID: fid,
				Message: fmt.Sprintf("Figure %s is mentioned on pages %v and its caption text does not single one out; the ambiguity is recorded, not guessed", fid, res.candidates[fid])})
		}
	}
	if len(res.noLabelPage) > 0 {
		ex.findings = append(ex.findings, Finding{Kind: "no-label-page", FigureID: res.noLabelPage[0],
			Message: fmt.Sprintf("%s: no PDF page carries the label; recorded without an image", figureList(res.noLabelPage))})
	}
	var unclaimed []rawImage
	var unclaimedPages []int
	seenUnclaimed := map[int]bool{}
	for _, img := range imgs {
		if placedPages[img.page] {
			continue
		}
		unclaimed = append(unclaimed, img)
		if !seenUnclaimed[img.page] {
			seenUnclaimed[img.page] = true
			unclaimedPages = append(unclaimedPages, img.page)
		}
	}
	if len(unclaimed) > 0 {
		sort.Ints(unclaimedPages)
		for _, img := range unclaimed {
			ex.manifest.Unmapped = append(ex.manifest.Unmapped, UnmappedImage{Src: img.src})
		}
		ex.findings = append(ex.findings, Finding{Kind: "unclaimed-images",
			Message: fmt.Sprintf("%d image(s) on %d page(s) no placed figure claims (%s…): recorded as unmapped", len(unclaimed), len(unclaimedPages), pageListHead(unclaimedPages, 5))})
	}
}

// pdfNorm keeps only the [a-z0-9] skeleton of a page's text or a caption:
// kerning splits, dashes, quotes and font-encoding garbage all vanish, so a
// caption's glyphs can be found in the page's glyphs even when the encoding
// mangles everything between them.
func pdfNorm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if isPDFWordRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isPDFWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// pdfConfirmPage singles the caption's page out among the pages that mention
// its label: first by the caption's own text appearing in the page's text
// (decreasing prefix cuts, so a font-encoded ligature partway through the
// caption still matches on its head), then by a word-score confirmation, and
// whenever several pages still tie, by preferring the pages that carry an
// image — the caption's own page is the image's page. No single page: not
// confirmed, the caption is recorded instead.
func pdfConfirmPage(fid string, cand []int, captionText string, normPage map[int]string, imgsByPage map[int][]rawImage) (int, bool) {
	matched := pdfContiguousPages(captionText, cand, normPage)
	if len(matched) == 0 {
		matched = pdfWordScoredPages(captionText, cand, normPage)
	}
	if len(matched) > 1 {
		if withImgs := pdfPagesWithImages(matched, imgsByPage); len(withImgs) > 0 {
			matched = withImgs
		}
	}
	if len(matched) == 1 {
		return matched[0], true
	}
	return 0, false
}

// pdfContiguousPages returns the candidate pages whose text contains the
// caption's glyphs, trying the full caption first and shorter heads after.
func pdfContiguousPages(captionText string, cand []int, normPage map[int]string) []int {
	ctext := pdfNorm(captionText)
	for _, cut := range []int{len(ctext), 40, 24, 12} {
		if cut > len(ctext) {
			continue
		}
		var matched []int
		for _, pg := range cand {
			if strings.Contains(normPage[pg], ctext[:cut]) {
				matched = append(matched, pg)
			}
		}
		if len(matched) > 0 {
			return matched
		}
	}
	return nil
}

// pdfWordScoredPages returns the candidate pages sharing enough of the
// caption's longer words — the fallback when the glyphs stop matching
// contiguously, e.g. across a font-encoded ligature.
func pdfWordScoredPages(captionText string, cand []int, normPage map[int]string) []int {
	var strong []string
	total := 0
	for _, w := range pdfWords(captionText) {
		if len(w) >= 4 {
			strong = append(strong, w)
			total += len(w)
		}
	}
	if total == 0 {
		return nil
	}
	var matched []int
	for _, pg := range cand {
		s := 0
		for _, w := range strong {
			if strings.Contains(normPage[pg], w) {
				s += len(w)
			}
		}
		if s*100 >= total*85 && s >= 16 {
			matched = append(matched, pg)
		}
	}
	return matched
}

// pdfWords splits a caption into its alphanumeric tokens, longest first.
func pdfWords(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !isPDFWordRune(r)
	})
	sort.Slice(f, func(i, j int) bool { return len(f[i]) > len(f[j]) })
	return f
}

func pdfPagesWithImages(pages []int, imgsByPage map[int][]rawImage) []int {
	var out []int
	for _, pg := range pages {
		if len(imgsByPage[pg]) > 0 {
			out = append(out, pg)
		}
	}
	return out
}

// pdfStrictDetail names the counts and the first divergence point of a strict
// refusal: the first caption (reading order) whose page is ambiguous.
func pdfStrictDetail(pg int, fids []string, images []rawImage, captionCount int) string {
	capsPart := make([]string, 0, len(fids))
	for _, fid := range fids {
		capsPart = append(capsPart, "Figure "+fid)
	}
	if len(fids) > 1 {
		return fmt.Sprintf("page %d carries %s and %d image(s): the captions cannot be told apart on the page (all captions: %d)", pg, strings.Join(capsPart, ", "), len(images), captionCount)
	}
	return fmt.Sprintf("%s matches page %d carrying %d image(s): which one is the figure is not decidable (all captions: %d)", capsPart[0], pg, len(images), captionCount)
}

func sortedIntKeys(m map[int][]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// figureList renders figure ids comma-separated.
func figureList(fids []string) string {
	parts := make([]string, 0, len(fids))
	for _, fid := range fids {
		parts = append(parts, "Figure "+fid)
	}
	return strings.Join(parts, ", ")
}

// pageListHead renders the head of a page list for a finding.
func pageListHead(pages []int, max int) string {
	parts := make([]string, 0, max)
	for i, pg := range pages {
		if i == max {
			break
		}
		parts = append(parts, fmt.Sprintf("page %d", pg))
	}
	return strings.Join(parts, ", ")
}

// placePDF writes one page-text image under its caption's figure id. The
// pairing is by page text: the caption's own page is the image's page. A PDF
// carries no alt text to report on.
func (ex *extractor) placePDF(img rawImage, fid string) error {
	name := fmt.Sprintf("figure-%s%s", fid, img.ext)
	name, err := ex.sink.writeData(img.data, fid, name)
	if err != nil {
		return err
	}
	ex.seen[fid] = true
	ex.manifest.Figures = append(ex.manifest.Figures,
		figureEntry(fid, chapterOf(fid), img.src, img.alt, name, ex.caps[fid], ex.previous, "page-text"))
	return nil
}

// place writes one image and records its manifest entry. An empty name means
// the caption's own filename; the findings an EPUB pairing reports (an empty
// alt, a repeated caption's second image) belong to its callers, since a PDF
// pairing has no alt and no repetition to report.
func (ex *extractor) place(chapter string, img rawImage, fid, name, pairedBy string) error {
	if name == "" {
		name = fmt.Sprintf("figure-%s%s", fid, img.ext)
	}
	name, err := ex.sink.writeData(img.data, fid, name)
	if err != nil {
		return err
	}
	ex.seen[fid] = true
	ex.manifest.Figures = append(ex.manifest.Figures, figureEntry(fid, chapter, img.src, img.alt, name, ex.caps[fid], ex.previous, pairedBy))
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

// imageExt resolves an image's extension from its resolved source path,
// defaulting to .png like the script.
func imageExt(resolved string) string {
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

// writeData stores one image's bytes under name; on a collision it returns
// the suffixed name it actually used.
func (s *imageSink) writeData(data []byte, fid, name string) (string, error) {
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
	if err := os.WriteFile(filepath.Join(s.out, name), data, 0o644); err != nil { //#nosec G304 G306 -- author-facing asset under the caller's OutDir
		return "", fmt.Errorf("cannot write %s: %w", name, err)
	}
	return name, nil
}

func figureEntry(fid, chapter, xhtml, alt, name string, c *caption, previous map[string]string, pairedBy string) manifestFigure {
	disposition := previous[fid]
	if disposition == "" {
		disposition = "unclassified"
	}
	f := manifestFigure{
		ID: fid, Chapter: chapter, File: name,
		Alt: alt, XHTML: xhtml,
		ConvertedLine: c.Line, Anchor: c.Anchor, Span: c.Span,
		Disposition: disposition,
	}
	if pairedBy != "" {
		f.PairedBy = pairedBy
	}
	return f
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

func ambiguityDetail(missing []string, caps map[string]*caption, images []rawImage) string {
	var capsPart, imgPart []string
	for _, fid := range missing {
		capsPart = append(capsPart, fmt.Sprintf("Figure %s (converted line %d)", fid, caps[fid].Line))
	}
	for _, img := range images {
		imgPart = append(imgPart, img.src)
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
