package main

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/delegation"
)

type delegatedTaskView struct {
	ID                     string                  `json:"id"`
	Status                 delegation.TaskStatus   `json:"status"`
	Role                   string                  `json:"role"`
	Agent                  string                  `json:"agent,omitempty"`
	Model                  string                  `json:"model,omitempty"`
	CreatedAt              string                  `json:"created_at"`
	LatestAttemptStartedAt string                  `json:"latest_attempt_started_at,omitempty"`
	Plan                   string                  `json:"plan,omitempty"`
	Phase                  string                  `json:"phase,omitempty"`
	Workspace              string                  `json:"workspace,omitempty"`
	Deadline               string                  `json:"deadline,omitempty"`
	PendingPermissions     []string                `json:"pending_permissions,omitempty"`
	PendingPermissionCount int                     `json:"pending_permission_count,omitempty"`
	TraceArtifacts         []string                `json:"trace_artifacts,omitempty"`
	RecentActivity         []delegation.StateEvent `json:"recent_activity,omitempty"`
	ResultReady            bool                    `json:"result_ready"`
	NextAction             string                  `json:"next_action"`
}

type delegatedTasksResponse struct {
	Tasks   []delegatedTaskView `json:"tasks"`
	Omitted int                 `json:"omitted,omitempty"`
}

const maxViewerDelegatedTasks = 100

func (s *server) handleDelegationTasks(w http.ResponseWriter, r *http.Request) {
	if !s.delegationReadOnly {
		writeErr(w, http.StatusForbidden, "delegated task status is disabled on a non-loopback bind host")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeErr(w, http.StatusMethodNotAllowed, "this endpoint takes a GET")
		return
	}
	if !agentOrigin(r) {
		writeErr(w, http.StatusForbidden, errAgentOrigin)
		return
	}
	tasks, err := delegation.NewStore(s.root).List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delegated task status could not be read")
		return
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt.After(tasks[j].CreatedAt) })
	response := delegatedTasksResponse{Tasks: make([]delegatedTaskView, 0, min(len(tasks), maxViewerDelegatedTasks))}
	if len(tasks) > maxViewerDelegatedTasks {
		response.Omitted = len(tasks) - maxViewerDelegatedTasks
		tasks = tasks[:maxViewerDelegatedTasks]
	}
	for _, task := range tasks {
		response.Tasks = append(response.Tasks, delegatedTaskViewFor(task, s.root))
	}
	writeJSON(w, http.StatusOK, response)
}

func delegatedTaskViewFor(task delegation.Task, root string) delegatedTaskView {
	view := delegatedTaskView{
		ID: task.ID, Status: task.Status, Role: task.Role,
		Plan:        task.PlanReference,
		Phase:       task.PlanPhase,
		ResultReady: task.Status == delegation.TaskResultReady || task.Status == delegation.TaskAccepted,
		NextAction:  delegationNextAction(task.Status),
	}
	if !task.CreatedAt.IsZero() {
		view.CreatedAt = task.CreatedAt.UTC().Format(time.RFC3339)
	}
	if len(task.Attempts) > 0 {
		last := task.Attempts[len(task.Attempts)-1]
		view.Agent, view.Model = last.Agent, last.Model
		if !last.StartedAt.IsZero() {
			view.LatestAttemptStartedAt = last.StartedAt.UTC().Format(time.RFC3339)
		}
	}
	if !task.Deadline.IsZero() {
		view.Deadline = task.Deadline.UTC().Format("2006-01-02T15:04:05Z")
	}
	view.Workspace = delegatedWorkspace(root, task.Source.Workspace)
	addPendingPermissions(&view, task.Permissions)
	addTraceArtifacts(&view, task.Artifacts)
	addRecentActivity(&view, task.Events)
	return view
}

func delegatedWorkspace(root, workspace string) string {
	rel, err := filepath.Rel(root, workspace)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func addPendingPermissions(view *delegatedTaskView, permissions []delegation.PermissionRecord) {
	for _, permission := range permissions {
		if permission.Disposition != delegation.PermissionPending {
			continue
		}
		view.PendingPermissionCount++
		if len(view.PendingPermissions) < 20 {
			view.PendingPermissions = append(view.PendingPermissions, permission.RequestID)
		}
	}
}

func addRecentActivity(view *delegatedTaskView, events []delegation.StateEvent) {
	start := len(events) - 5
	if start < 0 {
		start = 0
	}
	for _, event := range events[start:] {
		view.RecentActivity = append(view.RecentActivity, delegation.StateEvent{ID: event.ID, At: event.At, Type: event.Type})
	}
}

func addTraceArtifacts(view *delegatedTaskView, artifacts []delegation.Artifact) {
	for _, artifact := range artifacts {
		if validTraceArtifactName(artifact.Name) {
			view.TraceArtifacts = append(view.TraceArtifacts, artifact.Name)
		}
	}
}

func delegationNextAction(status delegation.TaskStatus) string {
	switch status {
	case delegation.TaskQueued, delegation.TaskRunning:
		return "wait or inspect status"
	case delegation.TaskInterventionRequired:
		return "inspect the failure and decide whether to correct or reassign"
	case delegation.TaskResultReady:
		return "review the result, then accept or request a correction"
	case delegation.TaskAccepted:
		return "preview and digest-check apply"
	case delegation.TaskCompleted:
		return "verify the applied changes and update the linked plan"
	case delegation.TaskCancelling:
		return "confirm worker shutdown"
	default:
		return "inspect task status"
	}
}
