package cmd

import (
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeImageFixture writes a solid image, then lets the caller repaint pixels
// before encoding, so a diff fixture is one function instead of four files.
func writeImageFixture(t *testing.T, path string, w, h int, base color.RGBA, paint func(img *image.RGBA)) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, base)
		}
	}
	if paint != nil {
		paint(img)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestImageInspect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "solid.png")
	writeImageFixture(t, path, 4, 3, color.RGBA{R: 10, G: 20, B: 30, A: 255}, nil)

	out := string(execute(t, imageInspectCmd, nil, path))
	for _, want := range []string{"format png", "4x3", "alpha false", "bytes"} {
		if !strings.Contains(out, want) {
			t.Errorf("inspect output missing %q:\n%s", want, out)
		}
	}
}

func TestImageInspectYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "solid.png")
	writeImageFixture(t, path, 2, 2, color.RGBA{A: 255}, nil)

	out := string(execute(t, imageInspectCmd, nil, path, "--format", "yaml"))
	for _, want := range []string{"format: png", "width: 2", "height: 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("yaml output missing %q:\n%s", want, out)
		}
	}
}

func TestImageInspectAlpha(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alpha.png")
	writeImageFixture(t, path, 2, 2, color.RGBA{A: 255}, func(img *image.RGBA) {
		img.Set(0, 0, color.RGBA{A: 128})
	})

	out := string(execute(t, imageInspectCmd, nil, path))
	if !strings.Contains(out, "alpha true") {
		t.Errorf("inspect output missing alpha true:\n%s", out)
	}
}

func TestImageInspectJPEGAndGIF(t *testing.T) {
	dir := t.TempDir()
	jpegPath := filepath.Join(dir, "photo.jpg")
	gifPath := filepath.Join(dir, "icon.gif")

	img := image.NewRGBA(image.Rect(0, 0, 5, 4))
	f, err := os.Create(jpegPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	f.Close()
	f, err = os.Create(gifPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if out := string(execute(t, imageInspectCmd, nil, jpegPath)); !strings.Contains(out, "format jpeg") {
		t.Errorf("jpeg inspect missed the format:\n%s", out)
	}
	if out := string(execute(t, imageInspectCmd, nil, gifPath)); !strings.Contains(out, "format gif") {
		t.Errorf("gif inspect missed the format:\n%s", out)
	}
}

func TestImageDiffReportsBoxAndRatio(t *testing.T) {
	dir := t.TempDir()
	black := color.RGBA{A: 255}
	before := filepath.Join(dir, "before.png")
	after := filepath.Join(dir, "after.png")
	writeImageFixture(t, before, 4, 4, black, nil)
	writeImageFixture(t, after, 4, 4, black, func(img *image.RGBA) {
		img.Set(1, 2, color.RGBA{R: 255, A: 255})
	})

	out := string(execute(t, imageDiffCmd, nil, before, after))
	for _, want := range []string{
		"pixels 16  differing 1  ratio 0.062500",
		"max delta 255  threshold 0",
		"box x=1 y=2 w=1 h=1",
		"note ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diff output missing %q:\n%s", want, out)
		}
	}
}

func TestImageDiffThreshold(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.png")
	after := filepath.Join(dir, "after.png")
	writeImageFixture(t, before, 2, 1, color.RGBA{A: 255}, nil)
	writeImageFixture(t, after, 2, 1, color.RGBA{A: 255}, func(img *image.RGBA) {
		img.Set(0, 0, color.RGBA{R: 5, A: 255})
	})

	out := string(execute(t, imageDiffCmd, nil, before, after, "--threshold", "5"))
	if !strings.Contains(out, "differing 0") {
		t.Errorf("threshold 5 should absorb a delta of 5:\n%s", out)
	}
	out = string(execute(t, imageDiffCmd, nil, before, after, "--threshold", "4"))
	if !strings.Contains(out, "differing 1") {
		t.Errorf("threshold 4 should count a delta of 5:\n%s", out)
	}
}

func TestImageDiffDimensionMismatch(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.png")
	after := filepath.Join(dir, "after.png")
	writeImageFixture(t, before, 4, 4, color.RGBA{A: 255}, nil)
	writeImageFixture(t, after, 4, 5, color.RGBA{A: 255}, nil)

	out := string(execute(t, imageDiffCmd, nil, before, after))
	if !strings.Contains(out, "dimensions differ: 4x4 vs 4x5") {
		t.Errorf("mismatch not reported:\n%s", out)
	}
	if strings.Contains(out, "differing") {
		t.Errorf("a dimension mismatch must skip the pixel pass:\n%s", out)
	}
}

func TestImageDiffJSONAndOut(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.png")
	after := filepath.Join(dir, "after.png")
	diffPath := filepath.Join(dir, "diff.png")
	writeImageFixture(t, before, 3, 3, color.RGBA{R: 20, G: 20, B: 20, A: 255}, nil)
	writeImageFixture(t, after, 3, 3, color.RGBA{R: 20, G: 20, B: 20, A: 255}, func(img *image.RGBA) {
		img.Set(2, 0, color.RGBA{R: 255, A: 255})
	})

	out := execute(t, imageDiffCmd, nil, before, after, "--out", diffPath, "--format", "json")
	var report imageDiffReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("json output is not a diff report: %v\n%s", err, out)
	}
	if report.Differing != 1 || report.Box == nil || report.Box.X != 2 || report.Box.Y != 0 {
		t.Errorf("report = %+v, want one differing pixel at 2,0", report)
	}
	if report.Out != diffPath {
		t.Errorf("out = %q, want %q", report.Out, diffPath)
	}

	f, err := os.Open(diffPath)
	if err != nil {
		t.Fatalf("diff image not written: %v", err)
	}
	defer f.Close()
	written, err := png.Decode(f)
	if err != nil {
		t.Fatalf("diff image is not a png: %v", err)
	}
	r, g, b, _ := written.At(2, 0).RGBA()
	if r != 0xffff || g != 0 || b != 0 {
		t.Errorf("differing pixel = (%d,%d,%d), want red", r>>8, g>>8, b>>8)
	}
}

func TestImageRefusesBadInput(t *testing.T) {
	dir := t.TempDir()
	notImage := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notImage, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, imageInspectCmd, nil))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, imageInspectCmd, nil, notImage))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, imageDiffCmd, nil, notImage, notImage))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, imageDiffCmd, nil, notImage))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, imageDiffCmd, nil, notImage, notImage, "--threshold", "256"))
	})
}
