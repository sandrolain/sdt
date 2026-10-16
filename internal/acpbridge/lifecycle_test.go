package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge/acptest"
)

// spawnHelperMarker tells a re-executed test binary that it is the agent process.
// The arguments after it are the scratch directory the child reports into and,
// optionally, how it should behave. They are arguments rather than environment
// variables because Start inherits the test process's environment: an environment
// marker would make this same test serve as a helper inside the parent and hang the
// suite.
const spawnHelperMarker = "sdt-acp-spawn-helper"

// spawnHelperDies is the mode in which the child ends on its own a short time after
// it starts serving — a working agent that dies, which is what the restart bound is
// about. The delay is what makes it deterministic: the handshake over a local pipe
// finishes long before it.
const (
	spawnHelperDieMode      = "die"
	spawnHelperStubbornMode = "stubborn"
	spawnHelperLoadMode     = "load"
	spawnHelperRefuseMode   = "refuseload"
	spawnHelperLife         = 400 * time.Millisecond
	// spawnHelperText is what the child's peer answers a turn with.
	spawnHelperText = "hello from the spawned agent"
)

// spawnHelperRequest is what a parent encodes for the child process.
type spawnHelperRequest struct {
	// Scratch is the directory the child reports its pid and its working directory
	// into: a file per process, so the parent counts processes rather than guessing.
	Scratch string
	// Mode is empty (or "serve") to serve until the parent closes the stream, and
	// "die" to end shortly after starting — an agent that worked and then died.
	Mode string
}

// spawnHelperRequestFrom reads the request from a re-executed binary's arguments.
func spawnHelperRequestFrom(args []string) (spawnHelperRequest, bool) {
	for i, arg := range args {
		if arg != spawnHelperMarker || i+1 >= len(args) {
			continue
		}
		req := spawnHelperRequest{Scratch: args[i+1]}
		if i+2 < len(args) {
			req.Mode = args[i+2]
		}
		return req, true
	}
	return spawnHelperRequest{}, false
}

// TestSpawnHelper is not a test: it is the agent process the spawn tests re-execute
// from this same binary, serving the ACP peer on its own stdin and stdout. A real
// process with real pipes is the point — an in-process stub would prove nothing
// about spawning, about the pipes, about the group the child is put in, or about
// what happens when the agent dies.
func TestSpawnHelper(t *testing.T) {
	req, ok := spawnHelperRequestFrom(os.Args)
	if !ok {
		t.Skip("helper process: spawned by the tests that start an agent")
	}
	if err := os.MkdirAll(req.Scratch, 0o755); err != nil {
		t.Fatalf("scratch directory: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("the helper must know its own directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(req.Scratch, "cwd"), []byte(wd), 0o600); err != nil { //#nosec G306 -- the helper's own scratch file
		t.Fatalf("recording the working directory: %v", err)
	}
	appendPID(t, filepath.Join(req.Scratch, "pids"))
	if req.Mode == spawnHelperDieMode {
		time.AfterFunc(spawnHelperLife, func() { os.Exit(0) })
	}
	if req.Mode == spawnHelperStubbornMode {
		// A child that ignores the graceful signal, and a grandchild that ignores it too:
		// only a kill addressed to the whole group can end these, which is what makes the
		// group an observable thing rather than a claim. The grandchild inherits this
		// process's group, because the bridge put this process in one of its own.
		signal.Ignore(syscall.SIGTERM)
		//#nosec G204 -- a test helper starting its own grandchild.
		grandchild := exec.Command("/bin/sh", "-c",
			"trap '' TERM; echo $$ > "+filepath.Join(req.Scratch, "grandchild")+"; sleep 300")
		if err := grandchild.Start(); err != nil {
			t.Fatalf("starting the grandchild: %v", err)
		}
	}
	agent := acptest.NewStream(t, os.Stdin, os.Stdout, recordCalls(spawnHandler(req.Mode), filepath.Join(req.Scratch, "calls")))
	<-agent.Done()
}

// spawnHandler is what the child answers with: the plain peer, or one that carries the
// loadSession capability and either takes a session back or refuses it.
func spawnHandler(mode string) acptest.Handler {
	switch mode {
	case spawnHelperLoadMode:
		return acptest.Loadable(spawnHelperText, false)
	case spawnHelperRefuseMode:
		return acptest.Loadable(spawnHelperText, true)
	}
	return acptest.Streams(spawnHelperText)
}

// recordCalls appends every method the child is asked to a file. It is how a parent sees
// what happened inside the process it spawned — a session/load in particular, which is
// otherwise invisible from outside.
func recordCalls(inner acptest.Handler, path string) acptest.Handler {
	return func(agent *acptest.Agent, method string, params json.RawMessage) any {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //#nosec G304 -- the helper's own scratch file
		if err == nil {
			_, _ = f.WriteString(method + "\n")
			_ = f.Close()
		}
		return inner(agent, method, params)
	}
}

// askedMethods lists the methods the child was asked, in order.
func askedMethods(t *testing.T, scratch string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(scratch, "calls")) //#nosec G304 -- a path this test built
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading the calls file: %v", err)
	}
	return strings.Fields(string(raw))
}

