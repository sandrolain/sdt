package acpbridge

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Status is the process view of the bridge: the part a panel can show without
// knowing anything about ACP.
type Status string

const (
	// StatusIdle is a bridge with nothing running: no command is configured, or one
	// has not been asked for.
	StatusIdle Status = "idle"
	// StatusStarting is a start in flight.
	StatusStarting Status = "starting"
	// StatusActive is a handshaken agent with a live session.
	StatusActive Status = "active"
	// StatusRestarting is an agent that died while working, with another attempt
	// already scheduled.
	StatusRestarting Status = "restarting"
	// StatusStopped is a stop somebody asked for, or the end of the attempts. The
	// bridge holds no process and carries the sentence that says why.
	StatusStopped Status = "stopped"
)

// The restart bound. An agent that was working and died is started again, up to
// maxRestarts attempts inside restartWindow, with a delay that doubles between them.
// Past the cap the bridge stops and says why: a command that cannot run is usually a
// typo or a missing login, and a loop around either one is a busy machine and a
// reader who never learns what happened.
const (
	maxRestarts   = 3
	restartWindow = 2 * time.Minute
)

// restartDelay is the first pause between attempts; it doubles for each further one
// and is capped by the window. It is a variable rather than a constant so a test can
// exercise the whole bound in milliseconds — it is not a configuration key, and the
// shipped value is the one below.
var restartDelay = 500 * time.Millisecond

// ErrNoCommand is returned when a start is asked for and the project declared no
// command to start.
var ErrNoCommand = errors.New("acpbridge: no agent command is configured")

// EnsureStarted answers the live agent when there is one and starts the configured
// command when there is not: the same call twice is one process, two callers racing
// reach the same one, and only a bridge with no live agent spawns. The decision is a
// state transition here rather than a check in the HTTP layer, where two requests
// could pass it together.
//
// The process's life is the bridge's own, not the caller's: ctx bounds the *start* —
// the handshake is the part that can hang — while Stop and Close are what end an
// agent. A request that ends therefore cannot take the conversation with it.
func (b *Bridge) EnsureStarted(ctx context.Context, command []string, dir string) (string, error) {
	// The start lock, not the state lock, is what makes this idempotent: holding the
	// state lock across a handshake would freeze every reader for as long as an
	// agent takes to answer, and holding neither would let two callers spawn two
	// processes.
	b.startMu.Lock()
	defer b.startMu.Unlock()

	if session, live := b.liveSession(); live {
		return session, nil
	}
	if len(command) == 0 {
		return "", ErrNoCommand
	}
	// A start somebody asked for is a new decision: the bound exists to stop a loop
	// around a command that cannot run, not to hold a reader's own start against the
	// budget of an agent that died before it.
	b.restarts, b.restartFrom = 0, time.Time{}
	b.setStatus(StatusStarting, "")

	agentCtx, cancel := context.WithCancel(b.life)
	proc, session, note, err := b.startWith(agentCtx, command, dir, nil)
	if err != nil {
		cancel()
		b.setStatus(StatusStopped, err.Error())
		return "", err
	}

	b.mu.Lock()
	b.proc, b.cancel, b.intended = proc, cancel, false
	b.status, b.statusErr = StatusActive, note
	b.mu.Unlock()

	go b.watch(proc, agentCtx, command, dir)
	return session, nil
}

// Stop ends the agent on purpose. The exit is marked as intended before anything is
// signalled, so a process that happens to die in the same moment is still read as a
// stop rather than as a death to restart, and the bridge is left inert with no
// process and no session. Unlike Close it leaves the bridge able to start again.
func (b *Bridge) Stop() error {
	b.mu.Lock()
	cancel := b.cancel
	b.intended = true
	b.mu.Unlock()

	if cancel != nil {
		// The process's context is the bridge's own, so cancelling it reaches only
		// this agent; Close's transport then reaps the group.
		cancel()
	}
	err := b.teardown()
	b.setStatus(StatusStopped, "")
	return err
}

// Status reports the process view and the sentence that goes with it: the reason the
// bridge stopped, or the note that says an agent could not be put back on the
// conversation it had, which is worth saying even while the bridge is active.
func (b *Bridge) Status() (Status, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status, b.statusErr
}

