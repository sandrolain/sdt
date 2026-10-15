package main

// The agent bridge's browser surface: one inert bridge owned by the server, the
// state a browser may read before it does anything, the activation the user
// performs, and the WebSocket that carries one ACP session to the browser.
//
// Three rules hold the trust story together, and each is a check rather than a
// convention. Nothing is dialled before the user activates: the socket path is
// typed into the UI, never defaulted to and never remembered. Both the activation
// POST and the upgrade must carry the viewer's own Origin, so a page the user
// happens to visit cannot drive the bridge. And decision 0027's posture applies
// here too — a non-loopback bind disables the surface entirely, because an agent
// bridge on a remote interface is a remote-control surface, whatever the origin
// gate says.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

const (
	agentStatePath      = "/api/agent/state"
	agentActivatePath   = "/api/agent/activate"
	agentDeactivatePath = "/api/agent/deactivate"
	agentWSPath         = "/api/agent/ws"

	// agentWriteWait bounds one write to a browser; agentReadLimit bounds one
	// command from it.
	agentWriteWait = 10 * time.Second
	agentReadLimit = 1 << 20

	// agentBodyLimit bounds the activation body, which carries one path.
	agentBodyLimit = 4096
)

// Refusal bodies. Each one is what the user reads, so it says what to do rather
// than what failed internally.
const (
	errAgentOrigin      = "the request must come from the viewer's own origin"
	errAgentDisabled    = "the agent bridge is disabled on a non-loopback bind host"
	errAgentNotActive   = "the agent bridge is not activated"
	errAgentMethod      = "this endpoint takes a POST"
	errAgentBody        = `the body must be {"socket":"<absolute unix socket path>"}`
	errAgentPath        = "the socket path must be an absolute unix socket path the agent listens on"
	errAgentUnreachable = "no agent is listening on that socket"
	errAgentVersion     = "the agent speaks an ACP version this viewer does not"
	errAgentHandshake   = "the agent did not complete the ACP handshake"
)

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

// handleAgentDeactivate returns the server to inert and closes the browser
// connections: a socket left open would show a thread nothing can feed any more.
func (s *server) handleAgentDeactivate(w http.ResponseWriter, r *http.Request) {
	if s.agentUnavailable(w, r) {
		return
	}
	if err := s.agent.Close(); err != nil {
		slog.Warn("sdtviewer: agent close", "err", err)
	}
	s.closeAgentConnections()
	writeJSON(w, http.StatusOK, agentStateResponse{})
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

// handleAgentWS upgrades one browser connection to the ACP session it will own.
//
// The upgrade is gated like the POST: same origin, and only after the user has
// activated. From there the connection carries commands up and whole-thread state
// down; the state is the whole thread every time, so a browser never reassembles
// chunks and a slower writer loses nothing but intermediate pictures.
func (s *server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
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

	// The origin check is ours (above), so the library's own is left at its default.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Warn("sdtviewer: agent upgrade refused", "err", err)
		return
	}
	defer func() {
		if err := conn.CloseNow(); err != nil {
			slog.Debug("sdtviewer: agent connection close", "err", err)
		}
	}()
	conn.SetReadLimit(agentReadLimit)

	// The request's own context is not usable after a hijack, so the connection
	// gets its own, cancelled when this handler returns.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := s.agent.NewSession(ctx)
	if err != nil {
		slog.Warn("sdtviewer: agent session", "err", err)
		if err := conn.Close(websocket.StatusInternalError, "the agent did not open a session"); err != nil {
			slog.Debug("sdtviewer: agent connection close", "err", err)
		}
		return
	}
	defer func() { session.Close() }()
	s.trackAgentConnection(conn)

	out := make(chan []byte, 1)
	session.State().Listen(func(thread acpbridge.Thread) {
		if payload, err := acpbridge.EncodeState(session.ID(), thread); err == nil {
			pushLatest(out, payload)
		}
	})
	// The state as it stands, so a browser joining a live session paints at once.
	if payload, err := acpbridge.EncodeState(session.ID(), session.State().Snapshot()); err == nil {
		pushLatest(out, payload)
	}

	// One writer: the updates arrive from the agent's own goroutines, and the
	// WebSocket library is written for one writer at a time.
	go s.writeAgentStates(ctx, conn, out)

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		command, err := acpbridge.DecodeCommand(data)
		if err != nil {
			// A browser that sends something else is refused one message at a time:
			// the socket carries a closed vocabulary, and a broken one is not fatal.
			slog.Debug("sdtviewer: agent command refused", "err", err)
			continue
		}
		if err := s.runAgentCommand(ctx, conn, session, command); err != nil {
			slog.Warn("sdtviewer: agent command failed", "err", err)
			if closeErr := conn.Close(websocket.StatusInternalError, "the agent failed the turn"); closeErr != nil {
				slog.Debug("sdtviewer: agent connection close", "err", closeErr)
			}
			return
		}
	}
}

