package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendRead(t *testing.T) {
	root := t.TempDir()
	hits := 3
	first := Entry{TS: "2026-10-30T06:00:00Z", Cmd: "context search", Exit: 0, MS: 12, Out: 240, Hits: &hits}
	second := Entry{TS: "2026-10-30T06:01:00Z", Cmd: "ls", Exit: 1, MS: 3, Out: 0}
	if err := Append(root, first); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := Append(root, second); err != nil {
		t.Fatalf("append second: %v", err)
	}

	entries, err := Read(root, time.Time{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("read %d entries, want 2", len(entries))
	}
	if entries[0].Cmd != "context search" || entries[0].Hits == nil || *entries[0].Hits != 3 {
		t.Fatalf("first entry = %+v", entries[0])
	}
	if entries[1].Exit != 1 || entries[1].Hits != nil {
		t.Fatalf("second entry = %+v", entries[1])
	}
}

func TestMonthRollover(t *testing.T) {
	root := t.TempDir()
	for _, ts := range []string{"2026-10-30T23:59:59Z", "2026-11-01T00:00:01Z"} {
		if err := Append(root, Entry{TS: ts, Cmd: "ls"}); err != nil {
			t.Fatalf("append %s: %v", ts, err)
		}
	}
	for _, month := range []string{"2026-10", "2026-11"} {
		path := filepath.Join(Dir(root), filePrefix+month+fileSuffix)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
}

func TestPruneDropsOldMonths(t *testing.T) {
	root := t.TempDir()
	dir := Dir(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, month := range []string{"2026-01", "2026-04", "2026-10"} {
		path := filepath.Join(dir, filePrefix+month+fileSuffix)
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Six months before 2026-10 is 2026-04, so January goes and April stays.
	if err := Append(root, Entry{TS: "2026-10-30T06:00:00Z", Cmd: "ls"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filePrefix+"2026-01"+fileSuffix)); !os.IsNotExist(err) {
		t.Fatalf("2026-01 should be pruned, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filePrefix+"2026-04"+fileSuffix)); err != nil {
		t.Fatalf("2026-04 should be kept: %v", err)
	}
}

func TestReadSince(t *testing.T) {
	root := t.TempDir()
	for _, ts := range []string{"2026-10-01T00:00:00Z", "2026-10-30T00:00:00Z"} {
		if err := Append(root, Entry{TS: ts, Cmd: "ls"}); err != nil {
			t.Fatal(err)
		}
	}
	since := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	entries, err := Read(root, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].TS != "2026-10-30T00:00:00Z" {
		t.Fatalf("read since = %+v", entries)
	}
}

func TestReadMissingDirectory(t *testing.T) {
	entries, err := Read(t.TempDir(), time.Time{})
	if err != nil {
		t.Fatalf("missing usage dir must not fail: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %+v", entries)
	}
}

func TestReadSkipsMalformedLine(t *testing.T) {
	root := t.TempDir()
	if err := Append(root, Entry{TS: "2026-10-30T06:00:00Z", Cmd: "ls"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(root), filePrefix+"2026-10"+fileSuffix)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not json\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(root, time.Time{})
	if err != nil {
		t.Fatalf("a malformed tail must not fail the read: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected the good line only, got %+v", entries)
	}
}

func TestAppendUnwritableIsAnError(t *testing.T) {
	root := t.TempDir()
	// .sdt is a file, so the usage directory cannot be created under it.
	if err := os.WriteFile(filepath.Join(root, stateDir), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, Entry{TS: "2026-10-30T06:00:00Z", Cmd: "ls"}); err == nil {
		t.Fatal("expected an error from an unwritable usage directory")
	}
}

func TestAppendRejectsEmptyRoot(t *testing.T) {
	if err := Append("", Entry{TS: "2026-10-30T06:00:00Z", Cmd: "ls"}); err == nil {
		t.Fatal("expected an error with no project root")
	}
}

func TestAppendRejectsBadTimestamp(t *testing.T) {
	if err := Append(t.TempDir(), Entry{TS: "yesterday", Cmd: "ls"}); err == nil {
		t.Fatal("expected an error for a timestamp that is not RFC 3339")
	}
}

func TestParseWindow(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"all", time.Time{}},
		{"", time.Time{}},
		{"7d", now.Add(-7 * 24 * time.Hour)},
		{"24h", now.Add(-24 * time.Hour)},
		{"90m", now.Add(-90 * time.Minute)},
	}
	for _, tc := range cases {
		got, err := ParseWindow(tc.in, now)
		if err != nil {
			t.Fatalf("ParseWindow(%q): %v", tc.in, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("ParseWindow(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := ParseWindow("whenever", now); err == nil {
		t.Fatal("expected an error for an unparsable window")
	}
}

func TestBeginHitsFinish(t *testing.T) {
	root := t.TempDir()
	Begin("context search")
	Hits(0)
	CountOut(11)
	CountOut(5)
	if err := Finish(root, 1); err != nil {
		t.Fatalf("finish: %v", err)
	}
	entries, err := Read(root, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %+v", entries)
	}
	e := entries[0]
	if e.Cmd != "context search" || e.Exit != 1 || e.Out != 16 {
		t.Fatalf("entry = %+v", e)
	}
	if e.Hits == nil || *e.Hits != 0 {
		t.Fatalf("a reported zero must survive: %+v", e.Hits)
	}
	if e.Source != "" {
		t.Fatalf("a CLI call carries no source: %+v", e)
	}
}

func TestBeginInternalSource(t *testing.T) {
	t.Setenv(EnvSource, "internal")
	root := t.TempDir()
	Begin("context status set")
	if err := Finish(root, 0); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(root, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Source != SourceInternal {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestFinishWithoutBegin(t *testing.T) {
	if err := Finish(t.TempDir(), 0); err != nil {
		t.Fatalf("finish without begin must be a no-op: %v", err)
	}
}

func TestFinishTwiceWritesOneEntry(t *testing.T) {
	root := t.TempDir()
	Begin("ls")
	if err := Finish(root, 0); err != nil {
		t.Fatal(err)
	}
	if err := Finish(root, 0); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(root, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
}

// TestEntryKeysAreTheContract pins the wire shape: a new field is a decision,
// and no field may ever carry an argument, a path or a query.
func TestEntryKeysAreTheContract(t *testing.T) {
	root := t.TempDir()
	Begin("b64")
	Finish(root, 0) //nolint:errcheck // the read below is the assertion
	entries, err := Read(root, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	line, err := os.ReadFile(filepath.Join(Dir(root), filePrefix+entries[0].TS[:7]+fileSuffix))
	if err != nil {
		t.Fatal(err)
	}
	got := string(line)
	for _, key := range []string{`"ts"`, `"cmd"`, `"exit"`, `"ms"`, `"out"`} {
		if !strings.Contains(got, key) {
			t.Fatalf("record %s is missing %s", got, key)
		}
	}
	for _, key := range []string{`"hits"`, `"source"`} {
		if strings.Contains(got, key) {
			t.Fatalf("record %s must not carry %s unless it is known", got, key)
		}
	}
}
