package figure

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// pdfLabelRe matches the figure label as it appears in page text (lowercased):
// a page carrying "Figure N-M" references the figure, and the caption's own
// page is the image's page.
var pdfLabelRe = regexp.MustCompile(`figure (\d+)-(\d+)`)

// ── the source seam ───────────────────────────────────────────────────────────
//
// The EPUB and PDF extraction paths differ only in how the raw images and
// their reading order come out. Everything downstream — the caption index,
// the pairing policy, the sink and the manifest writer — is source-agnostic
// and shared.

// rawImage is one raw image of a source, in reading order.
type rawImage struct {
	data    []byte // viewable image bytes, ready to write (png, jpg, tiff, …)
	ext     string // the file extension, dot included, lowercase
	src     string // the identity the manifest records: the zip path (EPUB), the page label (PDF)
	alt     string // the alt text; a PDF carries none
	chapter string // the spine chapter number; empty for a PDF (pages carry no chapter)
	href    string // the spine document the image belongs to (EPUB); empty for a PDF
	page    int    // the PDF page the image sits on; 0 for an EPUB (pages carry no images)
}

// zipReader opens a zip image held in memory.
func zipReader(data []byte) (*zip.Reader, error) {
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

// sourceImages returns a source's raw images in reading order — the seam the
// EPUB and PDF paths share. A .epub yields its spine chapters' images in
// spine order; a .pdf yields pdfcpu's decoded images in page order.
func sourceImages(path string) ([]rawImage, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub":
		return epubImages(path)
	case ".pdf":
		return pdfImages(path)
	default:
		return nil, fmt.Errorf("unsupported source %s: expected .epub or .pdf", path)
	}
}

// ── EPUB: the spine's images ──────────────────────────────────────────────────

// epubImages returns the EPUB's chapter images in spine order: every <img> of
// every ch<N> XHTML document, resolved against the chapter's directory, with
// its alt text and its image bytes loaded.
func epubImages(path string) ([]rawImage, error) {
	zr, targets, err := openEPUB(path)
	if err != nil {
		return nil, err
	}
	var out []rawImage
	for _, t := range targets {
		doc, err := zipRead(zr, t.href)
		if err != nil {
			return nil, err
		}
		images, err := collectImages(doc)
		if err != nil {
			return nil, fmt.Errorf("chapter %d (%s): %w", t.num, t.href, err)
		}
		for _, img := range images {
			resolved := resolveImage(t.href, img.src)
			data, err := zipRead(zr, resolved)
			if err != nil {
				return nil, err
			}
			out = append(out, rawImage{
				data:    data,
				ext:     imageExt(resolved),
				src:     resolved,
				alt:     img.alt,
				chapter: strconv.Itoa(t.num),
				href:    t.href,
			})
		}
	}
	return out, nil
}

// epubChapters returns the EPUB spine's ch<N> chapter numbers in reading
// order — the chapters the extraction is active over, whether or not a
// chapter carries images.
func epubChapters(path string) ([]int, error) {
	_, targets, err := openEPUB(path)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.num)
	}
	return out, nil
}

// openEPUB reads an EPUB's spine and its ch<N> chapter documents.
func openEPUB(path string) (*zip.Reader, []chapterTarget, error) {
	zipBytes, err := os.ReadFile(path) //#nosec G304 -- caller-provided input path
	if err != nil {
		return nil, nil, fmt.Errorf("unreadable input: %s: %w", path, err)
	}
	zr, err := zipReader(zipBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("unreadable EPUB %s: %w", path, err)
	}
	spine, err := readSpine(zr)
	if err != nil {
		return nil, nil, fmt.Errorf("unreadable EPUB %s: %w", path, err)
	}
	targets := chapterTargets(spine)
	if len(targets) == 0 {
		return nil, nil, fmt.Errorf("no chapter matched: the spine carries no ch<N> XHTML document")
	}
	return zr, targets, nil
}

// ── PDF: pdfcpu's page-order images ──────────────────────────────────────────

