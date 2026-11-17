//go:build unix

package delegation

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

type PermissionRule struct {
	Capability string
	Decision   string
	Version    string
}

type PermissionPolicy func(context.Context, string) (PermissionRule, error)

type ACPWorkerOptions struct {
	Store             *Store
	Notifier          *Notifier
	TaskID            string
	AttemptID         string
	Agent             string
	Model             string
	AvailabilityGroup string
	Cooldown          string
	Command           []string
	Workspace         string
	Brief             string
	Policy            PermissionPolicy
	bridgeStart       func(context.Context, *acpbridge.Bridge, []string, string) (string, error)
}

// RunACPAttempt executes one prompt in the task workspace and retains its visible
// thread as a Markdown artifact. ACP turn completion records readiness, never acceptance.
// Client-mediated file callbacks are confined to Workspace; tools run by the agent itself
// are not OS-sandboxed by this boundary.
//
//nolint:gocognit // This function preserves a multi-step state transition as one auditable unit.
//nolint:gocognit,gocyclo // This function preserves a multi-step state transition as one auditable unit.
//nolint:gocyclo
func RunACPAttempt(ctx context.Context, opts ACPWorkerOptions) error { //nolint:gocyclo
	if opts.Store == nil || opts.TaskID == "" || opts.AttemptID == "" || opts.Workspace == "" || len(opts.Command) == 0 {
		return fmt.Errorf("ACP attempt requires store, task, attempt, workspace, and command")
	}
	var captureMu sync.Mutex
	var captureErr, permissionErr error
	bridge := acpbridge.New(acpbridge.Options{
		Root: opts.Workspace, Version: "sdt", Models: nonEmpty(opts.Model),
		PermissionHandler: func(ctx context.Context, request acpbridge.Permission) (string, error) {
			optionID, err := opts.handlePermission(ctx, request)
			if err != nil {
				captureMu.Lock()
				if permissionErr == nil {
					permissionErr = err
				}
				captureMu.Unlock()
			}
			return optionID, err
		},
	})
	defer bridge.Close() //nolint:errcheck // Shutdown is best-effort after the task result has been recorded.
	start := opts.bridgeStart
	if start == nil {
		start = func(ctx context.Context, bridge *acpbridge.Bridge, command []string, dir string) (string, error) {
			return bridge.Start(ctx, command, dir)
		}
	}
	if _, err := start(ctx, bridge, opts.Command, opts.Workspace); err != nil {
		return opts.failAttempt("ACP initialization failed", err)
	}
	if _, note := bridge.Status(); note != "" {
		return opts.failAttempt("ACP model or session configuration was rejected", errors.New(note))
	}
	session, err := bridge.CurrentSession(ctx)
	if err != nil {
		return opts.failAttempt("ACP session unavailable", err)
	}
	session.State().Listen(func(thread acpbridge.Thread) {
		captureMu.Lock()
		defer captureMu.Unlock()
		if captureErr != nil {
			return
		}
		captureErr = opts.recordThreadEvent(thread)
	})
	if err := session.Prompt(ctx, opts.Brief); err != nil {
		return opts.failAttempt("ACP prompt failed", err)
	}
	captureMu.Lock()
	if captureErr != nil {
		err := captureErr
		captureMu.Unlock()
		return opts.failAttempt("ACP event capture failed", err)
	}
	if permissionErr != nil && !errors.Is(permissionErr, context.Canceled) {
		err := permissionErr
		captureMu.Unlock()
		return opts.failAttempt("ACP permission policy or decision failed", err)
	}
	captureMu.Unlock()
	thread := session.State().Snapshot()
	task, err := opts.Store.Read(opts.TaskID)
	if err != nil {
		return err
	}
	for _, permission := range task.Permissions {
		if permission.AttemptID == opts.AttemptID && (permission.Disposition == PermissionPending || permission.Disposition == "stale" || permission.Disposition == string(TaskCancelled) || permission.Disposition == "expired") {
			_ = opts.update("attempt_intervention", "ACP attempt stopped with an unresolved permission request", func(task *Task) error { //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
				if len(task.Attempts) > 0 && task.Attempts[len(task.Attempts)-1].ID == opts.AttemptID {
					last := &task.Attempts[len(task.Attempts)-1]
					last.Status, last.FinishedAt = "intervention_required", time.Now().UTC()
				}
				return nil
			})
			_ = opts.publishLatest() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
			return fmt.Errorf("attempt has unresolved permission request %q (%s)", permission.RequestID, permission.Disposition)
		}
	}
	result := renderACPResult(thread)
	artifactName := "result-" + opts.AttemptID
	path := filepath.Join(opts.Store.taskDir(opts.TaskID), "artifacts", artifactName+".md")
	if err := atomicWrite(path, []byte(result)); err != nil {
		return opts.failAttempt("ACP result artifact failed", err)
	}
	if err := opts.update("result_ready", "ACP turn completed; result awaits principal review", func(task *Task) error {
		if task.Status != TaskRunning {
			return fmt.Errorf("task is not running")
		}
		if len(task.Attempts) == 0 || task.Attempts[len(task.Attempts)-1].ID != opts.AttemptID {
			return fmt.Errorf("attempt is no longer current")
		}
		last := &task.Attempts[len(task.Attempts)-1]
		last.Status, last.FinishedAt = "result_ready", time.Now().UTC()
		task.Status = TaskResultReady
		task.Artifacts = append(task.Artifacts, Artifact{Name: artifactName, Path: path})
		return nil
	}); err != nil {
		return err
	}
	return opts.publishLatest()
}

