package figure

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ── fixtures ─────────────────────────────────────────────────────────────────

const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const pngByte = "\x89PNG\r\n\x1a\n0000"

// writeBook builds a minimal EPUB (one chapter) plus its converted Markdown,
// the shape the real books carry: zip + spine + XHTML images and
// `###### Figure N-M.` headings.
func writeBook(t *testing.T, dir, markdown string, chapterXHTML string) (epub, converted string) {
	t.Helper()
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<manifest><item id="ch1" href="ch01.xhtml" media-type="application/xhtml+xml"/></manifest>
<spine><itemref idref="ch1"/></spine>
</package>`
	epub = filepath.Join(dir, "fixturebook.epub")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct{ name, body string }{
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", opf},
		{"OEBPS/ch01.xhtml", chapterXHTML},
		{"OEBPS/assets/f1.png", pngByte},
		{"OEBPS/assets/f2.png", pngByte},
	} {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(epub, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	converted = filepath.Join(dir, "fixturebook.md")
	if err := os.WriteFile(converted, []byte(markdown), 0o644); err != nil {
		t.Fatal(err)
	}
	return epub, converted
}

func optsFor(epub, converted, out string, strict bool) Options {
	return Options{
		EPUBPath:         epub,
		SourceDisplay:    "context/refs/fixturebook.epub",
		ConvertedPath:    converted,
		ConvertedDisplay: "context/refs/converted/fixturebook-000000000000.md",
		OutDir:           out,
		StrictPosition:   strict,
	}
}

func readManifest(t *testing.T, path string) manifest {
	t.Helper()
	data, err := os.ReadFile(path) //#nosec G304 -- test fixture path
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	return m
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestExtractPairsCaptionsByAlt(t *testing.T) {
	dir := t.TempDir()
	md := "# Book\n\n[Figure 1-1](#fig11)\n\nSome prose.\n\n###### Figure 1-1. Some diagram\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png" alt="Some diagram"/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, false))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Figures) != 1 {
		t.Fatalf("figures = %d, want 1", len(m.Figures))
	}
	f := m.Figures[0]
	if f.ID != "1-1" || f.File != "figure-1-1.png" || f.Alt != "Some diagram" {
		t.Errorf("figure = %+v", f)
	}
	if f.XHTML != "OEBPS/assets/f1.png" {
		t.Errorf("xhtml = %q, want the zip path", f.XHTML)
	}
	if f.ConvertedLine != 7 || f.Anchor != "fig11" || f.Span != "3-7" {
		t.Errorf("caption location = line %d anchor %q span %q", f.ConvertedLine, f.Anchor, f.Span)
	}
	if f.Disposition != "unclassified" || f.PairedBy != "" {
		t.Errorf("disposition = %q paired_by = %q", f.Disposition, f.PairedBy)
	}
	if _, err := os.Stat(filepath.Join(out, "figure-1-1.png")); err != nil {
		t.Errorf("image not written: %v", err)
	}
	if res.Counts.Found != 1 || res.Counts.Placed != 1 || res.Counts.Dropped != 0 {
		t.Errorf("counts = %+v", res.Counts)
	}
	if len(res.Findings) != 0 {
		t.Errorf("unexpected findings: %+v", res.Findings)
	}
}

func TestExtractEmptyAltIsNamedFindingPairedByPosition(t *testing.T) {
	dir := t.TempDir()
	md := "[Figure 1-1](#fig11)\n\nprose\n\n###### Figure 1-1. A diagram\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png" alt=""/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, false))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Figures) != 1 || m.Figures[0].PairedBy != "position" || m.Figures[0].Alt != "" {
		t.Fatalf("figures = %+v", m.Figures)
	}
	found := false
	for _, f := range res.Findings {
		if f.Kind == "empty-alt" && f.FigureID == "1-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("no empty-alt finding among %+v", res.Findings)
	}
	if len(m.Unmapped) != 0 || len(m.CaptionsWithoutImage) != 0 {
		t.Errorf("unmapped = %+v missing = %v", m.Unmapped, m.CaptionsWithoutImage)
	}
}

func TestExtractDuplicateCaptionGetsSuffixedFile(t *testing.T) {
	dir := t.TempDir()
	md := "[Figure 1-1](#a)\n\n###### Figure 1-1. Layered architecture\n\nbody\n\n###### Figure 1-1. Layered architecture\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png" alt="Layered architecture"/><img src="assets/f2.png" alt="Layered architecture"/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, false))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	files := []string{m.Figures[0].File, m.Figures[1].File}
	if files[0] != "figure-1-1.png" || files[1] != "figure-1-1.1.png" {
		t.Errorf("files = %v, want figure-1-1.png then figure-1-1.1.png", files)
	}
	if m.Figures[1].PairedBy != "duplicate" {
		t.Errorf("paired_by = %q, want duplicate", m.Figures[1].PairedBy)
	}
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("image %s missing: %v", name, err)
		}
	}
	kinds := map[string]int{}
	for _, f := range res.Findings {
		kinds[f.Kind]++
	}
	if kinds["duplicate-caption"] != 1 || kinds["duplicate-image"] != 1 {
		t.Errorf("findings = %+v", res.Findings)
	}
	if res.Counts.Duplicates != 1 {
		t.Errorf("duplicates = %d, want 1", res.Counts.Duplicates)
	}
}

func TestExtractSecondImageOfOneCaptionJoinsThePositionPool(t *testing.T) {
	// The proven script's own case: two images in a chapter carry the same
	// alt, the caption heading appears once. The second image is not guessed
	// into a suffixed file: it joins the unmapped pool, and the positional
	// pairing (counts agreeing) disposes of it exactly as the script does.
	dir := t.TempDir()
	md := "[Figure 1-1](#a)\n\n###### Figure 1-1. Same caption\n\n[Figure 1-2](#b)\n\n###### Figure 1-2. Other caption\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png" alt="Same caption"/><img src="assets/f1.png" alt="Same caption"/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, false))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Figures) != 2 {
		t.Fatalf("figures = %+v", m.Figures)
	}
	byID := map[string]manifestFigure{m.Figures[0].ID: m.Figures[0], m.Figures[1].ID: m.Figures[1]}
	if f := byID["1-1"]; f.File != "figure-1-1.png" || f.PairedBy != "" {
		t.Errorf("figure 1-1 = %+v", f)
	}
	if f := byID["1-2"]; f.File != "figure-1-2.png" || f.PairedBy != "position" {
		t.Errorf("figure 1-2 = %+v", f)
	}
	if res.Counts.Duplicates != 0 || len(res.Findings) != 0 {
		t.Errorf("counts = %+v findings = %+v", res.Counts, res.Findings)
	}
}

func TestImageSinkNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	epub, _ := writeBook(t, dir, "###### Figure 1-1. A diagram\n", `<html><body><img src="assets/f1.png" alt="A diagram"/></body></html>`)
	data, err := os.ReadFile(epub) //#nosec G304 -- test fixture path
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var findings []Finding
	sink := newImageSink(dir, &findings)
	name, err := sink.write(zr, "OEBPS/ch01.xhtml", "assets/f1.png", "1-1", "figure-1-1.png")
	if err != nil || name != "figure-1-1.png" {
		t.Fatalf("first write = %q, %v", name, err)
	}
	name, err = sink.write(zr, "OEBPS/ch01.xhtml", "assets/f1.png", "1-1", "figure-1-1.png")
	if err != nil {
		t.Fatal(err)
	}
	if name != "figure-1-1.1.png" {
		t.Errorf("second write = %q, want figure-1-1.1.png", name)
	}
	if len(findings) != 1 || findings[0].Kind != "duplicate-image" {
		t.Errorf("findings = %+v", findings)
	}
}

func TestExtractStrictPositionRefusesAmbiguity(t *testing.T) {
	dir := t.TempDir()
	md := "[Figure 1-1](#a)\n\n###### Figure 1-1. First thing\n\n[Figure 1-2](#b)\n\n###### Figure 1-2. Second thing\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png"/><img src="assets/f2.png"/></body></html>`)
	out := filepath.Join(dir, "out")

	_, err := Extract(optsFor(epub, converted, out, true))
	if err == nil {
		t.Fatal("strict mode accepted an ambiguous pairing")
	}
	msg := err.Error()
	for _, want := range []string{"Figure 1-1", "Figure 1-2", "OEBPS/assets/f1.png", "OEBPS/assets/f2.png"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not name %s", msg, want)
		}
	}
}

