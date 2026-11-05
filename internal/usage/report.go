package usage

import "sort"

// CommandStat is one row of the counted report.
type CommandStat struct {
	Command      string `json:"command"       yaml:"command"`
	Calls        int    `json:"calls"         yaml:"calls"`
	Failures     int    `json:"failures"      yaml:"failures"`
	Out          int64  `json:"out"           yaml:"out"`
	HitsReported int    `json:"hits_reported" yaml:"hits_reported"`
	HitsFound    int    `json:"hits_found"    yaml:"hits_found"`
}

// Report is the counted summary of a window: facts, never a counterfactual.
type Report struct {
	Calls        int
	Failures     int
	Out          int64
	HitsReported int
	HitsFound    int
	Commands     []CommandStat
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
		self := e.Cmd == "gain"
		internal := e.Source == SourceInternal
		if !includeAll {
			if self {
				rep.ExcludedSelf++
			}
			if internal {
				rep.ExcludedInternal++
			}
			if self || internal {
				continue
			}
		}
		row, ok := rows[e.Cmd]
		if !ok {
			row = &CommandStat{Command: e.Cmd}
			rows[e.Cmd] = row
		}
		row.Calls++
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
