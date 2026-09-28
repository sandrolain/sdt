package cmd

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setPersistent sets a root persistent flag for the duration of the test.
func setPersistent(t *testing.T, name, value string) {
	t.Helper()
	resetCmdFlags(rootCmd)
	t.Cleanup(func() { resetCmdFlags(rootCmd) })
	if err := rootCmd.PersistentFlags().Set(name, value); err != nil {
		t.Fatalf("set --%s: %v", name, err)
	}
}

func TestLogFormatFlagExists(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("log-format")
	if f == nil {
		t.Fatal("--log-format is not a root persistent flag")
	}
	if f.DefValue != "text" {
		t.Errorf("--log-format default = %q, want text", f.DefValue)
	}
	if !strings.Contains(f.Usage, "text|json") {
		t.Errorf("--log-format usage %q does not mention the accepted values", f.Usage)
	}
	if help := rootCmd.UsageString(); !strings.Contains(help, "--log-format") {
		t.Error("--log-format is missing from the root usage")
	}
}

func TestInstallLoggerLevel(t *testing.T) {
	tests := []struct {
		name       string
		quiet      string
		wantInfo   bool
		wantWarn   bool
		wantInText string
	}{
		{name: "default shows info and warn", wantInfo: true, wantWarn: true, wantInText: "INF visible"},
		{name: "quiet hides info, keeps warn", quiet: "true", wantWarn: true, wantInText: "WRN visible"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.quiet != "" {
				setPersistent(t, "quiet", tt.quiet)
			} else {
				resetCmdFlags(rootCmd)
				t.Cleanup(func() { resetCmdFlags(rootCmd) })
			}
			logs := captureLogs(t)

			// installLogger owns the routing; captureLogs pre-installs a buffer
			// logger, so re-run it against the test buffer.
			rootCmd.SetErr(logs)
			t.Cleanup(func() { rootCmd.SetErr(nil) })
			installLogger(rootCmd)

			slog.Info("visible")
			slog.Warn("visible")
			got := logs.String()
			if strings.Contains(got, "INF visible") != tt.wantInfo {
				t.Errorf("info line present = %v, want %v (%q)", !tt.wantInfo, tt.wantInfo, got)
			}
			if !strings.Contains(got, "WRN visible") {
				t.Errorf("warn line missing from %q", got)
			}
			if !tt.wantWarn {
				t.Error("wantWarn = false but the warn line was present")
			}
		})
	}
}

func TestInstallLoggerJSONFormat(t *testing.T) {
	setPersistent(t, "log-format", "json")
	logs := captureLogs(t)
	rootCmd.SetErr(logs)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	installLogger(rootCmd)

	slog.Warn("converted", "path", "out.md")
	raw := strings.TrimSpace(logs.String())
	if strings.Contains(raw, "\x1b[") {
		t.Errorf("json log %q contains ANSI", raw)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", raw, err)
	}
	if rec["level"] != "WARN" || rec["msg"] != "converted" || rec["path"] != "out.md" {
		t.Errorf("json record = %v, want WARN/converted/out.md", rec)
	}
	if _, ok := rec["time"]; ok {
		t.Errorf("CLI json record %v carries a timestamp, want none", rec)
	}
}

func TestInstallLoggerTextFormatHasNoANSIWhenPiped(t *testing.T) {
	setPersistent(t, "log-format", "text")
	logs := captureLogs(t)
	rootCmd.SetErr(logs)
	t.Cleanup(func() { rootCmd.SetErr(nil) })
	installLogger(rootCmd)

	slog.Error("boom", "err", "broken")
	got := logs.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("text log %q contains ANSI even though the stream is not a terminal", got)
	}
	if !strings.Contains(got, "ERR boom") {
		t.Errorf("text log %q has no error line", got)
	}
}

func TestInstallLoggerRejectsUnknownFormat(t *testing.T) {
	setPersistent(t, "log-format", "yaml")
	logs := captureLogs(t)
	rootCmd.SetErr(logs)
	t.Cleanup(func() { rootCmd.SetErr(nil) })

	shouldExitWithCode(t, 1, func() string {
		installLogger(rootCmd)
		return ""
	})
	if got := logs.String(); !strings.Contains(got, "invalid --log-format") {
		t.Errorf("log %q does not report the invalid --log-format", got)
	}
}

func TestExecuteWithInvalidLogFormat(t *testing.T) {
	logs := captureLogs(t)
	rootCmd.SetErr(logs)
	t.Cleanup(func() { rootCmd.SetErr(nil) })

	shouldExitWithCode(t, 1, func() string {
		resetCmdFlags(rootCmd)
		execute(t, schemaCmd, nil, "--log-format", "yaml")
		return ""
	})
	if got := logs.String(); !strings.Contains(got, "invalid --log-format") {
		t.Errorf("log %q does not report the invalid --log-format", got)
	}
}

// TestLoggingKeepsStdoutResultOnly checks the contract the plan protects:
// command results stay on stdout, diagnostics go to stderr.
func TestLoggingKeepsStdoutResultOnly(t *testing.T) {
	setPersistent(t, "log-format", "text")
	out := new(bytes.Buffer)
	logs := captureLogs(t)
	rootCmd.SetOut(out)
	rootCmd.SetErr(logs)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	got := execute(t, schemaCmd, nil, "--command", "tokens")
	if len(got) == 0 {
		t.Fatal("no result on stdout")
	}
	if logs.Len() != 0 {
		t.Errorf("stderr log %q should be empty for a successful command", logs.String())
	}
	if strings.Contains(out.String(), "ERR ") {
		t.Errorf("stdout %q carries a log line", out.String())
	}
}

// TestExecuteWithErrorLogsToStderr asserts an error surfaces through the
// shared logger on stderr, with exit code 1 and stdout left empty.
func TestExecuteWithErrorLogsToStderr(t *testing.T) {
	out := new(bytes.Buffer)
	logs := captureLogs(t)
	rootCmd.SetOut(out)
	rootCmd.SetErr(logs)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	shouldExitWithCode(t, 1, func() string {
		resetCmdFlags(rootCmd)
		execute(t, schemaCmd, nil, "--command", "nonexistent-xyz")
		return ""
	})
	if got := logs.String(); !strings.Contains(got, "ERR") || !strings.Contains(got, "nonexistent-xyz") {
		t.Errorf("stderr log %q does not carry the error", got)
	}
	if out.Len() != 0 {
		t.Errorf("stdout %q must stay empty on error", out.String())
	}
}

func TestExitWithErrorIgnoresNilError(t *testing.T) {
	logs := captureLogs(t)
	exited := -1
	origExit := exit
	exit = func(code int) { exited = code }
	t.Cleanup(func() { exit = origExit })

	exitWithError((*cobra.Command)(nil), nil)
	if exited != -1 {
		t.Errorf("exitWithError(nil error) exited with %d, want no exit", exited)
	}
	if logs.Len() != 0 {
		t.Errorf("exitWithError(nil error) logged %q", logs.String())
	}
}
