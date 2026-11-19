//go:build unix

package delegation

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxTaskSubscribers    = 32
	workerOperationCancel = "cancel"
)

type workerSocketOwner struct {
	TaskID   string          `json:"task_id"`
	Identity ProcessIdentity `json:"identity"`
}

type subscribeRequest struct {
	Operation string `json:"operation"`
	TaskID    string `json:"task_id"`
	Token     string `json:"token"`
}

type revisionMessage struct {
	Revision uint64 `json:"revision"`
}

type Notifier struct {
	store    *Store
	taskID   string
	identity ProcessIdentity
	path     string
	owner    string
	listener *net.UnixListener

	mu           sync.Mutex
	clients      map[net.Conn]chan uint64
	closed       chan struct{}
	cancel       chan struct{}
	cancelOnce   sync.Once
	lastRevision uint64
	closeMu      sync.Once
}

type Subscription struct {
	Conn            *net.UnixConn
	InitialRevision uint64
	Revisions       <-chan uint64
	close           func()
}

func (s *Subscription) Close() error {
	if s.close != nil {
		s.close()
	}
	return s.Conn.Close()
}

func (s *Store) Listen(ctx context.Context, taskID string, identity ProcessIdentity) (*Notifier, error) {
	if !safeID.MatchString(taskID) || identity.PID != os.Getpid() || identity.GroupID != identity.PID || identity.StartToken == "" {
		return nil, fmt.Errorf("task id or worker identity is invalid")
	}
	task, err := s.Read(taskID)
	if err != nil {
		return nil, err
	}
	owned := task.Worker != nil && *task.Worker == identity
	if !owned && (len(task.Attempts) == 0 || task.Attempts[len(task.Attempts)-1].ProcessStart != identity.StartToken) {
		return nil, fmt.Errorf("worker identity does not own task %q", taskID)
	}
	dir := filepath.Join(s.root, "w")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path, ownerPath := workerSocketPaths(dir, taskID)
	if err := removeStaleWorkerSocket(path, ownerPath); err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close() //nolint:errcheck
		os.Remove(path)  //nolint:errcheck
		return nil, err
	}
	ownerData, err := json.Marshal(workerSocketOwner{TaskID: taskID, Identity: identity})
	if err != nil {
		listener.Close() //nolint:errcheck
		os.Remove(path)  //nolint:errcheck
		return nil, err
	}
	if err := atomicWrite(ownerPath, append(ownerData, '\n')); err != nil {
		listener.Close() //nolint:errcheck
		os.Remove(path)  //nolint:errcheck
		return nil, err
	}
	n := &Notifier{store: s, taskID: taskID, identity: identity, path: path, owner: ownerPath, listener: listener, clients: map[net.Conn]chan uint64{}, closed: make(chan struct{}), cancel: make(chan struct{}), lastRevision: task.Revision}
	go n.acceptLoop()
	go func() {
		select {
		case <-ctx.Done():
			_ = n.Close() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		case <-n.closed:
		}
	}()
	return n, nil
}

// CancelRequests is closed when an authenticated principal requests cancellation.
func (n *Notifier) CancelRequests() <-chan struct{} { return n.cancel }

func (n *Notifier) requestCancel() { n.cancelOnce.Do(func() { close(n.cancel) }) }

