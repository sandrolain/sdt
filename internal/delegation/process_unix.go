//go:build unix

package delegation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

var ErrStaleWorker = errors.New("delegation worker identity no longer matches")
var ErrInvalidWorkerIdentity = errors.New("delegation worker identity is incomplete or invalid")
var regexpToken = regexp.MustCompile(`^[0-9a-f]{32}$`)

type ProcessIdentity struct {
	PID         int    `json:"pid"`
	GroupID     int    `json:"group_id"`
	StartToken  string `json:"start_token"`
	StartTimeMS int64  `json:"start_time_ms,omitempty"`
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
	pid, err := checkedPID(identity.PID)
	if err != nil {
		_ = syscall.Kill(-identity.GroupID, syscall.SIGKILL) //nolint:errcheck
		_ = cmd.Wait()                                       //nolint:errcheck
		return ProcessIdentity{}, err
	}
	info, err := process.NewProcessWithContext(context.Background(), pid)
	if err == nil {
		identity.StartTimeMS, err = info.CreateTimeWithContext(context.Background())
	}
	if err != nil {
		_ = syscall.Kill(-identity.GroupID, syscall.SIGKILL) //nolint:errcheck // Cleanup must not replace process-inspection failure.
		_ = cmd.Wait()                                       //nolint:errcheck // Reap the child after the group has been terminated.
		return ProcessIdentity{}, fmt.Errorf("inspect started worker process: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		_ = syscall.Kill(-identity.GroupID, syscall.SIGKILL) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		return ProcessIdentity{}, err
	}
	return identity, nil
}

func FindWorker(token string) (ProcessIdentity, bool, error) {
	if !regexpToken.MatchString(token) {
		return ProcessIdentity{}, false, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return ProcessIdentity{}, false, fmt.Errorf("locate SDT executable for worker discovery: %w", err)
	}
	executableName := filepath.Base(executable)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pids, err := process.PidsWithContext(ctx)
	if err != nil {
		return ProcessIdentity{}, false, fmt.Errorf("enumerate processes: %w", err)
	}
	needle := "--sdt-delegation-worker=" + token
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		identity, found, err := inspectWorkerCandidate(ctx, pid, executableName, needle, token)
		if err != nil {
			return ProcessIdentity{}, false, err
		}
		if found {
			return identity, true, nil
		}
	}
	return ProcessIdentity{}, false, nil
}

func inspectWorkerCandidate(ctx context.Context, pid int32, executableName, needle, token string) (ProcessIdentity, bool, error) {
	info, err := process.NewProcessWithContext(ctx, pid)
	if errors.Is(err, process.ErrorProcessNotRunning) {
		return ProcessIdentity{}, false, nil
	}
	if err != nil {
		return ProcessIdentity{}, false, fmt.Errorf("inspect process %d during worker discovery: %w", pid, err)
	}
	name, err := info.NameWithContext(ctx)
	if err != nil {
		if processGoneOrZombie(ctx, info, err) || processOwnedByOtherUser(info) {
			return ProcessIdentity{}, false, nil
		}
		return ProcessIdentity{}, false, fmt.Errorf("read process %d name during worker discovery: %w", pid, err)
	}
	if name != executableName {
		return ProcessIdentity{}, false, nil
	}
	args, err := info.CmdlineSliceWithContext(ctx)
	if err != nil {
		if processGoneOrZombie(ctx, info, err) {
			return ProcessIdentity{}, false, nil
		}
		return ProcessIdentity{}, false, fmt.Errorf("read arguments for process %d during worker discovery: %w", pid, err)
	}
	if !hasWorkerArgument(args, needle) {
		return ProcessIdentity{}, false, nil
	}
	started, err := info.CreateTimeWithContext(ctx)
	if err != nil {
		return ProcessIdentity{}, false, fmt.Errorf("read start time for worker %d: %w", pid, err)
	}
	return ProcessIdentity{PID: int(pid), GroupID: int(pid), StartToken: token, StartTimeMS: started}, true, nil
}

func processGoneOrZombie(ctx context.Context, info *process.Process, cause error) bool {
	if errors.Is(cause, process.ErrorProcessNotRunning) {
		return true
	}
	status, err := info.StatusWithContext(ctx)
	return err == nil && containsProcessStatus(status, process.Zombie)
}

func processOwnedByOtherUser(info *process.Process) bool {
	uids, err := info.Uids()
	if err != nil || len(uids) == 0 {
		return false
	}
	return fmt.Sprint(uids[0]) != fmt.Sprint(os.Geteuid())
}

func checkedPID(pid int) (int32, error) {
	if pid <= 0 || int64(pid) > math.MaxInt32 {
		return 0, fmt.Errorf("worker PID %d is outside the supported range", pid)
	}
	return int32(pid), nil
}

