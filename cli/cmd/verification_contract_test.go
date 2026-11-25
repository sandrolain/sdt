package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

type gateClosedWriter struct{}

func (gateClosedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestGateReportClosedStderrPreservesStepResults(t *testing.T) {
	root := runInTempDir(t)
	steps := []GateStep{
		{Name: "ok", Run: "echo success"},
		{Name: "bad", Run: "echo first; sleep 0.05; echo last >&2; exit 3"},
	}
	writeGateConfig(t, steps...)
	report, err := newGateReport(root, steps)
	if err != nil {
		t.Fatal(err)
	}
	if failed := report.execute(context.Background(), root, gateClosedWriter{}); failed != "bad" {
		t.Fatalf("failed step = %q, want bad", failed)
	}
	if report.Results[0].Status != "passed" || report.Results[1].Reason != "exit status 3" {
		t.Fatalf("unexpected step results: %+v", report.Results)
	}
	full := mustReadFile(t, report.Results[1].Log)
	if !strings.Contains(full, "first\n") || !strings.Contains(full, "last\n") {
		t.Fatalf("incomplete failure log: %q", full)
	}
}

func TestGateReportYAMLKeepsChildOutputOnStderr(t *testing.T) {
	runInTempDir(t)
	writeGateConfig(t, GateStep{Name: "ok", Run: "echo child-stdout; echo child-stderr >&2"})
	var stderr bytes.Buffer
	previousStderr := rootCmd.ErrOrStderr()
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() { rootCmd.SetErr(previousStderr) })
	out := execute(t, agentGateCmd, nil, "--format", "yaml")
	var report gateReport
	if err := yaml.Unmarshal(out, &report); err != nil {
		t.Fatalf("decode YAML report: %v: %s", err, out)
	}
	if err := report.validate(); err != nil || report.Status != "passed" {
		t.Fatalf("unexpected YAML report: %+v (%v)", report, err)
	}
	for _, marker := range []string{"child-stdout", "child-stderr"} {
		if !strings.Contains(stderr.String(), marker+"\n") || strings.Contains("\n"+string(out), "\n"+marker+"\n") {
			t.Fatalf("misplaced %q: stdout=%q stderr=%q", marker, out, stderr.String())
		}
	}
}

func TestGateReportCompleteAndShared(t *testing.T) {
	path := gateRecordedTask(t)
	writeGateConfig(t,
		GateStep{Name: "ok", Run: "printf 'ok-out\\n'; printf 'ok-err\\n' >&2"},
		GateStep{Name: "bad", Run: "printf 'boom-out\\n'; printf 'boom-err\\n' >&2; exit 3"},
		GateStep{Name: "unknown", Run: "touch should-not-exist"},
		GateStep{Name: "omitted", Run: "false", Exclude: "not applicable to this target"})
	var stderr bytes.Buffer
	previousStderr := rootCmd.ErrOrStderr()
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() { rootCmd.SetErr(previousStderr) })
	var out string
	shouldExitWithCode(t, 1, func() string {
		out = string(execute(t, agentGateCmd, nil, "--format", "json", "--record", "--plan", "p.md"))
		return out
	})
	var report gateReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if err := report.validate(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ok-out\n", "ok-err\n", "boom-out\n", "boom-err\n"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("child output %q was not forwarded to stderr: %q", want, stderr.String())
		}
	}
	if !strings.Contains(report.Results[1].Detail, "boom-out") || !strings.Contains(report.Results[1].Detail, "boom-err") {
		t.Fatalf("failure detail lost child output: %+v", report.Results[1])
	}
	if report.SchemaVersion != 1 || report.Status != "failed" || len(report.ConfigSHA256) != 64 || report.Revision == "" {
		t.Fatalf("missing provenance or false pass: %+v", report)
	}
	for i, status := range []string{"passed", "failed", "not-run", "excluded"} {
		if report.Results[i].Status != status {
			t.Fatalf("result %d: %+v", i, report.Results[i])
		}
	}
	if _, err := os.Stat("should-not-exist"); !os.IsNotExist(err) {
		t.Fatal("gate continued after failure")
	}
	record := mustReadFile(t, path)
	for _, result := range report.Results {
		if !strings.Contains(record, result.Step+": "+result.Status+" (") || !strings.Contains(record, result.Reason) {
			t.Fatalf("record differs from JSON: %s", record)
		}
	}
	if !strings.Contains(record, report.ConfigSHA256) {
		t.Fatal("record lost configuration provenance")
	}
}

func TestGateReportCancellationExclusionAndIntegrity(t *testing.T) {
	root := runInTempDir(t)
	for _, steps := range [][]GateStep{
		{{Name: "excluded", Exclude: "manual target"}},
		{{Name: "pending", Run: "touch should-not-run"}},
	} {
		writeGateConfig(t, steps...)
		report, err := newGateReport(root, steps)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		report.execute(ctx, root, io.Discard)
		if report.Status == "passed" {
			t.Fatal("excluded/cancelled gate passed")
		}
		if err := report.validate(); err != nil {
			t.Fatal(err)
		}
		report.Results = nil
		if report.validate() == nil {
			t.Fatal("incomplete report accepted")
		}
	}
	if validateGateSteps([]GateStep{{Name: "same", Run: "true"}, {Name: "same", Run: "false"}}) == nil {
		t.Fatal("overlapping step names accepted")
	}
}

