package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/cli/utils/converter"
	"github.com/sandrolain/sdt/internal/doc2md"
	"github.com/spf13/cobra"
)

// doc2mdConvert and doc2mdTools are the exec seams; tests replace them so no
// real converter is required.
var (
	doc2mdConvert = doc2md.Convert
	doc2mdTools   = doc2md.Tools
)

var doc2mdCmd = &cobra.Command{
	Use:     "doc2md <file>...",
	Aliases: []string{"d2m"},
	Short:   "Convert documents to Markdown with the first runnable converter",
	Long: `Convert one or more local documents to GitHub-Flavored Markdown.

The converter chain is tried in order: anydoc, docling, markitdown. A tool
counts as available only when it launches (an exec probe, never a PATH lookup),
and a tool that fails, times out or produces no markdown advances the chain.

Markdown goes to stdout to read. --keep stores the document under
context/refs/converted/<slug>-<sha256[:12]>.md with provenance frontmatter
and prints the path; --output-file writes the same stamped document to an
explicit path. Exit codes are normalized:
  0 success · 1 failed · 2 no runnable converter · 3 PDF needs OCR

This command replaces the removed context/scripts/convert-to-md.sh wrapper
(the old invocation was "context/scripts/convert-to-md.sh <file> [outdir]").`,
	Args: cobra.MinimumNArgs(1),
	Run:  runDoc2md,
}

var doc2mdListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the kept converted documents",
	Long: `List the kept converted corpus. The rows are read from the per-file
frontmatter under context/refs/converted/ (never from index.md), so the
projection can never become a second source of truth. --format json|yaml
prints the machine view.

Examples:
  sdt doc2md list
  sdt doc2md list --format json`,
	Args: cobra.NoArgs,
	Run:  runDoc2mdList,
}

var doc2mdSetCmd = &cobra.Command{
	Use:   "set <file>",
	Short: "Enrich a kept document's summary and links",
	Long: `Set the summary and links of a kept document and refresh both index
projections. <file> is the basename of a file under context/refs/converted/
(or a path inside it). It is the only mutator of the kept corpus; a document
with no relation simply carries no links.

Examples:
  sdt doc2md set report-ab12cd34ef56.md --summary "Q3 financials"
  sdt doc2md set report-ab12cd34ef56.md --link analysis/<analysis-file>.md`,
	Args: cobra.ExactArgs(1),
	Run:  runDoc2mdSet,
}

var doc2mdReindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Regenerate the converted-corpus index projections",
	Long: `Regenerate context/refs/converted/index.md and index.json from the per-file
frontmatter. Neither file is written when unchanged.

Examples:
  sdt doc2md reindex`,
	Args: cobra.NoArgs,
	Run:  runDoc2mdReindex,
}

var doc2mdToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Probe the converter chain and print which tools are runnable",
	Long: `Probe every converter in the chain (anydoc, docling, markitdown) and print
whether it launches, plus its version when it reports one.

A tool is available only if it launches; presence on PATH is not enough.`,
	Args: cobra.NoArgs,
	Run:  runDoc2mdTools,
}

func runDoc2md(cmd *cobra.Command, args []string) {
	timeout, err := cmd.Flags().GetDuration("timeout")
	exitWithError(cmd, err)
	tool := getStringFlag(cmd, "tool", false)
	keep := getBoolFlag(cmd, "keep", false)
	name := getStringFlag(cmd, "name", false)
	outputFile := getStringFlag(cmd, "output-file", false)
	if keep && outputFile != "" {
		exitWithError(cmd, fmt.Errorf("cannot use both --keep and --output-file"))
		return
	}

	results := make([]doc2md.Result, 0, len(args))
	for _, input := range args {
		if converter.DocumentExtension(input) == "" {
			results = append(results, doc2md.Result{
				Status: doc2md.StatusFailed,
				Reason: fmt.Sprintf("unsupported input %q (supported: %s)", input, strings.Join(converter.DocumentExtensions, " ")),
			})
			continue
		}
		res := doc2mdConvert(context.Background(), doc2md.Options{
			Input:   input,
			Timeout: timeout,
			Tool:    tool,
		})
		if res.Status == doc2md.StatusSuccess && (keep || outputFile != "") {
			path, err := writeDoc2mdFile(input, res, keep, outputFile, name)
			if err != nil {
				res.Status = doc2md.StatusFailed
				res.Reason = err.Error()
			} else {
				res.KeptFile = path
				res.Markdown = ""
			}
		}
		results = append(results, res)
	}

	switch getFormat(cmd) {
	case fmtJSON:
		data, err := json.MarshalIndent(results, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(data)+"\n")
	case fmtYAML:
		data, err := yaml.Marshal(results)
		exitWithError(cmd, err)
		outputBytes(cmd, data)
	default:
		for _, r := range results {
			if r.Status != doc2md.StatusSuccess {
				continue
			}
			switch {
			case keep && r.KeptFile != "":
				outputString(cmd, r.KeptFile+"\n")
			case outputFile != "":
				// crawldown's --output-file is silent on stdout.
			default:
				markdown := r.Markdown
				if !strings.HasSuffix(markdown, "\n") {
					markdown += "\n"
				}
				outputString(cmd, markdown)
			}
		}
	}

	exitWithDoc2mdStatus(results)
}

