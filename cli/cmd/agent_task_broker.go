//go:build unix

package cmd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sandrolain/sdt/internal/delegation"
	"github.com/spf13/cobra"
)

const brokerWorkerArg = "--sdt-delegation-worker="

var agentTaskBrokerCmd = &cobra.Command{Use: "broker", Short: "Manage the project-local task observer"}
var agentTaskBrokerStartCmd = &cobra.Command{Use: "start", Args: cobra.NoArgs, RunE: runAgentTaskBrokerStart}
var agentTaskBrokerStopCmd = &cobra.Command{Use: "stop", Args: cobra.NoArgs, RunE: runAgentTaskBrokerStop}
var agentTaskBrokerStatusCmd = &cobra.Command{Use: ctxMapStatus, Args: cobra.NoArgs, RunE: runAgentTaskBrokerStatus}
var agentTaskBrokerRunCmd = &cobra.Command{Use: "run", Hidden: true, Args: cobra.ArbitraryArgs, RunE: runAgentTaskBroker}

func init() {
	agentTaskBrokerRunCmd.Flags().String("project-root", "", "Project root")
	agentTaskBrokerCmd.AddCommand(agentTaskBrokerStartCmd, agentTaskBrokerStopCmd, agentTaskBrokerStatusCmd, agentTaskBrokerRunCmd)
	agentTaskCmd.AddCommand(agentTaskBrokerCmd)
}

type brokerStatusView struct {
	Running    bool                        `json:"running"`
	Worker     *delegation.ProcessIdentity `json:"worker,omitempty"`
	StartedAt  time.Time                   `json:"started_at,omitempty"`
	ObservedAt time.Time                   `json:"observed_at,omitempty"`
	Attention  int                         `json:"attention_count"`
}

func runAgentTaskBrokerStart(cmd *cobra.Command, _ []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	root, err := filepath.Abs(filepath.Dir(filepath.Dir(store.Root())))
	if err != nil {
		return err
	}
	state, err := ensureTaskBroker(root, store)
	if err != nil {
		return err
	}
	return outputAction(cmd, fmt.Sprintf("Task broker running (pid %d)\n", state.Worker.PID), brokerStatusView{Running: true, Worker: state.Worker, StartedAt: state.StartedAt, ObservedAt: state.ObservedAt, Attention: state.Attention})
}

func runAgentTaskBrokerStop(cmd *cobra.Command, _ []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	state, err := store.ReadBrokerState()
	if err != nil {
		return err
	}
	if state.Worker == nil {
		return outputAction(cmd, "Task broker is not running.\n", brokerStatusView{})
	}
	alive, err := delegation.InspectWorker(*state.Worker)
	if err != nil {
		return fmt.Errorf("task broker state is uncertain: %w", err)
	}
	if alive {
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		err = delegation.StopWorkerGroup(ctx, *state.Worker)
		cancel()
		if err != nil {
			return err
		}
	}
	state.Worker = nil
	if err := store.WriteBrokerState(state); err != nil {
		return err
	}
	return outputAction(cmd, "Task broker stopped.\n", brokerStatusView{Running: false, Attention: state.Attention})
}

func runAgentTaskBrokerStatus(cmd *cobra.Command, _ []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	state, err := store.ReadBrokerState()
	if err != nil {
		return err
	}
	view := brokerStatusView{Worker: state.Worker, StartedAt: state.StartedAt, ObservedAt: state.ObservedAt, Attention: state.Attention}
	if state.Worker != nil {
		view.Running, err = delegation.InspectWorker(*state.Worker)
		if err != nil {
			return fmt.Errorf("task broker state is uncertain: %w", err)
		}
	}
	if getFormat(cmd) == formatText {
		if view.Running {
			return outputAction(cmd, fmt.Sprintf("Task broker running (pid %d); %d attention items\n", state.Worker.PID, state.Attention), view)
		}
		return outputAction(cmd, "Task broker is stopped. Run `sdt agent task broker start` to resume observation.\n", view)
	}
	return outputTask(cmd, view)
}

