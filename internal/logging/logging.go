// Package logging builds the log/slog handler shared by the sdt CLI and
// sdtviewer: tinted text on a terminal, plain text when piped, JSON on
// request. It owns handler construction, TTY detection and colour gating so
// the two binaries cannot drift.
package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// Options configures the logger built by New and Setup.
type Options struct {
	// Writer receives the log lines; os.Stderr in both binaries so stdout
	// stays the command result channel.
	Writer io.Writer
	// Level is the minimum level emitted.
	Level slog.Level
	// NoColor disables colour regardless of the destination.
	NoColor bool
	// JSON selects slog's JSON handler instead of the tinted text handler.
	JSON bool
	// Time keeps the timestamp on each line; the CLI drops it, the long
	// running viewer keeps it.
	Time bool
}

// New returns a logger writing to opts.Writer, defaulting to os.Stderr.
func New(opts Options) *slog.Logger {
	if opts.Writer == nil {
		opts.Writer = os.Stderr
	}
	return slog.New(newHandler(opts))
}

// Setup builds the logger and installs it as the slog default, so packages
// call slog.Info/Warn/Error directly instead of threading a logger around.
func Setup(opts Options) *slog.Logger {
	logger := New(opts)
	slog.SetDefault(logger)
	return logger
}

// newHandler picks the tinted text handler or the JSON handler and applies
// the level, colour and timestamp options.
func newHandler(opts Options) slog.Handler {
	if opts.JSON {
		return slog.NewJSONHandler(opts.Writer, &slog.HandlerOptions{
			Level:       opts.Level,
			ReplaceAttr: replaceTime(opts.Time),
		})
	}
	return tint.NewTextHandler(opts.Writer, &tint.Options{
		Level:       opts.Level,
		NoColor:     !ColorEnabled(opts.Writer, opts.NoColor),
		ReplaceAttr: replaceTime(opts.Time),
	})
}

// replaceTime returns a ReplaceAttr func dropping the timestamp when the
// caller does not want it. A zero Attr makes the handler discard the field.
func replaceTime(keepTime bool) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, attr slog.Attr) slog.Attr {
		if !keepTime && len(groups) == 0 && attr.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return attr
	}
}

// IsTerminal reports whether w is a character device, i.e. a terminal. It is
// the same os.ModeCharDevice test stdinIsTTY uses, so no extra dependency is
// needed.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ColorEnabled reports whether ANSI colour may be written to w: w must be a
// terminal and neither noColor nor the NO_COLOR environment variable may
// suppress it.
func ColorEnabled(w io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	return IsTerminal(w)
}
