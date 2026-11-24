//go:build !wasm

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/usage"
	"github.com/spf13/cobra"
)

var gainCmd = &cobra.Command{
	Use:   "gain",
	Short: "Report what SDT actually did, counted from the local usage log",
	Long: `Report the counted facts of this project's local usage log: how often each
command ran, how many of those runs failed, how many bytes came back and — for
the commands that report a count — how often an invocation found at least one
item.

WHEN — you want to know which verbs the work actually uses, which ones fail, and
whether the searches return anything, instead of guessing.

WHEN NOT — you want a cost estimate. No counterfactual saving is computed: the
log holds what happened, never what would have happened without SDT.

The log lives at <root>/.sdt/usage/usage-YYYY-MM.jsonl. It records command,
exit, duration, stdout bytes and optional hits/source. usage.arguments (default
off) opts into argv capture bounded to 1 KiB. usage.memory (default on) records
peak RSS and cumulative Go allocations; usage.cpu (default off) records process
user/system time and sampled peak utilization. sdt gain never displays argv.
SDT_USAGE=off disables it, as does usage.enabled: false in .sdt.yaml.

gain itself and the internal calls SDT makes on its own behalf are excluded
from the totals and counted separately; --all includes them.`,
	Example: `  sdt gain
  sdt gain --since 30d
  sdt gain --since all --json`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		root := gainRoot(cmd)
		now := time.Now().UTC()
		spec := getStringFlag(cmd, "since", false)
		since, err := usage.ParseWindow(spec, now)
		exitWithError(cmd, err)
		entries, err := usage.Read(root, since)
		exitWithError(cmd, err)
		rep := usage.Summarize(entries, getBoolFlag(cmd, "all", false))
		renderGain(cmd, spec, since, now, rep)
	},
}

// gainRoot resolves the project whose log is read. The three ways the log can be
// off are reported separately: they have different remedies.
func gainRoot(cmd *cobra.Command) string {
	if strings.EqualFold(os.Getenv(usage.EnvOff), "off") {
		exitWithError(cmd, fmt.Errorf("%s=off: the usage log is disabled for this process", usage.EnvOff))
	}
	cfg, root, err := findProjectConfigAt()
	exitWithError(cmd, err)
	if root == "" {
		exitWithError(cmd, fmt.Errorf("no .sdt.yaml found: the usage log only exists inside a project"))
	}
	if cfg != nil && cfg.Usage.Enabled != nil && !*cfg.Usage.Enabled {
		exitWithError(cmd, fmt.Errorf("usage.enabled is false in .sdt.yaml: the usage log is off"))
	}
	return root
}

// gainPayload is the machine-shaped report. It is spelled out instead of
// embedding usage.Report so the field names are the wire contract and a change
// in the counting type cannot silently rename them.
type gainPayload struct {
	Since            string              `json:"since"             yaml:"since"`
	Until            string              `json:"until"             yaml:"until"`
	Window           string              `json:"window"            yaml:"window"`
	Calls            int                 `json:"calls"             yaml:"calls"`
	Failures         int                 `json:"failures"          yaml:"failures"`
	Out              int64               `json:"out"               yaml:"out"`
	HitsReported     int                 `json:"hits_reported"     yaml:"hits_reported"`
	HitsFound        int                 `json:"hits_found"        yaml:"hits_found"`
	MinMS            *int64              `json:"min_ms,omitempty"   yaml:"min_ms,omitempty"`
	MeanMS           *float64            `json:"mean_ms,omitempty"  yaml:"mean_ms,omitempty"`
	MaxMS            *int64              `json:"max_ms,omitempty"   yaml:"max_ms,omitempty"`
	PeakRSSBytes     *int64              `json:"peak_rss_bytes,omitempty" yaml:"peak_rss_bytes,omitempty"`
	GoAllocatedBytes *int64              `json:"go_allocated_bytes,omitempty" yaml:"go_allocated_bytes,omitempty"`
	CPUUserMS        *int64              `json:"cpu_user_ms,omitempty" yaml:"cpu_user_ms,omitempty"`
	CPUSystemMS      *int64              `json:"cpu_system_ms,omitempty" yaml:"cpu_system_ms,omitempty"`
	CPUPeakPercent   *float64            `json:"cpu_peak_percent,omitempty" yaml:"cpu_peak_percent,omitempty"`
	ExcludedSelf     int                 `json:"excluded_self"     yaml:"excluded_self"`
	ExcludedInternal int                 `json:"excluded_internal" yaml:"excluded_internal"`
	Commands         []usage.CommandStat `json:"commands"          yaml:"commands"`
}