func TestExtractPositionPairingWhenCountsAgree(t *testing.T) {
	dir := t.TempDir()
	md := "[Figure 1-1](#a)\n\n###### Figure 1-1. First thing\n\n[Figure 1-2](#b)\n\n###### Figure 1-2. Second thing\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png"/><img src="assets/f2.png"/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, false))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if m.Figures[0].ID != "1-1" || m.Figures[1].ID != "1-2" {
		t.Errorf("pairing = %s/%s", m.Figures[0].ID, m.Figures[1].ID)
	}
	for _, f := range m.Figures {
		if f.PairedBy != "position" {
			t.Errorf("figure %s paired_by = %q", f.ID, f.PairedBy)
		}
	}
	if res.Counts.Placed != 2 {
		t.Errorf("placed = %d, want 2", res.Counts.Placed)
	}
}

func TestExtractStrictAllowsUnambiguousPairing(t *testing.T) {
	dir := t.TempDir()
	md := "[Figure 1-1](#a)\n\n###### Figure 1-1. The only thing\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png"/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, true))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Figures) != 1 || m.Figures[0].PairedBy != "position" {
		t.Fatalf("figures = %+v", m.Figures)
	}
	if res.Counts.Placed != 1 {
		t.Errorf("placed = %d, want 1", res.Counts.Placed)
	}
}

