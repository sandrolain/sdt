package acpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// The wire log's tests drive a recording sink rather than a file: the bridge's
// side of the contract is an io.WriteCloser per run, so a buffer proves the
// bytes and the viewer's own tests own the filesystem half.

// syncBuffer is one run's writer: a buffer that remembers being closed, so a
// write after the run ended is an error rather than a silent resurrection.
type syncBuffer struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, errors.New("write on a closed trace writer")
	}
	return b.buf.Write(p)
}

func (b *syncBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// traceSinkRecorder hands out one buffer per run and records what each open was
// named from; with err set it refuses every open, which is how a path that
// cannot be written reaches the bridge.
type traceSinkRecorder struct {
	mu      sync.Mutex
	runs    []RunInfo
	writers []*syncBuffer
	err     error
}

func (s *traceSinkRecorder) Open(run RunInfo) (io.WriteCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	s.runs = append(s.runs, run)
	w := &syncBuffer{}
	s.writers = append(s.writers, w)
	return w, nil
}

func (s *traceSinkRecorder) openCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.runs)
}

func (s *traceSinkRecorder) run(i int) RunInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[i]
}

func (s *traceSinkRecorder) writer(i int) *syncBuffer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writers[i]
}

// traceLines reads a run's capture back as parsed entries: a line that is not
// JSON is the defect this format exists to prevent, so it fails the test.
func traceLines(t *testing.T, w *syncBuffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(w.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("the capture line %q is not JSON: %v", line, err)
		}
		out = append(out, entry)
	}
	return out
}