//nolint:gocognit // Permission persistence, policy and waiting are one audit boundary.
//nolint:gocyclo
func (opts ACPWorkerOptions) handlePermission(ctx context.Context, request acpbridge.Permission) (string, error) { //nolint:gocyclo
	rule := PermissionRule{Capability: request.ToolKind, Decision: "ask"}
	if opts.Policy != nil {
		current, err := opts.Policy(ctx, request.ToolKind)
		if err != nil {
			return "", err
		}
		if current.Capability == request.ToolKind && (current.Decision == "allow" || current.Decision == "deny" || current.Decision == "ask") {
			rule = current
		}
	}
	choices := make([]PermissionChoice, 0, len(request.Options))
	for _, option := range request.Options {
		choices = append(choices, PermissionChoice{OptionID: option.OptionID, Name: option.Name, Kind: option.Kind})
	}
	base := PermissionRecord{
		RequestID: request.RequestID, AttemptID: opts.AttemptID, Agent: opts.Agent, Model: opts.Model,
		ToolCallID: request.ToolCallID, Capability: request.ToolKind,
		Scope:   strings.TrimSpace(strings.Join([]string{request.ToolKind, request.Title}, ": ")),
		Options: choices, PolicyVersion: rule.Version,
	}
	if optionID := policyOption(rule.Decision, choices); optionID != "" {
		base.Disposition, base.Source, base.PolicyRule, base.OptionID = PermissionResolved, "automatic", rule.Capability+":"+rule.Decision, optionID
		base.ResolvedAt = time.Now().UTC()
		if err := opts.Store.RecordPermission(opts.TaskID, base); err != nil {
			return "", err
		}
		if err := opts.publishLatest(); err != nil {
			return "", err
		}
		return optionID, nil
	}
	base.Disposition = PermissionPending
	base.Escalation = "Principal decision required; escalate to the user when outside the existing mandate or uncertain."
	if err := opts.Store.RecordPermission(opts.TaskID, base); err != nil {
		return "", err
	}
	if err := opts.publishLatest(); err != nil {
		return "", err
	}
	subscription, err := opts.Store.Subscribe(ctx, opts.TaskID)
	if err != nil {
		return "", err
	}
	defer subscription.Close() //nolint:errcheck
	for {
		task, err := opts.Store.Read(opts.TaskID)
		if err != nil {
			return "", err
		}
		for _, record := range task.Permissions {
			if record.RequestID == request.RequestID && record.AttemptID == opts.AttemptID {
				if record.Disposition == PermissionResolved {
					if record.PolicyVersion != rule.Version && record.Source != PermissionUser {
						return "", fmt.Errorf("permission policy version is stale")
					}
					return record.OptionID, nil
				}
				if record.Disposition != PermissionPending {
					return "", context.Canceled
				}
			}
		}
		select {
		case <-ctx.Done():
			_ = opts.update("permission_cancelled", "Pending ACP permission wait ended", func(task *Task) error { //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
				for i := range task.Permissions {
					p := &task.Permissions[i]
					if p.RequestID == request.RequestID && p.AttemptID == opts.AttemptID && p.Disposition == PermissionPending {
						p.Disposition, p.ResolvedAt = string(TaskCancelled), time.Now().UTC()
						if task.Status == TaskInterventionRequired {
							task.Status = TaskRunning
						}
					}
				}
				return nil
			})
			_ = opts.publishLatest() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
			return "", ctx.Err()
		case _, ok := <-subscription.Revisions:
			if !ok {
				return "", fmt.Errorf("worker notification connection closed")
			}
		}
	}
}