// pdfImages returns the PDF's embedded images in page order, decoded and
// re-rendered by pdfcpu into viewable files. The order within a page is
// pdfcpu's canonical object order; the page label ("page N") is the identity
// the manifest records, since a PDF names no file paths and carries no alt
// text.
func pdfImages(path string) ([]rawImage, error) {
	pdfBytes, err := os.ReadFile(path) //#nosec G304 -- caller-provided input path
	if err != nil {
		return nil, fmt.Errorf("unreadable input: %s: %w", path, err)
	}
	pages, err := api.ExtractImagesRaw(context.Background(), bytes.NewReader(pdfBytes), nil, model.NewDefaultConfiguration())
	if err != nil {
		return nil, fmt.Errorf("unreadable PDF %s: %w", path, err)
	}
	var out []rawImage
	for _, images := range pages {
		objNrs := make([]int, 0, len(images))
		for objNr := range images {
			objNrs = append(objNrs, objNr)
		}
		sort.Ints(objNrs)
		for _, objNr := range objNrs {
			img := images[objNr]
			if img.Reader == nil {
				return nil, fmt.Errorf("page %d image obj#%d: no image reader", img.PageNr, objNr)
			}
			data, err := io.ReadAll(img.Reader)
			if err != nil {
				return nil, fmt.Errorf("page %d image obj#%d: %w", img.PageNr, objNr, err)
			}
			fileType := img.FileType
			if fileType == "" {
				fileType = "img"
			}
			out = append(out, rawImage{
				data: data,
				ext:  "." + fileType,
				src:  fmt.Sprintf("page %d", img.PageNr),
				page: img.PageNr,
			})
		}
	}
	return out, nil
}

// ── PDF: the pages' text ──────────────────────────────────────────────────────

// pdfPageTexts returns, per PDF page, the text the page's content stream
// draws: every string literal concatenated in order with no separator. A
// PDF's kerning splits words — even single letters — across literals and TJ
// array elements, and the glyphs themselves stay contiguous, so the joined
// literals reconstruct the page's glyphs without decoding any font: the
// matching below works on the [a-z0-9] skeleton only. Hex strings carry
// font-specific encodings and are skipped. pdfcpu offers no plain-text
// extraction, so this is the page text its content extraction (ExtractContent)
// yields.
func pdfPageTexts(path string) (map[int]string, error) {
	pdfBytes, err := os.ReadFile(path) //#nosec G304 -- caller-provided input path
	if err != nil {
		return nil, fmt.Errorf("unreadable input: %s: %w", path, err)
	}
	texts := map[int]string{}
	err = api.ExtractContent(context.Background(), bytes.NewReader(pdfBytes), nil, func(rd io.Reader, pageNr int) error {
		data, err := io.ReadAll(rd)
		if err != nil {
			return fmt.Errorf("page %d content: %w", pageNr, err)
		}
		texts[pageNr] = pdfPageText(data)
		return nil
	}, model.NewDefaultConfiguration())
	if err != nil {
		return nil, fmt.Errorf("unreadable PDF %s: %w", path, err)
	}
	return texts, nil
}

// pdfPageText concatenates every parenthesized string literal of a decoded
// content stream, in order, with no separator.
func pdfPageText(content []byte) string {
	var b strings.Builder
	for i := 0; i < len(content); i++ {
		if content[i] != '(' {
			continue
		}
		depth := 1
		i++
		start := i
		for i < len(content) && depth > 0 {
			switch content[i] {
			case '\\':
				i += 2
				continue
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					b.Write(pdfUnescapeLiteral(content[start:i]))
				}
			}
			i++
		}
	}
	return b.String()
}

// pdfUnescapeLiteral resolves a PDF string literal's escapes; unknown escapes
// keep their byte.
func pdfUnescapeLiteral(lit []byte) []byte {
	var b []byte
	for i := 0; i < len(lit); i++ {
		c := lit[i]
		if c != '\\' || i+1 >= len(lit) {
			b = append(b, c)
			continue
		}
		i++
		switch lit[i] {
		case 'n', 'r', 't', 'b', 'f':
			b = append(b, ' ')
		case '(', ')', '\\':
			b = append(b, lit[i])
		default:
			if lit[i] >= '0' && lit[i] <= '7' {
				val := 0
				n := 0
				for n < 3 && i < len(lit) && lit[i] >= '0' && lit[i] <= '7' {
					val = val*8 + int(lit[i]-'0')
					i++
					n++
				}
				i--
				b = append(b, byte(val))
			} else {
				b = append(b, '\\', lit[i])
			}
		}
	}
	return b
}
