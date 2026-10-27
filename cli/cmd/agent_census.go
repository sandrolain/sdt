package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/census"
	"github.com/spf13/cobra"
)

// agentCensusCmd is the repeatable adoption measurement: it reads a JSONL of
// agent shell calls, classifies each command into the analysis's shapes and
// prints the counts. The extractor that produces the JSONL is a project script,
// so the verb never opens an agent's database.
var agentCensusCmd = &cobra.Command{
	Use:   "census",
	Short: "Classify agent shell calls into the adoption shapes",
	Long: `Read one JSON object per line — {"ts": "<RFC3339>", "command": "<shell>"} —
classify each command into the adoption shapes, and print the counts and shares.

WHEN — re-running the fallback-shape measurement, or comparing a window before and
after a verb shipped. The input comes from an extractor script, not from a
database this command knows about.

RETURNS — a table by default, a {window, total, shapes} object with --format json.
A malformed line is refused with its number; unknown fields are ignored.

EXAMPLE — sdt agent census --file calls.jsonl --since 2026-10-01
          sdt agent census --file - --until 2026-10-24 --format json`,
	Args: cobra.NoArgs,
	Run:  runAgentCensus,
}

// censusRow is one JSONL record: only the two facts the classifier needs.
type censusRow struct {
	TS      string `json:"ts"`
	Command string `json:"command"`
}

// censusShape is one line of the answer.
type censusShape struct {
	Name  string  `json:"name" yaml:"name"`
	Count int     `json:"count" yaml:"count"`
	Share float64 `json:"share" yaml:"share"`
}

type censusWindow struct {
	Since string `json:"since,omitempty" yaml:"since,omitempty"`
	Until string `json:"until,omitempty" yaml:"until,omitempty"`
}

type censusResult struct {
	Window censusWindow  `json:"window" yaml:"window"`
	Total  int           `json:"total" yaml:"total"`
	Shapes []censusShape `json:"shapes" yaml:"shapes"`
}

func runAgentCensus(cmd *cobra.Command, args []string) {
	path := getStringFlag(cmd, "file", true)
	var in io.Reader = os.Stdin
	if path != "-" {
		//#nosec G304 -- the caller names the extract
		f, err := os.Open(path)
		exitWithError(cmd, err)
		if err != nil {
			return
		}
		defer func() { _ = f.Close() }() //nolint:errcheck // read-only handle
		in = f
	}

	since, until, err := censusWindowOf(cmd)
	exitWithError(cmd, err)
	if err != nil {
		return
	}

	counts := map[string]int{}
	total := 0
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var row censusRow
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			exitWithError(cmd, fmt.Errorf("%s line %d: not a JSON object: %w", path, line, err))
			return
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(row.TS))
		if err != nil {
			exitWithError(cmd, fmt.Errorf("%s line %d: ts is not RFC3339: %w", path, line, err))
			return
		}
		if !since.IsZero() && ts.Before(since) {
			continue
		}
		if !until.IsZero() && ts.After(until) {
			continue
		}
		counts[census.Classify(row.Command)]++
		total++
	}
	exitWithError(cmd, scanner.Err())
	if err := scanner.Err(); err != nil {
		return
	}

	res := censusResult{Window: censusWindow{Since: strings.TrimSpace(getStringFlag(cmd, "since", false)), Until: strings.TrimSpace(getStringFlag(cmd, "until", false))}, Total: total}
	for _, name := range census.Shapes {
		n := counts[name]
		share := 0.0
		if total > 0 {
			share = float64(n) / float64(total) * 100
		}
		res.Shapes = append(res.Shapes, censusShape{Name: name, Count: n, Share: share})
	}
	sort.SliceStable(res.Shapes, func(i, j int) bool { return res.Shapes[i].Count > res.Shapes[j].Count })

	switch getFormat(cmd) {
	case fmtJSON:
		out, err := json.MarshalIndent(res, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, append(out, '\n'))
	case fmtYAML:
		out, err := yaml.Marshal(res)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "%-22s %8s %8s\n", "shape", "count", "share")
		for _, s := range res.Shapes {
			fmt.Fprintf(&b, "%-22s %8d %7.1f%%\n", s.Name, s.Count, s.Share)
		}
		fmt.Fprintf(&b, "%-22s %8d\n", "total", res.Total)
		outputString(cmd, b.String())
	}
}

// censusWindowOf parses --since/--until: an RFC3339 instant or a date, whose
// day is included in full.
func censusWindowOf(cmd *cobra.Command) (time.Time, time.Time, error) {
	parse := func(flag string, endOfDay bool) (time.Time, error) {
		raw := strings.TrimSpace(getStringFlag(cmd, flag, false))
		if raw == "" {
			return time.Time{}, nil
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return t, nil
		}
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, fmt.Errorf("--%s wants RFC3339 or YYYY-MM-DD, got %q", flag, raw)
		}
		if endOfDay {
			return t.Add(24*time.Hour - time.Nanosecond), nil
		}
		return t, nil
	}
	since, err := parse("since", false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	until, err := parse("until", true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !since.IsZero() && !until.IsZero() && until.Before(since) {
		return time.Time{}, time.Time{}, fmt.Errorf("--until precedes --since")
	}
	return since, until, nil
}

func init() {
	f := agentCensusCmd.Flags()
	f.StringP("file", "f", "-", "JSONL extract (default: stdin; - for stdin)")
	f.String("since", "", "Include calls on/after this RFC3339 instant or date")
	f.String("until", "", "Include calls on/before this RFC3339 instant or date")
	agentCmd.AddCommand(agentCensusCmd)
}
