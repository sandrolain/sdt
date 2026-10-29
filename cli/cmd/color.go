package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// colourValue is a parsed opaque sRGB colour: the input as typed, the
// normalised hex and the channels the luminance maths needs.
type colourValue struct {
	Input     string  `json:"input" yaml:"input"`
	Hex       string  `json:"hex" yaml:"hex"`
	Luminance float64 `json:"luminance" yaml:"luminance"`
	R         uint8   `json:"-" yaml:"-"`
	G         uint8   `json:"-" yaml:"-"`
	B         uint8   `json:"-" yaml:"-"`
}

// wcagVerdicts is the WCAG 2.x threshold set for one pair. Non-text is the
// 3.0 floor for UI components and graphical objects, not a text size.
type wcagVerdicts struct {
	AANormal  bool `json:"aa_normal" yaml:"aa_normal"`
	AALarge   bool `json:"aa_large" yaml:"aa_large"`
	AAANormal bool `json:"aaa_normal" yaml:"aaa_normal"`
	AAALarge  bool `json:"aaa_large" yaml:"aaa_large"`
	NonText   bool `json:"non_text" yaml:"non_text"`
}

type colourReport struct {
	FG     colourValue  `json:"fg" yaml:"fg"`
	BG     colourValue  `json:"bg" yaml:"bg"`
	Ratio  float64      `json:"ratio" yaml:"ratio"`
	WCAG   wcagVerdicts `json:"wcag" yaml:"wcag"`
	Source string       `json:"source" yaml:"source"`
}

const wcagSource = "computed from the two values; a rendered surface can differ (opacity, gradients, blending, images)"

// parseColourValue accepts the shapes a token file actually carries: #rgb,
// #rrggbb, rgb(r, g, b) and rgba(r, g, b, 1). Alpha below 1 is refused rather
// than composited: WCAG contrast is defined between opaque colours, and picking
// a background for the caller would silently answer a different question.
func parseColourValue(s string) (colourValue, error) {
	in := strings.TrimSpace(s)
	if in == "" {
		return colourValue{}, fmt.Errorf("empty colour: use #rgb, #rrggbb, rgb(r, g, b) or rgba(r, g, b, 1)")
	}

	var r, g, b uint8
	switch {
	case strings.HasPrefix(in, "#"):
		hex := in[1:]
		switch len(hex) {
		case 3:
			nr, okr := hexNibble(hex[0])
			ng, okg := hexNibble(hex[1])
			nb, okb := hexNibble(hex[2])
			if !okr || !okg || !okb {
				return colourValue{}, colourSyntaxError(in)
			}
			r, g, b = nr*0x11, ng*0x11, nb*0x11
		case 6:
			v, ok := parseHexByte(hex[0:2])
			if !ok {
				return colourValue{}, colourSyntaxError(in)
			}
			r = v
			if v, ok = parseHexByte(hex[2:4]); !ok {
				return colourValue{}, colourSyntaxError(in)
			}
			g = v
			if v, ok = parseHexByte(hex[4:6]); !ok {
				return colourValue{}, colourSyntaxError(in)
			}
			b = v
		default:
			return colourValue{}, colourSyntaxError(in)
		}
	case strings.HasPrefix(in, "rgb(") || strings.HasPrefix(in, "rgba("):
		channels, alpha, err := parseColourFunction(in)
		if err != nil {
			return colourValue{}, err
		}
		r, g, b = channels[0], channels[1], channels[2]
		if alpha != 1 {
			return colourValue{}, fmt.Errorf("colour %q has alpha %.2f: contrast needs opaque colours, so supply the colour already composited on its background", in, alpha)
		}
	default:
		return colourValue{}, colourSyntaxError(in)
	}

	v := colourValue{Input: in, Hex: fmt.Sprintf("#%02x%02x%02x", r, g, b), R: r, G: g, B: b}
	v.Luminance = relativeLuminance(v)
	return v, nil
}

func colourSyntaxError(in string) error {
	return fmt.Errorf("unsupported colour %q: use #rgb, #rrggbb, rgb(r, g, b) or rgba(r, g, b, 1)", in)
}

func hexNibble(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func parseHexByte(s string) (uint8, bool) {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, false
	}
	return uint8(v), true
}

