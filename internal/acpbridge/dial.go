package acpbridge

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// dialTimeout bounds the connect to the user's socket. A socket that does not
// answer becomes "no agent reachable"; it never becomes a hang.
const dialTimeout = 5 * time.Second

// maxSocketPath bounds the path the policy accepts. A unix socket address is
// limited by the kernel (104 bytes on Darwin, 108 on Linux) and an over-long path
// fails at bind time with an error that names nothing useful, so the policy
// refuses it where the message can say why.
const maxSocketPath = 100

// dialSocket is the one place a connection is created: it enforces the dial
// policy and then connects.
func dialSocket(ctx context.Context, path string) (net.Conn, error) {
	if err := checkDialTarget(path); err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrUnreachable, path, err)
	}
	return conn, nil
}

// checkDialTarget refuses everything the dial policy does not name: an empty
// path, a relative path, a URL, an over-long path, a path that does not exist,
// and a path that exists but is not a socket.
//
// The refusals are the security boundary rather than a convenience, so each one
// is decided here and asserted by a test: a host and port would be a remote
// endpoint, a relative path would make the target depend on the process's working
// directory, and a default would let the viewer reach an agent the user never
// named.
func checkDialTarget(path string) error {
	switch {
	case path == "":
		return fmt.Errorf("%w: the path is empty", ErrDialTarget)
	case !filepath.IsAbs(path):
		return fmt.Errorf("%w: %q is not absolute", ErrDialTarget, path)
	case strings.Contains(path, "://"):
		return fmt.Errorf("%w: %q is a URL", ErrDialTarget, path)
	case len(path) > maxSocketPath:
		return fmt.Errorf("%w: %q is longer than %d bytes", ErrDialTarget, path, maxSocketPath)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDialTarget, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%w: %q exists but is not a socket", ErrDialTarget, path)
	}
	return nil
}
