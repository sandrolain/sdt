package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

// TestViewerConfigReadsTheAgentLog covers the additive contract of the wire-log
// key: it is read when present, absent or blank counts as off, and a malformed
// file degrades to the zero value like every other key the viewer reads.
func TestViewerConfigReadsTheAgentLog(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "declared",
			config: "agent:\n  command: [\"opencode\", \"acp\"]\n  log: context/tmp/wire.jsonl\n",
			want:   "context/tmp/wire.jsonl",
		},
		{
			name:   "declared with placeholders",
			config: "agent:\n  command: [\"opencode\"]\n  log: \"context/tmp/wire-{session}.jsonl\"\n",
			want:   "context/tmp/wire-{session}.jsonl",
		},
		{
			name:   "an unknown key beside it is ignored",
			config: "agent:\n  log: wire.jsonl\n  mystery: 1\n",
			want:   "wire.jsonl",
		},
		{name: "absent", config: "agent:\n  command: [\"opencode\"]\n"},
		{name: "the section with no log", config: "agent: {}\n"},
		{name: "blank", config: "agent:\n  log: \"   \"\n"},
		{name: "empty", config: "agent:\n  log: \"\"\n"},
		{name: "no file at all"},
		{name: "malformed", config: "agent:\n  log: [\"wire\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.config != "" {
				writeFixture(t, root, ".sdt.yaml", tc.config)
			}
			if got := loadViewerConfig(root).agentLog(); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestTraceTemplateRefusals covers the declarations a reader gets a sentence for:
// an unknown placeholder, a path that leaves the root, and a symlink that leads
// out of it. A path that only looks like it leads out — a symlink staying inside —
// is allowed.
func TestTraceTemplateRefusals(t *testing.T) {
	t.Run("blank is off", func(t *testing.T) {
		tmpl, err := newTraceTemplate(t.TempDir(), "   ")
		if err != nil || tmpl != nil {
			t.Fatalf("a blank declaration must be off, got %v, %v", tmpl, err)
		}
	})

	t.Run("an unknown placeholder", func(t *testing.T) {
		_, err := newTraceTemplate(t.TempDir(), "wire-{run}.jsonl")
		if !strings.Contains(err.Error(), "placeholder") {
			t.Fatalf("want a placeholder refusal, got %v", err)
		}
	})

	t.Run("an unclosed placeholder", func(t *testing.T) {
		_, err := newTraceTemplate(t.TempDir(), "wire-{session.jsonl")
		if !strings.Contains(err.Error(), "placeholder") {
			t.Fatalf("want a placeholder refusal, got %v", err)
		}
	})

	t.Run("a parent escape", func(t *testing.T) {
		_, err := newTraceTemplate(t.TempDir(), "../outside.jsonl")
		if !strings.Contains(err.Error(), "inside the project root") {
			t.Fatalf("want a confinement refusal, got %v", err)
		}
	})

	t.Run("an absolute path outside", func(t *testing.T) {
		_, err := newTraceTemplate(t.TempDir(), filepath.Join(t.TempDir(), "outside.jsonl"))
		if !strings.Contains(err.Error(), "inside the project root") {
			t.Fatalf("want a confinement refusal, got %v", err)
		}
	})

	t.Run("a symlink out of the root", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "sub", "leak")); err != nil {
			t.Fatal(err)
		}
		_, err := newTraceTemplate(root, "sub/leak/wire.jsonl")
		if !strings.Contains(err.Error(), "inside the project root") {
			t.Fatalf("want a confinement refusal, got %v", err)
		}
	})

	t.Run("a symlink staying inside is allowed", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "alias")); err != nil {
			t.Fatal(err)
		}
		tmpl, err := newTraceTemplate(root, "alias/wire.jsonl")
		if err != nil || tmpl == nil {
			t.Fatalf("a symlink inside the root must be usable, got %v, %v", tmpl, err)
		}
	})
}

