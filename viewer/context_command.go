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
	// sdtBinaryName is the CLI looked up on PATH and beside this binary.
	sdtBinaryName = "sdt"
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
// the running binary) and returns nil when none is found.
func newDelegator(root, explicit string, self string) *delegator {
	bin := resolveSdtBinary(explicit, self)
	if bin == "" {
		return nil
	}
	return &delegator{bin: bin, root: root, run: runMutator}
}

// resolveSdtBinary returns the first usable CLI path as an absolute path, or an
// empty string. An explicit path must exist and is resolved to an absolute one:
// the mutator runs with Dir set to the served root, so a relative name would be
// looked up there rather than next to the viewer. PATH and the sibling of this
// binary are already absolute.
func resolveSdtBinary(explicit, self string) string {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil || !isExecutableFile(abs) {
			return ""
		}
		return abs
	}
	if p, err := exec.LookPath(sdtBinaryName); err == nil {
		return p
	}
	if self != "" {
		candidate := filepath.Join(filepath.Dir(self), sdtBinaryName)
		if isExecutableFile(candidate) {
			return candidate
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
	//#nosec G204 -- bin is resolved from an explicit path/PATH/own directory and
	// args come from a closed allowlist, never from the request body verbatim.
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
			reason = "no sdt CLI found for this root"
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
	if d := newDelegator(s.root, sdtBin, self); d != nil {
		s.cmd = d
		slog.Info("sdtviewer: document editing enabled", "sdt", d.bin)
		return
	}
	s.writeReason = "no sdt CLI found for this root"
	slog.Info("sdtviewer: document editing disabled", "reason", "no sdt CLI found for this root")
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
