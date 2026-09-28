package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"
)

// charDevicePath is a character device on the unix targets this project
// supports; opening it exercises the TTY branch without a real terminal.
const charDevicePath = "/dev/null"

// notAFile is a Writer that is not an *os.File, e.g. an in-memory buffer
// wrapped by a command.
type notAFile struct{}

func (notAFile) Write(p []byte) (int, error) { return len(p), nil }

func TestIsTerminal(t *testing.T) {
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer pipeR.Close()
	defer pipeW.Close()

	dev, err := os.Open(charDevicePath)
	if err != nil {
		t.Skipf("open %s: %v", charDevicePath, err)
	}

	closed, err := os.Open(charDevicePath)
	if err != nil {
		t.Fatalf("open %s: %v", charDevicePath, err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	tests := []struct {
		name string
		w    io.Writer
		want bool
	}{
		{"pipe", pipeW, false},
		{"buffer", &bytes.Buffer{}, false},
		{"char device", dev, true},
		{"closed file", closed, false},
		{"not a file", notAFile{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTerminal(tt.w); got != tt.want {
				t.Errorf("IsTerminal(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestColorEnabled(t *testing.T) {
	t.Run("pipe is never coloured", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()
		if ColorEnabled(w, false) {
			t.Error("ColorEnabled(pipe, false) = true, want false")
		}
	})

	tests := []struct {
		name    string
		noColor bool
		env     string
		want    bool
	}{
		{"terminal and no suppression", false, "", true},
		{"caller asked for no color", true, "", false},
		{"NO_COLOR set", false, "1", false},
		{"NO_COLOR empty means unset", false, "", true},
		{"both suppress", true, "1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.env)
			dev, err := os.Open(charDevicePath)
			if err != nil {
				t.Skipf("open %s: %v", charDevicePath, err)
			}
			defer dev.Close()
			if got := ColorEnabled(dev, tt.noColor); got != tt.want {
				t.Errorf("ColorEnabled(%s, %v) with NO_COLOR=%q = %v, want %v",
					tt.name, tt.noColor, tt.env, got, tt.want)
			}
		})
	}
}

func TestNewTextHandler(t *testing.T) {
	timeRe := regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d{3}`)

	tests := []struct {
		name       string
		opts       Options
		log        func(*slog.Logger)
		want       []string
		wantAbsent []string
	}{
		{
			name: "cli shape without time",
			opts: Options{NoColor: true},
			log:  func(l *slog.Logger) { l.Info("converted", "path", "out.md") },
			want: []string{"INF converted", "path=out.md"},
			// A dropped timestamp must not leave a field behind.
			wantAbsent: []string{"time="},
		},
		{
			name: "viewer shape keeps time",
			opts: Options{NoColor: true, Time: true},
			log:  func(l *slog.Logger) { l.Info("serving", "addr", "127.0.0.1:8443") },
			want: []string{"INF serving", "addr=127.0.0.1:8443"},
		},
		{
			name:       "no color emits no ANSI",
			opts:       Options{NoColor: true},
			log:        func(l *slog.Logger) { l.Error("boom", "err", "broken") },
			want:       []string{"ERR boom", "err=broken"},
			wantAbsent: []string{"\x1b["},
		},
		{
			name:       "color requested without a terminal stays plain",
			opts:       Options{},
			log:        func(l *slog.Logger) { l.Error("boom") },
			want:       []string{"ERR boom"},
			wantAbsent: []string{"\x1b["},
		},
		{
			name: "quiet level hides info and debug",
			opts: Options{Level: slog.LevelWarn, NoColor: true},
			log: func(l *slog.Logger) {
				l.Debug("debug line")
				l.Info("info line")
				l.Warn("warn line")
			},
			want:       []string{"WRN warn line"},
			wantAbsent: []string{"debug line", "info line"},
		},
		{
			name: "error level keeps warnings and errors",
			opts: Options{Level: slog.LevelError, NoColor: true},
			log: func(l *slog.Logger) {
				l.Warn("warn line")
				l.Error("error line")
			},
			want:       []string{"ERR error line"},
			wantAbsent: []string{"warn line"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := tt.opts
			opts.Writer = &buf
			tt.log(New(opts))
			got := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("output %q does not contain %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("output %q unexpectedly contains %q", got, absent)
				}
			}
		})
	}

	t.Run("time present when requested", func(t *testing.T) {
		var buf bytes.Buffer
		New(Options{Writer: &buf, NoColor: true, Time: true}).Info("serving")
		if got := buf.String(); !timeRe.MatchString(got) {
			t.Errorf("output %q carries no timestamp, want one", got)
		}
	})
}

func TestNewJSONHandler(t *testing.T) {
	tests := []struct {
		name       string
		time       bool
		log        func(*slog.Logger)
		wantAbsent []string
	}{
		{
			name:       "cli json has no time",
			log:        func(l *slog.Logger) { l.Warn("search unavailable", "err", "no index") },
			wantAbsent: []string{"time"},
		},
		{
			name: "viewer json keeps time",
			time: true,
			log:  func(l *slog.Logger) { l.Warn("search unavailable", "err", "no index") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			New(Options{Writer: &buf, JSON: true, Time: tt.time}).
				Warn("search unavailable", "err", "no index")
			raw := buf.String()
			if strings.Contains(raw, "\x1b[") {
				t.Errorf("json output %q contains ANSI", raw)
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &rec); err != nil {
				t.Fatalf("json.Unmarshal(%q): %v", raw, err)
			}
			if rec["level"] != "WARN" {
				t.Errorf("level = %v, want WARN", rec["level"])
			}
			if rec["msg"] != "search unavailable" {
				t.Errorf("msg = %v, want search unavailable", rec["msg"])
			}
			if rec["err"] != "no index" {
				t.Errorf("err = %v, want no index", rec["err"])
			}
			_, hasTime := rec["time"]
			if hasTime != tt.time {
				t.Errorf("json has time = %v, want %v (%q)", hasTime, tt.time, raw)
			}
			for _, absent := range tt.wantAbsent {
				if _, ok := rec[absent]; ok {
					t.Errorf("json record %v unexpectedly has %q", rec, absent)
				}
			}
		})
	}

	t.Run("level filters json records", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(Options{Writer: &buf, JSON: true, Level: slog.LevelError})
		logger.Info("hidden")
		logger.Error("shown")
		if strings.Contains(buf.String(), "hidden") {
			t.Errorf("output %q kept an info record at error level", buf.String())
		}
		if !strings.Contains(buf.String(), "shown") {
			t.Errorf("output %q dropped the error record", buf.String())
		}
	})
}

func TestNewDefaultsToStderr(t *testing.T) {
	// A nil Writer must fall back to os.Stderr instead of panicking.
	if logger := New(Options{}); logger == nil {
		t.Fatal("New returned nil")
	}
}

func TestSetupInstallsSlogDefault(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	var buf bytes.Buffer
	installed := Setup(Options{Writer: &buf, NoColor: true})
	if installed == nil {
		t.Fatal("Setup returned nil")
	}
	slog.Info("through the default logger")
	if got := buf.String(); !strings.Contains(got, "through the default logger") {
		t.Errorf("default logger wrote %q, want the message", got)
	}
	if !strings.Contains(buf.String(), "INF through the default logger") {
		t.Errorf("output %q does not use the installed handler", buf.String())
	}
}