// parseColourFunction parses the inside of rgb()/rgba(): three integer
// channels and an optional alpha, comma separated.
func parseColourFunction(in string) ([3]uint8, float64, error) {
	var channels [3]uint8
	open := strings.IndexByte(in, '(')
	closeIdx := strings.LastIndexByte(in, ')')
	if open < 0 || closeIdx < open {
		return channels, 0, colourSyntaxError(in)
	}
	parts := strings.Split(in[open+1:closeIdx], ",")
	if len(parts) != 3 && len(parts) != 4 {
		return channels, 0, colourSyntaxError(in)
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil || v < 0 || v > 255 {
			return channels, 0, colourSyntaxError(in)
		}
		channels[i] = uint8(v)
	}
	alpha := 1.0
	if len(parts) == 4 {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if err != nil || v < 0 || v > 1 {
			return channels, 0, colourSyntaxError(in)
		}
		alpha = v
	}
	return channels, alpha, nil
}

// relativeLuminance is the WCAG 2.x definition over sRGB channels: the
// transfer function first, then the 0.2126/0.7152/0.0722 weights. Skipping the
// transfer function is the classic defect that makes a mid-grey pair look
// compliant.
func relativeLuminance(c colourValue) float64 {
	return 0.2126*linearChannel(c.R) + 0.7152*linearChannel(c.G) + 0.0722*linearChannel(c.B)
}

func linearChannel(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// contrastRatio is (lighter + 0.05) / (darker + 0.05), so it is symmetric and
// always in [1, 21].
func contrastRatio(a, b colourValue) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func wcagVerdictsFor(ratio float64) wcagVerdicts {
	return wcagVerdicts{
		AANormal:  ratio >= 4.5,
		AALarge:   ratio >= 3.0,
		AAANormal: ratio >= 7.0,
		AAALarge:  ratio >= 4.5,
		NonText:   ratio >= 3.0,
	}
}

func renderColourReport(r colourReport) string {
	verdict := func(ok bool) string {
		if ok {
			return "pass"
		}
		return "fail"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "fg  %-9s luminance %.4f\n", r.FG.Hex, r.FG.Luminance)
	fmt.Fprintf(&b, "bg  %-9s luminance %.4f\n", r.BG.Hex, r.BG.Luminance)
	fmt.Fprintf(&b, "contrast %.2f:1\n", r.Ratio)
	fmt.Fprintf(&b, "AA  normal 4.5  %s\n", verdict(r.WCAG.AANormal))
	fmt.Fprintf(&b, "AA  large  3.0  %s\n", verdict(r.WCAG.AALarge))
	fmt.Fprintf(&b, "AAA normal 7.0  %s\n", verdict(r.WCAG.AAANormal))
	fmt.Fprintf(&b, "AAA large  4.5  %s\n", verdict(r.WCAG.AAALarge))
	fmt.Fprintf(&b, "non-text   3.0  %s\n", verdict(r.WCAG.NonText))
	return b.String()
}

var colourContrastCmd = &cobra.Command{
	Use:   "contrast <fg> <bg>",
	Short: "WCAG 2.x contrast ratio of two colour values",
	Long: `Report the WCAG 2.x contrast ratio of two colour values.

WHEN — you are choosing or checking token values and need the ratio before a
render exists: a colour pair in a token file, a proposed palette, a review claim
about text on a surface.

WHEN NOT — a rendered surface. Opacity, gradients, blending and images change
what the user sees; the ratio here is computed from the two values you pass, and
the rendered check still belongs to ` + "`browser.md`" + `.

RETURNS — the two normalised colours and their relative luminance, the contrast
ratio, and the five WCAG 2.x thresholds (AA/AAA, normal/large text, non-text
3.0). ` + "`--format json|yaml`" + ` gives the same as one object. The exit status is 0
for any valid pair: a failing ratio is a finding, not a command error.

Accepted input: #rgb, #rrggbb, rgb(r, g, b), rgba(r, g, b, 1). An alpha below 1
is refused, because the composited colour is the one that has a ratio.

EXAMPLE — sdt color contrast '#777777' '#ffffff'`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			exitWithError(cmd, fmt.Errorf("two colours required: sdt color contrast <fg> <bg>"))
			return
		}
		fg, err := parseColourValue(args[0])
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		bg, err := parseColourValue(args[1])
		exitWithError(cmd, err)
		if err != nil {
			return
		}

		ratio := contrastRatio(fg, bg)
		report := colourReport{
			FG:     fg,
			BG:     bg,
			Ratio:  math.Round(ratio*10000) / 10000,
			WCAG:   wcagVerdictsFor(ratio),
			Source: wcagSource,
		}

		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(report, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, append(out, '\n'))
		case fmtYAML:
			out, err := yaml.Marshal(report)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, renderColourReport(report))
		}
	},
}

var colorCmd = &cobra.Command{
	Use:   "color",
	Short: "Colour tools",
	Long:  `Colour tools: WCAG contrast over opaque colour values.`,
}

func init() {
	colorCmd.AddCommand(colourContrastCmd)
	rootCmd.AddCommand(colorCmd)
}