func (n *Notifier) Publish(revision uint64) error {
	task, err := n.store.Read(n.taskID)
	if err != nil {
		return err
	}
	if revision == 0 || revision > task.Revision {
		return fmt.Errorf("notification revision %d is not durably published (current %d)", revision, task.Revision)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if revision <= n.lastRevision {
		return nil
	}
	n.lastRevision = revision
	for _, queue := range n.clients {
		select {
		case queue <- revision:
		default:
			select {
			case <-queue:
			default:
			}
			queue <- revision
		}
	}
	return nil
}

func (n *Notifier) Update(eventType, summary string, mutate func(*Task) error) error {
	if err := n.store.Update(n.taskID, eventType, summary, mutate); err != nil {
		return err
	}
	task, err := n.store.Read(n.taskID)
	if err != nil {
		return err
	}
	return n.Publish(task.Revision)
}

func (n *Notifier) Close() error {
	var closeErr error
	n.closeMu.Do(func() {
		close(n.closed)
		closeErr = n.listener.Close()
		n.mu.Lock()
		for conn := range n.clients {
			_ = conn.Close() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
			delete(n.clients, conn)
		}
		n.mu.Unlock()
		var owner workerSocketOwner
		data, err := os.ReadFile(n.owner) //#nosec G304 -- fixed path derived from safe task id
		if err == nil && json.Unmarshal(data, &owner) == nil && owner.Identity.StartToken == n.identity.StartToken {
			_ = os.Remove(n.path)  //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
			_ = os.Remove(n.owner) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		}
	})
	if errors.Is(closeErr, net.ErrClosed) {
		return nil
	}
	return closeErr
}

func (n *Notifier) acceptLoop() {
	for {
		conn, err := n.listener.AcceptUnix()
		if err != nil {
			return
		}
		go n.serveClient(conn)
	}
}

func (n *Notifier) serveClient(conn *net.UnixConn) {
	defer conn.Close()                                    //nolint:errcheck
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	reader := bufio.NewReader(conn)
	var request subscribeRequest
	if err := json.NewDecoder(reader).Decode(&request); err != nil || request.TaskID != n.taskID || request.Token != n.identity.StartToken {
		return
	}
	task, err := n.store.Read(n.taskID)
	if err != nil {
		return
	}
	if request.Operation != "subscribe" {
		n.handleControlRequest(conn, request.Operation, task)
		return
	}
	n.serveSubscriber(conn, reader)
}

func (n *Notifier) handleControlRequest(conn *net.UnixConn, operation string, task Task) {
	if operation != "wake" && operation != "probe" && operation != workerOperationCancel {
		return
	}
	if operation == workerOperationCancel && task.Status != TaskCancelling {
		return
	}
	ack, err := json.Marshal(revisionMessage{Revision: task.Revision})
	if err != nil {
		return
	}
	if _, err := conn.Write(append(ack, '\n')); err != nil {
		return
	}
	switch operation {
	case "wake":
		// Ack first so a fast worker exit cannot hide receipt from the caller.
		_ = n.Publish(task.Revision) //nolint:errcheck // Caller can reread durable state if broadcasting fails.
	case workerOperationCancel:
		n.requestCancel()
	}
}

func (n *Notifier) serveSubscriber(conn *net.UnixConn, reader *bufio.Reader) {
	queue := make(chan uint64, 1)
	n.mu.Lock()
	if len(n.clients) >= maxTaskSubscribers {
		n.mu.Unlock()
		return
	}
	n.clients[conn] = queue
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		delete(n.clients, conn)
		n.mu.Unlock()
	}()
	// Registration precedes the revision read and acknowledgment. A caller then
	// rereads durable state, so a transition on either side of the read is visible.
	task, err := n.store.Read(n.taskID)
	if err != nil {
		return
	}
	ack, err := json.Marshal(revisionMessage{Revision: task.Revision})
	if err != nil {
		return
	}
	if _, err := conn.Write(append(ack, '\n')); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{}) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	disconnected := make(chan struct{})
	go func() {
		_, _ = reader.ReadByte() //nolint:errcheck // A subscriber sends nothing after its handshake; EOF releases its slot.
		close(disconnected)
	}()
	for {
		select {
		case <-n.closed:
			return
		case <-disconnected:
			return
		case revision := <-queue:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck // Bound a subscriber that stops reading.
			message, err := json.Marshal(revisionMessage{Revision: revision})
			if err != nil {
				return
			}
			if _, err := conn.Write(append(message, '\n')); err != nil {
				return
			}
		}
	}
}

