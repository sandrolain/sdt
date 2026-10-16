package main

// The agent bridge's browser surface: one inert bridge owned by the server, the
// state a browser may read before it does anything, the activation the user
// performs, a stream of whole-thread states, and the endpoint its commands go to.
//
// Three rules hold the trust story together, and each is a check rather than a
// convention. Nothing is dialled before the user activates: the socket path is
// typed into the UI, never defaulted to and never remembered. The activation POST,
// the command POST and the stream request must each carry the viewer's own Origin,
// so a page the user happens to visit cannot drive the bridge. And decision 0027's
// posture applies here too — a non-loopback bind disables the surface entirely,
// because an agent bridge on a remote interface is a remote-control surface,
// whatever the origin gate says.
//
// The transport is HTTP + SSE rather than a WebSocket (user decision 2026-10-15):
// the traffic is asymmetric — a few discrete commands up, a continuous stream down —
// so a command is a POST of one envelope and the stream is a GET that never ends.
// Consequence, and why commands carry a session id: the POST is a different request
// from the stream, so the browser tells the viewer which conversation it is talking
// to; without one, the most recent stream is assumed, which is the single-user case.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

const (
	agentStatePath      = "/api/agent/state"
	agentActivatePath   = "/api/agent/activate"
	agentDeactivatePath = "/api/agent/deactivate"
	agentCommandPath    = "/api/agent/command"
	agentStreamPath     = "/api/agent/stream"

	// agentWriteWait bounds one write to a browser, agentReadLimit one command from
	// it, and agentHeartbeat keeps a stream that has nothing to say from looking dead.
	agentWriteWait = 10 * time.Second
	agentReadLimit = 1 << 20
	agentHeartbeat = 25 * time.Second

	// agentBodyLimit bounds a command body, which carries one envelope.
	agentBodyLimit = 4096
)

// Refusal bodies. Each one is what the user reads, so it says what to do rather
// than what failed internally.
const (
	errAgentOrigin      = "the request must come from the viewer's own origin"
	errAgentDisabled    = "the agent bridge is disabled on a non-loopback bind host"
	errAgentNotActive   = "the agent bridge is not activated"
	errAgentNoStream    = "no browser is streaming the agent, so there is no session to command"
	errAgentMethod      = "this endpoint takes a POST"
	errAgentBody        = `the body must be {"socket":"<absolute unix socket path>"}`
	errAgentCommand     = "the body must be one command envelope"
	errAgentPath        = "the socket path must be an absolute unix socket path the agent listens on"
	errAgentUnreachable = "no agent is listening on that socket"
	errAgentVersion     = "the agent speaks an ACP version this viewer does not"
	errAgentHandshake   = "the agent did not complete the ACP handshake"
)

// agentStream is one open SSE response: which conversation it carries and the cancel
// that ends it. It is a pointer type because a cancel function cannot be a map key.
type agentStream struct {
	sessionID string
	cancel    context.CancelFunc
}

// agentActivateRequest is the activation body: the socket path the user typed.
type agentActivateRequest struct {
	Socket string `json:"socket"`
}