func TestGateReportStreamsBeforeStepExits(t *testing.T) {
	root := runInTempDir(t)
	steps := []GateStep{{Name: "wait", Run: "printf 'early output\\n'; while [ ! -f release ]; do sleep 0.01; done"}}
	writeGateConfig(t, steps...)
	report, err := newGateReport(root, steps)
	if err != nil {
		t.Fatal(err)
	}

	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan string, 1)
	go func() {
		done <- report.execute(ctx, root, writer)
		_ = writer.Close()
	}()

	type readResult struct {
		line string
		err  error
	}
	lineReady := make(chan readResult, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadString('\n')
		lineReady <- readResult{line: line, err: err}
	}()
	select {
	case got := <-lineReady:
		if got.err != nil || got.line != "early output\n" {
			t.Fatalf("streamed output = %q, %v", got.line, got.err)
		}
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("gate did not stream output before the child exited")
	}
	select {
	case <-done:
		t.Fatal("gate returned while the child was still waiting")
	default:
	}
	if err := os.WriteFile(filepath.Join(root, "release"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case failed := <-done:
		if failed != "" || report.Status != gateStatusPassed {
			t.Fatalf("gate result = %q, status %q", failed, report.Status)
		}
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("gate did not finish after releasing the child")
	}
	_ = reader.Close()
}

func TestGateReportCancellationWaitsForBoundedOutputAndMarksRemainingStepsNotRun(t *testing.T) {
	root := runInTempDir(t)
	steps := []GateStep{
		{Name: "wait", Run: "sleep 60 & echo $! > child.pid; printf 'ready\\n'; wait"},
		{Name: "remaining", Run: "touch should-not-run"},
	}
	writeGateConfig(t, steps...)
	report, err := newGateReport(root, steps)
	if err != nil {
		t.Fatal(err)
	}
	childPIDPath := filepath.Join(root, "child.pid")
	killChild := func() {
		data, err := os.ReadFile(childPIDPath)
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return
		}
		process, err := os.FindProcess(pid)
		if err == nil {
			_ = process.Kill()
		}
	}
	t.Cleanup(killChild)

	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan string, 1)
	go func() {
		done <- report.execute(ctx, root, writer)
		_ = writer.Close()
	}()
	type readResult struct {
		line string
		err  error
	}
	lineReady := make(chan readResult, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadString('\n')
		lineReady <- readResult{line: line, err: err}
	}()
	select {
	case got := <-lineReady:
		if got.err != nil || got.line != "ready\n" {
			t.Fatalf("streamed output = %q, %v", got.line, got.err)
		}
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("gate did not start the cancellable step")
	}

	cancel()
	started := time.Now()
	select {
	case failed := <-done:
		if failed != "wait" {
			t.Fatalf("failed step = %q, want wait", failed)
		}
		if elapsed := time.Since(started); elapsed > 3*time.Second {
			t.Fatalf("gate took %s to stop with WaitDelay", elapsed)
		}
	case <-time.After(4 * time.Second):
		killChild()
		<-done
		t.Fatal("gate did not bound waiting for a descendant holding output open")
	}
	killChild()
	if len(report.Results) != 2 || report.Results[0].Status != gateStatusFailed || report.Results[1].Status != gateStatusNotRun {
		t.Fatalf("cancellation results = %+v", report.Results)
	}
	if report.Results[0].Reason != context.Canceled.Error() {
		t.Fatalf("cancelled step reason = %q", report.Results[0].Reason)
	}
	_ = reader.Close()
}

func TestVerificationCoverageAndFormattingFailures(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "verify.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	goFile := filepath.Join(dir, "bad.go")
	original := "package fixture\nfunc bad( ){ }\n"
	if err := os.WriteFile(goFile, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	// A fake Go inventory makes the contract check independent of source coverage.
	fakeGo := "#!/bin/sh\ncase \"$*\" in\n'list -m') echo github.com/sandrolain/sdt;;\n*Dir*) printf '%s\\n' \"$FIXTURE_DIR\";;\n*) printf '%s\\n' github.com/sandrolain/sdt github.com/sandrolain/sdt/cli github.com/sandrolain/sdt/good github.com/sandrolain/sdt/cli/utils github.com/sandrolain/sdt/context/clone;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(fakeGo), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command("sh", append([]string{script}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE_DIR="+dir)
		return cmd.CombinedOutput()
	}
	for _, tc := range []struct {
		name, good, exception string
		pass                  bool
	}{
		{"eligible", "8", "2", true},
		{"coverage floor", "7", "3", true},
		{"ordinary regression", "6", "4", false},
		{"exception regression", "8", "1", false},
		{"improved exception", "9", "9", true},
		{"missing package", "", "2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := "mode: atomic\n"
			if tc.good != "" {
				profile += "github.com/sandrolain/sdt/good/good.go:1.1,2.2 " + tc.good + " 1\ngithub.com/sandrolain/sdt/good/good.go:3.1,4.2 " + map[string]string{"6": "4", "7": "3", "8": "2", "9": "1"}[tc.good] + " 0\n"
			}
			profile += "github.com/sandrolain/sdt/cli/utils/u.go:1.1,2.2 " + tc.exception + " 1\ngithub.com/sandrolain/sdt/cli/utils/u.go:3.1,4.2 " + map[string]string{"1": "9", "2": "8", "3": "7", "9": "1"}[tc.exception] + " 0\n"
			if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(profile), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := run("coverage")
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%t, err=%v: %s", tc.pass, err, out)
			}
			if strings.Contains(string(out), "context/clone") {
				t.Fatalf("corpus included: %s", out)
			}
		})
	}
	if out, err := run("fmt"); err == nil || !strings.Contains(string(out), "bad.go") {
		t.Fatalf("format drift passed: %v %s", err, out)
	}
	if got := mustReadFile(t, goFile); got != original {
		t.Fatal("format verification rewrote source")
	}
}