// TestTraceTemplateExpand covers the names a run resolves to: {timestamp} is the
// run's start, {session} the ACP id — sanitised, because the id is the agent's to
// choose — and a run with no session is still filed.
func TestTraceTemplateExpand(t *testing.T) {
	root := t.TempDir()
	started := time.Date(2026, 10, 17, 12, 34, 56, 0, time.UTC)

	cases := []struct {
		name     string
		declared string
		run      acpbridge.RunInfo
		want     string
	}{
		{
			name:     "no placeholder is the one appended file",
			declared: "context/tmp/wire.jsonl",
			run:      acpbridge.RunInfo{Started: started},
			want:     filepath.Join(root, "context/tmp/wire.jsonl"),
		},
		{
			name:     "timestamp names a run",
			declared: "wire-{timestamp}.jsonl",
			run:      acpbridge.RunInfo{Started: started},
			want:     filepath.Join(root, "wire-20261017-123456.jsonl"),
		},
		{
			name:     "session names a conversation",
			declared: "wire-{session}.jsonl",
			run:      acpbridge.RunInfo{Started: started, Session: "sess-abc123"},
			want:     filepath.Join(root, "wire-sess-abc123.jsonl"),
		},
		{
			name:     "a session id with separators stays one path element",
			declared: "wire-{session}.jsonl",
			run:      acpbridge.RunInfo{Started: started, Session: "sess/../etc"},
			want:     filepath.Join(root, "wire-sess_.._etc.jsonl"),
		},
		{
			name:     "a run with no session is still filed",
			declared: "wire-{session}.jsonl",
			run:      acpbridge.RunInfo{Started: started},
			want:     filepath.Join(root, "wire-nosession.jsonl"),
		},
		{
			name:     "both placeholders in one name",
			declared: "tmp/{timestamp}-{session}.jsonl",
			run:      acpbridge.RunInfo{Started: started, Session: "s1"},
			want:     filepath.Join(root, "tmp/20261017-123456-s1.jsonl"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := newTraceTemplate(root, tc.declared)
			if err != nil {
				t.Fatalf("newTraceTemplate: %v", err)
			}
			got, err := tmpl.expand(tc.run)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// TestAgentTraceSinkSingleFileAppendsAcrossRuns covers the declaration without a
// placeholder: every run appends to the one file, a run's writer does not close it,
// and the sink's owner closes it at shutdown.
func TestAgentTraceSinkSingleFileAppendsAcrossRuns(t *testing.T) {
	root := t.TempDir()
	sink, err := newAgentTraceSink(root, "context/tmp/wire.jsonl")
	if err != nil || sink == nil {
		t.Fatalf("newAgentTraceSink: %v, %v", sink, err)
	}
	path := filepath.Join(root, "context/tmp/wire.jsonl")
	first, err := sink.Open(acpbridge.RunInfo{Session: "s1"})
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := first.Write([]byte("run one\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// A run's writer is not the file's owner: closing it must leave the file usable.
	if err := first.Close(); err != nil {
		t.Fatalf("closing a run's writer: %v", err)
	}
	second, err := sink.Open(acpbridge.RunInfo{Session: "s2"})
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if _, err := second.Write([]byte("run two\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = second.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the declared file must exist: %v", err)
	}
	if got := info.Mode().Perm(); got != traceFileMode {
		t.Errorf("the wire log must be %v, got %v", traceFileMode, got)
	}
	if err := sink.close(); err != nil {
		t.Fatalf("sink close: %v", err)
	}
	raw, err := os.ReadFile(path) //#nosec G304 -- a path this test built
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(raw) != "run one\nrun two\n" {
		t.Fatalf("want both runs appended, got %q", raw)
	}
}

// TestAgentTraceSinkPerRunFiles covers the declarations with placeholders: each run
// or each conversation gets its own file, opened on its own.
func TestAgentTraceSinkPerRunFiles(t *testing.T) {
	root := t.TempDir()
	started := time.Date(2026, 10, 17, 12, 34, 56, 0, time.UTC)
	sink, err := newAgentTraceSink(root, "wire-{timestamp}-{session}.jsonl")
	if err != nil || sink == nil {
		t.Fatalf("newAgentTraceSink: %v, %v", sink, err)
	}
	runs := []acpbridge.RunInfo{
		{Started: started, Session: "s1"},
		{Started: started.Add(time.Minute), Session: "s1"},
	}
	for _, run := range runs {
		w, err := sink.Open(run)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, err := w.Write([]byte("frame\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("a per-run writer must really close: %v", err)
		}
	}
	for _, name := range []string{"wire-20261017-123456-s1.jsonl", "wire-20261017-123556-s1.jsonl"} {
		raw, err := os.ReadFile(filepath.Join(root, name)) //#nosec G304 -- a path this test built
		if err != nil {
			t.Fatalf("each run must have its own file %s: %v", name, err)
		}
		if string(raw) != "frame\n" {
			t.Fatalf("%s must hold its own run, got %q", name, raw)
		}
	}
	if err := sink.close(); err != nil {
		t.Fatalf("sink close: %v", err)
	}
}

// TestAgentTraceSinkReportsAPathItCannotWrite covers the failure posture on the
// viewer's side: an unwritable path ends the capture with one sentence, kept for
// the panel, and never anything louder.
func TestAgentTraceSinkReportsAPathItCannotWrite(t *testing.T) {
	root := t.TempDir()
	// A plain file where the declaration asks for a directory: creating it fails.
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	sink, err := newAgentTraceSink(root, "blocked/wire.jsonl")
	if err != nil || sink == nil {
		t.Fatalf("newAgentTraceSink: %v, %v", sink, err)
	}
	if _, err := sink.Open(acpbridge.RunInfo{Session: "s1"}); err == nil {
		t.Fatal("an unwritable path must refuse the open")
	}
	sentence := sink.issueSentence()
	if !strings.Contains(sentence, "the agent wire log is off") {
		t.Fatalf("want a reader-facing sentence, got %q", sentence)
	}
	if err := sink.close(); err != nil {
		t.Fatalf("close with nothing open: %v", err)
	}
}

// TestAgentStateShowsTheWireLogSentence covers the channel the sentence travels on:
// the state a panel polls carries it in Error, next to the agent's own note.
func TestAgentStateShowsTheWireLogSentence(t *testing.T) {
	root := t.TempDir()
	s := &server{root: root, agent: acpbridge.New(acpbridge.Options{Root: root})}
	s.agentTrace, _ = newAgentTraceSink(root, "blocked/wire.jsonl")
	s.agentTrace.issue = "the agent wire log is off: because"
	if got := s.agentState("").Error; got != "the agent wire log is off: because" {
		t.Fatalf("want the wire-log sentence, got %q", got)
	}

	s.agentLogReason = "the agent wire log is off: refused"
	if got := s.agentState("").Error; got != "the agent wire log is off: refused" {
		t.Fatalf("a refused declaration must win over a run's failure, got %q", got)
	}
}

// TestEnableAgentWiresTheDeclaredWireLog covers the join: the declaration read from
// .sdt.yaml becomes the bridge's sink, an absent key touches nothing, and a refused
// declaration leaves the agent running with a sentence.
func TestEnableAgentWiresTheDeclaredWireLog(t *testing.T) {
	t.Run("declared and usable", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, root, ".sdt.yaml", "agent:\n  command: [\"opencode\", \"acp\"]\n  log: context/tmp/wire.jsonl\n")
		s, err := newServer(root)
		if err != nil {
			t.Fatalf("newServer: %v", err)
		}
		s.enableAgent("127.0.0.1")
		if s.agentTrace == nil {
			t.Fatal("a usable declaration must install the sink")
		}
		if s.agentLogReason != "" {
			t.Fatalf("a usable declaration must not be refused: %q", s.agentLogReason)
		}
		if s.agentState("").Error != "" {
			t.Fatalf("nothing to report yet, got %q", s.agentState("").Error)
		}
	})

	t.Run("absent leaves it off", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, root, ".sdt.yaml", "agent:\n  command: [\"opencode\", \"acp\"]\n")
		s, err := newServer(root)
		if err != nil {
			t.Fatalf("newServer: %v", err)
		}
		s.enableAgent("127.0.0.1")
		if s.agentTrace != nil || s.agentLogReason != "" {
			t.Fatalf("an absent key must touch nothing, got %v / %q", s.agentTrace, s.agentLogReason)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) == 0 {
			t.Fatalf("the root must be untouched: %v", err)
		}
	})

	t.Run("a refused declaration", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, root, ".sdt.yaml", "agent:\n  command: [\"opencode\", \"acp\"]\n  log: ../outside.jsonl\n")
		s, err := newServer(root)
		if err != nil {
			t.Fatalf("newServer: %v", err)
		}
		s.enableAgent("127.0.0.1")
		if s.agentTrace != nil {
			t.Fatal("a refused declaration must install no sink")
		}
		if !strings.Contains(s.agentLogReason, "inside the project root") {
			t.Fatalf("want the confinement sentence, got %q", s.agentLogReason)
		}
		if s.agent == nil {
			t.Fatal("a refused wire log must leave the agent running")
		}
	})
}
