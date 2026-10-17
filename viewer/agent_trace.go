package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

// The viewer's half of the wire log: it owns the declared path — expansion, the
// confinement that keeps it inside the project, the file handles — while the
// bridge owns the protocol and writes what it captures into the writer this
// side hands it per run. The declaration is the user's act, so the path comes
// from .sdt.yaml and from nothing else: no request and no agent can name it.

const (
	// traceTimestampLayout names one run's file: a restart is a new run, so a
	// declared {timestamp} keeps each one instead of appending them together.
	traceTimestampLayout = "20060102-150405"
	// traceNoSessionName is what {session} becomes for a run that never resolved
	// a session, so a failed attach is still filed.
	traceNoSessionName = "nosession"
	// traceFileMode is the file's own mode: it may hold a prompt and a file the
	// agent read, so it is the reader's to read and nobody else's.
	traceFileMode = 0o600
)

var (
	// errAgentLogPlaceholder refuses a path this viewer cannot expand — a typo
	// must be a sentence, never a filename with braces in it.
	errAgentLogPlaceholder = errors.New("the declared agent.log path carries an unknown placeholder: only {timestamp} and {session} are understood")
	// errAgentLogOutside refuses a declaration that leaves the project, the same
	// confinement the file callbacks and the temp-file rule carry.
	errAgentLogOutside = errors.New("the declared agent.log path must stay inside the project root")
)

// traceTemplate is the project's declared wire-log path, validated once when the
// viewer learns it and expanded once per run.
type traceTemplate struct {
	root string
	raw  string
}

// newTraceTemplate validates a declaration: the placeholders must be known and
// the path — checked with a probe run — must stay inside the root, both lexically
// and through whatever symlinks already exist. A blank declaration returns a nil
// template: the capture is off and nothing is opened.
func newTraceTemplate(root, raw string) (*traceTemplate, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	t := &traceTemplate{root: root, raw: trimmed}
	if _, err := t.expand(acpbridge.RunInfo{Started: time.Unix(0, 0), Session: "probe"}); err != nil {
		return nil, err
	}
	return t, nil
}

// hasPlaceholders reports whether the declaration asks for a file per run or
// per session rather than one appended file.
func (t *traceTemplate) hasPlaceholders() bool { return strings.Contains(t.raw, "{") }

// expand turns the declaration and one run into the path to write. It refuses
// anything that escapes the project root: a `..`, an absolute path outside, or a
// symlink that leads out — a declared path is followed only where it is allowed
// to go.
func (t *traceTemplate) expand(run acpbridge.RunInfo) (string, error) {
	expanded, err := t.substitute(run)
	if err != nil {
		return "", err
	}
	abs := expanded
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(t.root, abs)
	}
	abs = filepath.Clean(abs)
	if !withinRoot(t.root, abs) {
		return "", fmt.Errorf("%w: %s", errAgentLogOutside, expanded)
	}
	if err := checkTraceSymlinks(t.root, abs); err != nil {
		return "", err
	}
	return abs, nil
}

// substitute replaces the declared placeholders: {timestamp} is the run's start,
// {session} the ACP session id — sanitised, because the id is the agent's to
// choose and a slash in it must not become a directory.
func (t *traceTemplate) substitute(run acpbridge.RunInfo) (string, error) {
	var b strings.Builder
	rest := t.raw
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		b.WriteString(rest[:open])
		end := strings.IndexByte(rest[open:], '}')
		if end < 0 {
			return "", fmt.Errorf("%w: got %q", errAgentLogPlaceholder, rest[open:])
		}
		token := rest[open+1 : open+end]
		switch token {
		case "timestamp":
			b.WriteString(run.Started.UTC().Format(traceTimestampLayout))
		case "session":
			b.WriteString(sanitiseSession(run.Session))
		default:
			return "", fmt.Errorf("%w: got {%s}", errAgentLogPlaceholder, token)
		}
		rest = rest[open+end+1:]
	}
}

// sanitiseSession keeps an agent-chosen session id inside one path element.
func sanitiseSession(session string) string {
	var b strings.Builder
	for _, r := range session {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return traceNoSessionName
	}
	return b.String()
}

