package usage

import "testing"

func hits(n int) *int { return &n }

func TestSummarizeExcludesSelfAndInternal(t *testing.T) {
	entries := []Entry{
		{TS: "2026-10-30T06:00:00Z", Cmd: "context search", Out: 100, Hits: hits(2)},
		{TS: "2026-10-30T06:00:01Z", Cmd: "context search", Out: 50, Hits: hits(0)},
		{TS: "2026-10-30T06:00:02Z", Cmd: "ls", Exit: 1, Out: 10, Hits: hits(0)},
		{TS: "2026-10-30T06:00:03Z", Cmd: "gain", Out: 999},
		{TS: "2026-10-30T06:00:04Z", Cmd: "context status set", Source: SourceInternal, Out: 999},
	}

	rep := Summarize(entries, false)
	if rep.Calls != 3 || rep.Failures != 1 || rep.Out != 160 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.ExcludedSelf != 1 || rep.ExcludedInternal != 1 {
		t.Fatalf("excluded = %d self, %d internal", rep.ExcludedSelf, rep.ExcludedInternal)
	}
	if rep.HitsReported != 3 || rep.HitsFound != 1 {
		t.Fatalf("hits = %d of %d", rep.HitsFound, rep.HitsReported)
	}
	if len(rep.Commands) != 2 {
		t.Fatalf("commands = %+v", rep.Commands)
	}
	if rep.Commands[0].Command != "context search" || rep.Commands[0].Calls != 2 {
		t.Fatalf("rows are most-used first: %+v", rep.Commands)
	}

	all := Summarize(entries, true)
	if all.Calls != 5 || all.ExcludedSelf != 0 || all.ExcludedInternal != 0 {
		t.Fatalf("--all report = %+v", all)
	}
}

func TestSummarizeTieOrderIsStable(t *testing.T) {
	entries := []Entry{
		{TS: "2026-10-30T06:00:00Z", Cmd: "read"},
		{TS: "2026-10-30T06:00:01Z", Cmd: "b64"},
	}
	rep := Summarize(entries, false)
	if rep.Commands[0].Command != "b64" || rep.Commands[1].Command != "read" {
		t.Fatalf("tie order = %+v", rep.Commands)
	}
}

func TestSummarizeCountsOnlyReportedHits(t *testing.T) {
	entries := []Entry{
		{TS: "2026-10-30T06:00:00Z", Cmd: "context search", Hits: hits(0)},
		{TS: "2026-10-30T06:00:01Z", Cmd: "context search"},
		{TS: "2026-10-30T06:00:02Z", Cmd: "context search", Hits: hits(4)},
	}
	rep := Summarize(entries, false)
	if rep.HitsReported != 2 || rep.HitsFound != 1 {
		t.Fatalf("a call that reported nothing must not count as a miss: %+v", rep)
	}
}
