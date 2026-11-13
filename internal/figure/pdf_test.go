package figure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// ── fixture: a PDF with images and caption text, generated with pdfcpu ───────

// writePDFBook builds a PDF plus its converted Markdown, and returns their
// paths. Every page carries one real PNG (ImportImages, one page per image,
// in slice order); stamps adds caption text to pages by number (Create over
// the imported PDF appends the text to the page's content stream), the way a
// book's page carries a figure caption under the image.
func writePDFBook(t *testing.T, dir, markdown string, pages int, stamps map[int]string) (pdf, converted string) {
	t.Helper()
	readers := make([]io.Reader, 0, pages)
	for i := 0; i < pages; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		img.Set(2, 2, color.RGBA{R: uint8(40 * (i + 1)), G: 90, B: 160, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		readers = append(readers, bytes.NewReader(buf.Bytes()))
	}
	var withImgs bytes.Buffer
	if err := api.ImportImages(context.Background(), nil, &withImgs, readers, api.DefaultImportConfig(), nil); err != nil {
		t.Fatalf("cannot generate the PDF fixture images: %v", err)
	}
	script := `{"pages": {` + pageStampJSON(pages, stamps) + `}}`
	var out bytes.Buffer
	if err := api.Create(context.Background(), bytes.NewReader(withImgs.Bytes()), strings.NewReader(script), &out, nil); err != nil {
		t.Fatalf("cannot generate the PDF fixture text: %v", err)
	}
	pdf = filepath.Join(dir, "fixturebook.pdf")
	if err := os.WriteFile(pdf, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	converted = filepath.Join(dir, "fixturebook.md")
	if err := os.WriteFile(converted, []byte(markdown), 0o644); err != nil {
		t.Fatal(err)
	}
	return pdf, converted
}

// pageStampJSON renders the create-JSON pages: every page must be present in
// order (the create pass renders the map by order, not by key), and a
// stamped page appends its caption text to the page's content.
func pageStampJSON(pages int, stamps map[int]string) string {
	var b strings.Builder
	for i := 1; i <= pages; i++ {
		if i > 1 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q: {\"content\": {", fmt.Sprint(i))
		if text, ok := stamps[i]; ok {
			value, err := json.Marshal(text)
			if err != nil {
				continue
			}
			fmt.Fprintf(&b, `"text": [{"value": %s, "font": {"name": "Helvetica", "size": 12}, "anchor": "c"}]`, value)
		}
		b.WriteString("}}")
	}
	return b.String()
}

func pdfOptsFor(pdf, converted, out string, strict bool) Options {
	return Options{
		EPUBPath:         pdf,
		SourceDisplay:    "context/refs/fixturebook.pdf",
		ConvertedPath:    converted,
		ConvertedDisplay: "context/refs/converted/fixturebook-000000000000.md",
		OutDir:           out,
		StrictPosition:   strict,
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestExtractPDFPlacesByPageText(t *testing.T) {
	// Each page carries one image and its caption's text: the caption's own
	// page is the image's page. Both pairing modes must place both figures.
	dir := t.TempDir()
	md := "###### Figure 1-1. First thing\n\n###### Figure 1-2. Second thing\n"
	stamps := map[int]string{1: "Figure 1-1. First thing", 2: "Figure 1-2. Second thing"}
	pdf, converted := writePDFBook(t, dir, md, 2, stamps)

	for _, strict := range []bool{false, true} {
		out := filepath.Join(dir, fmt.Sprintf("out-%v", strict))
		res, err := Extract(pdfOptsFor(pdf, converted, out, strict))
		if err != nil {
			t.Fatalf("strict=%v: %v", strict, err)
		}
		m := readManifest(t, filepath.Join(out, "manifest.json"))
		if len(m.Figures) != 2 {
			t.Fatalf("strict=%v figures = %+v", strict, m.Figures)
		}
		if m.Figures[0].ID != "1-1" || m.Figures[1].ID != "1-2" {
			t.Errorf("pairing = %s/%s", m.Figures[0].ID, m.Figures[1].ID)
		}
		wantPages := []string{"page 1", "page 2"}
		for i, f := range m.Figures {
			if f.PairedBy != "page-text" {
				t.Errorf("figure %s paired_by = %q, want page-text", f.ID, f.PairedBy)
			}
			if f.Alt != "" {
				t.Errorf("figure %s carries alt %q; a PDF has none", f.ID, f.Alt)
			}
			if f.XHTML != wantPages[i] {
				t.Errorf("figure %s xhtml = %q, want %s", f.ID, f.XHTML, wantPages[i])
			}
			if _, err := os.Stat(filepath.Join(out, f.File)); err != nil {
				t.Errorf("image %s missing: %v", f.File, err)
			}
		}
		if res.Counts.Found != 2 || res.Counts.Placed != 2 || res.Counts.Dropped != 0 {
			t.Errorf("counts = %+v", res.Counts)
		}
		if len(res.Findings) != 0 {
			t.Errorf("findings = %+v", res.Findings)
		}
	}
}

func TestExtractPDFRecordsTheAmbiguityAndDrops(t *testing.T) {
	// One page stamped with two figure labels carries one image: the page
	// cannot host both captions' images, and the ambiguity is never guessed.
	dir := t.TempDir()
	md := "###### Figure 1-1. First thing\n\n###### Figure 1-2. Second thing\n"
	stamps := map[int]string{2: "Figure 1-1. First thing Figure 1-2. Second thing"}
	pdf, converted := writePDFBook(t, dir, md, 2, stamps)
	out := filepath.Join(dir, "out")

	res, err := Extract(pdfOptsFor(pdf, converted, out, false))
	if err != nil {
		t.Fatalf("default mode must record, not refuse: %v", err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Figures) != 0 {
		t.Fatalf("figures = %+v, want none placed", m.Figures)
	}
	if len(m.Unmapped) != 2 {
		t.Fatalf("unmapped = %+v, want both page images recorded", m.Unmapped)
	}
	for i, want := range []string{"page 1", "page 2"} {
		if m.Unmapped[i].Src != want || m.Unmapped[i].Alt != "" {
			t.Errorf("unmapped[%d] = %+v, want src %s and no alt", i, m.Unmapped[i], want)
		}
	}
	if len(m.CaptionsWithoutImage) != 2 {
		t.Fatalf("captions_without_image = %v", m.CaptionsWithoutImage)
	}
	if res.Counts.Found != 2 || res.Counts.Placed != 0 || res.Counts.Dropped != 2 {
		t.Errorf("counts = %+v", res.Counts)
	}
	found := false
	for _, f := range res.Findings {
		if f.Kind == "ambiguous-page" && strings.Contains(f.Message, "page 2") &&
			strings.Contains(f.Message, "Figure 1-1") && strings.Contains(f.Message, "Figure 1-2") {
			found = true
		}
	}
	if !found {
		t.Errorf("no ambiguous-page finding among %+v", res.Findings)
	}
	if _, err := os.Stat(filepath.Join(out, "figure-1-1.png")); err == nil {
		t.Error("an unpaired image must not be written under a figure name")
	}
}

func TestExtractPDFStrictRefusesTheAmbiguity(t *testing.T) {
	dir := t.TempDir()
	md := "###### Figure 1-1. First thing\n\n###### Figure 1-2. Second thing\n"
	stamps := map[int]string{2: "Figure 1-1. First thing Figure 1-2. Second thing"}
	pdf, converted := writePDFBook(t, dir, md, 2, stamps)
	out := filepath.Join(dir, "out")

	_, err := Extract(pdfOptsFor(pdf, converted, out, true))
	if err == nil {
		t.Fatal("strict mode accepted an ambiguous page")
	}
	msg := err.Error()
	for _, want := range []string{"Figure 1-1", "page 2", "1 image"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not name %s", msg, want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err == nil {
		t.Error("a refused run must not write a manifest")
	}
}

func TestExtractPDFRecordsTheCaptionNoPageLocates(t *testing.T) {
	// No page carries the caption's label: the caption is recorded without an
	// image, its page image stays unmapped — both modes.
	dir := t.TempDir()
	md := "###### Figure 1-1. A diagram\n"
	pdf, converted := writePDFBook(t, dir, md, 2, map[int]string{1: "some prose without a figure label"})

	for _, strict := range []bool{false, true} {
		out := filepath.Join(dir, fmt.Sprintf("out-%v", strict))
		res, err := Extract(pdfOptsFor(pdf, converted, out, strict))
		if err != nil {
			t.Fatalf("strict=%v: recording a caption no page locates is not a guess: %v", strict, err)
		}
		m := readManifest(t, filepath.Join(out, "manifest.json"))
		if len(m.Figures) != 0 {
			t.Fatalf("strict=%v figures = %+v", strict, m.Figures)
		}
		if len(m.CaptionsWithoutImage) != 1 || m.CaptionsWithoutImage[0] != "1-1" {
			t.Errorf("strict=%v captions_without_image = %v", strict, m.CaptionsWithoutImage)
		}
		if len(m.Unmapped) != 2 {
			t.Errorf("strict=%v unmapped = %+v, want both images recorded", strict, m.Unmapped)
		}
		found := false
		for _, f := range res.Findings {
			if f.Kind == "no-label-page" && strings.Contains(f.Message, "Figure 1-1") {
				found = true
			}
		}
		if !found {
			t.Errorf("strict=%v no no-label-page finding among %+v", strict, res.Findings)
		}
	}
}
