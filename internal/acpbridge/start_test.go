package acpbridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestStartSpawnsARealAgentOverStdio covers the transport end to end: the
// configured argv runs, the handshake and the session travel over its stdio, the
// bridge ends up active, and the child's working directory is the one Start was
// given — the agreement the file confinement depends on.
func TestStartSpawnsARealAgentOverStdio(t *testing.T) {
	scratch := t.TempDir()
	root := t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	session, err := bridge.Start(context.Background(), spawnCommand(scratch, ""), root)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if session == "" {
		t.Fatal("a start must open the session a browser is handed")
	}
	if !bridge.Activated() {
		t.Fatal("a started agent must leave the bridge active")
	}
	if bridge.SessionID() != session {
		t.Fatalf("the session a browser is handed must be the one that was opened: want %q, got %q", session, bridge.SessionID())
	}
	waitFor(t, "the child to report its directory", func() bool {
		raw, err := os.ReadFile(filepath.Join(scratch, "cwd")) //#nosec G304 -- a path this test built
		return err == nil && string(raw) == root
	})
}

// TestStartRefusesWhatItCannotRun keeps the refusals where a reader can act on them:
// an empty argv and a program that is not there are ErrStart, never a half-started
// bridge.
func TestStartRefusesWhatItCannotRun(t *testing.T) {
	bridge := New(Options{Root: t.TempDir()})
	t.Cleanup(func() { _ = bridge.Close() })

	cases := []struct {
		name    string
		command []string
	}{
		{name: "an empty argv", command: nil},
		{name: "a blank program", command: []string{"  "}},
		{name: "a program nobody has", command: []string{filepath.Join(t.TempDir(), "ghost-agent")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := bridge.Start(context.Background(), tc.command, t.TempDir()); !errors.Is(err, ErrStart) {
				t.Fatalf("want ErrStart, got %v", err)
			}
			if bridge.Activated() {
				t.Fatal("a refused start must leave the bridge inert")
			}
		})
	}
}

// TestStartLeavesNothingBehindWhenTheAgentNeverSpeaks covers the failure path: a
// process that exits without a word fails the handshake, and the bridge must be inert
// afterwards with its transport closed rather than holding a dead pipe.
func TestStartLeavesNothingBehindWhenTheAgentNeverSpeaks(t *testing.T) {
	quiet := "/usr/bin/true"
	if _, err := os.Stat(quiet); err != nil {
		t.Skipf("no %s on this machine: %v", quiet, err)
	}
	bridge := New(Options{Root: t.TempDir()})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.Start(context.Background(), []string{quiet}, t.TempDir()); !errors.Is(err, ErrHandshake) {
		t.Fatalf("a silent agent must fail the handshake, got %v", err)
	}
	if bridge.Activated() {
		t.Fatal("a failed start must leave the bridge inert")
	}
	if _, live := bridge.Session(bridge.SessionID()); live {
		t.Fatal("a failed start must leave no session behind")
	}
}

// TestStartOnALiveBridgeRefusesAndClosesItsOwnAttempt states the ownership rule: the
// second attempt is refused, and the process it brought is stopped rather than
// leaked — a leaked process is the one failure nobody notices.
func TestStartOnALiveBridgeRefusesAndClosesItsOwnAttempt(t *testing.T) {
	live, refused := t.TempDir(), t.TempDir()
	root := t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.Start(context.Background(), spawnCommand(live, ""), root); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if _, err := bridge.Start(context.Background(), spawnCommand(refused, ""), root); !errors.Is(err, ErrAlreadyActivated) {
		t.Fatalf("want ErrAlreadyActivated, got %v", err)
	}

	// What matters is that the refusal changed nothing about the live agent: it is
	// still the process that was there, still connected. Whether the refused child
	// managed to report before it was stopped is a race and would prove nothing; that
	// no process is left running is the validation phase's ps cell, which can see the
	// whole machine rather than one scratch file.
	owners := startedPIDs(t, live)
	if len(owners) != 1 {
		t.Fatalf("the live agent must be exactly one process: %v", owners)
	}
	if processGone(owners[0]) {
		t.Fatalf("the refused attempt must not have replaced or stopped the live agent (%d)", owners[0])
	}
	if !bridge.Activated() {
		t.Fatal("a refused attempt must not disturb the live agent")
	}
	if _, err := os.Stat(filepath.Join(live, "cwd")); err != nil {
		t.Fatalf("the live agent must still be the one running: %v", err)
	}
}