// writeDoc2mdFile keeps the conversion under context/refs/converted/ or writes
// it to the explicit --output-file path, always with the provenance frontmatter.
func writeDoc2mdFile(input string, res doc2md.Result, keep bool, outputFile, name string) (string, error) {
	doc := doc2md.Document{Source: input, Name: name, Converter: res.Converter, Markdown: res.Markdown}
	if keep {
		path, created, err := doc2md.Keep(doc, sdtConvertedDir)
		if err != nil {
			return "", err
		}
		if created {
			if _, _, err := doc2md.Reindex(sdtConvertedDir); err != nil {
				return "", err
			}
		}
		return path, nil
	}
	body, err := doc2md.Render(doc)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outputFile, body, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", outputFile, err)
	}
	return outputFile, nil
}

func exitWithDoc2mdStatus(results []doc2md.Result) {
	code := 0
	for _, r := range results {
		if r.Status == doc2md.StatusSuccess {
			continue
		}
		if c := r.Status.ExitCode(); c > code {
			code = c
		}
		slog.Error("conversion failed", "status", string(r.Status), "reason", r.Reason)
	}
	if code != 0 {
		exit(code)
	}
}

func runDoc2mdList(cmd *cobra.Command, _ []string) {
	idx, err := doc2md.ReadIndex(sdtConvertedDir)
	if err != nil {
		exitWithError(cmd, err)
		return
	}
	switch getFormat(cmd) {
	case fmtJSON:
		data, err := json.MarshalIndent(idx.Documents, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(data)+"\n")
	case fmtYAML:
		data, err := yaml.Marshal(idx.Documents)
		exitWithError(cmd, err)
		outputBytes(cmd, data)
	default:
		if len(idx.Documents) == 0 {
			return
		}
		for _, e := range idx.Documents {
			line := e.File
			if e.Summary != "" {
				line += "\t" + e.Summary
			}
			if len(e.Links) > 0 {
				line += "\t" + strings.Join(e.Links, ",")
			}
			outputString(cmd, line+"\n")
		}
	}
}

func runDoc2mdSet(cmd *cobra.Command, args []string) {
	file := args[0]
	if strings.ContainsRune(file, os.PathSeparator) {
		file = filepathBase(file)
	}
	summary := getStringFlag(cmd, "summary", false)
	links := getStringArrayFlag(cmd, "link", false)
	if summary == "" && len(links) == 0 {
		exitWithError(cmd, fmt.Errorf("nothing to set (pass --summary and/or --link)"))
		return
	}
	if err := doc2md.SetMetadata(sdtConvertedDir, file, summary, links); err != nil {
		exitWithError(cmd, err)
		return
	}
	outputString(cmd, file+"\n")
}

func runDoc2mdReindex(cmd *cobra.Command, _ []string) {
	wroteMD, wroteJSON, err := doc2md.Reindex(sdtConvertedDir)
	exitWithError(cmd, err)
	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(struct {
			Markdown bool `json:"markdown"`
			JSON     bool `json:"json"`
		}{wroteMD, wroteJSON}, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(out)+"\n")
	default:
		outputString(cmd, fmt.Sprintf("index.md %s; index.json %s\n", changed(wroteMD), changed(wroteJSON)))
	}
}

func changed(wrote bool) string {
	if wrote {
		return "written"
	}
	return "unchanged"
}

func filepathBase(p string) string {
	if i := strings.LastIndexByte(p, os.PathSeparator); i >= 0 {
		return p[i+1:]
	}
	return p
}

func runDoc2mdTools(cmd *cobra.Command, _ []string) {
	timeout, err := cmd.Flags().GetDuration("timeout")
	exitWithError(cmd, err)
	statuses := doc2mdTools(context.Background(), timeout)

	switch getFormat(cmd) {
	case fmtJSON:
		data, err := json.MarshalIndent(statuses, "", "  ")
		exitWithError(cmd, err)
		outputString(cmd, string(data)+"\n")
	case fmtYAML:
		data, err := yaml.Marshal(statuses)
		exitWithError(cmd, err)
		outputBytes(cmd, data)
	default:
		for _, s := range statuses {
			state := "unavailable"
			if s.Available {
				state = "available"
			}
			line := fmt.Sprintf("%-11s %-11s", s.Name, state)
			switch {
			case s.Version != "":
				line += " " + s.Version
			case s.Reason != "":
				line += " " + s.Reason
			}
			outputString(cmd, line+"\n")
		}
	}
}

func init() {
	doc2mdCmd.Flags().String("tool", "", "Pin one converter (anydoc, docling, markitdown)")
	doc2mdCmd.Flags().Bool("keep", false, "Keep the document under context/refs/converted/ with provenance frontmatter")
	doc2mdCmd.Flags().String("name", "", "Override the kept filename stem (with --keep)")
	doc2mdCmd.Flags().String("output-file", "", "Write the document to an explicit path instead of stdout")
	doc2mdSetCmd.Flags().String("summary", "", "Document summary")
	doc2mdSetCmd.Flags().StringArray("link", nil, "Related document path (repeatable)")
	doc2mdCmd.PersistentFlags().Duration("timeout", doc2md.DefaultTimeout, "Per-process timeout")
	doc2mdCmd.AddCommand(doc2mdListCmd, doc2mdSetCmd, doc2mdReindexCmd, doc2mdToolsCmd)
	rootCmd.AddCommand(doc2mdCmd)
}
