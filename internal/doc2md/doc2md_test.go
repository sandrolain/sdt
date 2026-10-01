package doc2md

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls  []fakeCall
	handle func(name string, args []string) ExecResult
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) ExecResult {
	f.calls = append(f.calls, fakeCall{name: name, args: args})
	return f.handle(name, args)
}

func withFake(t *testing.T, handle func(name string, args []string) ExecResult) *fakeRunner {
	t.Helper()
	f := &fakeRunner{handle: handle}
	previous := execCommand
	execCommand = f.run
	t.Cleanup(func() { execCommand = previous })
	return f
}

func ok(stdout string) ExecResult          { return ExecResult{Stdout: []byte(stdout)} }
func failed(code int, s string) ExecResult { return ExecResult{ExitCode: code, Stderr: []byte(s)} }

func outputDir(args []string) string {
	for i, a := range args {
		if a == "--output" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func countCalls(r *fakeRunner, name string) int {
	n := 0
	for _, c := range r.calls {
		if c.name == name {
			n++
		}
	}
	return n
}

func TestConvertSuccessAnydoc(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch {
		case name == "anydoc" && args[0] == "--version":
			return ok("0.2.4\n")
		case name == "anydoc":
			return ok("# Title\n\nbody\n")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusSuccess {
		t.Fatalf("status = %s (%s)", res.Status, res.Reason)
	}
	if res.Converter != "anydoc" {
		t.Errorf("converter = %q", res.Converter)
	}
	if res.Markdown != "# Title\n\nbody\n" {
		t.Errorf("markdown = %q", res.Markdown)
	}
	if len(res.Attempts) != 1 || res.Attempts[0].Stage != StageConvert {
		t.Errorf("attempts = %+v", res.Attempts)
	}
}

func TestConvertProbeFallback(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch name {
		case "anydoc":
			return failed(1, "ModuleNotFoundError: No module named 'anydoc.cli.main'")
		case "docling":
			if args[0] == "--version" {
				return ok("2.113.0\n")
			}
			if dir := outputDir(args); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# From docling\n"), 0o600)
			}
			return ok("")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusSuccess || res.Converter != "docling" {
		t.Fatalf("status = %s converter = %q (%s)", res.Status, res.Converter, res.Reason)
	}
	if len(res.Attempts) != 2 {
		t.Fatalf("attempts = %+v", res.Attempts)
	}
	if res.Attempts[0].Converter != "anydoc" || res.Attempts[0].Stage != StageProbe {
		t.Errorf("first attempt = %+v", res.Attempts[0])
	}
	if res.Attempts[1].Converter != "docling" || res.Attempts[1].Stage != StageConvert {
		t.Errorf("second attempt = %+v", res.Attempts[1])
	}
}

func TestConvertDoclingArgv(t *testing.T) {
	runner := withFake(t, func(name string, args []string) ExecResult {
		switch {
		case name == "anydoc":
			return failed(1, "not runnable")
		case name == "docling" && args[0] == "--version":
			return ok("2.113.0\n")
		case name == "docling":
			if dir := outputDir(args); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, "report.md"), []byte("# R\n"), 0o600)
			}
			return ok("")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "/tmp/docs/report.pdf"})
	if res.Status != StatusSuccess {
		t.Fatalf("status = %s (%s)", res.Status, res.Reason)
	}
	var convertArgs []string
	for _, c := range runner.calls {
		if c.name == "docling" && len(c.args) > 0 && c.args[0] == "convert" {
			convertArgs = c.args
		}
	}
	want := []string{"convert", "/tmp/docs/report.pdf", "--to", "md", "--output"}
	if len(convertArgs) != len(want)+1 {
		t.Fatalf("docling args = %v", convertArgs)
	}
	for i, w := range want {
		if convertArgs[i] != w {
			t.Errorf("docling arg %d = %q, want %q", i, convertArgs[i], w)
		}
	}
}

func TestConvertConversionFallback(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch name {
		case "anydoc":
			if args[0] == "--version" {
				return ok("0.2.4")
			}
			return failed(1, "unsupported format\n")
		case "docling":
			return failed(1, "broken install")
		case "markitdown":
			if args[0] == "--version" {
				return ok("1.0")
			}
			return ok("# md\n")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.docx"})
	if res.Status != StatusSuccess || res.Converter != "markitdown" {
		t.Fatalf("status = %s converter = %q (%s)", res.Status, res.Converter, res.Reason)
	}
	if len(res.Attempts) != 3 {
		t.Fatalf("attempts = %+v", res.Attempts)
	}
	if !strings.Contains(res.Attempts[0].Reason, "unsupported format") {
		t.Errorf("anydoc reason = %q", res.Attempts[0].Reason)
	}
	if !strings.Contains(res.Attempts[1].Reason, "broken install") {
		t.Errorf("docling reason = %q", res.Attempts[1].Reason)
	}
}

func TestConvertEmptyOutputAdvances(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch name {
		case "anydoc":
			if args[0] == "--version" {
				return ok("0.2.4")
			}
			return ok("   \n")
		case "docling":
			if args[0] == "--version" {
				return ok("2.0")
			}
			if dir := outputDir(args); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# ok\n"), 0o600)
			}
			return ok("")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusSuccess || res.Converter != "docling" {
		t.Fatalf("status = %s converter = %q (%s)", res.Status, res.Converter, res.Reason)
	}
	if !strings.Contains(res.Attempts[0].Reason, "produced no markdown") {
		t.Errorf("anydoc reason = %q", res.Attempts[0].Reason)
	}
}

func TestConvertNeedsOCRIsTerminal(t *testing.T) {
	runner := withFake(t, func(name string, args []string) ExecResult {
		switch {
		case name == "anydoc" && args[0] == "--version":
			return ok("0.2.4")
		case name == "anydoc":
			return failed(3, "PDF needs OCR\n")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "scan.pdf"})
	if res.Status != StatusNeedsOCR {
		t.Fatalf("status = %s (%s)", res.Status, res.Reason)
	}
	if res.Status.ExitCode() != 3 {
		t.Errorf("exit code = %d", res.Status.ExitCode())
	}
	if countCalls(runner, "docling") != 0 || countCalls(runner, "markitdown") != 0 {
		t.Errorf("chain advanced after needs_ocr: %+v", runner.calls)
	}
}

func TestConvertUnavailable(t *testing.T) {
	withFake(t, func(name string, _ []string) ExecResult {
		return failed(1, name+" is not launchable")
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusUnavailable {
		t.Fatalf("status = %s (%s)", res.Status, res.Reason)
	}
	if res.Status.ExitCode() != 2 {
		t.Errorf("exit code = %d", res.Status.ExitCode())
	}
	if len(res.Attempts) != 3 {
		t.Fatalf("attempts = %+v", res.Attempts)
	}
	for _, a := range res.Attempts {
		if a.Stage != StageProbe {
			t.Errorf("attempt = %+v", a)
		}
	}
	if !strings.Contains(res.Reason, "uvx") {
		t.Errorf("reason = %q", res.Reason)
	}
}

func TestConvertToolPin(t *testing.T) {
	runner := withFake(t, func(name string, args []string) ExecResult {
		switch {
		case name == "markitdown" && args[0] == "--version":
			return ok("1.0")
		case name == "markitdown":
			return ok("# pinned\n")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.docx", Tool: "markitdown"})
	if res.Status != StatusSuccess || res.Converter != "markitdown" {
		t.Fatalf("status = %s converter = %q (%s)", res.Status, res.Converter, res.Reason)
	}
	if countCalls(runner, "anydoc") != 0 || countCalls(runner, "docling") != 0 {
		t.Errorf("pinned chain ran other tools: %+v", runner.calls)
	}
}

func TestConvertUnknownTool(t *testing.T) {
	withFake(t, func(name string, _ []string) ExecResult { return failed(1, name) })
	res := Convert(context.Background(), Options{Input: "doc.pdf", Tool: "pandoc"})
	if res.Status != StatusFailed {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.Contains(res.Reason, "unknown tool") {
		t.Errorf("reason = %q", res.Reason)
	}
}

func TestConvertNoInput(t *testing.T) {
	withFake(t, func(name string, _ []string) ExecResult { return ok("") })
	res := Convert(context.Background(), Options{})
	if res.Status != StatusFailed || !strings.Contains(res.Reason, "no input") {
		t.Fatalf("result = %+v", res)
	}
}

func TestConvertTimeoutAdvances(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch name {
		case "anydoc":
			if args[0] == "--version" {
				return ok("0.2.4")
			}
			return ExecResult{Err: context.DeadlineExceeded}
		case "docling":
			if args[0] == "--version" {
				return ok("2.0")
			}
			if dir := outputDir(args); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# ok\n"), 0o600)
			}
			return ok("")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf", Timeout: time.Millisecond})
	if res.Status != StatusSuccess || res.Converter != "docling" {
		t.Fatalf("status = %s converter = %q (%s)", res.Status, res.Converter, res.Reason)
	}
	if !strings.Contains(res.Attempts[0].Reason, "deadline exceeded") {
		t.Errorf("anydoc reason = %q", res.Attempts[0].Reason)
	}
}

func TestConvertDoclingMissingOutputFails(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch name {
		case "anydoc":
			return failed(1, "not runnable")
		case "docling":
			if args[0] == "--version" {
				return ok("2.0")
			}
			return ok("")
		case "markitdown":
			return failed(1, "not runnable")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusFailed {
		t.Fatalf("status = %s (%s)", res.Status, res.Reason)
	}
	if !strings.Contains(res.Reason, "produced no markdown") {
		t.Errorf("reason = %q", res.Reason)
	}
}

func TestConvertProbeLaunchError(t *testing.T) {
	withFake(t, func(name string, _ []string) ExecResult {
		return ExecResult{ExitCode: -1, Err: errors.New("exec: \"" + name + "\": executable file not found in $PATH")}
	})
	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusUnavailable {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.Contains(res.Attempts[0].Reason, "executable file not found") {
		t.Errorf("attempt reason = %q", res.Attempts[0].Reason)
	}
}

func TestProbeHelpFallback(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch {
		case name == "anydoc" && args[0] == "--version":
			return failed(2, "usage")
		case name == "anydoc" && args[0] == "--help":
			return ok("usage: anydoc\n")
		case name == "anydoc":
			return ok("# via help probe\n")
		}
		return failed(1, "unexpected "+name)
	})

	res := Convert(context.Background(), Options{Input: "doc.pdf"})
	if res.Status != StatusSuccess || res.Converter != "anydoc" {
		t.Fatalf("status = %s converter = %q (%s)", res.Status, res.Converter, res.Reason)
	}
}

func TestTools(t *testing.T) {
	withFake(t, func(name string, args []string) ExecResult {
		switch name {
		case "anydoc":
			if args[0] == "--version" {
				return ok("0.2.4\n")
			}
		case "docling":
			return failed(1, "broken install")
		case "markitdown":
			if args[0] == "--version" {
				return failed(126, "no version is set")
			}
			return ok("usage: markitdown\n")
		}
		return failed(1, "unexpected "+name)
	})

	statuses := Tools(context.Background(), time.Second)
	if len(statuses) != 3 {
		t.Fatalf("statuses = %+v", statuses)
	}
	if !statuses[0].Available || statuses[0].Version != "0.2.4" {
		t.Errorf("anydoc = %+v", statuses[0])
	}
	if statuses[1].Available || !strings.Contains(statuses[1].Reason, "broken install") {
		t.Errorf("docling = %+v", statuses[1])
	}
	if !statuses[2].Available || statuses[2].Version != "" {
		t.Errorf("markitdown = %+v", statuses[2])
	}
}

func TestRunCommand(t *testing.T) {
	res := runCommand(context.Background(), "/bin/sh", "-c", "printf out; printf err >&2; exit 7")
	if res.ExitCode != 7 {
		t.Errorf("exit = %d", res.ExitCode)
	}
	if string(res.Stdout) != "out" || string(res.Stderr) != "err" {
		t.Errorf("stdout = %q stderr = %q", res.Stdout, res.Stderr)
	}
	if res.Err != nil {
		t.Errorf("err = %v", res.Err)
	}

	missing := runCommand(context.Background(), "sdt-doc2md-not-a-real-binary")
	if missing.Err == nil || missing.ExitCode != -1 {
		t.Errorf("missing binary result = %+v", missing)
	}
}

func TestRunCommandContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := runCommand(ctx, "/bin/sh", "-c", "sleep 5")
	if res.Err == nil {
		t.Fatalf("expected context error, got %+v", res)
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Errorf("err = %v", res.Err)
	}
}

func TestReasonFromOutput(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := reasonFromOutput([]byte(long + "\nsecond line"))
	if !strings.HasSuffix(got, "…") {
		t.Errorf("got = %q", got)
	}
	if len(got) > 200+len("…") {
		t.Errorf("len = %d", len(got))
	}
	if got := reasonFromOutput(nil); got != "" {
		t.Errorf("empty = %q", got)
	}

	traceback := "Traceback (most recent call last):\n  File \"bin/docling\", line 3, in <module>\nModuleNotFoundError: No module named 'docling.cli.main'\n"
	if got := reasonFromOutput([]byte(traceback)); !strings.Contains(got, "ModuleNotFoundError") {
		t.Errorf("traceback reason = %q", got)
	}

	shim := "No version is set for command markitdown\nConsider adding one of the following versions\npython 3.12.13\n"
	if got := reasonFromOutput([]byte(shim)); !strings.Contains(got, "No version is set") {
		t.Errorf("shim reason = %q", got)
	}
}

func TestStatusExitCode(t *testing.T) {
	cases := []struct {
		status Status
		want   int
	}{
		{StatusSuccess, 0},
		{StatusFailed, 1},
		{StatusUnavailable, 2},
		{StatusNeedsOCR, 3},
	}
	for _, tc := range cases {
		if got := tc.status.ExitCode(); got != tc.want {
			t.Errorf("%s exit = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestNames(t *testing.T) {
	names := Names()
	want := []string{"anydoc", "docling", "markitdown"}
	if len(names) != len(want) {
		t.Fatalf("names = %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}
