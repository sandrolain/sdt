package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateReportCompleteAndShared(t *testing.T) {
	path := gateRecordedTask(t)
	writeGateConfig(t,
		GateStep{Name: "ok", Run: "true"},
		GateStep{Name: "bad", Run: "echo boom; exit 3"},
		GateStep{Name: "unknown", Run: "touch should-not-exist"},
		GateStep{Name: "omitted", Run: "false", Exclude: "not applicable to this target"})
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
		report.execute(ctx, root)
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
