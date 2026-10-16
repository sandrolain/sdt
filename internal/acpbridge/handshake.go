package acpbridge

import (
	"context"
	"fmt"
	"time"

	"github.com/coder/acp-go-sdk"
)

// handshakeTimeout bounds initialize + session/new, so a stalled or confused agent
// becomes a named failure instead of a spinner.
const handshakeTimeout = 20 * time.Second

// handshake initializes the connection, gates on the protocol version, and returns
// the response. Opening a session is NewSession's job, so the handshake stays the
// negotiation and nothing else — but the negotiation's answer is worth returning:
// the agent's capabilities are read from it, and session/load is the one the bridge
// acts on, because a restart can reattach a session only when the agent says it can
// carry one.
//
// The version gate is the one place a future protocol cannot pass: ACP v2 is
// already drafted upstream, and a bridge that accepted a version it does not
// speak would mis-render a session instead of refusing it — so a mismatch is
// ErrProtocolVersion and no session is opened.
func handshake(ctx context.Context, conn *acp.ClientSideConnection, version string) (acp.InitializeResponse, error) {
	res, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientInfo: &acp.Implementation{
			Name:    ClientName,
			Title:   acp.Ptr("SDT viewer"),
			Version: version,
		},
		ClientCapabilities: acp.ClientCapabilities{
			// Both file operations are advertised because the policy serves them
			// — confined to the project root. The terminal is not advertised
			// because every terminal method refuses, so an agent has no reason to
			// offer one.
			Fs:       acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
			Terminal: false,
		},
	})
	if err != nil {
		return acp.InitializeResponse{}, fmt.Errorf("%w: initialize: %v", ErrHandshake, err)
	}
	if res.ProtocolVersion != acp.ProtocolVersionNumber {
		return res, fmt.Errorf("%w: the agent speaks %d, this bridge speaks %d",
			ErrProtocolVersion, res.ProtocolVersion, acp.ProtocolVersionNumber)
	}
	return res, nil
}

// loadSession asks the agent to take a conversation back. The protocol gates it on the
// agent's loadSession capability — the caller has already read that from the handshake —
// and a refusal is an error the caller degrades from rather than a failure of the
// bridge: an agent that no longer holds the session is a normal thing to meet.
//
// The answer's config options come back with it: an agent that takes a session back
// answers what it now offers for it, and an agent that says nothing leaves the caller
// with what the thread already holds.
func loadSession(ctx context.Context, conn *acp.ClientSideConnection, sessionID, root string) ([]acp.SessionConfigOption, error) {
	loaded, err := conn.LoadSession(ctx, acp.LoadSessionRequest{
		SessionId:  acp.SessionId(sessionID),
		Cwd:        root,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: session/load: %v", ErrHandshake, err)
	}
	return loaded.ConfigOptions, nil
}

// openSession opens one ACP session. Its workspace is the project root — the same
// directory the file callbacks are confined to, so the agent's own relative paths
// and the confinement agree about where "here" is.
//
// The answer's config options are returned beside the id because this is the only
// place they are ever announced: an agent advertises what it offers for a session —
// the model it will use, its mode — in the answer to session/new, and a real agent
// sends no update for it afterwards. The caller folds them into the thread.
func openSession(ctx context.Context, conn *acp.ClientSideConnection, root string) (string, []acp.SessionConfigOption, error) {
	session, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        root,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return "", nil, fmt.Errorf("%w: session/new: %v", ErrHandshake, err)
	}
	if session.SessionId == "" {
		return "", nil, fmt.Errorf("%w: session/new returned an empty session id", ErrHandshake)
	}
	return string(session.SessionId), session.ConfigOptions, nil
}