// renderGain prints the report as a table, JSON or YAML.
func renderGain(cmd *cobra.Command, spec string, since, until time.Time, rep usage.Report) {
	payload := gainPayload{
		Since: stamp(since), Until: stamp(until), Window: windowLabel(spec, since),
		Calls: rep.Calls, Failures: rep.Failures, Out: rep.Out,
		HitsReported: rep.HitsReported, HitsFound: rep.HitsFound,
		ExcludedSelf: rep.ExcludedSelf, ExcludedInternal: rep.ExcludedInternal,
		PeakRSSBytes:     rep.PeakRSSBytes,
		GoAllocatedBytes: rep.GoAllocatedBytes,
		CPUUserMS:        rep.CPUUserMS, CPUSystemMS: rep.CPUSystemMS, CPUPeakPercent: rep.CPUPeakPercent,
		Commands: rep.Commands,
	}
	if rep.Calls > 0 {
		payload.MinMS = &rep.MinMS
		payload.MeanMS = &rep.MeanMS
		payload.MaxMS = &rep.MaxMS
	}
	format := getFormat(cmd)
	if getBoolFlag(cmd, "json", false) {
		format = fmtJSON
	}
	switch format {
	case fmtJSON:
		out, err := json.MarshalIndent(payload, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, append(out, '\n'))
	case fmtYAML:
		out, err := yaml.Marshal(payload)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		outputString(cmd, gainText(payload))
	}
}

// gainText renders the counted report: window, totals, the per-command table,
// what was left out, and the one sentence that keeps the numbers honest.
func gainText(p gainPayload) string {
	var b strings.Builder
	from := p.Since
	if from == "" {
		from = "the beginning"
	}
	fmt.Fprintf(&b, "window %s .. %s (%s)\n", from, p.Until, p.Window)
	fmt.Fprintf(&b, "calls %d, failures %d, stdout %s\n", p.Calls, p.Failures, humanSize(p.Out))
	if p.MinMS == nil || p.MeanMS == nil || p.MaxMS == nil {
		b.WriteString("duration ms: unavailable\n")
	} else {
		fmt.Fprintf(&b, "duration ms min %d, mean %.1f, max %d\n", *p.MinMS, *p.MeanMS, *p.MaxMS)
	}
	if p.PeakRSSBytes == nil {
		b.WriteString("peak sdt RSS: unavailable\n")
	} else {
		fmt.Fprintf(&b, "peak sdt RSS: %s\n", humanSize(*p.PeakRSSBytes))
	}
	writeGainResourceSummary(&b, p)
	fmt.Fprintf(&b, "hits %s\n\n", hitSummary(p.HitsFound, p.HitsReported))

	width := gainCommandWidth(p.Commands)
	writeGainTableHeader(&b, width)
	for _, row := range p.Commands {
		b.WriteString(formatGainRow(row, width))
	}
	if len(p.Commands) == 0 {
		b.WriteString("(nothing recorded in this window)\n")
	}
	fmt.Fprintf(&b, "\nexcluded %d self, %d internal (--all includes them)\n", p.ExcludedSelf, p.ExcludedInternal)
	b.WriteString("No counterfactual saving is computed: these are counted facts, not a claim about what the commands would have cost without SDT.\n")
	return b.String()
}

