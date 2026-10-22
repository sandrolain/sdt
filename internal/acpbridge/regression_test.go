package acpbridge

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"
)

// The tests in this file are the regressions for four defects an independent review pass
// found in the delivered domain work, each one contradicting a decision this wave had
// written down. They are kept together and named after the behaviour rather than the
// mechanism, because that is what broke: the tests that existed covered only the window
// in which the behaviour already held.

// TestAStopCallsOffAScheduledRestart covers the first defect: a stop that arrives while
// the bridge is waiting to start the agent again has to end the story. Before the fix the
// scheduled attempt went ahead anyway, undoing a decision the reader had just made.
func TestAStopCallsOffAScheduledRestart(t *testing.T) {
	// A delay long enough that the bridge is observably inside its waiting window, and
	// then a wait long enough that a restart would have happened had it not been called off.
	restartDelayFor(t, 300*time.Millisecond)
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	if err := syscallKill(bridge.agentPID()); err != nil {
		t.Fatalf("killing the agent: %v", err)
	}
	waitFor(t, "a restart to be scheduled (status="+bridge.statusLine()+")", func() bool {
		status, _ := bridge.Status()
		return status == StatusRestarting
	})
	if err := bridge.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	time.Sleep(700 * time.Millisecond)

	if started := len(startedPIDs(t, scratch)); started != 1 {
		t.Fatalf("a stop must call off a restart it finds scheduled: %d processes were started", started)
	}
	if status, _ := bridge.Status(); status != StatusStopped {
		t.Fatalf("a stopped bridge stays stopped: status=%q", status)
	}
}

// TestAStopEndsTheWholeGroup covers the second defect: the kill has to reach the agent's
// group whether or not the agent itself leaves first. The helper here ignores the graceful
// signal and spawns a grandchild that ignores it too, so nothing but a kill addressed to
// the group can end them — which is what makes the group an observable thing rather than a
// claim in a comment.
func TestAStopEndsTheWholeGroup(t *testing.T) {
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, spawnHelperStubbornMode), root); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	pid := bridge.agentPID()
	grandchild := grandchildPID(t, scratch)
	if pid == 0 || grandchild == 0 {
		t.Fatalf("the agent and its grandchild must both be processes: %d, %d", pid, grandchild)
	}

	if err := bridge.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, "the whole agent group to be gone", func() bool {
		return processGone(pid) && processGone(grandchild)
	})
}

// TestAClosedConversationLeavesTheRegistry covers the third defect: a reattach that had to
// open a fresh session gives the conversation a new id, and closing it afterwards must
// remove the key the registry actually holds. Before the fix a closed conversation stayed
// registered under its new id, reachable by a command nobody was there to answer.
func TestAClosedConversationLeavesTheRegistry(t *testing.T) {
	restartWith(t)
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	// The plain peer never advertises session/load, so the restart is the degrading one:
	// a fresh session id for the same conversation, which is the case that broke.
	session, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root)
	if err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	conversation, live := bridge.Session(session)
	if !live {
		t.Fatalf("the session %q must be in the registry", session)
	}
	if err := conversation.Prompt(context.Background(), "una domanda"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if err := syscallKill(bridge.agentPID()); err != nil {
		t.Fatalf("killing the agent: %v", err)
	}
	waitFor(t, "the degraded restart (status="+bridge.statusLine()+")", func() bool {
		status, _ := bridge.Status()
		return len(startedPIDs(t, scratch)) >= 2 && status == StatusActive
	})

	current := conversation.ID()
	conversation.Close()
	if _, still := bridge.Session(current); still {
		t.Fatalf("a closed conversation must leave the registry, and %q is still there", current)
	}
}

// TestAStartAndAWatcherShareTheBoundUnderTheLock is the -race smoke check for the two
// accesses the supervisor used to make outside its state lock: a start resets the bound
// while the watcher of the cycle it replaces may still be unwinding it, and a watcher read
// the state after releasing the lock. A deterministic overlap cannot be forced from outside
// the package, so this drives both paths hard and the race detector is what judges them.
func TestAStartAndAWatcherShareTheBoundUnderTheLock(t *testing.T) {
	restartWith(t)
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	command := spawnCommand(scratch, spawnHelperDieMode)
	for i := 0; i < 8; i++ {
		if _, err := bridge.EnsureStarted(context.Background(), command, root); err != nil {
			t.Fatalf("EnsureStarted %d: %v", i, err)
		}
		// The kill is what leaves a watcher behind for the next start to overlap with.
		if pid := bridge.agentPID(); pid != 0 {
			if err := syscallKill(pid); err != nil && !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("killing the agent: %v", err)
			}
		}
	}
	// The last cycle has to be accounted for before the bridge closes, so the detector sees
	// the watcher's own accesses rather than only the teardown's.
	started := len(startedPIDs(t, scratch))
	waitFor(t, "the last watcher to restart the agent the loop killed", func() bool {
		return len(startedPIDs(t, scratch)) > started
	})
}

// TestAStartSomebodyAsksForResetsTheBound covers the fourth defect: the bound exists to
// stop a loop around a command that cannot run, and it must not hold a reader's own start
// against the budget an earlier agent spent.
func TestAStartSomebodyAsksForResetsTheBound(t *testing.T) {
	restartWith(t)
	scratch, root := t.TempDir(), t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, spawnHelperDieMode), root); err != nil {
		t.Fatalf("first EnsureStarted: %v", err)
	}
	waitFor(t, "the first cycle to run out (status="+bridge.statusLine()+")", func() bool {
		status, _ := bridge.Status()
		return status == StatusStopped
	})
	spent := len(startedPIDs(t, scratch))
	if spent != maxRestarts+1 {
		t.Fatalf("the first cycle is the first start plus %d attempts: %d processes", maxRestarts, spent)
	}

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, spawnHelperDieMode), root); err != nil {
		t.Fatalf("the start a reader asked for: %v", err)
	}
	// The new cycle dies on its own too, so it must spend attempts of its own rather than
	// inherit an exhausted budget: two more processes is its start plus at least one
	// restart.
	waitFor(t, "the new cycle to restart on its own", func() bool {
		return len(startedPIDs(t, scratch)) >= spent+2
	})
}
