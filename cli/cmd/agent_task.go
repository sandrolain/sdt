//go:build unix

package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/delegation"
	"github.com/spf13/cobra"
)

const (
	delegationFieldTaskID = "task_id"
	delegationFieldWorker = "worker"
	permissionDecisionAsk = "ask"
)

var agentTaskCmd = &cobra.Command{
	Use: "task", Short: "Delegate and inspect asynchronous ACP tasks",
	Long: `Run bounded ACP tasks in retained Git worktrees.

Example project configuration (verify each configured server's ACP behavior):

  delegation:
    max_attempts: 3
    max_duration: 45m
    agents:
      - name: reviewer-a
        command: ["your-agent", "acp"]
        model: provider/model
        roles: [reviewer]
        capabilities: [file-read, search]
        concurrency: 1
        cooldown: 10m

Token usage, provider cost, and ACP communication bytes remain unknown unless
the server reports measurable values. Results require principal acceptance or
an explicit correction; cleanup never runs automatically.

Routing filters the configured agent order by an existing SDT role and every
requested capability, then enforces entry and shared-group concurrency. The
delegate receives a separate Git worktree; this is not an OS sandbox for tools
the agent starts. Permission rules are reevaluated from the live project config
at each authorization, default to ask, and pending choices are resolved by
their exact offered option id.

Automatic continuation uses structured ACP quota or provider-unavailable
signals only. Authentication, permission, unknown, and ordinary task failures
require principal action. Provider/model compatibility is specific to the
configured server version and is not inferred from ACP support alone.`,
}

type taskStartResult struct {
	ID        string                     `json:"id" yaml:"id"`
	Status    delegation.TaskStatus      `json:"status" yaml:"status"`
	Role      string                     `json:"role" yaml:"role"`
	Deadline  time.Time                  `json:"deadline" yaml:"deadline"`
	Workspace string                     `json:"workspace" yaml:"workspace"`
	Worker    delegation.ProcessIdentity `json:"worker" yaml:"worker"`
}

var agentTaskStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a bounded delegated task",
	Long: `Create a retained worktree and start an asynchronous ACP task.

The project must declare delegation.agents, max_attempts, and max_duration in
.sdt.yaml. Candidates are tried in declaration order after role/capability
filtering. Example: sdt agent task start --role reviewer --capability file-read --brief "Review this change".`,
	RunE: runAgentTaskStart,
}

func init() {
	agentTaskStartCmd.Flags().String("role", "", "Existing SDT role for the task")
	agentTaskStartCmd.Flags().String("brief", "", "Task objective and acceptance criteria")
	agentTaskStartCmd.Flags().StringSlice("capability", nil, "Indispensable capability (repeatable)")
	agentTaskStartCmd.Flags().String("commit", "HEAD", "Identified source commit")
	agentTaskStartCmd.Flags().StringSlice("include", nil, "Explicit project-relative dirty file to include (repeatable)")
	agentTaskStartCmd.Flags().StringSlice("reference", nil, "Project-relative reference file to copy into the stable task workspace (repeatable)")
	agentTaskStartCmd.Flags().String("id", "", "Task id (generated when omitted)")

	agentTaskCmd.AddCommand(agentTaskStartCmd, agentTaskStatusCmd, agentTaskResultCmd, agentTaskCancelCmd, agentTaskResumeCmd, agentTaskAcceptCmd, agentTaskCorrectCmd, agentTaskCleanupCmd, agentTaskWorkerCmd, agentTaskPermissionsCmd, agentTaskAvailabilityCmd)
	agentCmd.AddCommand(agentTaskCmd)
}