func (s *Store) Subscribe(ctx context.Context, taskID string) (*Subscription, error) {
	if !safeID.MatchString(taskID) {
		return nil, fmt.Errorf("invalid task id %q", taskID)
	}
	dir := filepath.Join(s.root, "w")
	path, ownerPath := workerSocketPaths(dir, taskID)
	data, err := os.ReadFile(ownerPath) //#nosec G304 -- fixed path derived from safe task id
	if err != nil {
		return nil, err
	}
	var owner workerSocketOwner
	if err := json.Unmarshal(data, &owner); err != nil || owner.TaskID != taskID || owner.Identity.StartToken == "" {
		return nil, fmt.Errorf("worker socket owner is stale or invalid")
	}
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() }) //nolint:errcheck // Cancellation releases a blocked handshake or reader.
	failed := true
	defer func() {
		if failed {
			stopCancel()
			_ = conn.Close() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	}
	request, err := json.Marshal(subscribeRequest{Operation: "subscribe", TaskID: taskID, Token: owner.Identity.StartToken})
	if err != nil {
		_ = conn.Close() //nolint:errcheck // Preserve the marshal error that caused this connection to be discarded.
		return nil, err
	}
	if _, err := conn.Write(append(request, '\n')); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	decoder := json.NewDecoder(reader)
	var ack revisionMessage
	if err := decoder.Decode(&ack); err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{}) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	revisions := make(chan uint64, 8)
	go func() {
		defer close(revisions)
		for {
			var message revisionMessage
			if err := decoder.Decode(&message); err != nil {
				return
			}
			select {
			case revisions <- message.Revision:
			default:
				select {
				case <-revisions:
				default:
				}
				revisions <- message.Revision
			}
		}
	}()
	failed = false
	return &Subscription{Conn: conn, InitialRevision: ack.Revision, Revisions: revisions, close: func() { stopCancel(); _ = conn.Close() }}, nil //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
}

func (s *Store) WakeWorker(taskID string) error {
	return s.workerCommand(context.Background(), taskID, "wake")
}

func (s *Store) RequestWorkerCancel(ctx context.Context, taskID string) error {
	return s.workerCommand(ctx, taskID, "cancel")
}

func (s *Store) ProbeWorker(ctx context.Context, taskID string) error {
	return s.workerCommand(ctx, taskID, "probe")
}

func (s *Store) WaitWorkerReady(ctx context.Context, taskID string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := s.ProbeWorker(ctx, taskID); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Store) workerCommand(ctx context.Context, taskID, operation string) error {
	if !safeID.MatchString(taskID) {
		return fmt.Errorf("invalid task id %q", taskID)
	}
	dir := filepath.Join(s.root, "w")
	path, ownerPath := workerSocketPaths(dir, taskID)
	data, err := os.ReadFile(ownerPath) //#nosec G304 -- fixed path derived from safe task id
	if err != nil {
		return err
	}
	var owner workerSocketOwner
	if err := json.Unmarshal(data, &owner); err != nil || owner.TaskID != taskID || owner.Identity.StartToken == "" {
		return fmt.Errorf("worker socket owner is stale or invalid")
	}
	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck
	deadline := time.Now().Add(time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	request, err := json.Marshal(subscribeRequest{Operation: operation, TaskID: taskID, Token: owner.Identity.StartToken})
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(request, '\n')); err != nil {
		return err
	}
	var ack revisionMessage
	if err := json.NewDecoder(conn).Decode(&ack); err != nil {
		return err
	}
	return nil
}

// WaitWorkerClosed waits for the worker-owned control endpoint to be removed after
// its ACP process teardown has completed.
func (s *Store) WaitWorkerClosed(ctx context.Context, taskID string) error {
	dir := filepath.Join(s.root, "w")
	path, ownerPath := workerSocketPaths(dir, taskID)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, socketErr := os.Lstat(path)
		_, ownerErr := os.Lstat(ownerPath)
		if os.IsNotExist(socketErr) && os.IsNotExist(ownerErr) {
			return nil
		}
		if socketErr != nil && !os.IsNotExist(socketErr) {
			return socketErr
		}
		if ownerErr != nil && !os.IsNotExist(ownerErr) {
			return ownerErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func workerSocketPaths(dir, taskID string) (string, string) {
	digest := sha256.Sum256([]byte(taskID))
	name := hex.EncodeToString(digest[:8])
	return filepath.Join(dir, name+".sock"), filepath.Join(dir, name+".owner.json")
}

func removeStaleWorkerSocket(path, ownerPath string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		_ = os.Remove(ownerPath) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("worker socket path is occupied by a non-socket file")
	}
	conn, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		return fmt.Errorf("worker socket is already accepting connections")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(ownerPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
