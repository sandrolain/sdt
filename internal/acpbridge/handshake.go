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

// handshake initializes the connection and gates on the protocol version. Opening
// a session is NewSession's job, so the handshake stays the negotiation and
// nothing else.
//
// The version gate is the one place a future protocol cannot pass: ACP v2 is
// already drafted upstream, and a bridge that accepted a version it does not
// speak would mis-render a session instead of refusing it — so a mismatch is
// ErrProtocolVersion and no session is opened.
func handshake(ctx context.Context, conn *acp.ClientSideConnection, version string) error {
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
		return fmt.Errorf("%w: initialize: %v", ErrHandshake, err)
	}
	if res.ProtocolVersion != acp.ProtocolVersionNumber {
		return fmt.Errorf("%w: the agent speaks %d, this bridge speaks %d",
			ErrProtocolVersion, res.ProtocolVersion, acp.ProtocolVersionNumber)
	}
	return nil
}

// openSession opens one ACP session. Its workspace is the project root — the same
// directory the file callbacks are confined to, so the agent's own relative paths
// and the confinement agree about where "here" is.
func openSession(ctx context.Context, conn *acp.ClientSideConnection, root string) (string, error) {
	session, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        root,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return "", fmt.Errorf("%w: session/new: %v", ErrHandshake, err)
	}
	if session.SessionId == "" {
		return "", fmt.Errorf("%w: session/new returned an empty session id", ErrHandshake)
	}
	return string(session.SessionId), nil
}