func runAgentTaskStart(cmd *cobra.Command, _ []string) (runErr error) {
	cfg, projectRoot, err := findProjectConfigAt()
	if err != nil {
		return fmt.Errorf("delegation requires a valid project .sdt.yaml: %w", err)
	}
	if cfg == nil {
		return fmt.Errorf("delegation requires a valid project .sdt.yaml")
	}
	if err := validateDelegationConfig(cfg.Delegation); err != nil {
		return err
	}
	if len(cfg.Delegation.Agents) == 0 {
		return fmt.Errorf("no delegation.agents are configured")
	}
	role := getStringFlag(cmd, "role", false)
	brief := strings.TrimSpace(getStringFlag(cmd, "brief", false))
	if role == "" || brief == "" {
		return fmt.Errorf("--role and --brief are required")
	}
	if _, ok := roleLookup(role); !ok {
		return fmt.Errorf("unknown SDT role %q", role)
	}
	capabilities, err := cmd.Flags().GetStringSlice("capability")
	if err != nil {
		return err
	}
	candidates := delegationCandidates(cfg.Delegation, role, capabilities)
	if len(candidates) == 0 {
		return fmt.Errorf("no configured agent matches role %q and required capabilities", role)
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	taskID := getStringFlag(cmd, "id", false)
	if taskID == "" {
		taskID = strings.ReplaceAll(newUID(), "-", "")
	}
	commit := getStringFlag(cmd, "commit", false)
	selected, err := cmd.Flags().GetStringSlice("include")
	if err != nil {
		return err
	}
	references, err := cmd.Flags().GetStringSlice("reference")
	if err != nil {
		return err
	}
	store := delegation.NewStore(root)
	duration, err := time.ParseDuration(cfg.Delegation.MaxDuration)
	if err != nil {
		return fmt.Errorf("invalid delegation.max_duration: %w", err)
	}
	task := delegation.Task{ID: taskID, Role: role, RequiredCapabilities: capabilities, Deadline: time.Now().Add(duration).UTC()}
	if err := store.Create(task); err != nil {
		return err
	}
	launched := false
	defer func() {
		if !launched && runErr != nil {
			_ = store.Update(taskID, "start_failed", "Task setup failed before a worker was launched: "+runErr.Error(), func(task *delegation.Task) error { task.Status = delegation.TaskInterventionRequired; return nil }) //nolint:errcheck // Preserve the original setup error returned to the caller.
		}
	}()
	workspace, err := delegation.PrepareWorkspace(cmd.Context(), root, taskID, commit, selected)
	if err != nil {
		return err
	}
	configBytes, err := json.Marshal(cfg.Delegation)
	if err != nil {
		return err
	}
	policyBytes, err := json.Marshal(cfg.Delegation.PermissionPolicy)
	if err != nil {
		return err
	}
	if err := store.RecordWorkspace(taskID, root, workspace, selected, configBytes, policyBytes); err != nil {
		return err
	}
	briefPath := filepath.Join(store.Root(), "tasks", taskID, "artifacts", "brief.md")
	stableBrief := renderDelegationBrief(delegationBrief{
		Goal: brief, Scope: "Role: " + role + "\nRequired capabilities: " + strings.Join(capabilities, ", "),
		References:  append([]string{"Task " + taskID, "Source commit " + workspace.Commit, "Workspace " + workspace.Path}, delegationReferenceLabels(references)...),
		Constraints: []string{"Work only in the retained task worktree.", "Do not claim checks that were not run."},
		Acceptance:  []string{"Report findings, changes, evidence, blockers, remaining work, and next action for principal review."},
		Role:        role, RoleProfile: filepath.ToSlash(filepath.Join("context", "roles", role+".md")),
	})
	if err := os.WriteFile(briefPath, []byte(stableBrief), 0o600); err != nil {
		return err
	}
	if err := store.Update(taskID, "brief_saved", "Stable task brief retained", func(task *delegation.Task) error {
		task.Artifacts = append(task.Artifacts, delegation.Artifact{Name: "brief", Path: briefPath})
		return nil
	}); err != nil {
		return err
	}
	if err := materializeDelegationRole(root, workspace.Path, role); err != nil {
		return err
	}
	referenceSnapshots, err := materializeDelegationReferences(root, workspace.Path, references)
	if err != nil {
		return err
	}
	if len(referenceSnapshots) > 0 {
		if err := store.Update(taskID, "references_materialized", "Stable reference copies and digests recorded", func(task *delegation.Task) error { task.Source.References = referenceSnapshots; return nil }); err != nil {
			return err
		}
	}
	identity, err := launchTaskWorker(store, root, taskID, workspace.Path, candidates[0].Name)
	if err != nil {
		return err
	}
	launched = true
	outputTaskStart(cmd, taskStartResult{ID: taskID, Status: delegation.TaskQueued, Role: role, Deadline: task.Deadline, Workspace: workspace.Path, Worker: identity})
	return nil
}

func delegationReferenceLabels(paths []string) []string {
	labels := make([]string, 0, len(paths))
	for _, path := range paths {
		labels = append(labels, filepath.ToSlash(filepath.Join("context", "delegation-references", path)))
	}
	return labels
}

func materializeDelegationReferences(projectRoot, workspace string, references []string) ([]delegation.ReferenceSnapshot, error) {
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return nil, err
	}
	workspaceRoot, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return nil, err
	}
	var snapshots []delegation.ReferenceSnapshot
	for _, reference := range references {
		if filepath.IsAbs(reference) {
			return nil, fmt.Errorf("reference %q must be project-relative", reference)
		}
		clean := filepath.Clean(reference)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("reference %q escapes the project", reference)
		}
		source, err := filepath.EvalSymlinks(filepath.Join(root, clean))
		if err != nil {
			return nil, fmt.Errorf("resolve reference %q: %w", reference, err)
		}
		if !withinRoot(root, source) {
			return nil, fmt.Errorf("reference %q escapes the project through a symlink", reference)
		}
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("reference %q must be a regular file", reference)
		}
		target := filepath.Join(workspaceRoot, "context", "delegation-references", clean)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return nil, err
		}
		if err := copyDelegationReference(source, target); err != nil {
			return nil, fmt.Errorf("copy stable reference %q: %w", reference, err)
		}
		content, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(content)
		snapshots = append(snapshots, delegation.ReferenceSnapshot{Source: filepath.ToSlash(clean), WorkspacePath: filepath.ToSlash(filepath.Join("context", "delegation-references", clean)), SHA256: hex.EncodeToString(digest[:])})
	}
	return snapshots, nil
}