// wasAsked reports whether a method appears in what the child was asked.
func wasAsked(t *testing.T, scratch, method string) bool {
	t.Helper()
	for _, asked := range askedMethods(t, scratch) {
		if asked == method {
			return true
		}
	}
	return false
}

// appendPID records this process in the scratch file, one line per start: the count
// is how a test sees one process rather than two, a restart, or four attempts.
func appendPID(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //#nosec G304 -- the helper's own scratch file
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("closing %s: %v", path, err)
		}
	}()
	if _, err := f.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// spawnCommand builds the argv a test hands to Start or EnsureStarted.
func spawnCommand(scratch, mode string) []string {
	command := []string{os.Args[0], "-test.run=TestSpawnHelper", spawnHelperMarker, scratch}
	if mode != "" {
		command = append(command, mode)
	}
	return command
}

// startedPIDs lists the processes that reported into the scratch directory, in the
// order they started. The count is how a test sees one process rather than two, a
// restart, or a bounded number of attempts; the pids are how it sees that a refused
// or stopped process is really gone rather than merely forgotten.
func startedPIDs(t *testing.T, scratch string) []int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(scratch, "pids")) //#nosec G304 -- a path this test built
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading the scratch file: %v", err)
	}
	var pids []int
	for _, field := range strings.Fields(string(raw)) {
		pid, convErr := strconv.Atoi(field)
		if convErr != nil {
			t.Fatalf("the scratch file holds %q, which is not a pid: %v", field, convErr)
		}
		pids = append(pids, pid)
	}
	return pids
}

// processGone reports whether a pid has ended, which is how a test sees that a
// stopped agent really went rather than changing state while it kept running.
func processGone(pid int) bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }

// waitFor polls until the condition holds or the deadline passes; a condition that
// never holds is a failure, not a flake.
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestEnsureStartedIsIdempotent covers the property the panel depends on: asking
// twice is one agent. The second call answers the live session, and the scratch file
// — written by the child itself — shows one process rather than two.
func TestEnsureStartedIsIdempotent(t *testing.T) {
	scratch := t.TempDir()
	root := t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	first, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root)
	if err != nil {
		t.Fatalf("first EnsureStarted: %v", err)
	}
	second, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root)
	if err != nil {
		t.Fatalf("second EnsureStarted: %v", err)
	}
	if first != second {
		t.Fatalf("the second call must answer the live session: %q then %q", first, second)
	}
	if started := len(startedPIDs(t, scratch)); started != 1 {
		t.Fatalf("a live agent must never be doubled: %d processes were started", started)
	}
	if status, reason := bridge.Status(); status != StatusActive || reason != "" {
		t.Fatalf("a live agent is active: status=%q reason=%q", status, reason)
	}
}

// TestEnsureStartedRefusesWithoutACommand states the boundary between the bridge and
// the config: nothing to start is a named refusal, not an empty spawn.
func TestEnsureStartedRefusesWithoutACommand(t *testing.T) {
	bridge := New(Options{Root: t.TempDir()})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), nil, t.TempDir()); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("want ErrNoCommand, got %v", err)
	}
	if bridge.Activated() {
		t.Fatal("a refused start must leave the bridge inert")
	}
}

// TestEnsureStartedRestartsAnAgentThatDies covers the requirement that a working
// agent dying is not the end of the conversation: the bridge starts it again, and the
// session is live again afterwards.
func TestEnsureStartedRestartsAnAgentThatDies(t *testing.T) {
	oldDelay := restartDelay
	restartDelay = 20 * time.Millisecond
	t.Cleanup(func() { restartDelay = oldDelay })

	scratch := t.TempDir()
	root := t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	first := bridge.agentPID()
	if first == 0 {
		t.Fatal("a started agent must be a process")
	}
	if err := syscallKill(first); err != nil {
		t.Fatalf("killing the agent: %v", err)
	}

	waitFor(t, "the agent to be started again (status="+bridge.statusLine()+")", func() bool {
		status, _ := bridge.Status()
		return len(startedPIDs(t, scratch)) >= 2 && status == StatusActive
	})
	if !bridge.Activated() {
		t.Fatal("a restarted agent must leave the bridge active")
	}
	if bridge.agentPID() == first {
		t.Fatal("the restarted agent is a new process")
	}
}