// watch observes the one thing that can end a working agent and decides what it
// means: a stop somebody asked for ends here, and anything else is a death worth
// another attempt — within the bound, and never at the cost of starting a second
// process. One watcher follows one process, and a process the bridge no longer owns
// is not this watcher's business.
//
// agentCtx is the life of this cycle, restart included: whoever asked for a stop
// cancels it, and the scheduled attempt gives way rather than undoing the reader's
// decision after the fact.
func (b *Bridge) watch(proc *process, agentCtx context.Context, command []string, dir string) {
	<-proc.Done()

	b.mu.Lock()
	if b.proc != proc {
		// A newer process owns the bridge: this exit is already accounted for.
		b.mu.Unlock()
		return
	}
	intended := b.intended
	b.proc = nil
	if intended {
		b.mu.Unlock()
		if b.status != StatusStopped {
			b.setStatus(StatusStopped, "")
		}
		return
	}

	exitErr := proc.Err()
	if b.restartFrom.IsZero() || time.Since(b.restartFrom) > restartWindow {
		b.restarts, b.restartFrom = 0, time.Now()
	}
	if b.restarts >= maxRestarts {
		b.status, b.statusErr = StatusStopped, exitReason(exitErr)
		b.mu.Unlock()
		// The bound is where the bridge has to let go rather than hold a connection
		// to a process that is gone: a stopped bridge owns nothing, and any wait the
		// dead agent left is released here.
		if err := b.teardown(); err != nil {
			b.log.Debug("closing the transport at the bound", "error", err)
		}
		b.setStatus(StatusStopped, exitReason(exitErr))
		b.log.Warn("the agent kept exiting; it will not be started again",
			"attempts", maxRestarts, "error", exitErr)
		return
	}
	b.restarts++
	attempt, delay := b.restarts, restartDelay<<(b.restarts-1)
	b.status, b.statusErr = StatusRestarting, exitReason(exitErr)
	b.mu.Unlock()

	// The conversations are lifted out before the dead connection is let go of: they
	// belong to their browsers, and the thread each one holds is what the panel shows.
	// Lifting them out is also where the dead agent's pending waits are released —
	// nobody can answer a permission an agent is no longer there to ask for.
	previous := b.takeRestartState()

	// The connection died with the process, so it has to be let go of before another one
	// can be built: the bridge would otherwise refuse the restart as "already activated"
	// by a connection whose process is gone.
	if err := b.teardown(); err != nil {
		b.log.Debug("closing the dead transport", "error", err)
	}
	b.setStatus(StatusRestarting, exitReason(exitErr))

	b.log.Info("the agent exited; starting it again",
		"attempt", attempt, "delay", delay, "error", exitErr)
	select {
	case <-time.After(delay):
	case <-agentCtx.Done():
		// A stop somebody asked for, or the bridge closing: starting the agent again
		// would undo a decision the reader just made, which is worse than a gap.
		b.log.Info("the restart was called off")
		return
	}

	// The attempt is serialised with a caller-driven start, and it re-checks the
	// state: a reader who asked for a start in the meantime already has one.
	b.startMu.Lock()
	defer b.startMu.Unlock()
	if _, live := b.liveSession(); live {
		return
	}
	agentCtx, cancel := context.WithCancel(b.life)
	next, _, note, err := b.startWith(agentCtx, command, dir, previous)
	if err != nil {
		cancel()
		b.setStatus(StatusStopped, err.Error())
		return
	}
	b.mu.Lock()
	b.proc, b.cancel, b.intended = next, cancel, false
	b.status, b.statusErr = StatusActive, note
	b.mu.Unlock()
	go b.watch(next, agentCtx, command, dir)
}

// exitReason turns how a process ended into a sentence a reader can act on. A
// process that exits without an error is still gone, and saying nothing would leave
// the panel with a stopped agent and no reason.
func exitReason(err error) string {
	if err == nil {
		return "the agent exited"
	}
	return "the agent exited: " + err.Error()
}

// liveSession reports the session a browser would be handed and whether the bridge
// holds a live connection.
func (b *Bridge) liveSession() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.initial, b.conn != nil
}

// setStatus records the process view under the state lock.
func (b *Bridge) setStatus(status Status, reason string) {
	b.mu.Lock()
	b.status, b.statusErr = status, reason
	b.mu.Unlock()
}

// setNote records a sentence about what just happened without touching the state it belongs
// to: a sentence is not a state change, and a state a reader needs — stopping, restarting —
// must not be overwritten by one. A sentence with nothing to say leaves the note as it is,
// so a session that went well cannot erase what a reader has not been told yet.
func (b *Bridge) setNote(sentence string) {
	if sentence == "" {
		return
	}
	b.mu.Lock()
	b.statusErr = sentence
	b.mu.Unlock()
}

// validateCommand is the refusal shared by every path that can start a process, so a
// command nobody could run is refused before a pipe exists.
func validateCommand(command []string) error {
	if len(command) == 0 || command[0] == "" {
		return fmt.Errorf("%w: the command is empty", ErrStart)
	}
	return nil
}