func ensureTaskBroker(root string, store *delegation.Store) (delegation.BrokerState, error) {
	var state delegation.BrokerState
	err := store.WithLock("broker_start", func() error {
		var err error
		state, err = ensureTaskBrokerLocked(root, store)
		return err
	})
	return state, err
}

func ensureTaskBrokerLocked(root string, store *delegation.Store) (delegation.BrokerState, error) {
	state, err := store.ReadBrokerState()
	if err != nil {
		return delegation.BrokerState{}, err
	}
	if state.Worker != nil {
		alive, inspectErr := delegation.InspectWorker(*state.Worker)
		if inspectErr != nil {
			return delegation.BrokerState{}, fmt.Errorf("existing task broker cannot be verified: %w", inspectErr)
		}
		if alive {
			return state, nil
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return delegation.BrokerState{}, err
	}
	identity, err := delegation.StartWorker(executable, root, filepath.Join(store.Root(), "broker.log"), "agent", "task", "broker", "run", "--project-root", root)
	if err != nil {
		return delegation.BrokerState{}, fmt.Errorf("start task broker: %w", err)
	}
	state.Worker = &identity
	state.StartedAt = time.Now().UTC()
	if state.Revisions == nil {
		state.Revisions = map[string]uint64{}
	}
	if err := store.WriteBrokerState(state); err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = delegation.StopWorkerGroup(ctx, identity) //nolint:errcheck
		cancel()
		return delegation.BrokerState{}, err
	}
	return state, nil
}