// TestEnsureStartedStopsAtTheBound covers what keeps a broken command from becoming a
// loop: a child that works and then dies on its own is started again a bounded number
// of times, and past the bound the bridge stops and carries the reason. The attempt
// count is asserted from the processes themselves.
func TestEnsureStartedStopsAtTheBound(t *testing.T) {
	oldDelay := restartDelay
	restartDelay = 20 * time.Millisecond
	t.Cleanup(func() { restartDelay = oldDelay })

	scratch := t.TempDir()
	root := t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, spawnHelperDieMode), root); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	waitFor(t, "the attempts to run out (status="+bridge.statusLine()+")", func() bool {
		status, _ := bridge.Status()
		return status == StatusStopped
	})

	status, reason := bridge.Status()
	if status != StatusStopped {
		t.Fatalf("the bound must end in a stop: status=%q", status)
	}
	if reason == "" {
		t.Fatal("a stop at the bound must carry the reason")
	}
	if started := len(startedPIDs(t, scratch)); started != maxRestarts+1 {
		t.Fatalf("the bound is %d restarts after the first start: %d processes were started", maxRestarts, started)
	}
	if bridge.Activated() {
		t.Fatal("a stopped bridge must hold no connection")
	}
}

// TestStopEndsTheAgentWithoutRestartingIt covers the other half of the exit rule: a
// stop somebody asked for is not a death, so nothing is started again — and the
// bridge is left able to start when asked, unlike a closed one.
func TestStopEndsTheAgentWithoutRestartingIt(t *testing.T) {
	oldDelay := restartDelay
	restartDelay = 20 * time.Millisecond
	t.Cleanup(func() { restartDelay = oldDelay })

	scratch := t.TempDir()
	root := t.TempDir()
	bridge := New(Options{Root: root})
	t.Cleanup(func() { _ = bridge.Close() })

	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}
	pid := bridge.agentPID()
	if pid == 0 {
		t.Fatal("a started agent must be a process")
	}
	if err := bridge.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, "the stopped agent to be gone", func() bool { return processGone(pid) })

	if started := len(startedPIDs(t, scratch)); started != 1 {
		t.Fatalf("an intended stop must not restart anything: %d processes were started", started)
	}
	if status, reason := bridge.Status(); status != StatusStopped || reason != "" {
		t.Fatalf("a stop somebody asked for carries no reason: status=%q reason=%q", status, reason)
	}
	if bridge.Activated() {
		t.Fatal("a stopped bridge holds no connection")
	}
	if _, err := bridge.EnsureStarted(context.Background(), spawnCommand(scratch, ""), root); err != nil {
		t.Fatalf("a stopped bridge must start again when asked: %v", err)
	}
	if started := len(startedPIDs(t, scratch)); started != 2 {
		t.Fatalf("the agent must be started again once: %d processes were started", started)
	}
}

// agentPID is the pid of the process the bridge currently owns: what a test kills to
// make a working agent die.
func (b *Bridge) agentPID() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.proc == nil || b.proc.cmd.Process == nil {
		return 0
	}
	return b.proc.cmd.Process.Pid
}

// syscallKill ends an agent the way a crash does: the process goes without a word.
func syscallKill(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }

// statusLine renders the process view for a timeout message: a test that waits for a
// transition must say which state it actually saw.
func (b *Bridge) statusLine() string {
	status, reason := b.Status()
	return string(status) + " " + reason
}

// grandchildPID waits for the pid the stubborn helper's grandchild reports.
func grandchildPID(t *testing.T, scratch string) int {
	t.Helper()
	var pid int
	waitFor(t, "the grandchild to report its pid", func() bool {
		raw, err := os.ReadFile(filepath.Join(scratch, "grandchild")) //#nosec G304 -- a path this test built
		if err != nil {
			return false
		}
		value, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if convErr != nil {
			return false
		}
		pid = value
		return true
	})
	return pid
}

// restartDelayFor shrinks the pause between attempts to a value a test can work inside,
// as opposed to restartWith's near-zero one.
func restartDelayFor(t *testing.T, delay time.Duration) {
	t.Helper()
	old := restartDelay
	restartDelay = delay
	t.Cleanup(func() { restartDelay = old })
}
