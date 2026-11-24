package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

var parallelHelperMode = flag.String("sdt-parallel-helper", "", "internal parallel command test helper")
var parallelHelperDir = flag.String("sdt-parallel-dir", "", "internal parallel command test helper directory")

func TestParallelHelperProcess(t *testing.T) {
	if *parallelHelperMode == "" {
		return
	}
	switch *parallelHelperMode {
	case "success":
		fmt.Fprint(os.Stdout, "hello")
		fmt.Fprint(os.Stderr, "note")
		os.Exit(0)
	case "fail":
		fmt.Fprint(os.Stderr, "failed")
		os.Exit(7)
	case "barrier":
		if err := os.WriteFile(filepath.Join(*parallelHelperDir, fmt.Sprintf("%d", os.Getpid())), nil, 0600); err != nil {
			os.Exit(8)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			entries, err := os.ReadDir(*parallelHelperDir)
			if err != nil {
				os.Exit(9)
			}
			if len(entries) >= 2 {
				fmt.Fprint(os.Stdout, "started together")
				os.Exit(0)
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.Exit(10)
	default:
		os.Exit(11)
	}
}

func parallelHelperArgv(mode string, extra ...string) []string {
	argv := []string{os.Args[0], "-test.run=^TestParallelHelperProcess$", "-sdt-parallel-helper=" + mode}
	return append(argv, extra...)
}

func TestParseParallelArgs(t *testing.T) {
	cases := []struct {
		name    string
		raw     []string
		wantErr string
	}{
		{name: "none", raw: nil, wantErr: "at least one --cmd"},
		{name: "not json", raw: []string{"go test"}, wantErr: "JSON string array"},
		{name: "not strings", raw: []string{`[1,2]`}, wantErr: "JSON string array"},
		{name: "empty array", raw: []string{`[]`}, wantErr: "non-empty executable"},
		{name: "null", raw: []string{`null`}, wantErr: "non-empty executable"},
		{name: "empty executable", raw: []string{`["","x"]`}, wantErr: "non-empty executable"},
		{name: "valid", raw: []string{`["go","version"]`, `["echo"]`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseParallelArgs(tc.raw)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.raw) {
				t.Fatalf("got %d argvs, want %d", len(got), len(tc.raw))
			}
		})
	}
}

func TestRunParallelCapturesOutputAndExitStatus(t *testing.T) {
	results := runParallel([][]string{
		parallelHelperArgv("success"),
		parallelHelperArgv("fail"),
		{filepath.Join(t.TempDir(), "sdt-parallel-no-such-binary")},
	})
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	if r := results[0]; r.Index != 1 || r.ExitCode != 0 || r.Stdout != "hello" || r.Stderr != "note" {
		t.Errorf("echo result = %+v", r)
	}
	if r := results[1]; r.Index != 2 || r.ExitCode != 7 || r.Stderr != "failed" {
		t.Errorf("failing helper result = %+v, want exit 7 with stderr", r)
	}
	if r := results[2]; r.Index != 3 || r.ExitCode != -1 || r.Error == "" {
		t.Errorf("missing binary result = %+v, want start error", r)
	}
}

func TestRunParallelStartsCommandsConcurrently(t *testing.T) {
	dir := t.TempDir()
	results := runParallel([][]string{
		parallelHelperArgv("barrier", "-sdt-parallel-dir="+dir),
		parallelHelperArgv("barrier", "-sdt-parallel-dir="+dir),
	})
	for i, result := range results {
		if result.ExitCode != 0 || result.Stdout != "started together" {
			t.Fatalf("command %d did not overlap: %+v", i+1, result)
		}
	}
}

func TestParallelCommandStructuredFormatsAndPartialExit(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			previousExit := exit
			exited := 0
			exit = func(code int) { exited = code }
			t.Cleanup(func() { exit = previousExit })

			good, bad := parallelHelperArgv("success"), parallelHelperArgv("fail")
			out, _, err := runRoot(t, "parallel", "--format", format, "--cmd", mustJSON(t, good), "--cmd", mustJSON(t, bad), "--quiet")
			if err != nil {
				t.Fatalf("run parallel: %v", err)
			}
			if exited != 3 {
				t.Fatalf("exit = %d, want partial-failure code 3", exited)
			}
			var results []parallelResult
			var decodeErr error
			if format == "json" {
				decodeErr = json.Unmarshal([]byte(out), &results)
			} else {
				decodeErr = yaml.Unmarshal([]byte(out), &results)
			}
			if decodeErr != nil {
				t.Fatalf("decode %s output: %v\n%s", format, decodeErr, out)
			}
			if len(results) != 2 || results[0].Index != 1 || results[1].Index != 2 || results[0].Stdout != "hello" || results[1].ExitCode != 7 {
				t.Fatalf("unexpected %s results: %+v", format, results)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal helper argv: %v", err)
	}
	return string(b)
}

func TestRenderParallelTextLabelsEachCommand(t *testing.T) {
	out := renderParallelText([]parallelResult{
		{Index: 1, Argv: []string{"echo", "hi"}, ExitCode: 0, Stdout: "hi\n"},
		{Index: 2, Argv: []string{"false"}, ExitCode: 1, Stderr: "boom"},
	})
	for _, want := range []string{`== [1] ["echo","hi"]  exit 0`, "--- stdout\nhi\n", `== [2] ["false"]  exit 1`, "--- stderr\nboom\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
