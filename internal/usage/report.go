package usage

import "sort"

// CommandStat is one row of the counted report.
type CommandStat struct {
	Command          string   `json:"command"       yaml:"command"`
	Calls            int      `json:"calls"         yaml:"calls"`
	Failures         int      `json:"failures"      yaml:"failures"`
	Out              int64    `json:"out"           yaml:"out"`
	HitsReported     int      `json:"hits_reported" yaml:"hits_reported"`
	HitsFound        int      `json:"hits_found"    yaml:"hits_found"`
	MinMS            int64    `json:"min_ms"         yaml:"min_ms"`
	MeanMS           float64  `json:"mean_ms"        yaml:"mean_ms"`
	MaxMS            int64    `json:"max_ms"         yaml:"max_ms"`
	PeakRSSBytes     *int64   `json:"peak_rss_bytes,omitempty" yaml:"peak_rss_bytes,omitempty"`
	GoAllocatedBytes *int64   `json:"go_allocated_bytes,omitempty" yaml:"go_allocated_bytes,omitempty"`
	CPUUserMS        *int64   `json:"cpu_user_ms,omitempty" yaml:"cpu_user_ms,omitempty"`
	CPUSystemMS      *int64   `json:"cpu_system_ms,omitempty" yaml:"cpu_system_ms,omitempty"`
	CPUPeakPercent   *float64 `json:"cpu_peak_percent,omitempty" yaml:"cpu_peak_percent,omitempty"`
}

// Report is the counted summary of a window: facts, never a counterfactual.
type Report struct {
	Calls            int
	Failures         int
	Out              int64
	HitsReported     int
	HitsFound        int
	MinMS            int64
	MeanMS           float64
	MaxMS            int64
	ElapsedMS        int64
	PeakRSSBytes     *int64
	GoAllocatedBytes *int64
	CPUUserMS        *int64
	CPUSystemMS      *int64
	CPUPeakPercent   *float64
	Commands         []CommandStat
	// ExcludedSelf and ExcludedInternal are what the report left out, so a
	// reader can tell a quiet week from a filter.
	ExcludedSelf     int
	ExcludedInternal int
}

// Summarize counts the entries, keeping every command in one row. By default
// `sdt gain` itself and the calls SDT made on its own behalf are excluded and
// only counted; includeAll keeps them.
func Summarize(entries []Entry, includeAll bool) Report {
	var rep Report
	rows := map[string]*CommandStat{}
	for _, e := range entries {
		if !includeEntry(e, includeAll, &rep) {
			continue
		}
		row, ok := rows[e.Cmd]
		if !ok {
			row = &CommandStat{Command: e.Cmd}
			rows[e.Cmd] = row
		}
		addEntry(&rep, row, e)
	}
	if rep.Calls > 0 {
		rep.MeanMS = float64(rep.ElapsedMS) / float64(rep.Calls)
	}
	for _, row := range rows {
		row.MeanMS /= float64(row.Calls)
	}
	rep.Commands = make([]CommandStat, 0, len(rows))
	for _, row := range rows {
		rep.Commands = append(rep.Commands, *row)
	}
	// Most used first, then by name, so the table reads the same for the same
	// history instead of following map order.
	sort.Slice(rep.Commands, func(i, j int) bool {
		if rep.Commands[i].Calls != rep.Commands[j].Calls {
			return rep.Commands[i].Calls > rep.Commands[j].Calls
		}
		return rep.Commands[i].Command < rep.Commands[j].Command
	})
	return rep
}

func includeEntry(e Entry, includeAll bool, rep *Report) bool {
	if includeAll {
		return true
	}
	self := e.Cmd == "gain"
	internal := e.Source == SourceInternal
	if self {
		rep.ExcludedSelf++
	}
	if internal {
		rep.ExcludedInternal++
	}
	return !self && !internal
}

func addEntry(rep *Report, row *CommandStat, e Entry) {
	row.Calls++
	if row.Calls == 1 || e.MS < row.MinMS {
		row.MinMS = e.MS
	}
	if row.Calls == 1 || e.MS > row.MaxMS {
		row.MaxMS = e.MS
	}
	if rep.Calls == 0 || e.MS < rep.MinMS {
		rep.MinMS = e.MS
	}
	if rep.Calls == 0 || e.MS > rep.MaxMS {
		rep.MaxMS = e.MS
	}
	rep.ElapsedMS += e.MS
	row.MeanMS += float64(e.MS)
	updatePeakRSS(&row.PeakRSSBytes, e.RSSBytes)
	updatePeakRSS(&rep.PeakRSSBytes, e.RSSBytes)
	addMetric(&row.GoAllocatedBytes, e.GoAllocatedBytes)
	addMetric(&rep.GoAllocatedBytes, e.GoAllocatedBytes)
	addMetric(&row.CPUUserMS, e.CPUUserMS)
	addMetric(&rep.CPUUserMS, e.CPUUserMS)
	addMetric(&row.CPUSystemMS, e.CPUSystemMS)
	addMetric(&rep.CPUSystemMS, e.CPUSystemMS)
	updatePeakFloat(&row.CPUPeakPercent, e.CPUPeakPercent)
	updatePeakFloat(&rep.CPUPeakPercent, e.CPUPeakPercent)
	rep.Calls++
	row.Out += e.Out
	rep.Out += e.Out
	if e.Exit != 0 {
		row.Failures++
		rep.Failures++
	}
	if e.Hits != nil {
		row.HitsReported++
		rep.HitsReported++
		if *e.Hits > 0 {
			row.HitsFound++
			rep.HitsFound++
		}
	}
}

func addMetric(total **int64, sample *int64) {
	if sample == nil {
		return
	}
	if *total == nil {
		value := int64(0)
		*total = &value
	}
	**total += *sample
}

func updatePeakFloat(current **float64, sample *float64) {
	if sample == nil || (*current != nil && *sample <= **current) {
		return
	}
	value := *sample
	*current = &value
}

func updatePeakRSS(current **int64, sample *int64) {
	if sample == nil || (*current != nil && *sample <= **current) {
		return
	}
	value := *sample
	*current = &value
}
