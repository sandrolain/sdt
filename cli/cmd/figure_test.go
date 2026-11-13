//go:build !wasm

package cmd

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The figure verb's cobra layer: the extraction core lives in internal/figure
// with its own tests; here the command resolves paths, writes the tree and
// reports, on a minimal EPUB fixture shaped like the real books.

const figureContainerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const figureOPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<manifest><item id="ch1" href="ch01.xhtml" media-type="application/xhtml+xml"/></manifest>
<spine><itemref idref="ch1"/></spine>
</package>`

func writeFigureFixture(t *testing.T, dir string) (epub, converted string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct{ name, body string }{
		{"META-INF/container.xml", figureContainerXML},
		{"OEBPS/content.opf", figureOPF},
		{"OEBPS/ch01.xhtml", `<html><body><img src="assets/f1.png" alt="Some diagram"/></body></html>`},
		{"OEBPS/assets/f1.png", "\x89PNG0000"},
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
	epub = filepath.Join(dir, "fixturebook.epub")
	if err := os.WriteFile(epub, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	converted = filepath.Join(dir, "fixturebook.md")
	md := "[Figure 1-1](#a)\n\n###### Figure 1-1. Some diagram\n"
	if err := os.WriteFile(converted, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return epub, converted
}

func TestFigureExtractCommandExtracts(t *testing.T) {
	dir := t.TempDir()
	epub, converted := writeFigureFixture(t, dir)
	out := filepath.Join(dir, "assets")

	got := execute(t, figureExtractCmd, nil, epub, "--out", out, "--converted", converted)
	for _, want := range []string{"figures: 1 mapped, 0 unmapped, 0 captions without image", "manifest:"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
	manifest := filepath.Join(out, "fixturebook", "manifest.json")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	if !strings.Contains(string(data), `"file": "figure-1-1.png"`) {
		t.Errorf("manifest = %s", data)
	}
	if _, err := os.Stat(filepath.Join(out, "fixturebook", "figure-1-1.png")); err != nil {
		t.Errorf("image not written: %v", err)
	}
}

func TestFigureExtractMissingEPUBFails(t *testing.T) {
	captureLogs(t)
	dir := t.TempDir()
	shouldExitWithCode(t, 1, func() string {
		execute(t, figureExtractCmd, nil, filepath.Join(dir, "missing.epub"), "--out", dir)
		return ""
	})
}

func TestFigureExtractJSONReport(t *testing.T) {
	dir := t.TempDir()
	epub, converted := writeFigureFixture(t, dir)
	out := filepath.Join(dir, "assets")

	got := execute(t, figureExtractCmd, nil, epub, "--out", out, "--converted", converted, "--format", "json")
	for _, want := range []string{`"counts"`, `"found": 1`, `"placed": 1`, `"dropped": 0`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("json output missing %s: %s", want, got)
		}
	}
}