// agentStateResponse is everything a browser may read before it activates.
type agentStateResponse struct {
	Activated bool   `json:"activated"`
	Connected bool   `json:"connected"`
	Session   string `json:"session,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleAgentState reports the bridge's posture: whether it is activated, which
// session a browser would join, and why it cannot be used when it is disabled. It
// is the only agent endpoint reachable before activation.
func (s *server) handleAgentState(w http.ResponseWriter, _ *http.Request) {
	state := agentStateResponse{Activated: s.agent.Activated(), Error: s.agentReason}
	if session := s.agent.SessionID(); session != "" {
		state.Session, state.Connected = session, true
	}
	writeJSON(w, http.StatusOK, state)
}

// handleAgentActivate dials the socket the user named. The typed refusal travels to
// the user, because "no agent is listening" and "that is not a socket" are the two
// things the reader has to tell apart.
func (s *server) handleAgentActivate(w http.ResponseWriter, r *http.Request) {
	if s.agentUnavailable(w, r) {
		return
	}
	var request agentActivateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, agentBodyLimit)).Decode(&request); err != nil {
		writeErr(w, http.StatusBadRequest, errAgentBody)
		return
	}
	session, err := s.agent.Activate(r.Context(), request.Socket)
	if err != nil {
		slog.Warn("sdtviewer: agent activation refused", "err", err)
		writeErr(w, http.StatusBadGateway, activationMessage(err))
		return
	}
	slog.Info("sdtviewer: agent bridge activated", "session", session)
	writeJSON(w, http.StatusOK, agentStateResponse{Activated: true, Connected: true, Session: session})
}

// handleAgentDeactivate returns the server to inert and ends the open streams: a
// stream left open would show a thread nothing can feed any more.
func (s *server) handleAgentDeactivate(w http.ResponseWriter, r *http.Request) {
	if s.agentUnavailable(w, r) {
		return
	}
	if err := s.agent.Close(); err != nil {
		slog.Warn("sdtviewer: agent close", "err", err)
	}
	s.stopAgentStreams()
	writeJSON(w, http.StatusOK, agentStateResponse{})
}

// handleAgentStream is the Server-Sent Events response: one ACP session for this
// browser, the current thread as the first frame, and one frame per state change
// until the client goes away — at which point the session closes and every wait it
// held is released.
func (s *server) handleAgentStream(w http.ResponseWriter, r *http.Request) {
	switch {
	case s.agentReason != "":
		writeErr(w, http.StatusForbidden, s.agentReason)
		return
	case !agentOrigin(r):
		writeErr(w, http.StatusForbidden, errAgentOrigin)
		return
	case !s.agent.Activated():
		writeErr(w, http.StatusForbidden, errAgentNotActive)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	session, err := s.agent.NewSession(ctx)
	if err != nil {
		slog.Warn("sdtviewer: agent session", "err", err)
		writeErr(w, http.StatusBadGateway, errAgentHandshake)
		return
	}
	defer session.Close()
	stream := &agentStream{sessionID: session.ID(), cancel: cancel}
	s.startAgentStream(stream)
	defer s.endAgentStream(stream)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	frames := make(chan []byte, 1)
	session.State().Listen(func(thread acpbridge.Thread) {
		if payload, err := acpbridge.EncodeState(session.ID(), thread); err == nil {
			pushLatest(frames, payload)
		}
	})
	// The state as it stands, so a browser joining a live session paints at once.
	if payload, err := acpbridge.EncodeState(session.ID(), session.State().Snapshot()); err == nil {
		pushLatest(frames, payload)
	}
	if !writeAgentFrame(w, flusher, <-frames) {
		return
	}

	heartbeat := time.NewTicker(agentHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-frames:
			if !writeAgentFrame(w, flusher, payload) {
				return
			}
		case <-heartbeat.C:
			// A comment keeps a quiet stream from looking dead to a proxy, and costs
			// the browser nothing: SSE ignores a line that is not a field.
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleAgentCommand applies one browser command to the session it names, or to the
// session the most recent stream opened.
func (s *server) handleAgentCommand(w http.ResponseWriter, r *http.Request) {
	if s.agentUnavailable(w, r) {
		return
	}
	if !s.agent.Activated() {
		writeErr(w, http.StatusForbidden, errAgentNotActive)
		return
	}
	body, err := limitedBody(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, errAgentCommand)
		return
	}
	command, err := acpbridge.DecodeCommand(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, errAgentCommand)
		return
	}
	session, ok := s.commandSession(command.SessionID)
	if !ok {
		writeErr(w, http.StatusConflict, errAgentNoStream)
		return
	}
	switch command.Kind {
	case acpbridge.EnvelopePrompt:
		// The turn blocks, and the response must not wait for it: the browser learns
		// what happened from the stream it is reading. It runs under the session's own
		// life rather than the request's, which ends as soon as this response is
		// written, and under the session's life rather than nothing at all, so the
		// agent's work dies with the browser that asked for it.
		go func(turn context.Context) {
			if err := session.Prompt(turn, command.Text); err != nil {
				slog.Warn("sdtviewer: agent turn failed", "err", err)
			}
		}(session.Context())
	case acpbridge.EnvelopeCancel:
		if err := session.Cancel(r.Context()); err != nil {
			slog.Warn("sdtviewer: agent cancel failed", "err", err)
		}
	case acpbridge.EnvelopePermission:
		// The card is resolved first, so the browser sees the decision in place, and
		// the answer is handed to the agent second, which releases its wait.
		session.State().ResolvePermission(command.RequestID, agentDecision(command.Allow))
		s.agent.Permissions().Answer(command.RequestID, command.Allow)
	default:
		writeErr(w, http.StatusBadRequest, errAgentCommand)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// commandSession resolves the conversation a command belongs to: the one it names,
// or the one the most recent stream opened. The second is the single-user default,
// and the first is what makes two browser tabs tell each other apart.
func (s *server) commandSession(named string) (*acpbridge.Session, bool) {
	if named != "" {
		return s.agent.Session(named)
	}
	s.agentMu.Lock()
	current := s.agentCurrent
	s.agentMu.Unlock()
	if current == "" {
		// No stream, no conversation a browser is reading: the session activation
		// opened is where the handshake was proven, and commanding it would send the
		// user's words to a thread nobody sees.
		return nil, false
	}
	return s.agent.Session(current)
}

// startAgentStream records a stream as the current conversation, so a command that
// names no session reaches the one this browser is reading.
func (s *server) startAgentStream(stream *agentStream) {
	s.agentMu.Lock()
	defer s.agentMu.Unlock()
	if s.agentStreams == nil {
		s.agentStreams = map[*agentStream]struct{}{}
	}
	s.agentStreams[stream] = struct{}{}
	s.agentCurrent = stream.sessionID
}

// endAgentStream forgets a stream that ended on its own, and stops calling its
// conversation current when it was the one being commanded.
func (s *server) endAgentStream(stream *agentStream) {
	s.agentMu.Lock()
	defer s.agentMu.Unlock()
	delete(s.agentStreams, stream)
	if s.agentCurrent == stream.sessionID {
		s.agentCurrent = ""
	}
}

// stopAgentStreams ends every open stream; each one closes its session on the way
// out, which is what releases the agent's pending waits.
func (s *server) stopAgentStreams() {
	s.agentMu.Lock()
	streams := make([]*agentStream, 0, len(s.agentStreams))
	for stream := range s.agentStreams {
		streams = append(streams, stream)
	}
	s.agentStreams = map[*agentStream]struct{}{}
	s.agentCurrent = ""
	s.agentMu.Unlock()
	for _, stream := range streams {
		stream.cancel()
	}
}

// agentUnavailable applies the gates every command endpoint shares: a disabled
// surface, a method other than POST, and an origin that is not the viewer's own.
// It reports whether a response was already written.
func (s *server) agentUnavailable(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case s.agentReason != "":
		writeErr(w, http.StatusForbidden, s.agentReason)
		return true
	case r.Method != http.MethodPost:
		writeErr(w, http.StatusMethodNotAllowed, errAgentMethod)
		return true
	case !agentOrigin(r):
		writeErr(w, http.StatusForbidden, errAgentOrigin)
		return true
	}
	return false
}

// activationMessage maps the bridge's typed refusals onto what the user can act on.
func activationMessage(err error) string {
	switch {
	case errors.Is(err, acpbridge.ErrDialTarget):
		return errAgentPath
	case errors.Is(err, acpbridge.ErrUnreachable):
		return errAgentUnreachable
	case errors.Is(err, acpbridge.ErrProtocolVersion):
		return errAgentVersion
	default:
		return errAgentHandshake
	}
}

func agentDecision(allow bool) string {
	if allow {
		return acpbridge.DecisionAllowed
	}
	return acpbridge.DecisionDenied
}

// writeAgentFrame writes one SSE frame and reports whether the browser is still
// there; a stream that has gone away ends the response rather than the process.
func writeAgentFrame(w http.ResponseWriter, flusher http.Flusher, payload []byte) bool {
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// pushLatest hands the newest state to the writer, replacing one it has not taken
// yet: the payload is the whole thread, so the newer state makes the older
// redundant rather than lost.
func pushLatest(out chan []byte, payload []byte) {
	select {
	case out <- payload:
		return
	default:
	}
	select {
	case <-out:
	default:
	}
	select {
	case out <- payload:
	default:
	}
}

// limitedBody reads a bounded request body.
func limitedBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var envelope json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, agentBodyLimit)).Decode(&envelope); err != nil {
		return nil, err
	}
	return envelope, nil
}

// agentOrigin is the agent endpoints' origin rule, and it is deliberately stricter
// than `sameOrigin` (the document-state endpoint's): that one allows a request with
// no Origin at all, because a local non-browser client is the legitimate caller
// there, while the bridge's only intended caller is the panel in the user's own
// page. So a missing Origin is refused here as firmly as a foreign one, and a
// cross-site fetch marker is refused even when the header agrees.
func agentOrigin(r *http.Request) bool {
	site := r.Header.Get("Sec-Fetch-Site")
	if site == "cross-site" || site == "same-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// A browser may omit Origin on a same-origin request, and `Sec-Fetch-Site` is
		// the one header a page cannot set from script: `same-origin` is proof the
		// request came from this viewer's own page, and any other site would have sent
		// `cross-site` or a foreign Origin.
		return site == "same-origin"
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host == r.Host
}

// enableAgent turns the surface on for a loopback bind and records why it is off
// otherwise, mirroring decision 0027's rule for document editing. The argv the
// project declared is read here and logged once, so a reader can see what a start
// will run before asking for one; whether it resolves is the start's business,
// because that is where a reader asked.
func (s *server) enableAgent(host string) {
	if !isLoopbackHost(host) {
		s.agentReason = errAgentDisabled
		slog.Info("sdtviewer: agent bridge disabled", "reason", "non-loopback bind host", "host", host)
		return
	}
	s.agent = acpbridge.New(acpbridge.Options{Root: s.root, Version: version})
	s.agentCommand = loadViewerConfig(s.root).agentCommand()
	if len(s.agentCommand) == 0 {
		slog.Info("sdtviewer: agent bridge available", "root", s.root, "agent", "none configured")
		return
	}
	slog.Info("sdtviewer: agent bridge available", "root", s.root, "agent", strings.Join(s.agentCommand, " "))
}