func outputTaskStart(cmd *cobra.Command, result taskStartResult) {
	switch getFormat(cmd) {
	case fmtJSON:
		data, err := json.Marshal(result)
		exitWithError(cmd, err)
		outputBytes(cmd, data)
	case fmtYAML:
		data, err := yaml.Marshal(result)
		exitWithError(cmd, err)
		outputBytes(cmd, data)
	default:
		outputString(cmd, fmt.Sprintf("Task %s queued for role %s\nWorkspace: %s\nDeadline: %s\n", result.ID, result.Role, result.Workspace, result.Deadline.Format(time.RFC3339)))
	}
}

var agentTaskStatusCmd = &cobra.Command{Use: "status <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskStatus}
var agentTaskResultCmd = &cobra.Command{Use: "result <task-id> [artifact]", Args: cobra.RangeArgs(1, 2), RunE: runAgentTaskResult}
var agentTaskCancelCmd = &cobra.Command{Use: "cancel <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskCancel}
var agentTaskResumeCmd = &cobra.Command{Use: "resume <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskResume}

func taskStore() (*delegation.Store, error) {
	_, root, err := findProjectConfigAt()
	if err != nil {
		return nil, fmt.Errorf("delegation requires a valid project .sdt.yaml: %w", err)
	}
	if root == "" {
		return nil, fmt.Errorf("delegation requires a valid project .sdt.yaml")
	}
	return delegation.NewStore(root), nil
}

func runAgentTaskStatus(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Reconcile(args[0])
	if err != nil {
		return err
	}
	return outputTask(cmd, boundedTaskStatus(task))
}

type taskStatusSummary struct {
	ID                 string                  `json:"id" yaml:"id"`
	Status             delegation.TaskStatus   `json:"status" yaml:"status"`
	Role               string                  `json:"role" yaml:"role"`
	Deadline           time.Time               `json:"deadline" yaml:"deadline"`
	AttemptCount       int                     `json:"attempt_count" yaml:"attempt_count"`
	Attempts           []delegation.Attempt    `json:"attempts,omitempty" yaml:"attempts,omitempty"`
	PendingPermissions []string                `json:"pending_permissions,omitempty" yaml:"pending_permissions,omitempty"`
	Artifacts          []delegation.Artifact   `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	RecentEvents       []delegation.StateEvent `json:"recent_events,omitempty" yaml:"recent_events,omitempty"`
	Metrics            taskMetricsSummary      `json:"metrics" yaml:"metrics"`
}

type taskMetricsSummary struct {
	AttemptElapsedMS   int64  `json:"attempt_elapsed_ms" yaml:"attempt_elapsed_ms"`
	PrincipalTokens    string `json:"principal_tokens" yaml:"principal_tokens"`
	DelegateTokens     string `json:"delegate_tokens" yaml:"delegate_tokens"`
	PrincipalCostUSD   string `json:"principal_cost_usd" yaml:"principal_cost_usd"`
	DelegateCostUSD    string `json:"delegate_cost_usd" yaml:"delegate_cost_usd"`
	TotalCostUSD       string `json:"total_cost_usd" yaml:"total_cost_usd"`
	CommunicationBytes string `json:"communication_bytes" yaml:"communication_bytes"`
}

func boundedTaskStatus(task delegation.Task) taskStatusSummary {
	view := taskStatusSummary{ID: task.ID, Status: task.Status, Role: task.Role, Deadline: task.Deadline, AttemptCount: len(task.Attempts)}
	for _, p := range task.Permissions {
		if p.Disposition == delegation.PermissionPending {
			view.PendingPermissions = append(view.PendingPermissions, p.RequestID)
		}
	}
	if len(task.Events) > 5 {
		view.RecentEvents = append([]delegation.StateEvent(nil), task.Events[len(task.Events)-5:]...)
	} else {
		view.RecentEvents = append([]delegation.StateEvent(nil), task.Events...)
	}
	view.Attempts = append([]delegation.Attempt(nil), task.Attempts...)
	view.Artifacts = append([]delegation.Artifact(nil), task.Artifacts...)
	var elapsed time.Duration
	for _, attempt := range task.Attempts {
		if !attempt.StartedAt.IsZero() {
			end := attempt.FinishedAt
			if end.IsZero() {
				end = time.Now()
			}
			elapsed += end.Sub(attempt.StartedAt)
		}
	}
	view.Metrics = taskMetricsSummary{AttemptElapsedMS: elapsed.Milliseconds(), PrincipalTokens: learnProvenanceUnknown, DelegateTokens: learnProvenanceUnknown, PrincipalCostUSD: learnProvenanceUnknown, DelegateCostUSD: learnProvenanceUnknown, TotalCostUSD: learnProvenanceUnknown, CommunicationBytes: learnProvenanceUnknown}
	if len(view.Attempts) > 3 {
		view.Attempts = append([]delegation.Attempt(nil), view.Attempts[len(view.Attempts)-3:]...)
	}
	if len(view.Artifacts) > 5 {
		view.Artifacts = append([]delegation.Artifact(nil), view.Artifacts[len(view.Artifacts)-5:]...)
	}
	return view
}

func runAgentTaskResult(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	if getBoolFlag(cmd, "log", false) {
		path := filepath.Join(store.Root(), "tasks", task.ID, "artifacts", "worker.log")
		data, err := os.ReadFile(path) //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
		if err != nil {
			return err
		}
		if getFormat(cmd) == formatText {
			outputString(cmd, string(data))
			return nil
		}
		return outputTask(cmd, map[string]string{delegationFieldTaskID: task.ID, "log": string(data)})
	}
	if len(task.Artifacts) == 0 {
		return fmt.Errorf("task %s has no retained artifacts", task.ID)
	}
	if len(args) == 2 {
		for _, artifact := range task.Artifacts {
			if artifact.Name == args[1] {
				data, err := os.ReadFile(artifact.Path)
				if err != nil {
					return err
				}
				if getFormat(cmd) == formatText {
					outputString(cmd, string(data))
					return nil
				}
				return outputTask(cmd, map[string]any{"artifact": artifact, "content": string(data)})
			}
		}
		return fmt.Errorf("artifact %q is not recorded for task %s", args[1], task.ID)
	}
	if getFormat(cmd) != formatText {
		return outputTask(cmd, task.Artifacts)
	}
	var out strings.Builder
	for _, artifact := range task.Artifacts {
		if artifact.Name == "worker.log" {
			continue
		}
		data, err := os.ReadFile(artifact.Path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "# %s\n\n%s\n", artifact.Name, string(data))
	}
	outputString(cmd, out.String())
	return nil
}

func runAgentTaskCancel(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	err = store.Cancel(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	return outputAction(cmd, "Task cancelled: "+args[0]+"\n", map[string]string{delegationFieldTaskID: args[0], ctxMapStatus: string(delegation.TaskCancelled)})
}

var agentTaskAcceptCmd = &cobra.Command{Use: "accept <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskAccept}
var agentTaskCorrectCmd = &cobra.Command{Use: "correct <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskCorrect}

func init() {
	agentTaskAcceptCmd.Flags().String("note", "", "Principal acceptance note")
	agentTaskCorrectCmd.Flags().String("instructions", "", "Correction instructions for a follow-up attempt")
}

func runAgentTaskAccept(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	if err := store.AcceptResult(args[0], getStringFlag(cmd, "note", false)); err != nil {
		return err
	}
	return outputAction(cmd, "Result accepted: "+args[0]+"\n", map[string]string{delegationFieldTaskID: args[0], ctxMapStatus: string(delegation.TaskAccepted)})
}

func runAgentTaskCorrect(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	instructions := strings.TrimSpace(getStringFlag(cmd, "instructions", false))
	if instructions == "" {
		return fmt.Errorf("--instructions is required")
	}
	var snapshot DelegationConfig
	if err := json.Unmarshal(task.ConfigSnapshot, &snapshot); err != nil {
		return err
	}
	if len(task.Attempts) >= snapshot.MaxAttempts {
		return fmt.Errorf("task %s has exhausted its original aggregate attempt limit", task.ID)
	}
	candidates := delegationCandidates(snapshot, task.Role, task.RequiredCapabilities)
	if len(candidates) == 0 {
		return fmt.Errorf("task configuration snapshot has no eligible correction agent")
	}
	if task.Worker != nil && delegation.WorkerMatches(*task.Worker) {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if err := delegation.StopWorkerGroup(ctx, *task.Worker); err != nil {
			return err
		}
	}
	if err := store.RequestCorrection(task.ID, instructions); err != nil {
		return err
	}
	identity, err := launchTaskWorker(store, task.Source.ProjectRoot, task.ID, task.Source.Workspace, candidates[0].Name)
	if err != nil {
		return err
	}
	return outputAction(cmd, fmt.Sprintf("Correction queued for task %s (worker pid %d)\n", task.ID, identity.PID), map[string]any{delegationFieldTaskID: task.ID, ctxMapStatus: delegation.TaskQueued, delegationFieldWorker: identity})
}

var agentTaskCleanupCmd = &cobra.Command{Use: "cleanup", Short: "Preview or explicitly remove retained task resources"}
var agentTaskCleanupPreviewCmd = &cobra.Command{Use: "preview <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskCleanupPreview}
var agentTaskCleanupExecuteCmd = &cobra.Command{Use: "execute <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskCleanupExecute}

func init() { agentTaskCleanupCmd.AddCommand(agentTaskCleanupPreviewCmd, agentTaskCleanupExecuteCmd) }

type cleanupPreview struct {
	TaskID         string                `json:"task_id" yaml:"task_id"`
	Status         delegation.TaskStatus `json:"status" yaml:"status"`
	Worktree       string                `json:"worktree" yaml:"worktree"`
	Branch         string                `json:"branch" yaml:"branch"`
	Artifacts      []string              `json:"artifacts" yaml:"artifacts"`
	StateDirectory string                `json:"state_directory" yaml:"state_directory"`
	DirtyPaths     []string              `json:"dirty_paths,omitempty" yaml:"dirty_paths,omitempty"`
}

func makeCleanupPreview(store *delegation.Store, task delegation.Task) cleanupPreview {
	files := make([]string, 0, len(task.Artifacts))
	for _, artifact := range task.Artifacts {
		files = append(files, artifact.Path)
	}
	files = append(files, filepath.Join(store.Root(), "tasks", task.ID, "artifacts", "worker.log"))
	return cleanupPreview{TaskID: task.ID, Status: task.Status, Worktree: task.Source.Workspace, Branch: task.Source.Branch, Artifacts: files, StateDirectory: filepath.Join(store.Root(), "tasks", task.ID)}
}
func runAgentTaskCleanupPreview(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	preview := makeCleanupPreview(store, task)
	if task.Source.Workspace != "" {
		if output, err := exec.CommandContext(cmd.Context(), "git", "-C", task.Source.Workspace, "status", "--short").Output(); err == nil { //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if line != "" {
					preview.DirtyPaths = append(preview.DirtyPaths, strings.TrimSpace(line))
				}
			}
		}
	}
	return outputTask(cmd, preview)
}

//nolint:gocognit // This function preserves a multi-step state transition as one auditable unit.
func runAgentTaskCleanupExecute(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	if task.Status == delegation.TaskRunning || task.Status == delegation.TaskQueued || task.Status == delegation.TaskCancelling {
		return fmt.Errorf("active or queued task resources cannot be cleaned up")
	}
	if task.Worker != nil && delegation.WorkerMatches(*task.Worker) {
		return fmt.Errorf("task still has a live worker")
	}
	preview := makeCleanupPreview(store, task)
	if task.Source.Workspace != "" {
		root, err := filepath.Abs(task.Source.ProjectRoot)
		if err != nil {
			return err
		}
		workspace, err := filepath.Abs(task.Source.Workspace)
		if err != nil {
			return err
		}
		worktreeRoot := filepath.Join(root, ".sdt", "delegation", "worktrees")
		if workspace != filepath.Join(worktreeRoot, task.ID) || !withinRoot(worktreeRoot, workspace) {
			return fmt.Errorf("refusing to remove workspace outside delegation worktrees")
		}
		if task.Source.Branch != "sdt/delegation/"+task.ID {
			return fmt.Errorf("refusing to remove an unexpected task branch %q", task.Source.Branch)
		}
		if info, statErr := os.Lstat(workspace); statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to remove a symlink worktree path")
			}
			if output, err := exec.CommandContext(cmd.Context(), "git", "-C", root, "worktree", "remove", "--force", workspace).CombinedOutput(); err != nil { //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
				return fmt.Errorf("remove task worktree: %w: %s", err, strings.TrimSpace(string(output)))
			}
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		branch := "refs/heads/" + task.Source.Branch
		if err := exec.CommandContext(cmd.Context(), "git", "-C", root, "show-ref", "--verify", "--quiet", branch).Run(); err == nil { //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
			if output, err := exec.CommandContext(cmd.Context(), "git", "-C", root, "branch", "-D", task.Source.Branch).CombinedOutput(); err != nil { //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
				return fmt.Errorf("remove task branch: %w: %s", err, strings.TrimSpace(string(output)))
			}
		}
	}
	if err := os.RemoveAll(filepath.Join(store.Root(), "tasks", task.ID)); err != nil {
		return err
	}
	_ = store.ReleaseCapacity(task.ID) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	return outputAction(cmd, "Removed retained resources for task "+preview.TaskID+"\n", map[string]string{delegationFieldTaskID: preview.TaskID, ctxMapStatus: "cleaned"})
}

func runAgentTaskResume(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Reconcile(args[0])
	if err != nil {
		return err
	}
	if task.Status != delegation.TaskQueued && task.Status != delegation.TaskInterventionRequired {
		return fmt.Errorf("task %s is %s; only queued or intervention-required tasks can resume", task.ID, task.Status)
	}
	if task.Worker != nil && delegation.WorkerMatches(*task.Worker) {
		return fmt.Errorf("task %s already has a live worker", task.ID)
	}
	var snapshot DelegationConfig
	if err := json.Unmarshal(task.ConfigSnapshot, &snapshot); err != nil {
		return fmt.Errorf("decode task configuration snapshot: %w", err)
	}
	if len(task.Attempts) >= snapshot.MaxAttempts {
		return fmt.Errorf("task %s has exhausted its original aggregate attempt limit", task.ID)
	}
	if task.Status == delegation.TaskInterventionRequired {
		if err := store.Update(task.ID, "principal_resume", "Principal explicitly resumed the retained task", func(task *delegation.Task) error { task.Status = delegation.TaskQueued; return nil }); err != nil {
			return err
		}
	}
	if _, err := store.ReconcileCapacity(time.Now()); err != nil {
		return err
	}
	candidates := delegationCandidates(snapshot, task.Role, task.RequiredCapabilities)
	if len(candidates) == 0 {
		return fmt.Errorf("no candidate in the task configuration snapshot matches the role and capabilities")
	}
	if err := reconcileTaskRecoveryProbes(store, task.ID, candidates); err != nil {
		return err
	}
	identity, err := launchTaskWorker(store, task.Source.ProjectRoot, task.ID, task.Source.Workspace, candidates[0].Name)
	if err != nil {
		return err
	}
	return outputAction(cmd, fmt.Sprintf("Task %s resumed with worker pid %d\n", task.ID, identity.PID), map[string]any{delegationFieldTaskID: task.ID, ctxMapStatus: delegation.TaskQueued, delegationFieldWorker: identity})
}

func reconcileTaskRecoveryProbes(store *delegation.Store, taskID string, candidates []DelegationAgentConfig) error {
	probes, err := store.RecoveryProbes(taskID)
	if err != nil {
		return err
	}
	for _, probe := range probes {
		var candidate *DelegationAgentConfig
		for i := range candidates {
			if probe.Key == "entry:"+candidates[i].Name || (candidates[i].AvailabilityGroup != "" && probe.Key == "group:"+candidates[i].AvailabilityGroup) {
				candidate = &candidates[i]
				break
			}
		}
		if candidate == nil {
			return fmt.Errorf("cannot reconcile probe %q from the task configuration snapshot", probe.Key)
		}
		cooldown, err := time.ParseDuration(candidate.Cooldown)
		if err != nil || cooldown <= 0 {
			return fmt.Errorf("task candidate %q has no valid recovery cooldown", candidate.Name)
		}
		if err := store.ReconcileRecoveryProbe(probe.Key, taskID, candidate.Name, candidate.AvailabilityGroup, cooldown, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func launchTaskWorker(store *delegation.Store, projectRoot, taskID, workspace, agent string) (delegation.ProcessIdentity, error) {
	executable, err := os.Executable()
	if err != nil {
		return delegation.ProcessIdentity{}, err
	}
	logPath := filepath.Join(store.Root(), "tasks", taskID, "artifacts", "worker.log")
	identity, err := delegation.StartWorker(executable, projectRoot, logPath, "agent", "task", "worker", "--project-root", projectRoot, "--task", taskID, "--workspace", workspace, "--agent", agent)
	if err != nil {
		return delegation.ProcessIdentity{}, err
	}
	if err := store.SetWorker(taskID, identity); err != nil {
		return delegation.ProcessIdentity{}, err
	}
	return identity, nil
}

func outputTask(cmd *cobra.Command, value any) error {
	if getFormat(cmd) == formatText {
		if status, ok := value.(taskStatusSummary); ok {
			var b strings.Builder
			fmt.Fprintf(&b, "Task %s — %s (%s)\nAttempts: %d\nDeadline: %s\n", status.ID, status.Status, status.Role, status.AttemptCount, status.Deadline.Format(time.RFC3339))
			for _, request := range status.PendingPermissions {
				fmt.Fprintf(&b, "Pending permission: %s\n", request)
			}
			for _, artifact := range status.Artifacts {
				fmt.Fprintf(&b, "Artifact %s: %s\n", artifact.Name, artifact.Path)
			}
			fmt.Fprintf(&b, "Attempt elapsed: %d ms; principal tokens: %s; delegate tokens: %s; principal cost: %s USD; delegate cost: %s USD; total cost: %s USD; communication bytes: %s\n", status.Metrics.AttemptElapsedMS, status.Metrics.PrincipalTokens, status.Metrics.DelegateTokens, status.Metrics.PrincipalCostUSD, status.Metrics.DelegateCostUSD, status.Metrics.TotalCostUSD, status.Metrics.CommunicationBytes)
			outputString(cmd, b.String())
			return nil
		}
		switch rows := value.(type) {
		case []delegation.PermissionRecord:
			var b strings.Builder
			for _, row := range rows {
				fmt.Fprintf(&b, "%s %s — %s (%s)\n", row.RequestID, row.Disposition, row.Capability, row.AttemptID)
				for _, choice := range row.Options {
					fmt.Fprintf(&b, "  %s: %s\n", choice.OptionID, choice.Name)
				}
			}
			outputString(cmd, b.String())
			return nil
		case []delegation.AvailabilityExclusion:
			var b strings.Builder
			for _, row := range rows {
				fmt.Fprintf(&b, "%s %s — %s until %s\n", row.Key, row.Kind, row.Reason, row.Until.Format(time.RFC3339))
			}
			outputString(cmd, b.String())
			return nil
		case cleanupPreview:
			var b strings.Builder
			fmt.Fprintf(&b, "Task %s (%s)\nWorktree: %s\nBranch: %s\nState: %s\n", rows.TaskID, rows.Status, rows.Worktree, rows.Branch, rows.StateDirectory)
			for _, path := range rows.Artifacts {
				fmt.Fprintf(&b, "Artifact: %s\n", path)
			}
			for _, path := range rows.DirtyPaths {
				fmt.Fprintf(&b, "Dirty path: %s\n", path)
			}
			outputString(cmd, b.String())
			return nil
		}
	}
	var data []byte
	var err error
	switch getFormat(cmd) {
	case fmtJSON:
		data, err = json.Marshal(value)
	case fmtYAML:
		data, err = yaml.Marshal(value)
	default:
		data = []byte(fmt.Sprintf("%+v\n", value))
	}
	if err != nil {
		return err
	}
	outputBytes(cmd, data)
	return nil
}

func outputAction(cmd *cobra.Command, text string, value any) error {
	if getFormat(cmd) == formatText {
		outputString(cmd, text)
		return nil
	}
	return outputTask(cmd, value)
}

var agentTaskPermissionsCmd = &cobra.Command{Use: "permissions", Short: "Inspect or resolve ACP permission requests"}
var agentTaskPermissionListCmd = &cobra.Command{Use: "list <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskPermissionList}
var agentTaskPermissionResolveCmd = &cobra.Command{Use: "resolve <task-id> <request-id>", Args: cobra.ExactArgs(2), RunE: runAgentTaskPermissionResolve}

func init() {
	agentTaskPermissionsCmd.AddCommand(agentTaskPermissionListCmd, agentTaskPermissionResolveCmd)
	agentTaskPermissionResolveCmd.Flags().String("option", "", "Exact offered permission option id")
	agentTaskPermissionResolveCmd.Flags().String("rationale", "", "Principal decision rationale")
	agentTaskResultCmd.Flags().Bool("log", false, "Retrieve the retained worker log")
}

func runAgentTaskPermissionList(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	return outputTask(cmd, task.Permissions)
}

func runAgentTaskPermissionResolve(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	cfg, _, err := findProjectConfigAt()
	if err != nil {
		return err
	}
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	var request *delegation.PermissionRecord
	for i := range task.Permissions {
		if task.Permissions[i].RequestID == args[1] && task.Permissions[i].Disposition == "pending" {
			request = &task.Permissions[i]
			break
		}
	}
	if request == nil {
		return fmt.Errorf("permission request %s is not pending", args[1])
	}
	option := getStringFlag(cmd, "option", false)
	versionData, err := json.Marshal(cfg.Delegation.PermissionPolicy)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(versionData)
	version := hex.EncodeToString(digest[:])
	if err := store.ResolvePermissionWithPolicy(args[0], request.AttemptID, request.RequestID, option, "user", getStringFlag(cmd, "rationale", false), version, "principal decision"); err != nil {
		return err
	}
	return outputAction(cmd, "Permission decision recorded and worker notified.\n", map[string]string{"task_id": args[0], "request_id": args[1], "status": "resolved"})
}

var agentTaskAvailabilityCmd = &cobra.Command{Use: "availability", Short: "Inspect or reset provider availability exclusions"}
var agentTaskAvailabilityListCmd = &cobra.Command{Use: useList, RunE: runAgentTaskAvailabilityList}
var agentTaskAvailabilityResetCmd = &cobra.Command{Use: "reset <key>", Args: cobra.ExactArgs(1), RunE: runAgentTaskAvailabilityReset}

func init() {
	agentTaskAvailabilityCmd.AddCommand(agentTaskAvailabilityListCmd, agentTaskAvailabilityResetCmd)
}

func runAgentTaskAvailabilityList(cmd *cobra.Command, _ []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	items, err := store.Availability(time.Now())
	if err != nil {
		return err
	}
	return outputTask(cmd, items)
}
func runAgentTaskAvailabilityReset(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	if err := store.ResetAvailability(args[0]); err != nil {
		return err
	}
	return outputAction(cmd, "Availability exclusion reset: "+args[0]+"\n", map[string]string{"key": args[0], "status": "reset"})
}

var agentTaskWorkerCmd = &cobra.Command{Use: "worker", Hidden: true, RunE: runAgentTaskWorker}

func init() {
	agentTaskWorkerCmd.Flags().String("project-root", "", "Project root")
	agentTaskWorkerCmd.Flags().String("task", "", "Task id")
	agentTaskWorkerCmd.Flags().String("workspace", "", "Task workspace")
	agentTaskWorkerCmd.Flags().String("agent", "", "Initial candidate")
}

//nolint:gocognit,gocyclo // This function preserves a multi-step state transition as one auditable unit.
func runAgentTaskWorker(cmd *cobra.Command, args []string) error {
	token := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "--sdt-delegation-worker=") {
			token = strings.TrimPrefix(arg, "--sdt-delegation-worker=")
		}
	}
	if token == "" {
		return fmt.Errorf("worker identity token is missing")
	}
	root := getStringFlag(cmd, "project-root", false)
	taskID := getStringFlag(cmd, "task", false)
	workspace := getStringFlag(cmd, "workspace", false)
	cfg := loadProjectConfig(filepath.Join(root, sdtConfigFile))
	if cfg == nil {
		return fmt.Errorf("cannot load project delegation config")
	}
	store := delegation.NewStore(root)
	identity := delegation.ProcessIdentity{PID: os.Getpid(), GroupID: os.Getpid(), StartToken: token}
	var task delegation.Task
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		task, err = store.Read(taskID)
		if err != nil {
			return err
		}
		if task.Worker != nil && task.Worker.PID == identity.PID && task.Worker.StartToken == token {
			break
		}
		if task.Worker != nil && delegation.WorkerMatches(*task.Worker) {
			return fmt.Errorf("task %s is owned by another live worker", taskID)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("worker identity was not durably assigned to task %s", taskID)
		}
		time.Sleep(20 * time.Millisecond)
	}
	briefData, err := os.ReadFile(filepath.Join(store.Root(), "tasks", taskID, "artifacts", "brief.md")) //nolint:gosec // Inputs are validated as project-confined paths or fixed process-management arguments.
	if err != nil {
		return err
	}
	if task.Correction != "" {
		briefData = append(briefData, []byte("\n\n## Principal correction\n\n"+task.Correction+"\n")...)
	}
	var snap DelegationConfig
	if err := json.Unmarshal(task.ConfigSnapshot, &snap); err != nil {
		return err
	}
	selected := getStringFlag(cmd, "agent", false)
	var initial *DelegationAgentConfig
	for i := range snap.Agents {
		if snap.Agents[i].Name == selected {
			initial = &snap.Agents[i]
			break
		}
	}
	if initial == nil {
		return fmt.Errorf("initial agent %q is absent from task config snapshot", selected)
	}
	defer store.ReleaseCapacity(taskID) //nolint:errcheck
	policy := func(_ context.Context, capability string) (delegation.PermissionRule, error) {
		live := loadProjectConfig(filepath.Join(root, sdtConfigFile))
		if live == nil {
			return delegation.PermissionRule{}, fmt.Errorf("current project permission policy is unavailable")
		}
		seen := map[string]bool{}
		for _, rule := range live.Delegation.PermissionPolicy {
			if strings.TrimSpace(rule.Capability) == "" || seen[rule.Capability] {
				return delegation.PermissionRule{}, fmt.Errorf("current permission policy is invalid")
			}
			seen[rule.Capability] = true
			switch rule.Decision {
			case "allow", "deny", permissionDecisionAsk:
			default:
				return delegation.PermissionRule{}, fmt.Errorf("current permission policy has invalid decision %q", rule.Decision)
			}
		}
		data, err := json.Marshal(live.Delegation.PermissionPolicy)
		if err != nil {
			return delegation.PermissionRule{}, err
		}
		digest := sha256.Sum256(data)
		version := hex.EncodeToString(digest[:])
		for _, rule := range live.Delegation.PermissionPolicy {
			if rule.Capability == capability {
				return delegation.PermissionRule{Capability: capability, Decision: rule.Decision, Version: version}, nil
			}
		}
		return delegation.PermissionRule{Capability: capability, Decision: "ask", Version: version}, nil
	}
	candidates := make([]delegation.ACPAgentCandidate, 0, len(snap.Agents))
	for _, candidate := range snap.Agents {
		groupConcurrency := 0
		for _, group := range snap.Groups {
			if group.Name == candidate.AvailabilityGroup {
				groupConcurrency = group.Concurrency
				break
			}
		}
		candidates = append(candidates, delegation.ACPAgentCandidate{
			Name: candidate.Name, Command: candidate.Command, Model: candidate.Model,
			Roles: candidate.Roles, Capabilities: candidate.Capabilities,
			AvailabilityGroup: candidate.AvailabilityGroup, Concurrency: candidate.Concurrency,
			GroupConcurrency: groupConcurrency, Cooldown: candidate.Cooldown,
		})
	}
	for {
		task, err = store.Read(taskID)
		if err != nil {
			return err
		}
		if task.Status == delegation.TaskCancelling || task.Status == delegation.TaskCancelled {
			return nil
		}
		runErr := delegation.RunACPTask(cmd.Context(), delegation.ACPTaskOptions{
			Store: store, TaskID: taskID, Workspace: workspace, Brief: string(briefData),
			Candidates: candidates, MaxAttempts: snap.MaxAttempts, Policy: policy, WorkerIdentity: identity,
			NotifierFactory: func() (*delegation.Notifier, error) { return store.Listen(cmd.Context(), taskID, identity) },
		})
		if !errors.Is(runErr, delegation.ErrWaitingAvailability) {
			return runErr
		}
		task, err = store.Read(taskID)
		if err != nil {
			return err
		}
		if !task.Deadline.IsZero() && !task.Deadline.After(time.Now()) {
			return nil
		}
		wait := time.Minute
		if exclusions, err := store.Availability(time.Now()); err == nil {
			for _, exclusion := range exclusions {
				if until := time.Until(exclusion.Until); until > 0 && until < wait {
					wait = until
				}
			}
		}
		if !task.Deadline.IsZero() {
			if until := time.Until(task.Deadline); until > 0 && until < wait {
				wait = until
			}
		}
		timer := time.NewTimer(wait)
		select {
		case <-cmd.Context().Done():
			timer.Stop()
			return cmd.Context().Err()
		case <-timer.C:
		}
	}
}
