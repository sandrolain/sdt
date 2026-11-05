// Package progress implements the single narration protocol every sdt command
// uses for long work: throttled tick lines while it runs, and exactly one
// closing `sdt-done` line per invocation. Everything goes to stderr, so stdout
// keeps carrying the command's result.
package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Mode is the narration mode selected by the global --progress flag.
type Mode string

const (
	// ModeAuto picks human on a terminal and none anywhere else.
	ModeAuto Mode = "auto"
	// ModeHuman draws a bar plus the closing line.
	ModeHuman Mode = "human"
	// ModeJSON prints the machine tick lines plus the same closing line.
	ModeJSON Mode = "json"
	// ModeNone prints nothing.
	ModeNone Mode = "none"
)

const (
	// minGap is the hard floor between two tick lines: a phase change bypasses
	// the 15 s window, never this gap, so a burst of phase changes cannot flood
	// a transcript.
	minGap = 2 * time.Second
	// interval is the steady-state window between two ticks of the same phase.
	interval = 15 * time.Second
	// barWidth is the human bar's width in cells.
	barWidth = 20
)

// ParseMode validates a --progress value.
func ParseMode(v string) (Mode, error) {
	switch m := Mode(v); m {
	case ModeAuto, ModeHuman, ModeJSON, ModeNone:
		return m, nil
	default:
		return "", fmt.Errorf("invalid --progress %q: use auto, human, json or none", v)
	}
}

// Resolve turns the requested mode into the one actually used: auto means
// human on a terminal, and silence when --quiet asked for no chatter or the
// stream is piped.
func Resolve(mode Mode, w io.Writer, quiet bool) Mode {
	if mode != ModeAuto {
		return mode
	}
	if quiet || !IsTerminal(w) {
		return ModeNone
	}
	return ModeHuman
}

// IsTerminal reports whether w is a character device. It reads the stat bit
// instead of linking a terminal-detection library, which would pull cgo or a
// dependency for one boolean.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// Reporter writes the protocol's lines. A nil Reporter is a valid no-op, so a
// command that never started one (an early usage error) still calls Finish
// safely.
type Reporter struct {
	mode Mode
	w    io.Writer
	now  func() time.Time

	start    time.Time
	last     time.Time
	phase    string
	finished bool
}

// New returns a Reporter in mode writing to w, starting the wall clock.
func New(w io.Writer, mode Mode) *Reporter {
	now := time.Now
	return &Reporter{mode: mode, w: w, now: now, start: now()}
}

// Mode reports the resolved mode.
func (r *Reporter) Mode() Mode {
	if r == nil {
		return ModeNone
	}
	return r.mode
}

// Progress reports one tick of phase. Throttling lives here, not at the call
// site: the first tick of a phase always shows, a phase change or the phase's
// last tick bypasses the 15 s window, and nothing is printed twice inside 2 s.
func (r *Reporter) Progress(phase string, done, total int) {
	if r == nil || r.finished || r.mode == ModeNone {
		return
	}
	now := r.now()
	if !r.last.IsZero() {
		since := now.Sub(r.last)
		if since < minGap {
			return
		}
		complete := total > 0 && done >= total
		if phase == r.phase && !complete && since < interval {
			return
		}
	}
	r.phase = phase
	r.last = now
	line := r.tick(phase, done, total)
	if _, err := fmt.Fprintln(r.w, line); err != nil {
		// A stream that cannot take the narration is not a command failure: the
		// result on stdout is still what the caller asked for.
		return
	}
}

// Finish writes the closing line, exactly once per invocation.
func (r *Reporter) Finish(ok bool, code int, reason string) {
	if r == nil || r.finished {
		return
	}
	r.finished = true
	if r.mode == ModeNone {
		return
	}
	wall := r.now().Sub(r.start).Seconds()
	if _, err := fmt.Fprintf(r.w, "sdt-done ok=%t exit=%d reason=%s wall=%.3fs\n", ok, code, Slug(reason), wall); err != nil {
		return
	}
}

// tick renders one progress line in the reporter's mode.
func (r *Reporter) tick(phase string, done, total int) string {
	if r.mode == ModeHuman {
		return fmt.Sprintf("sdt-bar phase=%s [%s] %d/%d", Slug(phase), bar(done, total), done, total)
	}
	return fmt.Sprintf("sdt-progress phase=%s done=%d total=%d", Slug(phase), done, total)
}

// bar draws the human progress bar, filled proportionally to done over total.
func bar(done, total int) string {
	filled := 0
	if total > 0 {
		filled = done * barWidth / total
	}
	filled = max(0, min(filled, barWidth))
	return strings.Repeat("=", filled) + strings.Repeat("-", barWidth-filled)
}

// Slug makes a value safe for one key=value token: no spaces, no newlines.
func Slug(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "done"
	}
	return strings.Join(strings.Fields(v), "-")
}
