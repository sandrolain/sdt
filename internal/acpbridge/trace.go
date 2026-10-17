package acpbridge

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// The capture's own bounds. The SDK reads a frame with a line scanner capped at
// 10 MB, so a frame larger than that never arrives as one message; the trace
// keeps the same number for its own reassembly and for the prefix it holds
// before a run's writer exists.
const (
	traceFrameLimit  = 10 << 20
	traceBufferLimit = 10 << 20
)

// traceNoSession is the name a run that never resolved a session is filed under,
// so a failed attach still leaves a readable file instead of no file at all.
const traceNoSession = "nosession"

// RunInfo is what one run of the wire log is: when it started, which transport
// carried it, what the project declared (the command, or the socket path), the
// spawned process's pid, and the ACP session id the run resolved to — or
// traceNoSession when it never did. It is handed to the sink whole, because the
// path the viewer opens is named from all of it.
type RunInfo struct {
	Started   time.Time
	Transport string
	Command   []string
	Socket    string
	PID       int
	Session   string
}

// TraceSink opens the writer for one run. The bridge calls it once per attach —
// after the session id is known, or with traceNoSession when the attach failed —
// and calls Close on what it returned when the run ends. Closing is the sink's
// decision: a per-run file is really closed, while a single appended file hands
// back a wrapper whose Close is a no-op and is closed by its owner. The viewer
// owns the path: the sink expands the project's declared template and refuses
// what escapes the root, so the bridge never sees a path at all.
type TraceSink interface {
	Open(run RunInfo) (io.WriteCloser, error)
}

// traceRun is the run's identity as the bridge knows it at attach time; it
// becomes a RunInfo when the session id completes it. Started is taken when the
// attach begins — the same instant as the header line — not when the session
// resolves, so a {timestamp} filename names the run and not the handshake's end.
type traceRun struct {
	started   time.Time
	transport string
	command   []string
	socket    string
	pid       int
}

// info is the RunInfo this run resolves to.
func (r traceRun) info(session string) RunInfo {
	started := r.started
	if started.IsZero() {
		started = time.Now().UTC()
	}
	return RunInfo{
		Started:   started,
		Transport: r.transport,
		Command:   r.command,
		Socket:    r.socket,
		PID:       r.pid,
		Session:   session,
	}
}

// traceEntry is one JSONL line of the capture. One struct carries every line
// shape — frame, header, close — because the file is read back by a person and
// a jq, not by a protocol: a field that is not part of a line's shape is absent
// rather than sent as a zero value, and the frame is a string so a truncated
// frame keeps the line valid JSON.
type traceEntry struct {
	T         string   `json:"t"`
	Dir       string   `json:"dir,omitempty"`
	Frame     string   `json:"frame,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
	Run       string   `json:"run,omitempty"`
	Version   string   `json:"version,omitempty"`
	Transport string   `json:"transport,omitempty"`
	Command   []string `json:"command,omitempty"`
	Socket    string   `json:"socket,omitempty"`
	PID       int      `json:"pid,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Dropped   int      `json:"dropped,omitempty"`
}

// now is the capture timestamp: when the bridge saw the line, not when the
// agent produced it — enough to order a conversation, not to profile it.
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// pendingFrame is the bytes of one frame not yet seen whole, per direction: the
// two directions reassemble independently because reads and writes interleave.
type pendingFrame struct {
	buf       []byte
	truncated bool
}

// runTrace is one run's capture. It reassembles the bytes the wrapper sees into
// whole frames, serialises every write behind one mutex — the SDK's send path is
// not serialised, so frames can cross it from more than one goroutine — and
// holds the prefix in memory until the run's writer exists: the session id
// names the file, and it is only known after the handshake.
//
// A failure ends the instrument, never the conversation: a refused sink or a
// failed write sets disabled, every later capture is dropped, and the error is
// kept for the bridge to report as a sentence.
type runTrace struct {
	sink TraceSink
	run  traceRun

	mu       sync.Mutex
	buf      []byte // the prefix: header and pre-session frames, held until open
	w        io.WriteCloser
	opened   bool
	closed   bool
	disabled bool
	err      error
	dropped  int
	pending  map[string]*pendingFrame
}

func newRunTrace(sink TraceSink, run traceRun) *runTrace {
	return &runTrace{
		sink:    sink,
		run:     run,
		pending: map[string]*pendingFrame{},
	}
}

// capture feeds the bytes the wrapper saw in one read or write into the
// reassembly for their direction; every newline completes a frame.
func (rt *runTrace) capture(dir string, p []byte) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.disabled || rt.closed {
		return
	}
	pf := rt.pending[dir]
	if pf == nil {
		pf = &pendingFrame{}
		rt.pending[dir] = pf
	}
	for len(p) > 0 {
		if i := bytes.IndexByte(p, '\n'); i >= 0 {
			pf.buf = appendFrame(pf.buf, p[:i], &pf.truncated)
			rt.emitLocked(traceEntry{T: now(), Dir: dir, Frame: string(pf.buf), Truncated: pf.truncated})
			pf.buf, pf.truncated = pf.buf[:0], false
			p = p[i+1:]
			continue
		}
		pf.buf = appendFrame(pf.buf, p, &pf.truncated)
		return
	}
}

