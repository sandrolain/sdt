package main

// Document-state actions offered by the viewer. Decision 0027: the viewer has
// no write surface of its own. It never touches document bytes; it runs the
// sanctioned CLI mutator (decisions 0014, 0020) as a subprocess rooted at the
// served project, so the guardrails — registry validation, derived-state
// refusal, cascade, upward-only — apply verbatim instead of being
// reimplemented here.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// contextCommandPath is the single write route (decision 0027, point 3).
	contextCommandPath = "/api/context/command"
	// commandTimeout bounds one mutator run; the CLI is a local subprocess.
	commandTimeout = 30 * time.Second
	// probeTimeout bounds the liveness probe of one candidate CLI. `sdt version`
	// answers in a few milliseconds; the budget only has to outlast a cold start.
	probeTimeout = 5 * time.Second
	// sdtBinaryName is the CLI looked up on PATH and beside this binary.
	sdtBinaryName = "sdt"
	// noCLIReason is the capability sentence when no candidate ran; a
	// resolution that failed for a reason carries it instead.
	noCLIReason = "no sdt CLI found for this root"
)

// contextCommandRequest is the closed request shape: a document, an allowlisted
// action and the value that action needs. It is never free-form argv.
type contextCommandRequest struct {
	Doc    string `json:"doc"`
	Action string `json:"action"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// contextCommandResponse reports what the mutator did.
type contextCommandResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
}

// contextCommandCapability is the GET answer: whether an action can run, and
// why not. The SPA reads it to disable the control with a reason rather than
// offering a button that always fails (decision 0027, point 5).
type contextCommandCapability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// contextActions maps an allowlisted action to the CLI verb that performs it.
// An action absent from the map is refused; the client cannot reach argv.
var contextActions = map[string]func(contextCommandRequest) ([]string, error){
	"close": func(r contextCommandRequest) ([]string, error) {
		args := []string{"context", "close", r.Doc}
		if r.Reason != "" {
			args = append(args, "--allow-unfinished", "--reason", r.Reason)
		}
		return args, nil
	},
	"status-set": func(r contextCommandRequest) ([]string, error) {
		if r.Value == "" {
			return nil, errors.New("status-set requires a value")
		}
		return []string{"context", "status", "set", r.Doc, "--status", r.Value}, nil
	},
}

// delegator runs one sanctioned mutator for the served root. A nil *delegator
// means the viewer has no write surface: either the bind host is not loopback
// or no CLI resolved. There is no fallback to writing the file directly
// (decision 0027, point 5).
type delegator struct {
	bin  string
	root string
	// run executes the mutator; injectable for tests.
	run func(ctx context.Context, bin, dir string, args ...string) (string, error)
}

// newDelegator resolves the CLI (explicit path, then PATH, then a sibling of
// the running binary) and returns nil with the reason no CLI resolved.
func newDelegator(root, explicit string, self string) (*delegator, string) {
	bin, reason := resolveSdtBinary(explicit, self)
	if bin == "" {
		return nil, reason
	}
	return &delegator{bin: bin, root: root, run: runMutator}, ""
}

// resolveSdtBinary returns the first candidate CLI that actually runs, as an
// absolute path, and the reason nothing resolved.
//
// A candidate must be executable *and* answer `sdt version`. The probe is what
// makes this resolve the CLI instead of the first file with an execute bit: an
// asdf/mise shim is executable, but with no version for the current project it
// exits 126 with a sentence about tool versions (the shim is regenerated when
// `sdt` is installed under a Go version). Without the probe such a shim wins the
// lookup and every document action fails with that sentence, even though a
// working CLI sits further down PATH or beside this binary.
//
// An explicit path that cannot run disables editing rather than falling through
// to another CLI: the user named that binary (decision 0027, point 5). PATH and
// the sibling of this binary are already absolute.
func resolveSdtBinary(explicit, self string) (bin, reason string) {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Sprintf("--sdt-bin %q is not a usable path: %v", explicit, err)
		}
		if !isExecutableFile(abs) {
			return "", fmt.Sprintf("--sdt-bin %s is not an executable file", abs)
		}
		if err := probeSdtBinary(abs); err != nil {
			return "", fmt.Sprintf("--sdt-bin %s cannot run: %v", abs, err)
		}
		return abs, ""
	}
	var misses []string
	if p, err := exec.LookPath(sdtBinaryName); err == nil {
		perr := probeSdtBinary(p)
		if perr == nil {
			return p, ""
		}
		misses = append(misses, fmt.Sprintf("%s cannot run: %v", p, perr))
	}
	if self != "" {
		candidate := filepath.Join(filepath.Dir(self), sdtBinaryName)
		if isExecutableFile(candidate) {
			perr := probeSdtBinary(candidate)
			if perr == nil {
				return candidate, ""
			}
			misses = append(misses, fmt.Sprintf("%s cannot run: %v", candidate, perr))
		}
	}
	if len(misses) == 0 {
		return "", noCLIReason
	}
	return "", noCLIReason + ": " + strings.Join(misses, "; ")
}

// probeSdtBinary reports whether path runs as the sanctioned CLI. It asks for
// `version`, the cheapest verb that touches nothing, and keeps the CLI's own
// first line of output as the reason it could not.
func probeSdtBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	//#nosec G204 -- path is a candidate CLI resolved from PATH or the viewer's
	// own directory, and the argv is the fixed {"version"}.
	out, err := exec.CommandContext(ctx, path, "version").CombinedOutput()
	if err == nil {
		return nil
	}
	if line := firstLine(string(out)); line != "" {
		return errors.New(line)
	}
	return err
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// isExecutableFile reports whether path is a regular file with any execute bit.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	//#nosec G302 -- the execute bit is what makes it runnable; a Windows build
	// has none and simply runs whatever is there.
	return info.Mode().Perm()&0o111 != 0
}

// runMutator executes the CLI and returns its combined output.
func runMutator(ctx context.Context, bin, dir string, args ...string) (string, error) {
	//#nosec G204,G702 -- bin is resolved from an explicit path/PATH/own directory.
	// The mutator's args come from a closed allowlist; the learning routes'
	// args are a fixed verb plus an id the handler has already matched against
	// learnIDRe, so no request value reaches argv unvalidated.
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// command runs one allowlisted action for doc (a corpus path under
// context/), returning the mutator output or the refusal to surface.
func (d *delegator) command(doc string, req contextCommandRequest) (string, error) {
	build, ok := contextActions[req.Action]
	if !ok {
		return "", fmt.Errorf("unknown action %q", req.Action)
	}
	args, err := build(req)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := d.run(ctx, d.bin, d.root, args...)
	trimmed := strings.TrimSpace(out)
	if err != nil {
		// a refusal speaks for itself on stderr; a failure to run says nothing,
		// so the exec error is the message
		if trimmed == "" {
			return "", fmt.Errorf("cannot run %s: %w", d.bin, err)
		}
		return trimmed, fmt.Errorf("sdt refused: %s", trimmed)
	}
	slog.Info("sdtviewer: context command", "bin", d.bin, "args", args)
	return trimmed, nil
}

// handleContextCommand runs a sanctioned document-state action, or reports
// whether one can run (decision 0027). GET is the capability probe the SPA
// disables its control from; POST executes. It answers 404 for a document
// outside the corpus, 403 for a cross-origin caller, 400 for a malformed or
// non-allowlisted request, 422 when the CLI refuses the mutation, and 200 when
// the document changed. Without a delegator it answers 404 on POST and
// `{available:false}` on GET, so the viewer is read-only and says why.
func (s *server) handleContextCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.capability())
		return
	}
	if s.cmd == nil {
		writeErr(w, http.StatusNotFound, errNotFound)
		return
	}
	if !sameOrigin(r) {
		writeErr(w, http.StatusForbidden, "cross-origin request refused")
		return
	}
	// a JSON content type makes the browser preflight a cross-origin POST, and
	// this server answers no preflight, so the request never leaves the tab.
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeErr(w, http.StatusBadRequest, "expected application/json")
		return
	}
	var req contextCommandRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request body")
		return
	}
	full, ok := s.safePath(req.Doc)
	if !ok {
		writeErr(w, http.StatusNotFound, errNotFound)
		return
	}
	// the CLI resolves the doc from the project root, so it gets the corpus
	// path, not the absolute one; safePath already confined it.
	rel, err := filepath.Rel(s.root, full)
	if err != nil {
		writeErr(w, http.StatusNotFound, errNotFound)
		return
	}
	req.Doc = filepath.ToSlash(rel)
	out, cerr := s.cmd.command(req.Doc, req)
	if cerr != nil {
		writeErr(w, http.StatusUnprocessableEntity, cerr.Error())
		return
	}
	writeJSON(w, http.StatusOK, contextCommandResponse{OK: true, Output: out})
}

// sameOrigin refuses a request that a page in the user's browser could forge
// from another site: a cross-site Sec-Fetch-Site, or an Origin that is present
// and not this server's own.
func sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // a same-origin fetch may omit it; a non-browser client is local
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}

// capability reports whether a document-state action can run, with the reason
// when it cannot.
func (s *server) capability() contextCommandCapability {
	if s.cmd == nil {
		reason := s.writeReason
		if reason == "" {
			reason = noCLIReason
		}
		return contextCommandCapability{Reason: reason}
	}
	return contextCommandCapability{Available: true}
}

// enableWrites turns on the delegation endpoint when the bind host is loopback
// (decision 0027, point 4) and a CLI resolves for the served root. It is called
// by serve before mux registration; without it the viewer is read-only.
func (s *server) enableWrites(host, sdtBin string) {
	if !isLoopbackHost(host) {
		s.writeReason = "document editing needs a loopback bind host"
		slog.Info("sdtviewer: document editing disabled", "reason", "non-loopback bind host", "host", host)
		return
	}
	self, err := os.Executable()
	if err != nil {
		self = ""
	}
	d, toolReason := newDelegator(s.root, sdtBin, self)
	if d != nil {
		s.cmd = d
		slog.Info("sdtviewer: document editing enabled", "sdt", d.bin)
		return
	}
	if toolReason == "" {
		toolReason = noCLIReason
	}
	s.writeReason = toolReason
	slog.Info("sdtviewer: document editing disabled", "reason", toolReason)
}

// isLoopbackHost reports whether a bind host keeps the server on this machine.
func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "localhost", "":
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
