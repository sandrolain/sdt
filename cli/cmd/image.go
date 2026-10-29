package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // register the decoder for inspect/diff
	_ "image/jpeg" // register the decoder for inspect/diff
	"image/png"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// imageInfo is what inspect reports: the facts a capture-validity check needs
// (format, dimensions, alpha) plus the byte size the assets convention budgets.
type imageInfo struct {
	Path   string `json:"path" yaml:"path"`
	Format string `json:"format" yaml:"format"`
	Width  int    `json:"width" yaml:"width"`
	Height int    `json:"height" yaml:"height"`
	Bytes  int64  `json:"bytes" yaml:"bytes"`
	Alpha  bool   `json:"alpha" yaml:"alpha"`
}

// imageBox is the changed region of a pixel diff, in image coordinates.
type imageBox struct {
	X int `json:"x" yaml:"x"`
	Y int `json:"y" yaml:"y"`
	W int `json:"w" yaml:"w"`
	H int `json:"h" yaml:"h"`
}

// imageDiffReport is the diff receipt. DimensionMismatch short-circuits the
// pixel pass: comparing two different grids would answer a different question.
type imageDiffReport struct {
	A                 string    `json:"a" yaml:"a"`
	B                 string    `json:"b" yaml:"b"`
	AWidth            int       `json:"a_width" yaml:"a_width"`
	AHeight           int       `json:"a_height" yaml:"a_height"`
	BWidth            int       `json:"b_width" yaml:"b_width"`
	BHeight           int       `json:"b_height" yaml:"b_height"`
	DimensionMismatch bool      `json:"dimension_mismatch" yaml:"dimension_mismatch"`
	Pixels            int       `json:"pixels" yaml:"pixels"`
	Differing         int       `json:"differing" yaml:"differing"`
	Ratio             float64   `json:"ratio" yaml:"ratio"`
	MaxDelta          int       `json:"max_delta" yaml:"max_delta"`
	Threshold         int       `json:"threshold" yaml:"threshold"`
	Box               *imageBox `json:"box,omitempty" yaml:"box,omitempty"`
	Out               string    `json:"out,omitempty" yaml:"out,omitempty"`
	Note              string    `json:"note" yaml:"note"`
}

const imageDiffNote = "a pixel delta is a regression tripwire, not a verdict: inspect the geometry of what moved"

// decodeImage reads one file and decodes it with the standard decoders. The
// format name comes from the decoder, so a mislabeled extension cannot lie
// about what was actually compared.
func decodeImage(path string) (image.Image, string, []byte, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- user-controlled path
	if err != nil {
		return nil, "", nil, err
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", nil, fmt.Errorf("%s: %w (supported: png, jpeg, gif)", path, err)
	}
	return img, format, data, nil
}

// hasAlpha asks the image's own Opaque method when it has one. Scanning every
// pixel for one non-opaque sample would cost a full pass before the real one.
func hasAlpha(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return !o.Opaque()
	}
	return true
}

// pixelDelta is the largest per-channel difference of two 8-bit RGBA samples.
func pixelDelta(a, b color.Color) int {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	delta := func(x, y uint32) int {
		dx := int(x>>8) - int(y>>8)
		if dx < 0 {
			dx = -dx
		}
		return dx
	}
	d := delta(ar, br)
	if v := delta(ag, bg); v > d {
		d = v
	}
	if v := delta(ab, bb); v > d {
		d = v
	}
	if v := delta(aa, ba); v > d {
		d = v
	}
	return d
}

