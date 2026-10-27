package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// Budget report: the measured byte cost of the surface a session always loads,
// named per surface with its largest entries. Read-only and informational — the
// report never fails a build and is never a gate (analysis
// 20261011-195714, design decision D1).
//
// The metric is bytes, lines and words — no tokenizer dependency. Any token
// figure is a labelled estimate derived from the byte count with a documented,
// overridable divisor, never a model-accurate count (HARD RULE 8; the shortlist
// and the user's choice of no dependency are recorded in
// plan/20261013-213337).

const (
	// ctxBudgetIndexFile is the generated entry point read at session start.
	ctxBudgetIndexFile = "context/index.md"
	// ctxBudgetBytesPerToken is the divisor behind the estimated-token figure.
	// It is a documented default, overridable with --bytes-per-token, and the
	// output always labels the result as an estimate.
	ctxBudgetBytesPerToken = 4
	// ctxBudgetTopDefault bounds the largest-entry list by default so the report
	// does not itself become a large output.
	ctxBudgetTopDefault = 10
)

// The two groups the report separates. A surface is either paid at the start of
// a session (the AGENTS.md block on every turn, the digest once) or read only
// when the work touches it: adding the two into one figure was the defect that
// made the report claim a session costs ~190k tokens.
const (
	ctxBudgetGroupStart  = "start"
	ctxBudgetGroupLookup = "lookup"
)

// ctxBudgetSurface is one surface and its measured size.
type ctxBudgetSurface struct {
	Name    string           `json:"name" yaml:"name"`
	Group   string           `json:"group" yaml:"group"`
	Detail  string           `json:"detail" yaml:"detail"`
	Files   int              `json:"files" yaml:"files"`
	Bytes   int              `json:"bytes" yaml:"bytes"`
	Lines   int              `json:"lines" yaml:"lines"`
	Words   int              `json:"words" yaml:"words"`
	Largest []ctxBudgetEntry `json:"largest" yaml:"largest"`
}

// ctxBudgetTotals is the size of one group.
type ctxBudgetTotals struct {
	Bytes           int `json:"bytes" yaml:"bytes"`
	Lines           int `json:"lines" yaml:"lines"`
	Words           int `json:"words" yaml:"words"`
	EstimatedTokens int `json:"estimated_tokens" yaml:"estimated_tokens"`
}

// ctxBudgetEntry is one file inside a surface, measured.
type ctxBudgetEntry struct {
	Path  string `json:"path" yaml:"path"`
	Bytes int    `json:"bytes" yaml:"bytes"`
	Lines int    `json:"lines" yaml:"lines"`
	Words int    `json:"words" yaml:"words"`
}

// ctxBudgetReport is the whole report: the surfaces plus the totals a reader
// compares against. The estimate is carried as an explicit, labelled field so
// no consumer can mistake it for a tokenizer count.
type ctxBudgetReport struct {
	Surfaces         []ctxBudgetSurface `json:"surfaces" yaml:"surfaces"`
	Start            ctxBudgetTotals    `json:"start" yaml:"start"`
	Lookup           ctxBudgetTotals    `json:"lookup" yaml:"lookup"`
	TotalBytes       int                `json:"total_bytes" yaml:"total_bytes"`
	TotalLines       int                `json:"total_lines" yaml:"total_lines"`
	TotalWords       int                `json:"total_words" yaml:"total_words"`
	EstimatedTokens  int                `json:"estimated_tokens" yaml:"estimated_tokens"`
	TokenBasis       string             `json:"token_basis" yaml:"token_basis"`
	BytesPerToken    int                `json:"bytes_per_token" yaml:"bytes_per_token"`
	InstructionFiles int                `json:"instruction_modules_bytes,omitempty" yaml:"instruction_modules_bytes,omitempty"`
}

// ctxBudgetSize is the measured size of one file's content, in the three units
// the report prints.
type ctxBudgetSize struct {
	Bytes int
	Lines int
	Words int
}

// ctxBudgetMeasure counts bytes, lines and words. Lines are counted as the
// newline count plus one for a trailing unterminated line, so a file without a
// final newline still reports its last line. Words are whitespace-separated
// fields, which is deliberately independent of any tokenizer.
func ctxBudgetMeasure(content string) ctxBudgetSize {
	words := 0
	for _, f := range strings.FieldsFunc(content, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
	}) {
		if utf8.RuneCountInString(f) > 0 {
			words++
		}
	}
	lines := strings.Count(content, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		lines++
	}
	return ctxBudgetSize{Bytes: len(content), Lines: lines, Words: words}
}

