package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestViewerConfigReadsTheAgentCommand covers the additive contract of the
// `agent` section: read when present, ignored when absent, and a malformed file
// degrading to the zero value rather than keeping the viewer from serving.
func TestViewerConfigReadsTheAgentCommand(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   []string
	}{
		{
			name:   "declared",
			config: "agent:\n  command: [\"opencode\", \"acp\"]\n",
			want:   []string{"opencode", "acp"},
		},
		{
			name:   "declared beside the keys the viewer already reads",
			config: "project: sdt_44f24890\nsearch:\n  semantic: true\nagent:\n  command: [\"opencode\", \"acp\", \"--stdio\"]\n",
			want:   []string{"opencode", "acp", "--stdio"},
		},
		{
			name:   "an unknown key beside it is ignored",
			config: "agent:\n  command: [\"opencode\"]\n  mystery: 1\nskipDirs: [dist]\n",
			want:   []string{"opencode"},
		},
		{name: "absent", config: "project: sdt_44f24890\n"},
		{name: "no file at all"},
		{name: "malformed", config: "agent:\n  command: [\"opencode\"\n"},
		{name: "the section with no command", config: "agent: {}\n"},
		{name: "an empty argv", config: "agent:\n  command: []\n"},
		{name: "a blank program", config: "agent:\n  command: [\"   \", \"acp\"]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.config != "" {
				writeFixture(t, root, ".sdt.yaml", tc.config)
			}
			got := loadViewerConfig(root).agentCommand()
			if len(tc.want) == 0 {
				if got != nil {
					t.Fatalf("want no agent declared, got %q", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestViewerConfigReadsTheAgentModel covers the other half of the same declaration: the
// model is read when present, blank counts as absent (it names nothing, and passing it on
// would ask the agent for something no reader wrote), and a missing one leaves the agent on
// its own default.
func TestViewerConfigReadsTheAgentModel(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "declared",
			config: "agent:\n  command: [\"opencode\", \"acp\"]\n  model: \"opencode/big-pickle\"\n",
			want:   "opencode/big-pickle",
		},
		{
			name:   "declared beside the keys the viewer already reads",
			config: "project: sdt_44f24890\nsearch:\n  semantic: true\nagent:\n  command: [\"opencode\"]\n  model: openrouter/meta/muse-spark-open\n",
			want:   "openrouter/meta/muse-spark-open",
		},
		{
			name:   "an unknown key beside it is ignored",
			config: "agent:\n  command: [\"opencode\"]\n  model: \"a/model\"\n  mystery: 1\n",
			want:   "a/model",
		},
		{name: "absent", config: "agent:\n  command: [\"opencode\", \"acp\"]\n"},
		{name: "the section with no model", config: "agent: {}\n"},
		{name: "blank", config: "agent:\n  command: [\"opencode\"]\n  model: \"   \"\n"},
		{name: "empty", config: "agent:\n  command: [\"opencode\"]\n  model: \"\"\n"},
		{name: "no file at all"},
		{name: "malformed", config: "agent:\n  model: [\"opencode\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.config != "" {
				writeFixture(t, root, ".sdt.yaml", tc.config)
			}
			if got := loadViewerConfig(root).agentModel(); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestResolveCommand covers how a configured argv becomes runnable: a name on
// PATH, a name beside the running binary, an explicit path, and the refusals.
// It runs at spawn time, so every refusal here is a sentence the panel shows.
func TestResolveCommand(t *testing.T) {
	onPath := t.TempDir()
	beside := t.TempDir()
	pathAgent := filepath.Join(onPath, "path-agent")
	besideAgent := filepath.Join(beside, "beside-agent")
	writeExecutable(t, pathAgent)
	writeExecutable(t, besideAgent)
	writePlainFile(t, filepath.Join(beside, "not-executable"))
	t.Setenv("PATH", onPath)

	cases := []struct {
		name    string
		argv    []string
		self    string
		want    []string
		wantErr string
	}{
		{
			name: "a name on PATH",
			argv: []string{"path-agent", "--stdio"},
			want: []string{pathAgent, "--stdio"},
		},
		{
			name: "a name beside the viewer",
			argv: []string{"beside-agent"},
			self: filepath.Join(beside, "sdtviewer"),
			want: []string{besideAgent},
		},
		{
			name: "an explicit path",
			argv: []string{besideAgent, "acp"},
			want: []string{besideAgent, "acp"},
		},
		{
			name:    "a name nobody has",
			argv:    []string{"ghost-agent"},
			wantErr: "ghost-agent",
		},
		{
			name:    "an explicit path that is not executable",
			argv:    []string{filepath.Join(beside, "not-executable")},
			wantErr: "not an executable file",
		},
		{
			name:    "an empty argv",
			argv:    nil,
			wantErr: "the command is empty",
		},
		{
			name:    "a blank program",
			argv:    []string{"  ", "acp"},
			wantErr: "the command is empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveCommand(tc.argv, tc.self)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want a refusal naming %q, got %q", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("the refusal must say %q, got %q", tc.wantErr, err)
				}
				if !errors.Is(err, errConfiguredCommand) {
					t.Fatalf("a refused command must be typed, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCommand: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestResolveCommandLeavesTheArgumentsAlone states the boundary that makes the
// argv form worth having: nothing splits, quotes or interpolates, so an argument
// that looks like shell syntax is passed through as the literal string it is.
func TestResolveCommandLeavesTheArgumentsAlone(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agent")
	writeExecutable(t, agent)
	t.Setenv("PATH", dir)

	quoted := resolveOrFail(t, []string{"agent", "-c", "echo $HOME && ls", "a b"})
	want := []string{agent, "-c", "echo $HOME && ls", "a b"}
	if !reflect.DeepEqual(quoted, want) {
		t.Fatalf("the arguments must pass through untouched: want %q, got %q", want, quoted)
	}
}

func resolveOrFail(t *testing.T, argv []string) []string {
	t.Helper()
	got, err := resolveCommand(argv, "")
	if err != nil {
		t.Fatalf("resolveCommand(%q): %v", argv, err)
	}
	return got
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil { //#nosec G306 -- a test helper needs the execute bit
		t.Fatal(err)
	}
}

func writePlainFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not a program\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