// TestTraceCapturesBothDirectionsOfARun is the instrument end to end over the
// socket transport: one run opens one writer, named from the session, and the
// file reads header, frames of both directions with the wire verbatim, close.
func TestTraceCapturesBothDirectionsOfARun(t *testing.T) {
	sink := &traceSinkRecorder{}
	agent := newFakeAgent(t, streamingAgent("The corpus has 224 analyses."))
	bridge := New(Options{Root: t.TempDir(), Version: "test"})
	bridge.SetTrace(sink)
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	if err := session.Prompt(context.Background(), "how big is the corpus?"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if sink.openCount() != 1 {
		t.Fatalf("one run must open one writer, got %d", sink.openCount())
	}
	run := sink.run(0)
	if run.Transport != "socket" || run.Socket != agent.Path() {
		t.Errorf("the run must be named from the socket transport, got %+v", run)
	}
	if run.Session != agent.LastSessionID() {
		t.Errorf("the run must be named from the session: want %q, got %q", agent.LastSessionID(), run.Session)
	}

	lines := traceLines(t, sink.writer(0))
	if len(lines) < 4 {
		t.Fatalf("the capture is too thin to be a conversation: %d lines", len(lines))
	}
	if first := lines[0]; first["run"] != "header" || first["transport"] != "socket" || first["socket"] != agent.Path() {
		t.Errorf("the first line must be the run header for the socket, got %v", first)
	}
	// The run's start is one instant: the header's timestamp and the value a
	// {timestamp} filename is built from must name the same moment, not the end
	// of the handshake.
	headerAt, err := time.Parse(time.RFC3339Nano, lines[0]["t"].(string))
	if err != nil {
		t.Fatalf("the header timestamp is not RFC3339: %v", err)
	}
	if delta := run.Started.Sub(headerAt); delta != 0 {
		t.Errorf("the run's start must be the header's instant, got %v apart", delta)
	}
	if last := lines[len(lines)-1]; last["run"] != "close" {
		t.Errorf("the last line must close the run, got %v", last)
	}
	dirs := map[string]int{}
	sawInitialize, sawPrompt := false, false
	for _, entry := range lines {
		dir, _ := entry["dir"].(string)
		if dir == "" {
			continue
		}
		dirs[dir]++
		frame, _ := entry["frame"].(string)
		var decoded map[string]any
		if err := json.Unmarshal([]byte(frame), &decoded); err != nil {
			t.Fatalf("the frame %q is not the wire's own JSON: %v", frame, err)
		}
		if dir == "out" && strings.Contains(frame, `"initialize"`) {
			sawInitialize = true
		}
		if dir == "out" && strings.Contains(frame, `"session/prompt"`) {
			sawPrompt = true
		}
	}
	if dirs["in"] == 0 || dirs["out"] == 0 {
		t.Errorf("both directions must be captured, got %v", dirs)
	}
	if !sawInitialize || !sawPrompt {
		t.Errorf("the capture must carry the handshake and the turn (initialize=%v prompt=%v)", sawInitialize, sawPrompt)
	}
}

// TestTraceStdioRunCarriesTheDeclaredCommandAndPID covers the other transport:
// the run is named from the argv the project declared and the process that ran.
func TestTraceStdioRunCarriesTheDeclaredCommandAndPID(t *testing.T) {
	scratch := t.TempDir()
	root := t.TempDir()
	sink := &traceSinkRecorder{}
	bridge := New(Options{Root: root, Version: "test"})
	bridge.SetTrace(sink)
	command := spawnCommand(scratch, "")
	if _, err := bridge.Start(context.Background(), command, root); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if sink.openCount() != 1 {
		t.Fatalf("one run must open one writer, got %d", sink.openCount())
	}
	run := sink.run(0)
	if run.Transport != "stdio" {
		t.Errorf("the transport must be stdio, got %q", run.Transport)
	}
	if len(run.Command) == 0 || run.Command[0] != command[0] {
		t.Errorf("the run must carry the declared command %v, got %v", command, run.Command)
	}
	if run.PID <= 0 {
		t.Errorf("the run must carry the spawned pid, got %d", run.PID)
	}
	lines := traceLines(t, sink.writer(0))
	if first := lines[0]; first["run"] != "header" || first["pid"] == nil {
		t.Errorf("the header must carry the pid, got %v", first)
	}
}

// TestTraceFailedAttachIsFiledUnderNoSession covers the failure path: an agent
// that cannot agree on the protocol version fails the handshake, and whatever
// crossed before the failure is still filed — under the nosession name, with a
// close marker carrying the cause.
func TestTraceFailedAttachIsFiledUnderNoSession(t *testing.T) {
	sink := &traceSinkRecorder{}
	agent := newFakeAgent(t, agentVersion(999, "sess-1"))
	bridge := New(Options{Root: t.TempDir(), Version: "test"})
	bridge.SetTrace(sink)
	if _, err := bridge.Activate(context.Background(), agent.Path()); !errors.Is(err, ErrProtocolVersion) {
		t.Fatalf("want ErrProtocolVersion, got %v", err)
	}

	if sink.openCount() != 1 {
		t.Fatalf("a failed attach must still open one writer, got %d", sink.openCount())
	}
	if run := sink.run(0); run.Session != traceNoSession {
		t.Errorf("a run with no session must be filed as %q, got %q", traceNoSession, run.Session)
	}
	lines := traceLines(t, sink.writer(0))
	if len(lines) < 2 {
		t.Fatalf("the failed run must leave header and close, got %d lines", len(lines))
	}
	if first := lines[0]; first["run"] != "header" {
		t.Errorf("the failed run must still start with its header, got %v", first)
	}
	if last := lines[len(lines)-1]; last["run"] != "close" || !strings.Contains(fmt.Sprint(last["reason"]), "version") {
		t.Errorf("the close must carry the version refusal, got %v", last)
	}
}

// TestTraceASinkThatRefusesLeavesTheConversationAlone is the failure posture:
// a sink that cannot open a writer ends the instrument, never the session.
func TestTraceASinkThatRefusesLeavesTheConversationAlone(t *testing.T) {
	sink := &traceSinkRecorder{err: errors.New("the declared path is not writable")}
	agent := newFakeAgent(t, streamingAgent("an answer anyway"))
	bridge := New(Options{Root: t.TempDir(), Version: "test"})
	bridge.SetTrace(sink)
	if _, err := bridge.Activate(context.Background(), agent.Path()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	if err := session.Prompt(context.Background(), "does the turn survive?"); err != nil {
		t.Fatalf("a refused trace must not stop the turn: %v", err)
	}
	_ = bridge.Close()
	if sink.openCount() != 0 {
		t.Fatal("a refusing sink must never record an open")
	}
}

// TestTraceIsOffWithoutASink keeps the default the ordinary case: no sink, no
// wrapper worth a second thought, and the conversation is the same.
func TestTraceIsOffWithoutASink(t *testing.T) {
	bridge, agent := activatedBridge(t, streamingAgent("unchanged"))
	session, ok := bridge.Session(bridge.SessionID())
	if !ok {
		t.Fatalf("the bridge holds no session %q", bridge.SessionID())
	}
	if err := session.Prompt(context.Background(), "still working?"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if asked := agent.Methods(); len(asked) == 0 {
		t.Fatal("the agent was never asked anything")
	}
}

// TestTraceReassemblesFramesAcrossReads is the framing half in isolation: a
// frame split across reads completes at its newline, the two directions
// reassemble independently, and every line parses.
func TestTraceReassemblesFramesAcrossReads(t *testing.T) {
	sink := &traceSinkRecorder{}
	rt := newRunTrace(sink, traceRun{transport: "socket", socket: "a.sock"})
	rt.emit(traceEntry{T: now(), Run: "header", Transport: "socket", Socket: "a.sock"})
	rt.capture("in", []byte(`{"jsonrpc":"2.0","id":1,`))
	rt.capture("out", []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`+"\n"))
	rt.capture("in", []byte(`"method":"initialize"}`+"\n"))
	rt.setSession("sess-9")
	rt.finish("closed")

	if sink.openCount() != 1 || sink.run(0).Session != "sess-9" {
		t.Fatalf("the run must open once under its session, got %+v", sink.runs)
	}
	lines := traceLines(t, sink.writer(0))
	if len(lines) != 4 {
		t.Fatalf("want header, two frames and close, got %d lines: %v", len(lines), lines)
	}
	if frame, _ := lines[1]["frame"].(string); frame != `{"jsonrpc":"2.0","id":1,"result":{}}` {
		t.Errorf("the out frame must be verbatim, got %q", frame)
	}
	if frame, _ := lines[2]["frame"].(string); frame != `{"jsonrpc":"2.0","id":1,"method":"initialize"}` {
		t.Errorf("the in frame must be reassembled whole, got %q", frame)
	}
	if lines[1]["dir"] != "out" || lines[2]["dir"] != "in" {
		t.Errorf("each frame must carry its direction, got %v %v", lines[1]["dir"], lines[2]["dir"])
	}
}

// TestTraceSerialisesConcurrentFrames proves the mutex earns its place: frames
// written from several goroutines at once still leave whole, separate lines.
func TestTraceSerialisesConcurrentFrames(t *testing.T) {
	sink := &traceSinkRecorder{}
	rt := newRunTrace(sink, traceRun{transport: "stdio"})
	rt.setSession("sess-1")

	const frames = 50
	var wg sync.WaitGroup
	for i := 0; i < frames; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rt.capture("out", []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d}`+"\n", i)))
		}(i)
	}
	wg.Wait()
	rt.finish("closed")

	lines := traceLines(t, sink.writer(0))
	seen := map[string]bool{}
	for _, entry := range lines {
		if dir, _ := entry["dir"].(string); dir == "" {
			continue
		}
		frame := fmt.Sprint(entry["frame"])
		if seen[frame] {
			t.Fatalf("the frame %q was written twice", frame)
		}
		seen[frame] = true
	}
	if len(seen) != frames {
		t.Fatalf("want %d whole frames, got %d", frames, len(seen))
	}
}