func policyOption(decision string, options []PermissionChoice) string {
	want := ""
	switch decision {
	case "allow":
		want = "allow_once"
	case "deny":
		want = "reject_once"
	default:
		return ""
	}
	for _, option := range options {
		if option.Kind == want && option.OptionID != "" {
			return option.OptionID
		}
	}
	return ""
}

func (opts ACPWorkerOptions) recordThreadEvent(thread acpbridge.Thread) error {
	if len(thread.Messages) == 0 {
		return nil
	}
	message := thread.Messages[len(thread.Messages)-1]
	if message.Role != acpbridge.RoleAssistant {
		return nil
	}
	for i := len(message.Parts) - 1; i >= 0; i-- {
		part := message.Parts[i]
		if part.Kind == acpbridge.PartThought || part.Kind == acpbridge.PartPermission || part.Kind == acpbridge.PartElicitation {
			continue
		}
		summary := ""
		switch part.Kind {
		case acpbridge.PartText:
			summary = "Assistant text update"
		case acpbridge.PartToolCall:
			if part.ToolCall != nil {
				summary = "Tool call update"
			}
		case acpbridge.PartError:
			summary = "ACP reported an error"
		case acpbridge.PartStop:
			summary = "ACP turn stopped"
		}
		if summary == "" {
			return nil
		}
		if err := opts.update("agent_event", summary, func(*Task) error { return nil }); err != nil {
			return err
		}
		return opts.publishLatest()
	}
	return nil
}

func renderACPResult(thread acpbridge.Thread) string {
	var b strings.Builder
	b.WriteString("## Result\n\n")
	for _, message := range thread.Messages {
		if message.Role != acpbridge.RoleAssistant {
			continue
		}
		for _, part := range message.Parts {
			switch part.Kind {
			case acpbridge.PartText:
				if strings.TrimSpace(part.Text) != "" {
					b.WriteString(part.Text)
					b.WriteString("\n\n")
				}
			case acpbridge.PartToolCall:
				if part.ToolCall != nil {
					fmt.Fprintf(&b, "- Tool: %s; status: %s\n", part.ToolCall.Title, part.ToolCall.Status)
				}
			case acpbridge.PartError:
				fmt.Fprintf(&b, "- ACP error: %s\n", part.Text)
			}
		}
	}
	b.WriteString("## Verification\n\nNo checks are recorded unless execution evidence was observed.\n\n## Acceptance\n\nPending principal review; ACP turn completion does not establish acceptance.\n")
	return b.String()
}

func (opts ACPWorkerOptions) failAttempt(event string, cause error) error {
	signal := ClassifyACPFailure(cause)
	status := TaskFailed
	attemptStatus := "failed"
	if signal.Kind == FailureAuth || signal.Kind == FailurePermission || isACPRequestError(cause) || strings.Contains(strings.ToLower(event), "permission") {
		status, attemptStatus = TaskInterventionRequired, string(signal.Kind)
	}
	if signal.Recoverable() {
		cooldown, parseErr := time.ParseDuration(opts.Cooldown)
		if parseErr != nil {
			cooldown = 0
		}
		if _, err := opts.Store.RecordAvailabilityFailure(opts.Agent, opts.AvailabilityGroup, signal, cooldown, time.Now()); err == nil {
			status, attemptStatus = TaskQueued, string(signal.Kind)
		} else {
			status, attemptStatus = TaskInterventionRequired, string(FailureUnknown)
		}
	}
	_ = opts.update("attempt_failed", event, func(task *Task) error { //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		if len(task.Attempts) > 0 && task.Attempts[len(task.Attempts)-1].ID == opts.AttemptID {
			last := &task.Attempts[len(task.Attempts)-1]
			last.Status, last.Error, last.FinishedAt = attemptStatus, event, time.Now().UTC()
		}
		if task.Status == TaskRunning {
			task.Status = status
		}
		return nil
	})
	_ = opts.publishLatest() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	return fmt.Errorf("%s: %w", event, cause)
}

func (opts ACPWorkerOptions) update(eventType, summary string, mutate func(*Task) error) error {
	return opts.Store.Update(opts.TaskID, eventType, summary, mutate)
}

func (opts ACPWorkerOptions) publishLatest() error {
	if opts.Notifier == nil {
		return nil
	}
	task, err := opts.Store.Read(opts.TaskID)
	if err != nil {
		return err
	}
	return opts.Notifier.Publish(task.Revision)
}

func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}