func TestExtractPreservesPreviousDispositions(t *testing.T) {
	dir := t.TempDir()
	md := "###### Figure 1-1. A diagram\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png" alt="A diagram"/></body></html>`)
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	previous := filepath.Join(out, "manifest.json")
	if err := os.WriteFile(previous, []byte(`{"figures":[{"id":"1-1","disposition":"redrawn: Mermaid in the guide"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Extract(optsFor(epub, converted, out, false)); err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, previous)
	if len(m.Figures) != 1 || m.Figures[0].Disposition != "redrawn: Mermaid in the guide" {
		t.Errorf("disposition not preserved: %+v", m.Figures)
	}
}

func TestExtractDropsAreRecorded(t *testing.T) {
	dir := t.TempDir()
	md := "###### Figure 1-1. Matched\n###### Figure 1-2. No image in the book\n"
	epub, converted := writeBook(t, dir, md, `<html><body><img src="assets/f1.png" alt="Matched"/></body></html>`)
	out := filepath.Join(dir, "out")

	res, err := Extract(optsFor(epub, converted, out, false))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.CaptionsWithoutImage) != 1 || m.CaptionsWithoutImage[0] != "1-2" {
		t.Errorf("captions_without_image = %v", m.CaptionsWithoutImage)
	}
	if len(m.Unmapped) != 0 {
		t.Errorf("unmapped = %+v", m.Unmapped)
	}
	if res.Counts.Found != 2 || res.Counts.Placed != 1 || res.Counts.Dropped != 1 {
		t.Errorf("counts = %+v", res.Counts)
	}
}

func TestNormMirrorsTheScript(t *testing.T) {
	cases := map[string]string{
		"  Team 1 working  on Sales ":                  "team 1 working on sales",
		"Contexts aligned with subdomains’ boundaries": "contexts aligned with subdomains' boundaries",
		"the “Submit Order” command":                   `the "submit order" command`,
		"integration — a shared kernel":                "integration - a shared kernel",
		"Model transformations":                        "model transformations",
		"Deployment pipeline (source: DORA report)":    "deployment pipeline",
		"`Ports & adapters` architecture":              "ports & adapters architecture",
		"A *bold* claim":                               "a bold claim",
	}
	for in, want := range cases {
		if got := norm(in); got != want {
			t.Errorf("norm(%q) = %q, want %q", in, got, want)
		}
	}
}

// ── golden (c4) ──────────────────────────────────────────────────────────────

// TestGoldenReextractManifestFieldIdentical re-runs the extraction of the two
// already-extracted books into context/tmp/figure-golden/ and asserts the
// manifest is field-identical to the manifest the pilot left behind.
func TestGoldenReextractManifestFieldIdentical(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	goldenDir := filepath.Join(root, "context", "tmp", "figure-golden")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"learningdomain-drivendesign", "softwarearchitecturemetrics"} {
		t.Run(id, func(t *testing.T) {
			epub := filepath.Join(root, "context", "refs", id+".epub")
			previousPath := filepath.Join(root, "context", "assets", "figures", id, "manifest.json")
			for _, p := range []string{epub, previousPath} {
				if _, err := os.Stat(p); err != nil {
					t.Skipf("missing %s: %v", p, err)
				}
			}
			previous, err := os.ReadFile(previousPath) //#nosec G304 -- fixed corpus path
			if err != nil {
				t.Fatal(err)
			}
			// Seed the re-run directory with the pilot's manifest so the
			// human-assigned dispositions survive, exactly like the script's
			// idempotent re-run.
			outDir := filepath.Join(goldenDir, id)
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), previous, 0o644); err != nil {
				t.Fatal(err)
			}
			display, read, err := FindConverted(root, id, epub)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Extract(Options{
				EPUBPath:         epub,
				SourceDisplay:    DisplayPath(root, epub),
				ConvertedPath:    read,
				ConvertedDisplay: display,
				OutDir:           outDir,
			})
			if err != nil {
				t.Fatal(err)
			}

			fresh, err := os.ReadFile(filepath.Join(outDir, "manifest.json")) //#nosec G304 -- fixed golden path
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal(previous, &want); err != nil {
				t.Fatalf("previous manifest unreadable: %v", err)
			}
			if err := json.Unmarshal(fresh, &got); err != nil {
				t.Fatalf("fresh manifest unreadable: %v", err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("manifest is not field-identical to the pilot's")
			}
			entries := got.(map[string]any)["figures"].([]any)
			images, _ := filepath.Glob(filepath.Join(outDir, "figure-*"))
			if len(images) != len(entries) {
				t.Errorf("image files = %d, manifest figures = %d", len(images), len(entries))
			}
			if res.Counts.Dropped != 0 || res.Counts.UnmappedImages != 0 || res.Counts.CaptionsWithoutImage != 0 {
				t.Errorf("golden books carry drops: %+v", res.Counts)
			}
		})
	}
}
