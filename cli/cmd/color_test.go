package cmd

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestParseColourValue(t *testing.T) {
	cases := []struct {
		in        string
		wantR     uint8
		wantG     uint8
		wantB     uint8
		wantHex   string
		wantError string
	}{
		{in: "#777777", wantR: 0x77, wantG: 0x77, wantB: 0x77, wantHex: "#777777"},
		{in: "#FFF", wantR: 255, wantG: 255, wantB: 255, wantHex: "#ffffff"},
		{in: "#0a0", wantR: 0, wantG: 170, wantB: 0, wantHex: "#00aa00"},
		{in: "rgb(0, 128, 255)", wantR: 0, wantG: 128, wantB: 255, wantHex: "#0080ff"},
		{in: "rgba(1, 2, 3, 1)", wantR: 1, wantG: 2, wantB: 3, wantHex: "#010203"},
		{in: "  #123456  ", wantR: 0x12, wantG: 0x34, wantB: 0x56, wantHex: "#123456"},
		{in: "rgba(0, 128, 255, 0.5)", wantError: "alpha"},
		{in: "rgb(256, 0, 0)", wantError: "unsupported colour"},
		{in: "rgb(0, 0)", wantError: "unsupported colour"},
		{in: "#12345", wantError: "unsupported colour"},
		{in: "#ggg", wantError: "unsupported colour"},
		{in: "#gggggg", wantError: "unsupported colour"},
		{in: "red", wantError: "unsupported colour"},
		{in: "", wantError: "empty colour"},
	}

	for _, tc := range cases {
		got, err := parseColourValue(tc.in)
		if tc.wantError != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("parseColourValue(%q) error = %v, want containing %q", tc.in, err, tc.wantError)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseColourValue(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got.R != tc.wantR || got.G != tc.wantG || got.B != tc.wantB || got.Hex != tc.wantHex {
			t.Errorf("parseColourValue(%q) = (%d,%d,%d,%s), want (%d,%d,%d,%s)",
				tc.in, got.R, got.G, got.B, got.Hex, tc.wantR, tc.wantG, tc.wantB, tc.wantHex)
		}
	}
}

func TestContrastRatio(t *testing.T) {
	black, _ := parseColourValue("#000000")
	white, _ := parseColourValue("#ffffff")
	grey, _ := parseColourValue("#777777")

	if got := contrastRatio(black, white); math.Abs(got-21) > 1e-9 {
		t.Errorf("black on white = %v, want 21", got)
	}
	if got := contrastRatio(white, black); math.Abs(got-21) > 1e-9 {
		t.Errorf("white on black = %v, want 21 (ratio must be symmetric)", got)
	}
	if got := contrastRatio(black, black); math.Abs(got-1) > 1e-9 {
		t.Errorf("black on black = %v, want 1", got)
	}
	if got := contrastRatio(grey, white); math.Abs(got-4.4776) > 0.001 {
		t.Errorf("#777777 on white = %v, want ~4.4776", got)
	}
}

func TestWCAGVerdicts(t *testing.T) {
	cases := []struct {
		ratio float64
		want  wcagVerdicts
	}{
		{7.0, wcagVerdicts{AANormal: true, AALarge: true, AAANormal: true, AAALarge: true, NonText: true}},
		{4.5, wcagVerdicts{AANormal: true, AALarge: true, AAANormal: false, AAALarge: true, NonText: true}},
		{3.0, wcagVerdicts{AANormal: false, AALarge: true, AAANormal: false, AAALarge: false, NonText: true}},
		{2.9, wcagVerdicts{}},
	}
	for _, tc := range cases {
		if got := wcagVerdictsFor(tc.ratio); got != tc.want {
			t.Errorf("wcagVerdictsFor(%v) = %+v, want %+v", tc.ratio, got, tc.want)
		}
	}
}

func TestColourContrastTextOutput(t *testing.T) {
	out := string(execute(t, colourContrastCmd, nil, "#777777", "#ffffff"))

	for _, want := range []string{
		"fg  #777777",
		"bg  #ffffff",
		"contrast 4.48:1",
		"AA  normal 4.5  fail",
		"AA  large  3.0  pass",
		"AAA normal 7.0  fail",
		"AAA large  4.5  fail",
		"non-text   3.0  pass",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

func TestColourContrastJSONOutput(t *testing.T) {
	out := execute(t, colourContrastCmd, nil, "#777777", "#ffffff", "--format", "json")

	var report colourReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("json output is not a colour report: %v\n%s", err, out)
	}
	if math.Abs(report.Ratio-4.4776) > 0.001 {
		t.Errorf("ratio = %v, want ~4.4776", report.Ratio)
	}
	if report.WCAG.AANormal || !report.WCAG.AALarge || !report.WCAG.NonText {
		t.Errorf("verdicts = %+v, want AA normal false, AA large true, non-text true", report.WCAG)
	}
	if report.Source == "" {
		t.Error("source note is empty: the report must say it measures values, not the render")
	}
}

func TestColourContrastRefusesBadInput(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, colourContrastCmd, nil, "#777777"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, colourContrastCmd, nil, "#777777", "rgba(0,0,0,0.5)"))
	})
	shouldExitWithCode(t, 1, func() string {
		return string(execute(t, colourContrastCmd, nil, "#777777", "not-a-colour"))
	})
}