func WorkerMatches(identity ProcessIdentity) bool {
	alive, err := InspectWorker(identity)
	return alive || err != nil && !errors.Is(err, ErrStaleWorker)
}

// WorkerStopped reports only a confirmed stop or an identity mismatch as gone.
// Inspection failures remain errors so callers cannot release capacity early.
func WorkerStopped(identity ProcessIdentity) (bool, error) {
	alive, err := InspectWorker(identity)
	if errors.Is(err, ErrStaleWorker) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !alive, nil
}

// InspectWorker verifies a process token and optional creation time. ErrStaleWorker
// means the PID is absent or belongs to a different process; other errors mean
// inspection was unavailable and callers must keep recovery blocked.
func InspectWorker(identity ProcessIdentity) (bool, error) {
	if identity.PID <= 0 || identity.GroupID != identity.PID {
		return false, ErrInvalidWorkerIdentity
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pid, err := checkedPID(identity.PID)
	if err != nil {
		return false, err
	}
	exists, err := process.PidExistsWithContext(ctx, pid)
	if err != nil {
		return false, fmt.Errorf("probe worker process %d: %w", identity.PID, err)
	}
	if !exists {
		return false, nil
	}
	if !regexpToken.MatchString(identity.StartToken) {
		return false, ErrInvalidWorkerIdentity
	}
	info, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return false, fmt.Errorf("inspect worker process %d: %w", identity.PID, err)
	}
	running, err := info.IsRunningWithContext(ctx)
	if err != nil {
		return false, fmt.Errorf("check worker process %d: %w", identity.PID, err)
	}
	if !running {
		return false, nil
	}
	args, err := info.CmdlineSliceWithContext(ctx)
	if err != nil {
		if errors.Is(err, process.ErrorProcessNotRunning) {
			return false, nil
		}
		status, statusErr := info.StatusWithContext(ctx)
		if statusErr == nil && containsProcessStatus(status, process.Zombie) {
			return false, nil
		}
		return false, fmt.Errorf("read worker process %d arguments: %w", identity.PID, err)
	}
	if !hasWorkerArgument(args, "--sdt-delegation-worker="+identity.StartToken) {
		return false, ErrStaleWorker
	}
	if identity.StartTimeMS > 0 {
		started, err := info.CreateTimeWithContext(ctx)
		if err != nil {
			return false, fmt.Errorf("read worker process %d start time: %w", identity.PID, err)
		}
		if started != identity.StartTimeMS {
			return false, ErrStaleWorker
		}
	}
	return true, nil
}

func hasWorkerArgument(args []string, expected string) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}

func containsProcessStatus(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func SignalWorkerGroup(identity ProcessIdentity, signal syscall.Signal) error {
	alive, err := InspectWorker(identity)
	if err != nil {
		return err
	}
	if !alive {
		return ErrStaleWorker
	}
	return syscall.Kill(-identity.GroupID, signal)
}

// StopWorkerGroup asks the worker process group to stop and escalates to SIGKILL
// only after the supplied grace deadline expires.
func StopWorkerGroup(ctx context.Context, identity ProcessIdentity) error {
	if err := SignalWorkerGroup(identity, syscall.SIGTERM); err != nil {
		return err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		alive, err := inspectStopState(identity)
		if err != nil || !alive {
			return err
		}
		select {
		case <-ctx.Done():
			return killAndConfirmWorker(identity)
		case <-ticker.C:
		}
	}
}

func inspectStopState(identity ProcessIdentity) (bool, error) {
	alive, err := InspectWorker(identity)
	if errors.Is(err, ErrStaleWorker) {
		return false, nil
	}
	return alive, err
}

func killAndConfirmWorker(identity ProcessIdentity) error {
	alive, err := inspectStopState(identity)
	if err != nil || !alive {
		return err
	}
	if err := syscall.Kill(-identity.GroupID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		alive, err = inspectStopState(identity)
		if err != nil {
			return err
		}
		if !alive {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	alive, err = inspectStopState(identity)
	if err != nil || !alive {
		return err
	}
	return fmt.Errorf("worker %d did not exit after SIGKILL", identity.PID)
}

// WorkerHealth observes a recorded worker without reconciling persistent state.
func WorkerHealth(identity *ProcessIdentity) string {
	if identity == nil {
		return "unrecorded"
	}
	alive, err := InspectWorker(*identity)
	if err != nil {
		if errors.Is(err, ErrStaleWorker) {
			return "identity_mismatch"
		}
		return "unknown"
	}
	if alive {
		return "running"
	}
	return "stopped"
}
