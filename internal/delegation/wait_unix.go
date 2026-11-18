//go:build unix

package delegation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	WaitModeAll  = "all"
	waitTerminal = "terminal"
)

// WaitSnapshot is a durable-state snapshot, not a claim that all tasks succeeded.
type WaitSnapshot struct {
	Mode      string   `json:"mode"`
	Reason    string   `json:"wake_reason"`
	Satisfied bool     `json:"condition_satisfied"`
	TimedOut  bool     `json:"timed_out"`
	Tasks     []Task   `json:"-"`
	Ready     []string `json:"ready"`
	Remaining []string `json:"remaining"`
}

func waitReason(task Task) (string, bool) {
	switch task.Status {
	case TaskAccepted, TaskCompleted, TaskFailed, TaskCancelled:
		return waitTerminal, true
	case TaskResultReady, TaskInterventionRequired:
		return string(task.Status), false
	}
	for _, p := range task.Permissions {
		if p.Disposition == PermissionPending {
			return "permission_pending", false
		}
	}
	return "", false
}

func (s *Store) waitSnapshot(ids []string, mode string) (WaitSnapshot, error) {
	result := WaitSnapshot{Mode: mode, Ready: []string{}, Remaining: []string{}}
	terminalCount := 0
	for _, id := range ids {
		task, err := s.Read(id)
		if err != nil {
			return result, err
		}
		result.Tasks = append(result.Tasks, task)
		reason, terminal := waitReason(task)
		if terminal {
			terminalCount++
		}
		if reason == "" {
			result.Remaining = append(result.Remaining, id)
			continue
		}
		result.Ready = append(result.Ready, id)
		if result.Reason == "" || !terminal {
			result.Reason = reason
		}
	}
	result.Satisfied = len(result.Ready) > 0
	if mode == WaitModeAll {
		result.Satisfied = terminalCount == len(ids)
	}
	return result, nil
}

// Wait subscribes before reading state. Closed sockets force a state reread and
// bounded reconnect, covering publication followed by worker death before notify.
func (s *Store) Wait(ctx context.Context, ids []string, mode string) (WaitSnapshot, error) {
	if err := s.validateWait(ids, mode); err != nil {
		return WaitSnapshot{}, err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	changed := make(chan error, len(ids))
	registered := make(chan struct{}, len(ids))
	for _, id := range ids {
		workers.Add(1)
		go func() {
			defer workers.Done()
			s.watchTask(watchCtx, id, registered, changed)
		}()
	}
	for range ids {
		select {
		case <-registered:
		case <-ctx.Done():
			return s.waitEnd(ids, mode, ctx.Err())
		}
	}
	return s.waitChanges(ctx, ids, mode, changed)
}

func (s *Store) validateWait(ids []string, mode string) error {
	if len(ids) == 0 || len(ids) > 128 || (mode != "one" && mode != "any" && mode != WaitModeAll) || (mode == "one" && len(ids) != 1) {
		return fmt.Errorf("wait requires 1..128 tasks and one, any or all mode")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !safeID.MatchString(id) || seen[id] {
			return fmt.Errorf("invalid or duplicate task id %q", id)
		}
		seen[id] = true
		if _, err := s.Read(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) waitChanges(ctx context.Context, ids []string, mode string, changed <-chan error) (WaitSnapshot, error) {
	for {
		snapshot, err := s.waitSnapshot(ids, mode)
		if err != nil || len(snapshot.Ready) > 0 && (mode != WaitModeAll || snapshot.Satisfied || snapshot.Reason != waitTerminal) {
			return snapshot, err
		}
		select {
		case <-ctx.Done():
			return s.waitEnd(ids, mode, ctx.Err())
		case err := <-changed:
			if err != nil {
				snapshot, readErr := s.waitSnapshot(ids, mode)
				if readErr != nil {
					return snapshot, readErr
				}
				if snapshot.Satisfied || snapshot.Reason != "" && snapshot.Reason != waitTerminal {
					return snapshot, nil
				}
				snapshot.Reason = "worker_unreachable"
				return snapshot, err
			}
		}
	}
}

func (s *Store) waitEnd(ids []string, mode string, cause error) (WaitSnapshot, error) {
	result, err := s.waitSnapshot(ids, mode)
	if err != nil {
		return result, err
	}
	if result.Satisfied || result.Reason != "" && result.Reason != waitTerminal {
		return result, nil
	}
	result.TimedOut = errors.Is(cause, context.DeadlineExceeded)
	result.Reason = "cancelled"
	if result.TimedOut {
		result.Reason = "timeout"
	}
	return result, cause
}

func (s *Store) watchTask(ctx context.Context, id string, registered chan<- struct{}, changed chan<- error) {
	announced := false
	announce := func() {
		if !announced {
			registered <- struct{}{}
			announced = true
		}
	}
	defer announce()
	reconnectBy := time.Now().Add(2 * time.Second)
	for ctx.Err() == nil {
		sub, err := s.Subscribe(ctx, id)
		announce()
		task, readErr := s.Read(id)
		if readErr != nil {
			sendWaitChange(ctx, changed, readErr)
			return
		}
		if reason, _ := waitReason(task); reason != "" {
			if sub != nil {
				_ = sub.Close() //nolint:errcheck // Observation must not replace durable task state.
			} //nolint:errcheck // Observation must not replace durable task state.
			sendWaitChange(ctx, changed, nil)
			return
		}
		if err != nil {
			if time.Now().After(reconnectBy) {
				sendWaitChange(ctx, changed, fmt.Errorf("task %s worker unreachable: %w", id, err))
				return
			}
			timer := time.NewTimer(50 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		closed := s.watchSubscription(ctx, sub, changed)
		_ = sub.Close() //nolint:errcheck // Subscriber owns only its connection.
		if !closed {
			return
		}
		reconnectBy = time.Now().Add(2 * time.Second)
		sendWaitChange(ctx, changed, nil)
	}
}

func (s *Store) watchSubscription(ctx context.Context, sub *Subscription, changed chan<- error) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case _, ok := <-sub.Revisions:
			if !ok {
				return true
			}
			sendWaitChange(ctx, changed, nil)
		}
	}
}

func sendWaitChange(ctx context.Context, changed chan<- error, err error) {
	select {
	case changed <- err:
	case <-ctx.Done():
	}
}