// TestTraceRecordsAnOverlongFrameAsTruncated keeps the promise that nothing is
// dropped silently: a frame beyond the SDK's own line cap is recorded cut to
// the cap, marked truncated, and the newline still ends it.
func TestTraceRecordsAnOverlongFrameAsTruncated(t *testing.T) {
	sink := &traceSinkRecorder{}
	rt := newRunTrace(sink, traceRun{transport: "stdio"})
	rt.setSession("sess-1")

	huge := bytes.Repeat([]byte("a"), traceFrameLimit+4096)
	for chunk := huge; len(chunk) > 0; {
		n := 1 << 20
		if len(chunk) < n {
			n = len(chunk)
		}
		rt.capture("in", chunk[:n])
		chunk = chunk[n:]
	}
	rt.capture("in", []byte("\n"))
	rt.finish("closed")

	for _, entry := range traceLines(t, sink.writer(0)) {
		if entry["dir"] != "in" {
			continue
		}
		if entry["truncated"] != true {
			t.Fatalf("the overlong frame must be marked truncated, got %v", entry)
		}
		if frame, _ := entry["frame"].(string); len(frame) != traceFrameLimit {
			t.Errorf("the truncated frame must be cut to the cap, got %d bytes", len(frame))
		}
		return
	}
	t.Fatal("the overlong frame was not captured at all")
}

// TestTraceCountsWhatThePrefixBufferDrops covers the other bound: entries that
// cross the prefix buffer before a run's writer exists are counted and reported
// as a dropped record once it opens, never lost without a word.
func TestTraceCountsWhatThePrefixBufferDrops(t *testing.T) {
	sink := &traceSinkRecorder{}
	rt := newRunTrace(sink, traceRun{transport: "stdio"})

	rt.capture("in", bytes.Repeat([]byte("a"), 6<<20))
	rt.capture("in", []byte("\n"))
	rt.capture("in", bytes.Repeat([]byte("b"), 6<<20))
	rt.capture("in", []byte("\n"))
	rt.setSession("sess-1")
	rt.finish("closed")

	var dropped map[string]any
	frames := 0
	for _, entry := range traceLines(t, sink.writer(0)) {
		if entry["run"] == "dropped" {
			dropped = entry
			continue
		}
		if entry["dir"] != nil {
			frames++
		}
	}
	if frames != 1 {
		t.Errorf("one frame fit in the buffer, want 1 captured, got %d", frames)
	}
	if dropped == nil {
		t.Fatal("the dropped entries must be reported as a record")
	}
	if got, _ := dropped["dropped"].(float64); got < 1 {
		t.Errorf("the drop count must be at least 1, got %v", got)
	}
}
