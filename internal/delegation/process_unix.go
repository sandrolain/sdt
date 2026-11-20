//go:build unix

package delegation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrStaleWorker = errors.New("delegation worker identity no longer matches")
var regexpToken = regexp.MustCompile(`^[0-9a-f]{32}$`)

type ProcessIdentity struct {
	PID        int    `json:"pid"`
	GroupID    int    `json:"group_id"`
	StartToken string `json:"start_token"`
}

func NewWorkerToken() (string, error) {
	var tokenBytes [16]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes[:]), nil
}

// StartWorker launches the current SDT executable directly, with no shell, in
// a new process group. The trailing identity argument lets recovery distinguish
// this process from a later process that reused its PID.
func StartWorker(executable, workingDir, logPath string, args ...string) (ProcessIdentity, error) {
	if strings.TrimSpace(executable) == "" {
		return ProcessIdentity{}, fmt.Errorf("worker executable is required")
	}
	token, err := NewWorkerToken()
	if err != nil {
		return ProcessIdentity{}, err
	}
	return StartWorkerWithToken(executable, workingDir, logPath, token, args...)
}

func StartWorkerWithToken(executable, workingDir, logPath, token string, args ...string) (ProcessIdentity, error) {
	if !regexpToken.MatchString(token) {
		return ProcessIdentity{}, fmt.Errorf("worker token must be 32 lowercase hexadecimal characters")
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return ProcessIdentity{}, err
	}
	args = append(append([]string(nil), args...), "--", "--sdt-delegation-worker="+token)
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //#nosec G304 -- log path is chosen under the task artifacts directory
	if err != nil {
		return ProcessIdentity{}, err
	}
	defer log.Close() //nolint:errcheck // the child has inherited the descriptor
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return ProcessIdentity{}, err
	}
	defer devNull.Close()                  //nolint:errcheck
	cmd := exec.Command(absolute, args...) //#nosec G204 -- Explicit user-configured executable is launched as direct argv without a shell.
	cmd.Dir = workingDir
	cmd.Stdin = devNull
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return ProcessIdentity{}, err
	}
	identity := ProcessIdentity{PID: cmd.Process.Pid, GroupID: cmd.Process.Pid, StartToken: token}
	if err := cmd.Process.Release(); err != nil {
		_ = syscall.Kill(-identity.GroupID, syscall.SIGKILL) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		return ProcessIdentity{}, err
	}
	return identity, nil
}

func FindWorker(token string) (ProcessIdentity, bool) {
	if !regexpToken.MatchString(token) {
		return ProcessIdentity{}, false
	}
	output, err := exec.Command("ps", "-axo", "pid=,args=").Output() //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
	if err != nil {
		return ProcessIdentity{}, false
	}
	needle := "--sdt-delegation-worker=" + token
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(line, needle) {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err == nil && pid > 0 {
			return ProcessIdentity{PID: pid, GroupID: pid, StartToken: token}, true
		}
	}
	return ProcessIdentity{}, false
}

func WorkerMatches(identity ProcessIdentity) bool {
	if identity.PID <= 0 || identity.GroupID != identity.PID || identity.StartToken == "" {
		return false
	}
	output, err := exec.Command("ps", "-p", strconv.Itoa(identity.PID), "-o", "args=").Output() //#nosec G204 -- Fixed ps executable and flags; PID is formatted from an integer, without a shell.
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "--sdt-delegation-worker="+identity.StartToken)
}

// InspectWorker distinguishes a confirmed stale identity from an unavailable process listing.
func InspectWorker(identity ProcessIdentity) (bool, error) {
	if identity.PID <= 0 || identity.GroupID != identity.PID || !regexpToken.MatchString(identity.StartToken) {
		return false, ErrStaleWorker
	}
	if err := syscall.Kill(identity.PID, 0); errors.Is(err, syscall.ESRCH) {
		return false, nil
	} else if err != nil && !errors.Is(err, syscall.EPERM) {
		return false, fmt.Errorf("probe worker process %d: %w", identity.PID, err)
	} else if errors.Is(err, syscall.EPERM) {
		return false, fmt.Errorf("worker process %d exists but cannot be inspected", identity.PID)
	}
	output, err := exec.Command("ps", "-p", strconv.Itoa(identity.PID), "-o", "args=").Output() //#nosec G204 -- Fixed ps executable and flags; PID is formatted from an integer, without a shell.
	if err != nil {
		return false, fmt.Errorf("inspect worker process %d: %w", identity.PID, err)
	}
	return strings.Contains(string(output), "--sdt-delegation-worker="+identity.StartToken), nil
}

func SignalWorkerGroup(identity ProcessIdentity, signal syscall.Signal) error {
	if !WorkerMatches(identity) {
		return ErrStaleWorker
	}
	return syscall.Kill(-identity.GroupID, signal)
}

// StopWorkerGroup asks the worker process group to stop and escalates to SIGKILL
// only after the supplied grace deadline expires.
func StopWorkerGroup(ctx context.Context, identity ProcessIdentity) error {
	if !WorkerMatches(identity) {
		return ErrStaleWorker
	}
	if err := SignalWorkerGroup(identity, syscall.SIGTERM); err != nil {
		return err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for WorkerMatches(identity) {
		select {
		case <-ctx.Done():
			if WorkerMatches(identity) {
				if err := syscall.Kill(-identity.GroupID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					return err
				}
				goneBy := time.Now().Add(2 * time.Second)
				for WorkerMatches(identity) && time.Now().Before(goneBy) {
					time.Sleep(25 * time.Millisecond)
				}
				if WorkerMatches(identity) {
					return fmt.Errorf("worker %d did not exit after SIGKILL", identity.PID)
				}
			}
			return nil
		case <-ticker.C:
		}
	}
	return nil
}

// WorkerHealth observes a recorded worker without reconciling persistent state.
func WorkerHealth(identity *ProcessIdentity) string {
	if identity == nil {
		return "unrecorded"
	}
	if WorkerMatches(*identity) {
		return "running"
	}
	if identity.PID > 0 && errors.Is(syscall.Kill(identity.PID, 0), syscall.ESRCH) {
		return "stopped"
	}
	return "unknown"
}