// writeAgentStates is the connection's single writer.
func (s *server) writeAgentStates(ctx context.Context, conn *websocket.Conn, out <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-out:
			writeCtx, cancel := context.WithTimeout(ctx, agentWriteWait)
			err := conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// runAgentCommand applies one browser message. A prompt runs in its own goroutine,
// because Prompt blocks for the whole turn and the browser must still be able to
// cancel it.
func (s *server) runAgentCommand(ctx context.Context, conn *websocket.Conn, session *acpbridge.Session, command acpbridge.Command) error {
	switch command.Kind {
	case acpbridge.EnvelopePrompt:
		go func() {
			err := session.Prompt(ctx, command.Text)
			switch {
			case err == nil:
				return
			case errors.Is(err, acpbridge.ErrTurnRunning):
				// A prompt arriving while a turn runs is a click too many, not a
				// broken conversation: the thread already shows the turn in flight.
				slog.Debug("sdtviewer: prompt refused, a turn is running")
				return
			default:
				slog.Warn("sdtviewer: agent turn failed", "err", err)
				if closeErr := conn.Close(websocket.StatusInternalError, "the agent failed the turn"); closeErr != nil {
					slog.Debug("sdtviewer: agent connection close", "err", closeErr)
				}
			}
		}()
		return nil
	case acpbridge.EnvelopeCancel:
		return session.Cancel(ctx)
	case acpbridge.EnvelopePermission:
		// The card is resolved first, so the browser sees the decision in place, and
		// the answer is handed to the agent second, which releases its wait.
		session.State().ResolvePermission(command.RequestID, agentDecision(command.Allow))
		s.agent.Permissions().Answer(command.RequestID, command.Allow)
		return nil
	default:
		return acpbridge.ErrCommand
	}
}

func agentDecision(allow bool) string {
	if allow {
		return acpbridge.DecisionAllowed
	}
	return acpbridge.DecisionDenied
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

// agentOrigin is the agent endpoints' origin rule, and it is deliberately stricter
// than `sameOrigin` (the document-state endpoint's): that one allows a request with
// no Origin at all, because a local non-browser client is the legitimate caller
// there, while the bridge's only intended caller is the panel in the user's own
// page. So a missing Origin is refused here as firmly as a foreign one, and a
// cross-site fetch marker is refused even when the header agrees.
func agentOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
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
// otherwise, mirroring decision 0027's rule for document editing.
func (s *server) enableAgent(host string) {
	if !isLoopbackHost(host) {
		s.agentReason = errAgentDisabled
		slog.Info("sdtviewer: agent bridge disabled", "reason", "non-loopback bind host", "host", host)
		return
	}
	s.agent = acpbridge.New(acpbridge.Options{Root: s.root, Version: version})
	slog.Info("sdtviewer: agent bridge available", "root", s.root)
}

// trackAgentConnection records an open browser connection so deactivate can close
// it, and closeAgentConnections does that.
func (s *server) trackAgentConnection(conn *websocket.Conn) {
	s.agentMu.Lock()
	defer s.agentMu.Unlock()
	if s.agentConns == nil {
		s.agentConns = map[*websocket.Conn]struct{}{}
	}
	s.agentConns[conn] = struct{}{}
}

func (s *server) closeAgentConnections() {
	s.agentMu.Lock()
	conns := make([]*websocket.Conn, 0, len(s.agentConns))
	for conn := range s.agentConns {
		conns = append(conns, conn)
	}
	s.agentConns = map[*websocket.Conn]struct{}{}
	s.agentMu.Unlock()
	for _, conn := range conns {
		if err := conn.CloseNow(); err != nil {
			slog.Debug("sdtviewer: agent connection close", "err", err)
		}
	}
}
