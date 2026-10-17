package acpbridge

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ErrStart is the refusal of a process that cannot be started at all: the command
// is not there, is not executable, or the operating system will not run it. It is
// separate from ErrUnreachable because the two failures read differently to the
// person who has to fix them.
var ErrStart = errors.New("acpbridge: the agent process could not be started")

// startGrace is how long a terminating agent is given to end by itself before it is
// killed, and startStopWait is how long that kill is then waited for. Both are
// short on purpose: a reader is waiting on a panel, not on a well-behaved exit.
const (
	startGrace    = 3 * time.Second
	startStopWait = 5 * time.Second
)

// Start launches the command the project declared and speaks ACP over its stdio.
// It is a second *transport*, not a second bridge: everything from the connection
// upwards — the client callbacks, the file confinement, the permission queue, the
// thread fold and the sessions — is attach's, shared with the socket transport.
//
// The command runs with dir as its working directory, so the agent's own relative
// paths and the confinement agree about where "here" is, and in its own process
// group, so an agent that spawns helpers leaves nothing behind when it stops.
// There is no shell: argv is executed as it was written, so nothing is split,
// quoted or interpolated. The context is the process's life and must be the
// viewer's own, never a request's — a request that ends must not end the agent.
func (b *Bridge) Start(ctx context.Context, command []string, dir string) (string, error) {
	_, session, err := b.start(ctx, command, dir)
	return session, err
}

// start is where a process comes into existence: a direct Start, EnsureStarted and a
// restart all come through here, so there is one spawn path and one transport to
// own, and an agent that dies leaves exactly what any other start leaves.
func (b *Bridge) start(ctx context.Context, command []string, dir string) (*process, string, error) {
	proc, session, _, err := b.startWith(ctx, command, dir, nil)
	return proc, session, err
}

// startWith is start with what a restart carries: the conversations the dead process
// left, which are put back on the new connection. It returns the sentence the panel
// should show when an agent could not be put back on what it had.
func (b *Bridge) startWith(ctx context.Context, command []string, dir string, previous *restartState) (*process, string, string, error) {
	if err := validateCommand(command); err != nil {
		return nil, "", "", err
	}
	//#nosec G204,G702 -- the argv is the project's own configuration, executed
	// directly with no shell; nothing in a browser request can reach it (phase 5
	// refuses a command in a body, and asserts the refusal).
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: stdin: %v", ErrStart, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: stdout: %v", ErrStart, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: stderr: %v", ErrStart, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, "", "", fmt.Errorf("%w: %q: %v", ErrStart, command[0], err)
	}

	proc := startProcess(cmd, b.log)
	proc.pumpStderr(stderr)

	session, note, err := b.attach(ctx, streamPair{reader: stdout, writer: stdin}, proc.terminate, previous,
		traceRun{transport: "stdio", command: command, pid: cmd.Process.Pid})
	if err != nil {
		return nil, "", "", err
	}
	b.log.Info("agent bridge started", "session", session.ID(), "command", strings.Join(command, " "))
	return proc, session.ID(), note, nil
}

// process is a running agent: the command, its own group, and the single wait that
// reports how it ended. It is a type rather than a few locals because its lifetime
// outlives the call that started it — the exit is observed later, and the stop path
// must not race that observation with a second wait.
type process struct {
	cmd  *exec.Cmd
	log  *slog.Logger
	exit chan struct{}
	err  error
}

// startProcess waits for the command in one goroutine and closes the exit channel
// when it is gone, so a stop and any watcher wait on the same signal instead of
// racing each other for the one permitted Wait.
func startProcess(cmd *exec.Cmd, log *slog.Logger) *process {
	p := &process{cmd: cmd, log: log, exit: make(chan struct{})}
	go func() {
		defer close(p.exit)
		p.err = cmd.Wait()
	}()
	return p
}

// Done is closed once the agent has exited.
func (p *process) Done() <-chan struct{} { return p.exit }

// Err is how the agent ended; it is meaningful once Done is closed.
func (p *process) Err() error { return p.err }

// pumpStderr carries the agent's own diagnostics into the viewer's log, line by
// line. An agent's stderr is not part of the protocol, and swallowing it would hide
// exactly the reason a start failed.
func (p *process) pumpStderr(stderr io.Reader) {
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			p.log.Info("agent", "stderr", scanner.Text())
		}
	}()
}

// terminate stops the agent the way an owner should: the whole group is asked to
// end, given a short grace, and then killed, with a bounded wait for that kill. The
// signal goes to the group rather than to the process because an agent may have
// spawned helpers, and killing the process alone would leave a grandchild running
// with the viewer gone.
func (p *process) terminate() error {
	if p.cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(p.cmd.Process.Pid)
	if err != nil {
		// The process is already gone, or its group is not ours to read; the direct
		// pid is the honest fallback.
		pgid = p.cmd.Process.Pid
	}
	p.signalGroup(pgid, syscall.SIGTERM)
	select {
	case <-p.exit:
	case <-time.After(startGrace):
	}
	// The kill goes to the group whether or not the leader left on its own: a helper the
	// agent spawned is in the same group, and leaving it running is the orphan this
	// ownership exists to prevent. An already-empty group answers ESRCH and nothing
	// happens.
	p.signalGroup(pgid, syscall.SIGKILL)
	select {
	case <-p.exit:
		return nil
	case <-time.After(startStopWait):
		return fmt.Errorf("%w: the agent did not stop after a kill", ErrStart)
	}
}

// signalGroup sends a signal to a whole process group, ignoring one that has
// already ended.
func (p *process) signalGroup(pgid int, signal syscall.Signal) {
	if pgid <= 0 {
		return
	}
	if err := syscall.Kill(-pgid, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
		p.log.Debug("signalling the agent group", "signal", signal, "error", err)
	}
}

// streamPair is a pair of streams shaped like a connection: the session loop and
// the ACP connection only ever read and write, so a process's stdout and stdin
// serve as well as a socket. Closing it is a no-op on purpose — the process's
// terminate closes the pipes, because they belong to the process.
type streamPair struct {
	reader io.Reader
	writer io.Writer
}

func (p streamPair) Read(b []byte) (int, error)  { return p.reader.Read(b) }
func (p streamPair) Write(b []byte) (int, error) { return p.writer.Write(b) }
func (p streamPair) Close() error                { return nil }