// ctxBudgetReadFile measures one file. A missing file yields a zero size and no
// error: the report degrades to silence on an absent surface rather than
// failing, because it is informational and never a gate.
func ctxBudgetReadFile(path string) ctxBudgetSize {
	data, err := os.ReadFile(path) //#nosec G304 -- project surface the user asked to measure
	if err != nil {
		return ctxBudgetSize{}
	}
	return ctxBudgetMeasure(string(data))
}

// ctxBudgetTopEntries returns the n largest entries, biggest first, ties broken
// by path so the output is deterministic. A non-positive n returns none.
func ctxBudgetTopEntries(entries []ctxBudgetEntry, n int) []ctxBudgetEntry {
	if n <= 0 || len(entries) == 0 {
		return []ctxBudgetEntry{}
	}
	sorted := make([]ctxBudgetEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Bytes != sorted[j].Bytes {
			return sorted[i].Bytes > sorted[j].Bytes
		}
		return sorted[i].Path < sorted[j].Path
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// ctxBudgetSurfaces walks the declared always-loaded surface set: the AGENTS.md
// instructions block (the generated block only, not the write-once project
// block), the generated index, and the two essential-tier directories. Each is
// reported separately so a reader sees which surface costs what.
//
// The per-type instruction modules are on demand, not always loaded, so they are
// only walked when includeModules is set.
func ctxBudgetSurfaces(top int, includeModules bool) []ctxBudgetSurface {
	instructions := ctxBudgetInstructionsSurface(top)
	instructions.Group = ctxBudgetGroupStart
	surfaces := []ctxBudgetSurface{instructions}
	if project := ctxBudgetProjectSurface(top); project.Name != "" {
		surfaces = append(surfaces, project)
	}

	if data, err := os.ReadFile(ctxBudgetIndexFile); err == nil { //#nosec G304 -- project surface the user asked to measure
		size := ctxBudgetMeasure(string(data))
		surfaces = append(surfaces, ctxBudgetSurface{
			Name:   "index",
			Group:  ctxBudgetGroupLookup,
			Detail: ctxBudgetIndexFile,
			Files:  1,
			Bytes:  size.Bytes,
			Lines:  size.Lines,
			Words:  size.Words,
			// The single file is named on the surface line, so it enters the
			// largest-entry list only when the bound actually admits it.
			Largest: ctxBudgetTopEntries([]ctxBudgetEntry{{
				Path: ctxBudgetIndexFile, Bytes: size.Bytes, Lines: size.Lines, Words: size.Words,
			}}, top),
		})
	}

	if data, err := os.ReadFile(sdtEssentialsFile); err == nil { //#nosec G304 -- project surface the user asked to measure
		size := ctxBudgetMeasure(string(data))
		surfaces = append(surfaces, ctxBudgetSurface{
			Name:   "essentials",
			Group:  ctxBudgetGroupStart,
			Detail: sdtEssentialsFile,
			Files:  1,
			Bytes:  size.Bytes,
			Lines:  size.Lines,
			Words:  size.Words,
			Largest: ctxBudgetTopEntries([]ctxBudgetEntry{{
				Path: sdtEssentialsFile, Bytes: size.Bytes, Lines: size.Lines, Words: size.Words,
			}}, top),
		})
	}

	surfaces = append(surfaces,
		ctxBudgetDirSurface("architecture", sdtArchitectureDir, top),
		ctxBudgetDirSurface("decisions", sdtDecisionsDir, top),
	)

	if includeModules {
		surfaces = append(surfaces, ctxBudgetDirSurface("instruction-modules", sdtInstrDir, top))
	}
	for i := range surfaces {
		if surfaces[i].Group == "" {
			surfaces[i].Group = ctxBudgetGroupLookup
		}
	}
	return surfaces
}

// ctxBudgetInstructionsSurface measures the generated AGENTS.md instructions
// block. Only the block between its markers is counted: the write-once project
// block beside it is project-authored, not part of the always-loaded contract.
func ctxBudgetInstructionsSurface(top int) ctxBudgetSurface {
	name := agentTargetDefault
	data, err := os.ReadFile(name) //#nosec G304 -- project surface the user asked to measure
	if err != nil {
		return ctxBudgetSurface{Name: agentSectionNameInstructions, Detail: name + " (absent)"}
	}
	block := sectionRegexp(agentSectionNameInstructions).Find(data)
	size := ctxBudgetMeasure(string(block))
	path := name + " (" + agentSectionNameInstructions + " block)"
	return ctxBudgetSurface{
		Name:   agentSectionNameInstructions,
		Detail: path,
		Files:  1,
		Bytes:  size.Bytes,
		Lines:  size.Lines,
		Words:  size.Words,
		// The single block is named on the surface line, so it enters the
		// largest-entry list only when the bound actually admits it.
		Largest: ctxBudgetTopEntries([]ctxBudgetEntry{{
			Path: path, Bytes: size.Bytes, Lines: size.Lines, Words: size.Words,
		}}, top),
	}
}

// ctxBudgetProjectSurface measures the write-once project block of AGENTS.md — the
// half of the every-turn file the instructions surface deliberately leaves out. The
// two together are what a session pays before its first lookup, so a report that
// shows only the bounded half shows half the bill (audit 20261025-154218, F1).
//
// The block is found from the *last* marker pair, not the first: the instructions
// block names the project marker in prose ("the write-once `<!-- sdt:begin:project -->`
// block below"), and a first-match search would start there and swallow the tail of
// the very block this one is supposed to sit beside. A file without the block
// contributes nothing rather than an error: the project block is authored, and a
// checkout may not have one.
func ctxBudgetProjectSurface(top int) ctxBudgetSurface {
	name := agentTargetDefault
	data, err := os.ReadFile(name) //#nosec G304 -- project surface the user asked to measure
	if err != nil {
		return ctxBudgetSurface{}
	}
	begin := bytes.LastIndex(data, []byte(sectionBeginMarker(agentSectionNameProject)))
	end := bytes.LastIndex(data, []byte(sectionEndMarker(agentSectionNameProject)))
	if begin < 0 || end < begin {
		return ctxBudgetSurface{}
	}
	block := data[begin : end+len(sectionEndMarker(agentSectionNameProject))]
	size := ctxBudgetMeasure(string(block))
	path := name + " (" + agentSectionNameProject + " block)"
	return ctxBudgetSurface{
		Name:   agentSectionNameProject,
		Group:  ctxBudgetGroupStart,
		Detail: path,
		Files:  1,
		Bytes:  size.Bytes,
		Lines:  size.Lines,
		Words:  size.Words,
		// The single block is named on the surface line, so it enters the
		// largest-entry list only when the bound actually admits it.
		Largest: ctxBudgetTopEntries([]ctxBudgetEntry{{
			Path: path, Bytes: size.Bytes, Lines: size.Lines, Words: size.Words,
		}}, top),
	}
}

// ctxBudgetDirSurface measures one essential-tier directory. A directory that
// does not exist contributes a zero surface rather than an error.
func ctxBudgetDirSurface(name, dir string, top int) ctxBudgetSurface {
	surface := ctxBudgetSurface{Name: name, Detail: dir, Largest: []ctxBudgetEntry{}}
	files, err := dirFiles(dir)
	if err != nil {
		return surface
	}
	entries := make([]ctxBudgetEntry, 0, len(files))
	for _, path := range files {
		size := ctxBudgetReadFile(path)
		surface.Bytes += size.Bytes
		surface.Lines += size.Lines
		surface.Words += size.Words
		entries = append(entries, ctxBudgetEntry{
			Path: path, Bytes: size.Bytes, Lines: size.Lines, Words: size.Words,
		})
	}
	surface.Files = len(files)
	surface.Largest = ctxBudgetTopEntries(entries, top)
	return surface
}

// ctxBudgetBuildReport aggregates the surfaces into the report, carrying the
// estimated-token figure with its basis so no consumer can read it as a
// tokenizer count.
func ctxBudgetBuildReport(top int, bytesPerToken int, includeModules bool) ctxBudgetReport {
	if bytesPerToken <= 0 {
		bytesPerToken = ctxBudgetBytesPerToken
	}
	rep := ctxBudgetReport{
		Surfaces:      ctxBudgetSurfaces(top, includeModules),
		BytesPerToken: bytesPerToken,
	}
	rep.TokenBasis = fmt.Sprintf("estimate: total bytes / %d (not a model tokenizer count)", bytesPerToken)
	for _, s := range rep.Surfaces {
		rep.TotalBytes += s.Bytes
		rep.TotalLines += s.Lines
		rep.TotalWords += s.Words
		switch s.Group {
		case ctxBudgetGroupStart:
			rep.Start.Bytes += s.Bytes
			rep.Start.Lines += s.Lines
			rep.Start.Words += s.Words
		default:
			rep.Lookup.Bytes += s.Bytes
			rep.Lookup.Lines += s.Lines
			rep.Lookup.Words += s.Words
		}
	}
	rep.EstimatedTokens = rep.TotalBytes / bytesPerToken
	rep.Start.EstimatedTokens = rep.Start.Bytes / bytesPerToken
	rep.Lookup.EstimatedTokens = rep.Lookup.Bytes / bytesPerToken
	return rep
}

// ctxBudgetRenderText prints the report as the default text view: the surfaces
// paid at the start of a session first, then the ones read only when the work
// touches them, each with its own total and estimate. The two groups are never
// added together, because a number that mixes them is what made the report claim
// a session costs six figures in tokens.
func ctxBudgetRenderText(rep ctxBudgetReport) string {
	var b strings.Builder
	b.WriteString("agent surface (measured bytes, lines, words)\n")
	groups := []struct {
		title  string
		group  string
		totals ctxBudgetTotals
	}{
		{"\npaid at the start of a session", ctxBudgetGroupStart, rep.Start},
		{"\nread on demand (lookup)", ctxBudgetGroupLookup, rep.Lookup},
	}
	for _, g := range groups {
		b.WriteString(g.title + "\n")
		for _, s := range rep.Surfaces {
			if s.Group != g.group {
				continue
			}
			fmt.Fprintf(&b, "  %-20s %7d bytes  %6d lines  %6d words  %2d file(s)  %s\n",
				s.Name+":", s.Bytes, s.Lines, s.Words, s.Files, s.Detail)
			// A single-file surface names its one file on the surface line, so a
			// largest-entry list under it would only repeat that line.
			if s.Files < 2 {
				continue
			}
			for _, e := range s.Largest {
				fmt.Fprintf(&b, "    %-18s %7d bytes  %6d lines  %6d words  %s\n", "", e.Bytes, e.Lines, e.Words, e.Path)
			}
		}
		fmt.Fprintf(&b, "  %-20s %7d bytes  %6d lines  %6d words  ~%d tokens\n",
			"subtotal:", g.totals.Bytes, g.totals.Lines, g.totals.Words, g.totals.EstimatedTokens)
	}
	b.WriteString("\n  estimates: bytes / " + fmt.Sprint(rep.BytesPerToken) + " (documented divisor, not a model tokenizer count)\n")
	b.WriteString("\n  informational only: this report never fails a build and is not a gate.\n")
	return b.String()
}

var contextBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Report the measured size of the always-loaded context/ surface",
	Long: `Report the measured byte, line and word cost of the surface a session
always loads: the AGENTS.md instructions block and its write-once project block
(the two together are the whole every-turn file), context/index.md, and the two
essential-tier directories context/architecture/ and context/decisions/. Each
surface is reported separately with its largest entries.

WHEN — you are deciding what the corpus costs to load, or judging whether a
proposed change makes the always-loaded surface bigger, and want a number rather
than an impression.

WHEN NOT — you want the cost of one document (sdt context show, or wc on the
file); the per-type instruction modules, which load on demand (pass
--include-modules to measure them anyway); or a model-accurate token count. Any
token figure here is a documented estimate derived from bytes, not a tokenizer
result.

RETURNS — bytes, lines and words per surface and per file, the n largest
entries per surface under --top, the totals, and an explicitly labelled token
estimate. Read-only: the report never writes a file, never exits non-zero
because a surface is large, and is never a gate.

EXAMPLE — sdt context budget
          sdt context budget --top 3 --format json
          sdt context budget --include-modules --bytes-per-token 3`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rep := ctxBudgetBuildReport(
			getIntFlag(cmd, "top", false),
			getIntFlag(cmd, "bytes-per-token", false),
			getBoolFlag(cmd, "include-modules", false),
		)
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(rep, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(rep)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, ctxBudgetRenderText(rep))
		}
	},
}

func init() {
	contextBudgetCmd.Flags().Int("top", ctxBudgetTopDefault, "Largest entries listed per surface (0 lists none)")
	contextBudgetCmd.Flags().Int("bytes-per-token", ctxBudgetBytesPerToken, "Divisor behind the estimated-token figure (a documented estimate, not a tokenizer count)")
	contextBudgetCmd.Flags().Bool("include-modules", false, "Also measure the per-type instruction modules, which load on demand")
}
