// Package delegation owns durable state for asynchronous agent tasks.
package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const stateSchemaVersion = 1

const (
	PermissionPending  = "pending"
	PermissionResolved = "resolved"
	PermissionUser     = "user"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

var ErrTaskNotFound = errors.New("delegation task not found")

type TaskStatus string

const (
	TaskQueued               TaskStatus = "queued"
	TaskRunning              TaskStatus = "running"
	TaskCancelling           TaskStatus = "cancelling"
	TaskInterventionRequired TaskStatus = "intervention_required"
	TaskResultReady          TaskStatus = "result_ready"
	TaskAccepted             TaskStatus = "accepted"
	TaskCompleted            TaskStatus = "completed"
	TaskFailed               TaskStatus = "failed"
	TaskCancelled            TaskStatus = "cancelled"
)

type Task struct {
	SchemaVersion            int                 `json:"schema_version"`
	ID                       string              `json:"id"`
	Status                   TaskStatus          `json:"status"`
	Role                     string              `json:"role"`
	RequiredCapabilities     []string            `json:"required_capabilities,omitempty"`
	CreatedAt                time.Time           `json:"created_at"`
	UpdatedAt                time.Time           `json:"updated_at"`
	Deadline                 time.Time           `json:"deadline"`
	Revision                 uint64              `json:"revision"`
	ConfigSnapshot           json.RawMessage     `json:"config_snapshot,omitempty"`
	PermissionPolicySnapshot json.RawMessage     `json:"permission_policy_snapshot,omitempty"`
	Source                   SourceSnapshot      `json:"source"`
	Attempts                 []Attempt           `json:"attempts,omitempty"`
	Artifacts                []Artifact          `json:"artifacts,omitempty"`
	Permissions              []PermissionRecord  `json:"permissions,omitempty"`
	Events                   []StateEvent        `json:"events,omitempty"`
	PrincipalDecision        *PrincipalDecision  `json:"principal_decision,omitempty"`
	PrincipalDecisions       []PrincipalDecision `json:"principal_decisions,omitempty"`
	Integration              *IntegrationReceipt `json:"integration,omitempty"`
	Worker                   *ProcessIdentity    `json:"worker,omitempty"`
	Correction               string              `json:"correction,omitempty"`
}

type SourceSnapshot struct {
	ProjectRoot     string              `json:"project_root"`
	Commit          string              `json:"commit"`
	Branch          string              `json:"branch"`
	Workspace       string              `json:"workspace"`
	SelectedChanges []string            `json:"selected_changes,omitempty"`
	ConfigDigest    string              `json:"config_digest"`
	PolicyDigest    string              `json:"policy_digest"`
	References      []ReferenceSnapshot `json:"references,omitempty"`
}

type ReferenceSnapshot struct {
	Source        string `json:"source"`
	WorkspacePath string `json:"workspace_path"`
	SHA256        string `json:"sha256"`
}

type Attempt struct {
	ID           string    `json:"id"`
	Agent        string    `json:"agent"`
	Model        string    `json:"model"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	ProcessID    int       `json:"process_id,omitempty"`
	ProcessStart string    `json:"process_start,omitempty"`
	Error        string    `json:"error,omitempty"`
}

type Artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type PermissionRecord struct {
	RequestID     string             `json:"request_id"`
	AttemptID     string             `json:"attempt_id"`
	Agent         string             `json:"agent"`
	Model         string             `json:"model"`
	ToolCallID    string             `json:"tool_call_id"`
	Capability    string             `json:"capability"`
	Scope         string             `json:"scope"`
	Options       []PermissionChoice `json:"options"`
	CreatedAt     time.Time          `json:"created_at"`
	ResolvedAt    time.Time          `json:"resolved_at,omitempty"`
	PolicyVersion string             `json:"policy_version,omitempty"`
	PolicyRule    string             `json:"policy_rule,omitempty"`
	Source        string             `json:"source,omitempty"`
	Rationale     string             `json:"rationale,omitempty"`
	Escalation    string             `json:"escalation,omitempty"`
	Disposition   string             `json:"disposition"`
	OptionID      string             `json:"option_id,omitempty"`
}

type PermissionChoice struct {
	OptionID string `json:"option_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind,omitempty"`
}

type StateEvent struct {
	ID      uint64    `json:"id"`
	At      time.Time `json:"at"`
	Type    string    `json:"type"`
	Summary string    `json:"summary"`
}

type PrincipalDecision struct {
	Action      string    `json:"action"`
	AcceptedAt  time.Time `json:"accepted_at,omitempty"`
	CorrectedAt time.Time `json:"corrected_at,omitempty"`
	Note        string    `json:"note,omitempty"`
}

type Store struct {
	root string
}

func NewStore(projectRoot string) *Store {
	return &Store{root: filepath.Join(projectRoot, ".sdt", "delegation")}
}

func (s *Store) Root() string { return s.root }

func (s *Store) Create(task Task) error {
	if !safeID.MatchString(task.ID) {
		return fmt.Errorf("invalid task id %q", task.ID)
	}
	return s.WithLock("task-"+task.ID, func() error {
		dir := s.taskDir(task.ID)
		if _, err := os.Stat(s.statePath(task.ID)); err == nil {
			return fmt.Errorf("task %q already exists", task.ID)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dir, "artifacts"), 0o700); err != nil {
			return err
		}
		now := time.Now().UTC()
		task.SchemaVersion = stateSchemaVersion
		task.Status = TaskQueued
		task.CreatedAt = now
		task.UpdatedAt = now
		task.Revision = 1
		task.Events = append(task.Events, StateEvent{ID: 1, At: now, Type: "created", Summary: "Task created"})
		return s.write(task)
	})
}

func (s *Store) Read(id string) (Task, error) {
	if !safeID.MatchString(id) {
		return Task{}, fmt.Errorf("invalid task id %q", id)
	}
	data, err := os.ReadFile(s.statePath(id)) //#nosec G304 -- task ids are restricted by safeID
	if err != nil {
		if os.IsNotExist(err) {
			return Task{}, ErrTaskNotFound
		}
		return Task{}, err
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return Task{}, fmt.Errorf("decode task %q: %w", id, err)
	}
	if task.SchemaVersion != stateSchemaVersion || task.ID != id {
		return Task{}, fmt.Errorf("task %q has unsupported or inconsistent state", id)
	}
	return task, nil
}

func (s *Store) List() ([]Task, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "tasks"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	tasks := make([]Task, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !safeID.MatchString(entry.Name()) {
			continue
		}
		task, err := s.Read(entry.Name())
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Reconcile marks a running task that has lost its exact worker identity as
// requiring intervention. Workspace and artifact references are retained.
//
//nolint:gocognit // This function preserves a multi-step state transition as one auditable unit.
func (s *Store) Reconcile(id string) (Task, error) {
	task, err := s.Read(id)
	if err != nil {
		return Task{}, err
	}
	if task.Status == TaskQueued && len(task.Attempts) > 0 {
		last := task.Attempts[len(task.Attempts)-1]
		if last.Status == "starting" && last.ProcessStart != "" {
			if identity, found := FindWorker(last.ProcessStart); found {
				if err := s.RecordAttemptWorker(id, last.ID, identity); err != nil {
					return Task{}, err
				}
				if err := s.AttachWorker(id, identity); err != nil {
					return Task{}, err
				}
				return s.Read(id)
			}
			return s.interrupt(id, "worker did not complete startup")
		}
	}
	if task.Status != TaskRunning {
		if task.Status == TaskCancelling {
			if len(task.Attempts) > 0 {
				last := task.Attempts[len(task.Attempts)-1]
				if identity, found := attemptWorker(last); found {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := StopWorkerGroup(ctx, identity); err != nil {
						return Task{}, err
					}
				}
			}
			return s.finishCancelled(id)
		}
		return task, nil
	}
	if len(task.Attempts) == 0 {
		return s.interrupt(id, "running task has no attempt record")
	}
	last := task.Attempts[len(task.Attempts)-1]
	identity, found := attemptWorker(last)
	if found && identity.PID == last.ProcessID {
		return task, nil
	}
	if found {
		if err := s.RecordAttemptWorker(id, last.ID, identity); err != nil {
			return Task{}, err
		}
		if err := s.AttachWorker(id, identity); err != nil {
			return Task{}, err
		}
		return s.Read(id)
	}
	return s.interrupt(id, "worker process identity is absent or no longer matches")
}

func (s *Store) finishCancelled(id string) (Task, error) {
	if err := s.Update(id, "cancelled", "Cancellation reconciled after worker exit", func(task *Task) error {
		task.Status = TaskCancelled
		if len(task.Attempts) > 0 {
			last := &task.Attempts[len(task.Attempts)-1]
			last.Status = string(TaskCancelled)
			last.FinishedAt = time.Now().UTC()
		}
		return nil
	}); err != nil {
		return Task{}, err
	}
	if err := s.ReleaseCapacity(id); err != nil {
		return Task{}, err
	}
	return s.Read(id)
}

func (s *Store) interrupt(id, reason string) (Task, error) {
	err := s.Update(id, "worker_interrupted", reason, func(task *Task) error {
		task.Status = TaskInterventionRequired
		if len(task.Attempts) > 0 {
			last := &task.Attempts[len(task.Attempts)-1]
			last.Status = "interrupted"
			last.FinishedAt = time.Now().UTC()
			last.Error = reason
		}
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	if err := s.ReleaseCapacity(id); err != nil {
		return Task{}, err
	}
	return s.Read(id)
}

// Cancel publishes the request before signaling the worker and records the
// terminal outcome only after the process group is stopped or proven stale.
func (s *Store) Cancel(ctx context.Context, id string) error {
	task, err := s.Read(id)
	if err != nil {
		return err
	}
	switch task.Status {
	case TaskCompleted, TaskFailed, TaskCancelled, TaskResultReady, TaskAccepted:
		return fmt.Errorf("task %q is not cancellable in state %q", id, task.Status)
	}
	if task.Status != TaskCancelling {
		if err := s.Update(id, "cancel_requested", "Cancellation requested", func(task *Task) error {
			task.Status = TaskCancelling
			return nil
		}); err != nil {
			return err
		}
	}
	task, err = s.Read(id)
	if err != nil {
		return err
	}
	if task.Worker != nil && WorkerMatches(*task.Worker) {
		if err := StopWorkerGroup(ctx, *task.Worker); err != nil {
			return err
		}
	} else if len(task.Attempts) > 0 {
		last := task.Attempts[len(task.Attempts)-1]
		if identity, found := attemptWorker(last); found {
			if err := StopWorkerGroup(ctx, identity); err != nil {
				return err
			}
		}
	}
	if err := s.Update(id, "cancelled", "Task cancelled by principal", func(task *Task) error {
		task.Status = TaskCancelled
		if len(task.Attempts) > 0 {
			last := &task.Attempts[len(task.Attempts)-1]
			last.Status = "cancelled"
			last.FinishedAt = time.Now().UTC()
		}
		return nil
	}); err != nil {
		return err
	}
	return s.ReleaseCapacity(id)
}

// SetWorker records the task-owned process group even before its first ACP attempt,
// so a queued worker waiting on provider availability can still be cancelled.
func (s *Store) SetWorker(id string, identity ProcessIdentity) error {
	if !WorkerMatches(identity) {
		return ErrStaleWorker
	}
	return s.Update(id, "worker_started", "Task worker process group recorded", func(task *Task) error {
		if task.Worker != nil && WorkerMatches(*task.Worker) {
			return fmt.Errorf("task %q already has a live worker", id)
		}
		copy := identity
		task.Worker = &copy
		return nil
	})
}

func attemptWorker(attempt Attempt) (ProcessIdentity, bool) {
	identity := ProcessIdentity{PID: attempt.ProcessID, GroupID: attempt.ProcessID, StartToken: attempt.ProcessStart}
	if WorkerMatches(identity) {
		return identity, true
	}
	return FindWorker(attempt.ProcessStart)
}

func (s *Store) Update(id, eventType, summary string, mutate func(*Task) error) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid task id %q", id)
	}
	return s.WithLock("task-"+id, func() error {
		task, err := s.Read(id)
		if err != nil {
			return err
		}
		oldStatus := task.Status
		if err := mutate(&task); err != nil {
			return err
		}
		if !validTaskTransition(oldStatus, task.Status) {
			return fmt.Errorf("invalid task state transition %q -> %q", oldStatus, task.Status)
		}
		task.Revision++
		task.UpdatedAt = time.Now().UTC()
		task.Events = append(task.Events, StateEvent{ID: task.Revision, At: task.UpdatedAt, Type: eventType, Summary: summary})
		return s.write(task)
	})
}

func (s *Store) AcceptResult(id, note string) error {
	return s.Update(id, "principal_accepted", "Principal accepted the retained result", func(task *Task) error {
		if task.Status != TaskResultReady {
			return fmt.Errorf("task result is not awaiting acceptance")
		}
		task.Status = TaskAccepted
		decision := PrincipalDecision{Action: "accepted", AcceptedAt: time.Now().UTC(), Note: note}
		task.PrincipalDecision = &decision
		task.PrincipalDecisions = append(task.PrincipalDecisions, decision)
		return nil
	})
}

func (s *Store) RequestCorrection(id, correction string) error {
	correction = strings.TrimSpace(correction)
	if correction == "" {
		return fmt.Errorf("correction instructions are required")
	}
	return s.Update(id, "principal_correction", "Principal requested a correction to the retained result", func(task *Task) error {
		if task.Status != TaskResultReady && task.Status != TaskAccepted {
			return fmt.Errorf("task has no result available for correction")
		}
		task.Status = TaskQueued
		task.Correction = correction
		decision := PrincipalDecision{Action: "correction_requested", CorrectedAt: time.Now().UTC(), Note: correction}
		task.PrincipalDecision = &decision
		task.PrincipalDecisions = append(task.PrincipalDecisions, decision)
		return nil
	})
}

func (s *Store) RecordWorkspace(id, projectRoot string, workspace Workspace, selectedChanges []string, configSnapshot, policySnapshot json.RawMessage) error {
	if !json.Valid(configSnapshot) || !json.Valid(policySnapshot) {
		return fmt.Errorf("configuration and permission policy snapshots must be valid JSON")
	}
	configDigest := sha256.Sum256(configSnapshot)
	policyDigest := sha256.Sum256(policySnapshot)
	return s.Update(id, "workspace_prepared", "Isolated workspace created from recorded source", func(task *Task) error {
		task.ConfigSnapshot = append(json.RawMessage(nil), configSnapshot...)
		task.PermissionPolicySnapshot = append(json.RawMessage(nil), policySnapshot...)
		task.Source = SourceSnapshot{
			ProjectRoot: projectRoot, Commit: workspace.Commit, Branch: workspace.Branch, Workspace: workspace.Path,
			SelectedChanges: append([]string(nil), selectedChanges...),
			ConfigDigest:    hex.EncodeToString(configDigest[:]), PolicyDigest: hex.EncodeToString(policyDigest[:]),
		}
		return nil
	})
}

func (s *Store) RecordAttemptWorker(id, attemptID string, identity ProcessIdentity) error {
	return s.Update(id, "worker_started", "Task worker process started", func(task *Task) error {
		if len(task.Attempts) == 0 || task.Attempts[len(task.Attempts)-1].ID != attemptID {
			return fmt.Errorf("attempt %q is not the task's current attempt", attemptID)
		}
		if identity.PID <= 0 || identity.GroupID != identity.PID || identity.StartToken == "" {
			return fmt.Errorf("worker identity is incomplete")
		}
		last := &task.Attempts[len(task.Attempts)-1]
		last.ProcessID = identity.PID
		last.ProcessStart = identity.StartToken
		last.Status = string(TaskRunning)
		task.Status = TaskRunning
		return nil
	})
}

func (s *Store) RecordAttemptStarting(id string, attempt Attempt, token string) error {
	if !regexpToken.MatchString(token) || !safeID.MatchString(attempt.ID) {
		return fmt.Errorf("attempt id or worker token is invalid")
	}
	return s.Update(id, "attempt_starting", "Attempt recorded before worker launch", func(task *Task) error {
		if task.Status != TaskQueued && task.Status != TaskAccepted {
			return fmt.Errorf("task %q cannot start an attempt from state %q", id, task.Status)
		}
		for _, current := range task.Attempts {
			if current.ID == attempt.ID {
				return fmt.Errorf("attempt %q already exists", attempt.ID)
			}
		}
		attempt.Status = "starting"
		attempt.ProcessStart = token
		task.Attempts = append(task.Attempts, attempt)
		return nil
	})
}

// BeginAgentAttempt binds the next provider configuration to the live task worker.
func (s *Store) BeginAgentAttempt(id string, attempt Attempt, identity ProcessIdentity) error {
	if !safeID.MatchString(attempt.ID) || attempt.Agent == "" || attempt.Model == "" {
		return fmt.Errorf("agent attempt identity, agent, and model are required")
	}
	if identity.PID > 0 && !WorkerMatches(identity) {
		return ErrStaleWorker
	}
	return s.Update(id, "agent_attempt_started", "ACP agent attempt started", func(task *Task) error {
		now := time.Now().UTC()
		if task.Status == TaskRunning && len(task.Attempts) > 0 {
			last := &task.Attempts[len(task.Attempts)-1]
			if last.ID == attempt.ID && last.Status == string(TaskRunning) {
				last.Agent, last.Model = attempt.Agent, attempt.Model
				if last.StartedAt.IsZero() {
					last.StartedAt = now
				}
				return nil
			}
		}
		if task.Status != TaskQueued && task.Status != TaskAccepted {
			return fmt.Errorf("task %q cannot start a new agent attempt from %q", id, task.Status)
		}
		for _, current := range task.Attempts {
			if current.ID == attempt.ID {
				return fmt.Errorf("attempt %q already exists", attempt.ID)
			}
		}
		attempt.Status, attempt.StartedAt = "running", now
		if identity.PID > 0 {
			attempt.ProcessID, attempt.ProcessStart = identity.PID, identity.StartToken
		}
		task.Attempts = append(task.Attempts, attempt)
		task.Status = TaskRunning
		return nil
	})
}

// RecordPermission appends a request after assigning it to the current attempt.
// Pending requests move the task to intervention_required before publication.
func (s *Store) RecordPermission(id string, record PermissionRecord) error {
	if record.RequestID == "" || record.AttemptID == "" || record.Capability == "" || len(record.Options) == 0 {
		return fmt.Errorf("permission request identity, capability, and options are required")
	}
	if record.Disposition != PermissionPending && record.Disposition != PermissionResolved {
		return fmt.Errorf("invalid permission disposition %q", record.Disposition)
	}
	if record.Disposition == "resolved" && (record.OptionID == "" || record.Source == "") {
		return fmt.Errorf("resolved permission requires an option and decision source")
	}
	return s.Update(id, "permission_"+record.Disposition, "ACP permission decision recorded", func(task *Task) error {
		if len(task.Attempts) == 0 || task.Attempts[len(task.Attempts)-1].ID != record.AttemptID {
			return fmt.Errorf("permission request belongs to a stale attempt")
		}
		for _, existing := range task.Permissions {
			if existing.RequestID == record.RequestID && existing.AttemptID == record.AttemptID {
				return fmt.Errorf("permission request already recorded")
			}
		}
		record.Options = append([]PermissionChoice(nil), record.Options...)
		record.CreatedAt = time.Now().UTC()
		task.Permissions = append(task.Permissions, record)
		if record.Disposition == PermissionPending {
			task.Status = TaskInterventionRequired
		}
		return nil
	})
}

// ResolvePermission records the exact offered option before a caller answers ACP.
func (s *Store) ResolvePermission(id, attemptID, requestID, optionID, source, rationale, policyVersion, policyRule string) error {
	if optionID == "" || source == "" {
		return fmt.Errorf("permission option and decision source are required")
	}
	if source != "principal" && source != PermissionUser {
		return fmt.Errorf("invalid permission decision source %q", source)
	}
	return s.Update(id, "permission_resolved", "ACP permission request resolved", func(task *Task) error {
		for i := range task.Permissions {
			record := &task.Permissions[i]
			if record.AttemptID != attemptID || record.RequestID != requestID {
				continue
			}
			if record.Disposition != PermissionPending {
				return fmt.Errorf("permission request is no longer pending")
			}
			if policyVersion != record.PolicyVersion && source != "user" {
				return fmt.Errorf("permission policy changed while request was pending")
			}
			if !hasPermissionChoice(record.Options, optionID) {
				return fmt.Errorf("option %q was not offered for this request", optionID)
			}
			record.Disposition = "resolved"
			record.OptionID = optionID
			record.Source = source
			record.Rationale = rationale
			record.PolicyVersion = policyVersion
			record.PolicyRule = policyRule
			record.ResolvedAt = time.Now().UTC()
			if task.Status == TaskInterventionRequired {
				task.Status = TaskRunning
			}
			return nil
		}
		return fmt.Errorf("permission request is not outstanding for this attempt")
	})
}

// MarkPermissionStale records that policy changed after the request was shown.
func (s *Store) MarkPermissionStale(id, attemptID, requestID, currentPolicyVersion string) error {
	return s.Update(id, "permission_stale", "Pending ACP permission invalidated by policy change", func(task *Task) error {
		for i := range task.Permissions {
			record := &task.Permissions[i]
			if record.AttemptID == attemptID && record.RequestID == requestID && record.Disposition == "pending" {
				record.Disposition = "stale"
				record.Escalation = "Permission policy changed while this request was pending; the old choices cannot be applied."
				record.PolicyRule = "invalidated; current policy version " + currentPolicyVersion
				record.ResolvedAt = time.Now().UTC()
				return nil
			}
		}
		return fmt.Errorf("permission request is not outstanding for this attempt")
	})
}

func hasPermissionChoice(options []PermissionChoice, optionID string) bool {
	for _, option := range options {
		if option.OptionID == optionID {
			return true
		}
	}
	return false
}

// ResolvePermissionAndWake publishes the decision before notifying the worker.
func (s *Store) ResolvePermissionAndWake(id, attemptID, requestID, optionID, source, rationale, policyVersion, policyRule string) error {
	if err := s.ResolvePermission(id, attemptID, requestID, optionID, source, rationale, policyVersion, policyRule); err != nil {
		return err
	}
	return s.WakeWorker(id)
}

// ResolvePermissionWithPolicy fences answers against the live policy version.
func (s *Store) ResolvePermissionWithPolicy(id, attemptID, requestID, optionID, source, rationale, currentPolicyVersion, policyRule string) error {
	task, err := s.Read(id)
	if err != nil {
		return err
	}
	for _, record := range task.Permissions {
		if record.AttemptID != attemptID || record.RequestID != requestID || record.Disposition != "pending" {
			continue
		}
		if record.PolicyVersion != currentPolicyVersion {
			if err := s.MarkPermissionStale(id, attemptID, requestID, currentPolicyVersion); err != nil {
				return err
			}
			if err := s.WakeWorker(id); err != nil {
				return fmt.Errorf("permission marked stale but worker notification failed: %w", err)
			}
			return fmt.Errorf("permission policy changed; request marked stale")
		}
		return s.ResolvePermissionAndWake(id, attemptID, requestID, optionID, source, rationale, currentPolicyVersion, policyRule)
	}
	return fmt.Errorf("permission request is not outstanding for this attempt")
}

func validTaskTransition(from, to TaskStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case TaskQueued:
		return to == TaskRunning || to == TaskCancelling || to == TaskCancelled || to == TaskInterventionRequired
	case TaskRunning:
		return to == TaskQueued || to == TaskCancelling || to == TaskCancelled || to == TaskInterventionRequired || to == TaskResultReady || to == TaskFailed
	case TaskCancelling:
		return to == TaskCancelled
	case TaskInterventionRequired:
		return to == TaskQueued || to == TaskRunning || to == TaskCancelling || to == TaskCancelled || to == TaskFailed
	case TaskResultReady:
		return to == TaskAccepted || to == TaskQueued
	case TaskAccepted:
		return to == TaskCompleted || to == TaskRunning || to == TaskQueued
	default:
		return false
	}
}

func (s *Store) WithLock(name string, fn func() error) error {
	if !safeID.MatchString(name) {
		return fmt.Errorf("invalid lock name %q", name)
	}
	lockDir := filepath.Join(s.root, "locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return err
	}
	return withFileLock(filepath.Join(lockDir, name+".lock"), fn)
}

func (s *Store) taskDir(id string) string   { return filepath.Join(s.root, "tasks", id) }
func (s *Store) statePath(id string) string { return filepath.Join(s.taskDir(id), "state.json") }

func (s *Store) write(task Task) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.statePath(task.ID), append(data, '\n'))
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // remove only if rename did not consume it
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return syncDirectoryTree(dir)
}

func syncDirectoryTree(dir string) error {
	for current := dir; ; current = filepath.Dir(current) {
		dirHandle, err := os.Open(current) //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
		if err != nil {
			return err
		}
		syncErr := dirHandle.Sync()
		closeErr := dirHandle.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}