// appendFrame adds at most what the frame bound still allows; the bytes beyond
// it are dropped and the frame is marked truncated, so a runaway frame is
// recorded as cut rather than silently missing.
func appendFrame(dst, add []byte, truncated *bool) []byte {
	if space := traceFrameLimit - len(dst); len(add) > space {
		if space > 0 {
			dst = append(dst, add[:space]...)
		}
		*truncated = true
		return dst
	}
	return append(dst, add...)
}

// emit writes one entry line — to the buffer while the run has no writer, to
// the writer once it does.
func (rt *runTrace) emit(e traceEntry) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.emitLocked(e)
}

func (rt *runTrace) emitLocked(e traceEntry) {
	if rt.disabled || rt.closed {
		return
	}
	line, err := json.Marshal(e)
	if err != nil {
		// A line this code built cannot fail to marshal; if one ever did, the
		// instrument stops rather than writing a file of half-lines.
		rt.disabled, rt.err = true, err
		return
	}
	line = append(line, '\n')
	if !rt.opened {
		if len(rt.buf)+len(line) > traceBufferLimit {
			rt.dropped++
			return
		}
		rt.buf = append(rt.buf, line...)
		return
	}
	if _, err := rt.w.Write(line); err != nil {
		rt.disabled, rt.err = true, err
	}
}

// setSession completes the run: the session id is what the file is named from,
// so the writer is opened only now, and the buffered prefix is flushed into it.
func (rt *runTrace) setSession(session string) {
	rt.open(session)
}

// open asks the sink for the run's writer and flushes the prefix into it. A
// sink that refuses ends the capture with the refusal kept for the sentence.
func (rt *runTrace) open(session string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.opened || rt.disabled || rt.closed {
		return
	}
	w, err := rt.sink.Open(rt.run.info(session))
	if err != nil {
		rt.disabled, rt.err = true, err
		return
	}
	rt.w, rt.opened = w, true
	if len(rt.buf) > 0 {
		if _, err := w.Write(rt.buf); err != nil {
			rt.disabled, rt.err = true, err
		}
		rt.buf = nil
	}
	if rt.dropped > 0 {
		rt.emitLocked(traceEntry{T: now(), Run: "dropped", Dropped: rt.dropped})
		rt.dropped = 0
	}
}

// fail ends a run whose attach did not complete: whatever crossed before the
// failure is still the reader's to see, so the writer opens under the
// nosession name, the close marker carries the cause, and the instrument ends.
func (rt *runTrace) fail(cause error) {
	reason := "the attach failed"
	if cause != nil {
		reason = cause.Error()
	}
	rt.open(traceNoSession)
	rt.finish(reason)
}

// finish writes the close marker, flushes whatever half-frame is left, and ends
// the run's writer. A run that never opened one — no sink, or a sink that
// refused — ends here with nothing to close.
func (rt *runTrace) finish(reason string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		return
	}
	if !rt.opened || rt.disabled {
		rt.closed = true
		if rt.w != nil {
			// The close error is kept for the bridge's report, not dropped: a
			// writer that cannot be closed is worth a line in the log even though
			// the run it belonged to is already over.
			if cerr := rt.w.Close(); cerr != nil && rt.err == nil {
				rt.err = cerr
			}
			rt.w = nil
		}
		return
	}
	for dir, pf := range rt.pending {
		if len(pf.buf) > 0 {
			rt.emitLocked(traceEntry{T: now(), Dir: dir, Frame: string(pf.buf), Truncated: true})
			pf.buf, pf.truncated = pf.buf[:0], false
		}
	}
	rt.emitLocked(traceEntry{T: now(), Run: "close", Reason: reason})
	rt.closed = true
	if rt.w != nil {
		if cerr := rt.w.Close(); cerr != nil && rt.err == nil {
			rt.err = cerr
		}
		rt.w = nil
	}
}

// err reports the failure that ended the capture, if one did.
func (rt *runTrace) failure() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.err
}

// tracedConn is the capture's seam: it is the rw every byte crosses on its way
// to and from the SDK, for both transports, so teeing here sees the whole
// protocol without touching it. Reads and writes are delegated first and the
// bytes are captured second, so the log never stands between the protocol and
// its transport.
type tracedConn struct {
	inner io.ReadWriteCloser
	rt    *runTrace
}

func (c *tracedConn) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	if n > 0 {
		c.rt.capture("in", p[:n])
	}
	return n, err
}

func (c *tracedConn) Write(p []byte) (int, error) {
	n, err := c.inner.Write(p)
	if n > 0 {
		c.rt.capture("out", p[:n])
	}
	return n, err
}

func (c *tracedConn) Close() error { return c.inner.Close() }