// diffImages compares two images of equal bounds and returns the receipt. The
// caller checks dimensions first; this function assumes they match.
func diffImages(aName, bName string, a, b image.Image, threshold int) imageDiffReport {
	bounds := a.Bounds()
	report := imageDiffReport{
		A:         aName,
		B:         bName,
		AWidth:    bounds.Dx(),
		AHeight:   bounds.Dy(),
		BWidth:    b.Bounds().Dx(),
		BHeight:   b.Bounds().Dy(),
		Pixels:    bounds.Dx() * bounds.Dy(),
		Threshold: threshold,
		Note:      imageDiffNote,
	}
	var minX, minY, maxX, maxY = -1, -1, -1, -1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			d := pixelDelta(a.At(x, y), b.At(x, y))
			if d > report.MaxDelta {
				report.MaxDelta = d
			}
			if d <= threshold {
				continue
			}
			report.Differing++
			if minX < 0 || x < minX {
				minX = x
			}
			if minY < 0 || y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if report.Pixels > 0 {
		report.Ratio = float64(report.Differing) / float64(report.Pixels)
	}
	if minX >= 0 {
		report.Box = &imageBox{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1}
	}
	return report
}

// writeDiffImage paints every differing pixel red over a dimmed copy of the
// first image, so a reader can see where the change is without the driver.
func writeDiffImage(path string, a, b image.Image, threshold int) error {
	bounds := a.Bounds()
	out := image.NewRGBA(bounds)
	red := color.RGBA{R: 255, A: 255}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if pixelDelta(a.At(x, y), b.At(x, y)) > threshold {
				out.Set(x, y, red)
				continue
			}
			r, g, bb, alpha := a.At(x, y).RGBA()
			//#nosec G115 -- a 16-bit channel shifted by 8 is 0-255, so the narrowing is exact.
			out.Set(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bb >> 8), A: uint8(alpha >> 8)})
		}
	}
	f, err := os.Create(path) //#nosec G304 -- user-controlled path
	if err != nil {
		return err
	}
	if err := png.Encode(f, out); err != nil {
		if cerr := f.Close(); cerr != nil {
			return fmt.Errorf("%w (close: %v)", err, cerr)
		}
		return err
	}
	return f.Close()
}

func renderImageInfo(info imageInfo) string {
	return fmt.Sprintf("%s\nformat %s  %dx%d  %d bytes  alpha %t\n",
		info.Path, info.Format, info.Width, info.Height, info.Bytes, info.Alpha)
}

func renderImageDiff(r imageDiffReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "a  %s  %dx%d\n", r.A, r.AWidth, r.AHeight)
	fmt.Fprintf(&b, "b  %s  %dx%d\n", r.B, r.BWidth, r.BHeight)
	if r.DimensionMismatch {
		fmt.Fprintf(&b, "dimensions differ: %dx%d vs %dx%d — no pixel comparison\n",
			r.AWidth, r.AHeight, r.BWidth, r.BHeight)
		return b.String()
	}
	fmt.Fprintf(&b, "pixels %d  differing %d  ratio %.6f\n", r.Pixels, r.Differing, r.Ratio)
	fmt.Fprintf(&b, "max delta %d  threshold %d\n", r.MaxDelta, r.Threshold)
	if r.Box != nil {
		fmt.Fprintf(&b, "box x=%d y=%d w=%d h=%d\n", r.Box.X, r.Box.Y, r.Box.W, r.Box.H)
	}
	if r.Out != "" {
		fmt.Fprintf(&b, "diff image %s\n", r.Out)
	}
	fmt.Fprintf(&b, "note %s\n", r.Note)
	return b.String()
}

func outputImageReport(cmd *cobra.Command, text string, v any) {
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(v, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, append(out, '\n'))
	case fmtYAML:
		out, err := yaml.Marshal(v)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, text)
	}
}