// withinRoot reports whether path is root itself or below it. It is lexical:
// the symlink half is checkTraceSymlinks's.
func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// checkTraceSymlinks resolves the deepest directory of the declared path that
// exists and refuses one that resolves outside the root: a symlinked directory
// inside the project can still point anywhere, and the declaration is not a way
// out of it.
func checkTraceSymlinks(root, path string) error {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("%w: %v", errAgentLogOutside, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	if !withinRoot(resolvedRoot, resolvedDir) {
		return fmt.Errorf("%w: %s leads out through a symlink", errAgentLogOutside, path)
	}
	return nil
}

// agentTraceSink is the viewer's acpbridge.TraceSink. One declaration, one sink:
// it expands the path per run, opens the file, and keeps the single appended file
// — when the declaration carries no placeholder — open across runs, because that
// file outlives any one run and only the viewer's shutdown closes it.
type agentTraceSink struct {
	tmpl *traceTemplate

	mu     sync.Mutex
	single *os.File
	// issue is the sentence a failure earned, said once: a path that cannot be
	// written turns the capture off, and the panel is told instead of the reader
	// being left with silence.
	issue string
}

// newAgentTraceSink validates the declaration and returns the sink, or nil when
// nothing is declared. A refused declaration returns the error to be said where
// the reader is: the agent still runs, only the capture is off.
func newAgentTraceSink(root, raw string) (*agentTraceSink, error) {
	tmpl, err := newTraceTemplate(root, raw)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, nil
	}
	return &agentTraceSink{tmpl: tmpl}, nil
}

// Open is called once per run by the bridge. A declared placeholder gives the run
// its own file; without one every run appends to the same file, and the writer
// handed over does not close it.
func (s *agentTraceSink) Open(run acpbridge.RunInfo) (io.WriteCloser, error) {
	path, err := s.tmpl.expand(run)
	if err != nil {
		s.report(err)
		return nil, err
	}
	if s.tmpl.hasPlaceholders() {
		f, err := openTraceFile(path)
		if err != nil {
			s.report(err)
			return nil, err
		}
		return f, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.single == nil {
		f, err := openTraceFile(path)
		if err != nil {
			s.reportLocked(err)
			return nil, err
		}
		s.single = f
	}
	return &sharedTraceWriter{f: s.single}, nil
}

// report records the sentence once and keeps it for the state the panel reads.
func (s *agentTraceSink) report(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reportLocked(err)
}

func (s *agentTraceSink) reportLocked(err error) {
	if err == nil || s.issue != "" {
		return
	}
	s.issue = fmt.Sprintf("the agent wire log is off: %v", err)
}

// issueSentence is the sentence the panel shows, or "" when the capture is fine.
func (s *agentTraceSink) issueSentence() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issue
}

// close releases the single appended file. It runs at shutdown, after the bridge
// has closed, so the close marker is already in the file.
func (s *agentTraceSink) close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.single == nil {
		return nil
	}
	err := s.single.Close()
	s.single = nil
	return err
}

// sharedTraceWriter is the single appended file seen as one run's writer: Close
// is a no-op on purpose, because the file belongs to the sink and outlives the
// run that wrote into it.
type sharedTraceWriter struct{ f *os.File }

func (w *sharedTraceWriter) Write(p []byte) (int, error) { return w.f.Write(p) }
func (w *sharedTraceWriter) Close() error                { return nil }

// closeAgentTrace releases the wire log at shutdown, after the bridge has closed.
// A viewer with no declared path, or a refused one, has no file to close.
func (s *server) closeAgentTrace() error {
	if s.agentTrace == nil {
		return nil
	}
	return s.agentTrace.close()
}

// openTraceFile opens the declared path for appending, creating the directories
// the project named inside its own tree: a declaration whose directory does not
// exist yet is a reader asking for a file there, not a mistake to report.
func openTraceFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	//#nosec G304 -- the path comes from the project's own declaration and is
	// confined to its root by expand before it gets here.
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, traceFileMode)
}
