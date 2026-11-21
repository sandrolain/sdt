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

func TestSummarizeDurationAndPeakRSS(t *testing.T) {
	zeroRSS := int64(0)
	largeRSS := int64(4096)
	entries := []Entry{
		{Cmd: "read", MS: 0, RSSBytes: &zeroRSS},
		{Cmd: "read", MS: 10, RSSBytes: &largeRSS},
		{Cmd: "gain", MS: 100, RSSBytes: &largeRSS},
		{Cmd: "internal", MS: 200, Source: SourceInternal, RSSBytes: &largeRSS},
	}

	report := Summarize(entries, false)
	if report.MinMS != 0 || report.MeanMS != 5 || report.MaxMS != 10 {
		t.Fatalf("duration summary = %d/%.1f/%d, want 0/5/10", report.MinMS, report.MeanMS, report.MaxMS)
	}
	if report.PeakRSSBytes == nil || *report.PeakRSSBytes != largeRSS {
		t.Fatalf("peak RSS = %v, want %d", report.PeakRSSBytes, largeRSS)
	}
	if len(report.Commands) != 1 {
		t.Fatalf("commands = %+v, want only read", report.Commands)
	}
	row := report.Commands[0]
	if row.MinMS != 0 || row.MeanMS != 5 || row.MaxMS != 10 {
		t.Fatalf("command duration = %d/%.1f/%d, want 0/5/10", row.MinMS, row.MeanMS, row.MaxMS)
	}
	if row.PeakRSSBytes == nil || *row.PeakRSSBytes != largeRSS {
		t.Fatalf("command peak RSS = %v, want %d", row.PeakRSSBytes, largeRSS)
	}

	all := Summarize(entries, true)
	if all.Calls != 4 || all.PeakRSSBytes == nil || *all.PeakRSSBytes != largeRSS {
		t.Fatalf("--all summary = %+v", all)
	}
}

func TestSummarizeEmptyAndUnavailableRSS(t *testing.T) {
	empty := Summarize(nil, false)
	if empty.Calls != 0 || empty.PeakRSSBytes != nil || len(empty.Commands) != 0 {
		t.Fatalf("empty summary = %+v", empty)
	}
	legacy := Summarize([]Entry{{Cmd: "read"}}, false)
	if legacy.PeakRSSBytes != nil || legacy.Commands[0].PeakRSSBytes != nil {
		t.Fatalf("legacy summary should have unavailable RSS: %+v", legacy)
	}
}