var imageInspectCmd = &cobra.Command{
	Use:   "inspect <file>",
	Short: "Format, dimensions, bytes and alpha of an image",
	Long: `Report an image's format, dimensions, byte size and whether it carries alpha.

WHEN — a capture or asset needs a validity check: "sane dimensions", the size a
corpus asset is budgeted at, or the format a file actually is rather than the
one its extension claims.

WHEN NOT — a visual claim. Dimensions prove the file decoded, not that the
surface rendered correctly; that is ` + "`browser.md`" + `.

RETURNS — one line per fact, or a JSON object with path, format, width, height,
bytes and alpha. Supported decoders: png, jpeg, gif (the standard library's).

EXAMPLE — sdt image inspect context/assets/brand-mark.png`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			exitWithError(cmd, fmt.Errorf("one image required: sdt image inspect <file>"))
			return
		}
		img, format, data, err := decodeImage(args[0])
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		bounds := img.Bounds()
		info := imageInfo{
			Path:   args[0],
			Format: format,
			Width:  bounds.Dx(),
			Height: bounds.Dy(),
			Bytes:  int64(len(data)),
			Alpha:  hasAlpha(img),
		}
		outputImageReport(cmd, renderImageInfo(info), info)
	},
}

var imageDiffCmd = &cobra.Command{
	Use:   "diff <a> <b>",
	Short: "Dimension-aware pixel diff of two images",
	Long: `Compare two images pixel by pixel and report the difference.

WHEN — a screenshot pair (before/after, light/dark, baseline/current) needs a
number and a location, on a machine without a browser driver.

WHEN NOT — a verdict on a render. A pixel delta is a regression tripwire: inspect
the geometry of what moved, never ship on the number alone (` + "`browser.md`" + `).
Use the adopted driver's own diff when it is installed; this command is the path
that does not need one.

RETURNS — the two dimensions (a mismatch is reported and the pixel pass is
skipped), differing pixels, their ratio, the largest per-channel delta, the
changed bounding box, and the threshold used. ` + "`--out <file>`" + ` writes a PNG with
the differing pixels marked. Supported decoders: png, jpeg, gif.

EXAMPLE — sdt image diff before.png after.png --threshold 8 --out context/tmp/diff.png`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			exitWithError(cmd, fmt.Errorf("two images required: sdt image diff <a> <b>"))
			return
		}
		threshold := getIntFlag(cmd, "threshold", false)
		if threshold < 0 || threshold > 255 {
			exitWithError(cmd, fmt.Errorf("--threshold must be between 0 and 255, got %d", threshold))
			return
		}
		a, _, _, err := decodeImage(args[0])
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		b, _, _, err := decodeImage(args[1])
		exitWithError(cmd, err)
		if err != nil {
			return
		}

		// A dimensions mismatch is a finding, not an error: report it and stop
		// before the pixel pass, which would compare two different grids.
		if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
			report := imageDiffReport{
				A: args[0], B: args[1],
				AWidth: a.Bounds().Dx(), AHeight: a.Bounds().Dy(),
				BWidth: b.Bounds().Dx(), BHeight: b.Bounds().Dy(),
				DimensionMismatch: true,
				Threshold:         threshold,
				Note:              imageDiffNote,
			}
			outputImageReport(cmd, renderImageDiff(report), report)
			return
		}

		report := diffImages(args[0], args[1], a, b, threshold)

		out := getStringFlag(cmd, "out", false)
		if out != "" && !report.DimensionMismatch {
			if err := writeDiffImage(out, a, b, threshold); err != nil {
				exitWithError(cmd, err)
				return
			}
			report.Out = out
		}
		outputImageReport(cmd, renderImageDiff(report), report)
	},
}

// imageCmdName is the verb's name, shared with totp's image subcommand; one
// literal keeps goconst from treating the same word as three unrelated strings.
const imageCmdName = "image"

var imageCmd = &cobra.Command{
	Use:   imageCmdName,
	Short: "Image tools",
	Long:  `Image tools: inspect dimensions and format, and diff two images without a browser.`,
}

func init() {
	imageDiffCmd.Flags().Int("threshold", 0, "Per-channel difference above which a pixel counts as changed (0-255)")
	imageDiffCmd.Flags().String("out", "", "Write a PNG with the differing pixels marked")
	imageCmd.AddCommand(imageInspectCmd)
	imageCmd.AddCommand(imageDiffCmd)
	rootCmd.AddCommand(imageCmd)
}
