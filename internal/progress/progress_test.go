package progress

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseMode(t *testing.T) {
	for _, v := range []string{"auto", "human", "json", "none"} {
		if _, err := ParseMode(v); err != nil {
			t.Fatalf("ParseMode(%q): unexpected error %v", v, err)
		}
	}
	if _, err := ParseMode("verbose"); err == nil {
		t.Fatal("ParseMode(verbose): expected an error")
	}
}

func TestResolve(t *testing.T) {
	var piped bytes.Buffer
	cases := []struct {
		name  string
		mode  Mode
		quiet bool
		want  Mode
	}{
		{"auto piped", ModeAuto, false, ModeNone},
		{"auto quiet", ModeAuto, true, ModeNone},
		{"human piped", ModeHuman, false, ModeHuman},
		{"human quiet", ModeHuman, true, ModeHuman},
		{"json piped", ModeJSON, false, ModeJSON},
		{"none", ModeNone, false, ModeNone},
	}
	for _, tc := range cases {
		if got := Resolve(tc.mode, &piped, tc.quiet); got != tc.want {
			t.Fatalf("%s: Resolve = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	if IsTerminal(&bytes.Buffer{}) {
		t.Fatal("a buffer is not a terminal")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if IsTerminal(f) {
		t.Fatal("a regular file is not a terminal")
	}
}

// newClockReporter returns a reporter whose clock the test advances by hand.
func newClockReporter(buf *bytes.Buffer, mode Mode) (*Reporter, *time.Time) {
	now := time.Date(2026, 10, 30, 6, 0, 0, 0, time.UTC)
	clock := &now
	r := New(buf, mode)
	r.now = func() time.Time { return *clock }
	r.start = *clock
	return r, clock
}

func TestProgressThrottle(t *testing.T) {
	buf := new(bytes.Buffer)
	r, clock := newClockReporter(buf, ModeJSON)

	at := func(offset time.Duration, phase string, done, total int) {
		*clock = clock.Add(offset)
		r.Progress(phase, done, total)
	}

	at(0, "embed", 1, 100)                    // first tick of the phase
	at(1*time.Second, "embed", 2, 100)        // inside the 2 s floor
	at(2*time.Second, "embed", 3, 100)        // inside the 15 s window, same phase
	at(13*time.Second, "embed", 4, 100)       // 16 s after the printed tick: printed
	at(16*time.Second, "embed", 5, 100)       // 16 s later: printed
	at(500*time.Millisecond, "decode", 0, 10) // phase change inside the 2 s floor
	at(2*time.Second, "decode", 1, 10)        // phase change, floor satisfied

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := []string{
		"sdt-progress phase=embed done=1 total=100",
		"sdt-progress phase=embed done=4 total=100",
		"sdt-progress phase=embed done=5 total=100",
		"sdt-progress phase=decode done=1 total=10",
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d ticks, got %d: %q", len(want), len(lines), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("tick %d = %q, want %q", i, lines[i], w)
		}
	}
}

func TestProgressCompleteBypassesInterval(t *testing.T) {
	buf := new(bytes.Buffer)
	r, clock := newClockReporter(buf, ModeJSON)
	r.Progress("embed", 1, 10)
	*clock = clock.Add(5 * time.Second)
	r.Progress("embed", 10, 10)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected the completing tick to print, got %q", lines)
	}
}

func TestProgressHumanBar(t *testing.T) {
	buf := new(bytes.Buffer)
	r, _ := newClockReporter(buf, ModeHuman)
	r.Progress("embed", 5, 10)
	got := strings.TrimSpace(buf.String())
	want := "sdt-bar phase=embed [" + strings.Repeat("=", 10) + strings.Repeat("-", 10) + "] 5/10"
	if got != want {
		t.Fatalf("bar = %q, want %q", got, want)
	}
}

func TestFinishOnce(t *testing.T) {
	buf := new(bytes.Buffer)
	r, clock := newClockReporter(buf, ModeHuman)
	*clock = clock.Add(1500 * time.Millisecond)
	r.Finish(true, 0, "done")
	r.Finish(false, 1, "error")
	got := strings.TrimSpace(buf.String())
	want := "sdt-done ok=true exit=0 reason=done wall=1.500s"
	if got != want {
		t.Fatalf("finish = %q, want %q", got, want)
	}
}

func TestNonePrintsNothing(t *testing.T) {
	buf := new(bytes.Buffer)
	r, _ := newClockReporter(buf, ModeNone)
	r.Progress("embed", 1, 10)
	r.Finish(true, 0, "done")
	if buf.Len() != 0 {
		t.Fatalf("none mode wrote %q", buf.String())
	}
}

func TestNilReporterIsNoop(t *testing.T) {
	var r *Reporter
	r.Progress("embed", 1, 10)
	r.Finish(true, 0, "done")
	if r.Mode() != ModeNone {
		t.Fatal("a nil reporter reports no mode")
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("  pages failed  "); got != "pages-failed" {
		t.Fatalf("slug = %q", got)
	}
	if got := Slug(""); got != "done" {
		t.Fatalf("empty slug = %q", got)
	}
}