func runAgentTaskBroker(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	token := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, brokerWorkerArg) {
			token = strings.TrimPrefix(arg, brokerWorkerArg)
		}
	}
	if token == "" {
		return fmt.Errorf("broker worker identity token is missing")
	}
	if len(token) != 32 {
		return fmt.Errorf("broker worker identity token is invalid")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return fmt.Errorf("broker worker identity token is invalid")
	}
	if syscall.Getpgrp() != os.Getpid() {
		return fmt.Errorf("broker worker process group could not be verified")
	}
	root, err := filepath.Abs(getStringFlag(cmd, "project-root", false))
	if err != nil {
		return err
	}
	store := delegation.NewStore(root)
	identity := delegation.ProcessIdentity{PID: os.Getpid(), GroupID: os.Getpid(), StartToken: token}
	deadline := time.Now().Add(3 * time.Second)
	for {
		state, readErr := store.ReadBrokerState()
		if readErr != nil {
			return readErr
		}
		if state.Worker != nil && state.Worker.StartToken == token {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("broker ownership was not persisted")
		}
		time.Sleep(25 * time.Millisecond)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := recordBrokerObservation(store, identity, ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func recordBrokerObservation(store *delegation.Store, identity delegation.ProcessIdentity, contexts ...context.Context) error {
	brokerCtx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		brokerCtx = contexts[0]
	}
	tasks, err := store.List()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := observeTaskVerification(brokerCtx, store, task); err != nil {
			return err
		}
	}
	tasks, err = store.List()
	if err != nil {
		return err
	}
	inbox, err := store.Inbox()
	if err != nil {
		return err
	}
	state, err := store.ReadBrokerState()
	if err != nil {
		return err
	}
	if state.Worker == nil || state.Worker.StartToken != identity.StartToken {
		return fmt.Errorf("broker ownership changed")
	}
	revisions := make(map[string]uint64, len(tasks))
	changed := len(state.Revisions) != len(tasks) || state.Attention != len(inbox)
	for _, task := range tasks {
		revisions[task.ID] = task.Revision
		if state.Revisions[task.ID] != task.Revision {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	state.Revisions = revisions
	state.Attention = len(inbox)
	state.ObservedAt = time.Now().UTC()
	return store.WriteBrokerState(state)
}

func observeTaskVerification(ctx context.Context, store *delegation.Store, task delegation.Task) error {
	if task.Status == delegation.TaskQueued || task.Status == delegation.TaskRunning || task.Status == delegation.TaskCancelling {
		if _, err := store.Reconcile(task.ID); err != nil {
			return fmt.Errorf("reconcile task %s: %w", task.ID, err)
		}
	}
	if task.Verification != nil && task.Verification.Status == delegation.VerificationRunning {
		return store.Update(task.ID, "verification_interrupted", "Broker stopped during declared checks; outcome is unknown", func(task *delegation.Task) error {
			task.Verification.Status = delegation.VerificationInterrupted
			task.Verification.FinishedAt = time.Now().UTC()
			return nil
		})
	}
	if task.Status == delegation.TaskResultReady && task.Verification == nil && len(task.VerificationConfig) > 0 {
		if err := runTaskVerification(ctx, store, task); err != nil {
			return fmt.Errorf("verify task %s: %w", task.ID, err)
		}
	}
	return nil
}

func runTaskVerification(brokerCtx context.Context, store *delegation.Store, task delegation.Task) error {
	run := &delegation.VerificationRun{Status: delegation.VerificationRunning, CandidateDigest: task.CandidateDigest, StartedAt: time.Now().UTC(), Checks: make([]delegation.VerificationCheckResult, len(task.VerificationConfig))}
	for i, config := range task.VerificationConfig {
		run.Checks[i] = delegation.VerificationCheckResult{Name: config.Name, Status: delegation.VerificationPending}
	}
	if err := store.Update(task.ID, "verification_started", "Declared result checks started", func(current *delegation.Task) error {
		if current.Status != delegation.TaskResultReady || current.CandidateDigest != task.CandidateDigest || current.Verification != nil {
			return fmt.Errorf("task changed before checks started")
		}
		current.Verification = run
		return nil
	}); err != nil {
		return err
	}
	status := delegation.VerificationPassed
	if task.CandidateDigest == "" {
		status = delegation.VerificationStale
		run.Checks[0].Status = delegation.VerificationStale
	}
	for i := range run.Checks {
		if status != delegation.VerificationPassed {
			break
		}
		status = runOneVerificationCheck(brokerCtx, store, task, run.CandidateDigest, task.VerificationConfig[i], &run.Checks[i])
	}
	run.Status, run.FinishedAt = status, time.Now().UTC()
	return store.Update(task.ID, "verification_"+status, "Declared result checks completed with status "+status, func(current *delegation.Task) error {
		if current.Verification == nil || current.Verification.CandidateDigest != run.CandidateDigest {
			return fmt.Errorf("verification run changed")
		}
		current.Verification = run
		return nil
	})
}

func runOneVerificationCheck(brokerCtx context.Context, store *delegation.Store, task delegation.Task, digest string, config delegation.VerificationCheckConfig, check *delegation.VerificationCheckResult) string {
	ctx := brokerCtx
	cancel := func() {}
	if !task.Deadline.IsZero() {
		ctx, cancel = context.WithDeadline(brokerCtx, task.Deadline)
	}
	defer cancel()
	if err := store.ValidateCandidateDigest(ctx, task.ID, digest); err != nil {
		check.Status = delegation.VerificationStale
		return delegation.VerificationStale
	}
	started := time.Now()
	// #nosec G204 -- run is snapshotted from the project's explicitly selected gate command.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", config.Run)
	cmd.Dir = task.Source.Workspace
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	ctxErr := ctx.Err()
	check.Duration = time.Since(started)
	check.Status, check.ExitCode = delegation.VerificationPassed, 0
	if err != nil {
		check.Status = delegation.VerificationFailed
		if ctxErr != nil {
			check.Status = delegation.VerificationInterrupted
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			check.ExitCode = exitErr.ExitCode()
		}
	}
	if err := store.ValidateCandidateDigest(context.Background(), task.ID, digest); err != nil {
		check.Status = delegation.VerificationStale
	}
	return check.Status
}
