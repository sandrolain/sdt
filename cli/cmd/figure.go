//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandrolain/sdt/internal/figure"
	"github.com/spf13/cobra"
)

// The figure lane is author-facing: the extracted images and their manifest
// live under context/assets/figures/<source-id>/ and are never referenced from
// wiki, material or deliverable — they are the raw material a redraw decision
// reads. `figure extract` replaces context/scripts/figures-extract.py.

var figureCmd = &cobra.Command{
	Use:   "figure",
	Short: "Extract a book's figures with their captions into assets",
}

var figureExtractCmd = &cobra.Command{
	Use:   "extract <epub|pdf>",
	Short: "Extract one source's chapter figures and write their manifest",
	Long: `Walk a source's images and map them to the converted Markdown's ` + "`###### Figure N-M. <caption>`" + `
headings. The mapping is a string match, not a vision call.

An EPUB is walked by its spine: every <img> pairs with its alt text, and the
alt maps to the caption. A PDF carries no alt text and no file paths: its
pairing is purely positional — the converted captions in reading order
against the extracted images in page order. A count mismatch is never
guessed: the default records the unpaired ones as dropped; --strict-position
refuses the ambiguity, naming the counts and the first divergence point.

The images and manifest.json go to <out>/<source-id>/ (default
context/assets/figures/), in the manifest schema the extracted books already
carry. The converted Markdown is found under context/refs/converted/ by the
source's digest, the name doc2md --keep gives it; --converted names it
explicitly.

An image with no matching caption and a caption with no image are both
recorded in the manifest and named on the output, so a silent drop is
impossible. An empty alt is a named finding; its image is paired by position
only when the chapter's counts agree. A repeated caption is a named finding
and its second image keeps a suffixed filename (figure-8-1.1), never an
overwrite. --strict-position fails instead of guessing when a caption's
positional pairing is ambiguous, naming the caption and the candidate
positions.

The images are author-facing: never referenced from wiki, material or a
deliverable. Exit 0 on a complete mapping; dropped figures exit 3 with the
drops recorded in the manifest.

Examples:
  sdt figure extract context/refs/monolithtomicroservices.epub
  sdt figure extract <source> --out /tmp/figs --strict-position`,
	Args: cobra.ExactArgs(1),
	Run:  runFigureExtract,
}

func runFigureExtract(cmd *cobra.Command, args []string) {
	out := getStringFlag(cmd, "out", false)
	strict := getBoolFlag(cmd, "strict-position", false)
	convertedFlag := getStringFlag(cmd, "converted", false)

	root := sdtProjectRoot()
	if root == "" {
		root = "."
	}
	if out == "" {
		out = filepath.Join(root, "context", "assets", "figures")
	}

	sourceArg := args[0]
	sourceRead := sourceArg
	if !filepath.IsAbs(sourceRead) {
		if _, err := os.Stat(sourceRead); err != nil {
			sourceRead = filepath.Join(root, sourceArg)
		}
	}
	if _, err := os.Stat(sourceRead); err != nil {
		exitWithError(cmd, fmt.Errorf("unreadable input: %s: %w", sourceArg, err))
		return
	}
	sourceID := strings.TrimSuffix(filepath.Base(sourceRead), filepath.Ext(sourceRead))

	convertedRead, convertedDisplay := convertedFlag, convertedFlag
	if convertedRead == "" {
		display, read, err := figure.FindConverted(root, sourceID, sourceRead)
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		convertedRead, convertedDisplay = read, display
	}

	opts := figure.Options{
		EPUBPath:         sourceRead,
		SourceDisplay:    figure.DisplayPath(root, sourceRead),
		ConvertedPath:    convertedRead,
		ConvertedDisplay: convertedDisplay,
		OutDir:           filepath.Join(out, sourceID),
		StrictPosition:   strict,
	}
	res, err := figure.Extract(opts)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	reportFigureResult(cmd, res)
	drops := res.Counts.UnmappedImages + res.Counts.CaptionsWithoutImage
	if drops > 0 {
		if res.Counts.Placed > 0 {
			exitWithJunk(cmd, "figures-unmapped")
			return
		}
		exitWithError(cmd, fmt.Errorf("%d image(s) unmapped and %d caption(s) without image: nothing was placed", res.Counts.UnmappedImages, res.Counts.CaptionsWithoutImage))
	}
}

// reportFigureResult prints the run's report: the counts line the proven
// script printed, then every named finding and every drop.
func reportFigureResult(cmd *cobra.Command, res *figure.Result) {
	switch getFormat(cmd) {
	case fmtJSON:
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			exitWithError(cmd, err)
			return
		}
		outputBytes(cmd, append(data, '\n'))
	default:
		outputString(cmd, fmt.Sprintf("extracted %d figure(s) into %s\n", res.Counts.Placed+res.Counts.Duplicates, res.OutDir))
		outputString(cmd, fmt.Sprintf("manifest: %s\n", res.ManifestPath))
		outputString(cmd, fmt.Sprintf("figures: %d mapped, %d unmapped, %d captions without image\n",
			res.Counts.Placed, res.Counts.UnmappedImages, res.Counts.CaptionsWithoutImage))
		for _, f := range res.Findings {
			outputString(cmd, fmt.Sprintf("finding [%s]: %s\n", f.Kind, f.Message))
		}
		for _, u := range res.Unmapped {
			outputString(cmd, fmt.Sprintf("unmapped image: %s (%s)\n", u.Src, u.Alt))
		}
		for _, fid := range res.Missing {
			outputString(cmd, fmt.Sprintf("caption without image: Figure %s\n", fid))
		}
	}
}

func init() {
	figureExtractCmd.Flags().String("out", "", "output root (default context/assets/figures/)")
	figureExtractCmd.Flags().Bool("strict-position", false, "fail instead of guessing when positional pairing is ambiguous")
	figureExtractCmd.Flags().String("converted", "", "path to the converted Markdown (default: discovered under context/refs/converted/)")
	figureCmd.AddCommand(figureExtractCmd)
	rootCmd.AddCommand(figureCmd)
}