func writeGainResourceSummary(b *strings.Builder, p gainPayload) {
	if p.GoAllocatedBytes != nil {
		fmt.Fprintf(b, "Go allocated: %s\n", humanSize(*p.GoAllocatedBytes))
	}
	if p.CPUPeakPercent == nil {
		return
	}
	user, system := "not recorded", "not recorded"
	if p.CPUUserMS != nil {
		user = fmt.Sprint(*p.CPUUserMS)
	}
	if p.CPUSystemMS != nil {
		system = fmt.Sprint(*p.CPUSystemMS)
	}
	fmt.Fprintf(b, "process CPU user/system: %s/%s ms, sampled peak %.1f%%\n", user, system, *p.CPUPeakPercent)
}

func gainCommandWidth(rows []usage.CommandStat) int {
	width := len("COMMAND")
	for _, row := range rows {
		width = max(width, len(row.Command))
	}
	return width
}

func writeGainTableHeader(b *strings.Builder, width int) {
	fmt.Fprintf(b, "%-*s  %5s  %4s  %8s  %s  %10s  %8s  %11s  %8s  %s\n", width, "COMMAND", "CALLS", "FAIL", "STDOUT", "HITS", "MS MIN/AVG/MAX", "ALLOC", "CPU USER/SYS", "CPU PEAK", "PEAK RSS")
}

func formatGainRow(row usage.CommandStat, width int) string {
	hits, rss, alloc, cpuTime, cpuPeak := "-", "-", "-", "-", "-"
	if row.HitsReported > 0 {
		hits = fmt.Sprintf("%d/%d", row.HitsFound, row.HitsReported)
	}
	if row.PeakRSSBytes != nil {
		rss = humanSize(*row.PeakRSSBytes)
	}
	if row.GoAllocatedBytes != nil {
		alloc = humanSize(*row.GoAllocatedBytes)
	}
	if row.CPUUserMS != nil || row.CPUSystemMS != nil {
		user, system := "-", "-"
		if row.CPUUserMS != nil {
			user = fmt.Sprint(*row.CPUUserMS)
		}
		if row.CPUSystemMS != nil {
			system = fmt.Sprint(*row.CPUSystemMS)
		}
		cpuTime = fmt.Sprintf("%s/%s", user, system)
	}
	if row.CPUPeakPercent != nil {
		cpuPeak = fmt.Sprintf("%.1f%%", *row.CPUPeakPercent)
	}
	return fmt.Sprintf("%-*s  %5d  %4d  %8s  %s  %d/%.1f/%d  %8s  %11s  %8s  %s\n", width, row.Command, row.Calls, row.Failures, humanSize(row.Out), hits, row.MinMS, row.MeanMS, row.MaxMS, alloc, cpuTime, cpuPeak, rss)
}

// hitSummary phrases the hit rate over the calls that reported a count.
func hitSummary(found, reported int) string {
	if reported == 0 {
		return "no command reported a hit count in this window"
	}
	return fmt.Sprintf("%d of %d reporting calls returned at least one item (%.0f%%)",
		found, reported, float64(found)*100/float64(reported))
}

// windowLabel names the window as the caller asked for it: a bare "all" reads
// better than a timestamp nobody recognises as a lower bound.
func windowLabel(spec string, since time.Time) string {
	if since.IsZero() {
		return "all time"
	}
	if strings.TrimSpace(spec) == "" {
		return "7d"
	}
	return spec
}

// stamp renders a time for the report, or an empty string for "no lower bound".
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func init() {
	f := gainCmd.Flags()
	f.String("since", "7d", "Window: a duration (24h), days (7d) or all")
	f.Bool("all", false, "Include this command and SDT's internal calls in the totals")
	f.Bool("json", false, "Print the report as JSON")
	rootCmd.AddCommand(gainCmd)
}
