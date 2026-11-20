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
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/goccy/go-yaml"
	"github.com/sandrolain/sdt/internal/delegation"
	"github.com/spf13/cobra"
)

const taskWorkerHealthRunning = "running"

const (
	delegationFieldTaskID = "task_id"
	delegationFieldWorker = "worker"
	permissionDecisionAsk = "ask"
	delegationWorkerEnv   = "SDT_DELEGATION_TASK_WORKER"
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
an explicit correction; cleanup never runs automatically. Apply accepted changes
through the explicit preview and digest-checked apply commands; they never
create a commit.

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
	Plan      string                     `json:"plan,omitempty" yaml:"plan,omitempty"`
	Phase     string                     `json:"phase,omitempty" yaml:"phase,omitempty"`
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
	agentTaskStartCmd.Flags().String("worktree", "", "Use an existing dedicated task worktree")
	agentTaskStartCmd.Flags().String("commit", "HEAD", "Identified source commit")
	agentTaskStartCmd.Flags().StringSlice("include", nil, "Explicit project-relative dirty file to include (repeatable)")
	agentTaskStartCmd.Flags().StringSlice("reference", nil, "Project-relative reference file to copy into the stable task workspace (repeatable)")
	agentTaskStartCmd.Flags().String("id", "", "Task id (generated when omitted)")
	agentTaskStartCmd.Flags().String("plan", "", "Project-relative plan reference to link")
	agentTaskStartCmd.Flags().String("phase", "", "Plan phase or checklist id; omit to link the entire plan")

	agentTaskCmd.AddCommand(agentTaskStartCmd, agentTaskListCmd, agentTaskStatusCmd, agentTaskWaitCmd, agentTaskResultCmd, agentTaskCancelCmd, agentTaskResumeCmd, agentTaskAcceptCmd, agentTaskCorrectCmd, agentTaskFailCmd, agentTaskDiscardCmd, agentTaskInboxCmd, agentTaskApplyCmd, agentTaskCleanupCmd, agentTaskWorkerCmd, agentTaskPermissionsCmd, agentTaskAvailabilityCmd, agentTaskMessageCmd)
	agentCmd.AddCommand(agentTaskCmd)
}

func runAgentTaskStart(cmd *cobra.Command, _ []string) (runErr error) {
	if taskID := strings.TrimSpace(os.Getenv(delegationWorkerEnv)); taskID != "" {
		return fmt.Errorf("nested SDT task dispatch is disabled inside delegated task %s", taskID)
	}
	start, err := prepareDelegationStart(cmd)
	if err != nil {
		return err
	}
	cfg, projectRoot, role, brief, capabilities, candidates := start.Config, start.ProjectRoot, start.Role, start.Brief, start.Capabilities, start.Candidates
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}
	verificationChecks, err := selectedVerificationChecks(cfg)
	if err != nil {
		return err
	}
	planReference := strings.TrimSpace(getStringFlag(cmd, "plan", false))
	planPhase := strings.TrimSpace(getStringFlag(cmd, "phase", false))
	planReference, err = validateTaskPlanReference(root, planReference, planPhase)
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
	existing := getStringFlag(cmd, "worktree", false)
	workspace, err := delegation.CheckWorkspace(cmd.Context(), root, taskID, commit, existing)
	if err != nil {
		return err
	}
	if existing != "" && len(selected) != 0 {
		return fmt.Errorf("--include requires a new task worktree")
	}
	store := delegation.NewStore(root)
	duration, err := time.ParseDuration(cfg.Delegation.MaxDuration)
	if err != nil {
		return fmt.Errorf("invalid delegation.max_duration: %w", err)
	}
	task := delegation.Task{ID: taskID, Role: role, RequiredCapabilities: capabilities, Deadline: time.Now().Add(duration).UTC(), PlanReference: planReference, PlanPhase: planPhase}
	if err := store.Create(task); err != nil {
		return err
	}
	launched := false
	defer func() {
		if !launched && runErr != nil {
			_ = store.Update(taskID, "start_failed", "Task setup failed before a worker was launched: "+runErr.Error(), func(task *delegation.Task) error { task.Status = delegation.TaskInterventionRequired; return nil }) //nolint:errcheck // Preserve the original setup error returned to the caller.
		}
	}()
	if _, err := ensureTaskBroker(root, store); err != nil {
		return err
	}
	workspace, err = prepareTaskWorkspace(cmd.Context(), root, taskID, workspace, existing, selected)
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
	if err := store.SetVerificationConfig(taskID, verificationChecks); err != nil {
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
	if err := recordTaskReferences(store, taskID, root, workspace.Path, references); err != nil {
		return err
	}
	identity, err := launchTaskWorker(store, root, taskID, workspace.Path, candidates[0].Name)
	if err != nil {
		return err
	}
	launched = true
	outputTaskStart(cmd, taskStartResult{ID: taskID, Status: delegation.TaskQueued, Role: role, Deadline: task.Deadline, Workspace: workspace.Path, Plan: planReference, Phase: planPhase, Worker: identity})
	return nil
}

type delegationStartOptions struct {
	Config       *ProjectConfig
	ProjectRoot  string
	Role         string
	Brief        string
	Capabilities []string
	Candidates   []DelegationAgentConfig
}

func prepareDelegationStart(cmd *cobra.Command) (delegationStartOptions, error) {
	cfg, projectRoot, err := findProjectConfigAt()
	if err != nil {
		return delegationStartOptions{}, fmt.Errorf("delegation requires a valid project .sdt.yaml: %w", err)
	}
	if cfg == nil {
		return delegationStartOptions{}, fmt.Errorf("delegation requires a valid project .sdt.yaml")
	}
	if err := validateDelegationConfig(cfg.Delegation); err != nil {
		return delegationStartOptions{}, err
	}
	if len(cfg.Delegation.Agents) == 0 {
		return delegationStartOptions{}, fmt.Errorf("no delegation.agents are configured")
	}
	role := getStringFlag(cmd, "role", false)
	brief := strings.TrimSpace(getStringFlag(cmd, "brief", false))
	if role == "" || brief == "" {
		return delegationStartOptions{}, fmt.Errorf("--role and --brief are required")
	}
	if _, ok := roleLookup(role); !ok {
		return delegationStartOptions{}, fmt.Errorf("unknown SDT role %q", role)
	}
	capabilities, err := cmd.Flags().GetStringSlice("capability")
	if err != nil {
		return delegationStartOptions{}, err
	}
	candidates := delegationCandidates(cfg.Delegation, role, capabilities)
	if len(candidates) == 0 {
		return delegationStartOptions{}, fmt.Errorf("no configured agent matches role %q and required capabilities", role)
	}
	if err := preflightDelegationCandidates(candidates); err != nil {
		return delegationStartOptions{}, err
	}
	return delegationStartOptions{Config: cfg, ProjectRoot: projectRoot, Role: role, Brief: brief, Capabilities: capabilities, Candidates: candidates}, nil
}

func preflightDelegationCandidates(candidates []DelegationAgentConfig) error {
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.Command[0]); err != nil {
			return fmt.Errorf("preflight: configured agent %q executable %q is unavailable: %w", candidate.Name, candidate.Command[0], err)
		}
	}
	return nil
}

func validateDelegationPlanReference(root, reference, phase string) (string, error) {
	if filepath.IsAbs(reference) {
		return "", fmt.Errorf("--plan must be project-relative")
	}
	clean := filepath.Clean(reference)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("--plan must stay inside the project")
	}
	reference = filepath.ToSlash(clean)
	projectReal, rootErr := filepath.EvalSymlinks(root)
	planReal, planErr := filepath.EvalSymlinks(filepath.Join(root, clean))
	info, statErr := os.Stat(filepath.Join(root, clean))
	if rootErr != nil || planErr != nil || statErr != nil || !info.Mode().IsRegular() || !withinRoot(projectReal, planReal) {
		return "", fmt.Errorf("plan reference %q is not a readable project file", reference)
	}
	planData, readErr := os.ReadFile(planReal)
	if readErr != nil {
		return "", fmt.Errorf("plan reference %q could not be read: %w", reference, readErr)
	}
	if phase != "" && !delegationPlanPhaseExists(string(planData), phase) {
		return "", fmt.Errorf("plan phase %q is not present in %s", phase, reference)
	}
	return reference, nil
}

func validateTaskPlanReference(root, reference, phase string) (string, error) {
	if reference == "" {
		if phase != "" {
			return "", fmt.Errorf("--phase requires --plan")
		}
		return "", nil
	}
	return validateDelegationPlanReference(root, reference, phase)
}

func delegationPlanPhaseExists(content, phase string) bool {
	phase = strings.TrimSpace(phase)
	if phase == "" || len(phase) > 128 || strings.ContainsAny(phase, "\r\n\x00") {
		return false
	}
	if strings.HasPrefix(phase, "c") && len(phase) > 1 {
		if _, err := strconv.Atoi(strings.TrimPrefix(phase, "c")); err == nil {
			return strings.Contains(content, "<!-- "+phase+" -->")
		}
	}
	prefix := "## Phase "
	if strings.HasPrefix(phase, "Phase ") {
		prefix = "## "
	}
	want := prefix + phase
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == want || strings.HasPrefix(line, want+" ") || strings.HasPrefix(line, want+" —") || strings.HasPrefix(line, want+" -") {
			return true
		}
	}
	return false
}

func recordTaskReferences(store *delegation.Store, taskID, root, workspace string, references []string) error {
	snapshots, err := materializeDelegationReferences(root, workspace, references)
	if err != nil || len(snapshots) == 0 {
		return err
	}
	return store.Update(taskID, "references_materialized", "Stable reference copies and digests recorded", func(task *delegation.Task) error {
		task.Source.References = snapshots
		return nil
	})
}

func prepareTaskWorkspace(ctx context.Context, root, id string, checked delegation.Workspace, existing string, selected []string) (delegation.Workspace, error) {
	if existing != "" {
		return checked, nil
	}
	return delegation.PrepareWorkspace(ctx, root, id, checked.Commit, selected)
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
		text := fmt.Sprintf("Task %s queued for role %s\nWorkspace: %s\nDeadline: %s\n", result.ID, result.Role, result.Workspace, result.Deadline.Format(time.RFC3339))
		if result.Plan != "" {
			text += formatPlanScopeLine(result.Plan, result.Phase)
		}
		outputString(cmd, text)
	}
}

var agentTaskStatusCmd = &cobra.Command{Use: "status <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskStatus}
var agentTaskListCmd = &cobra.Command{Use: "list", Short: "List delegated tasks and their next principal action", RunE: runAgentTaskList}
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
	task, err := store.Read(args[0])
	if err != nil {
		return err
	}
	status := boundedTaskStatus(task)
	probeTaskStatus(cmd.Context(), store, &status)
	return outputTask(cmd, status)
}

func runAgentTaskList(cmd *cobra.Command, _ []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	tasks, err := store.List()
	if err != nil {
		return err
	}
	statuses := make([]taskStatusSummary, 0, len(tasks))
	for _, task := range tasks {
		status := boundedTaskStatus(task)
		probeTaskStatus(cmd.Context(), store, &status)
		statuses = append(statuses, status)
	}
	if getFormat(cmd) == formatText {
		var out strings.Builder
		for _, status := range statuses {
			fmt.Fprintf(&out, "%s %s", status.ID, status.Status)
			if status.Plan != "" {
				fmt.Fprintf(&out, " · %s · %s", status.Plan, taskPlanScope(status.Plan, status.Phase))
			}
			fmt.Fprintf(&out, "\n  Next: %s\n", status.NextAction)
		}
		outputString(cmd, out.String())
		return nil
	}
	return outputTask(cmd, statuses)
}

type taskStatusSummary struct {
	Revision           uint64                  `json:"revision" yaml:"revision"`
	Workspace          string                  `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Plan               string                  `json:"plan,omitempty" yaml:"plan,omitempty"`
	Phase              string                  `json:"phase,omitempty" yaml:"phase,omitempty"`
	NextAction         string                  `json:"next_action" yaml:"next_action"`
	WorkerHealth       string                  `json:"worker_health" yaml:"worker_health"`
	MaxAttempts        int                     `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	ProviderQuota      string                  `json:"provider_quota" yaml:"provider_quota"`
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

func taskPlanScope(plan, phase string) string {
	phase = strings.TrimSpace(phase)
	if phase == "" {
		return "entire plan"
	}
	return phase
}

func formatPlanScopeLine(plan, phase string) string {
	if plan == "" {
		return ""
	}
	return fmt.Sprintf("Plan: %s · %s\n", plan, taskPlanScope(plan, phase))
}

type taskMetricsSummary struct {
	PreparationElapsedMS int64  `json:"preparation_elapsed_ms,omitempty" yaml:"preparation_elapsed_ms,omitempty"`
	AttemptElapsedMS     int64  `json:"attempt_elapsed_ms" yaml:"attempt_elapsed_ms"`
	ReviewElapsedMS      int64  `json:"review_elapsed_ms,omitempty" yaml:"review_elapsed_ms,omitempty"`
	IntegrationElapsedMS int64  `json:"integration_elapsed_ms,omitempty" yaml:"integration_elapsed_ms,omitempty"`
	PrincipalTokens      string `json:"principal_tokens" yaml:"principal_tokens"`
	DelegateTokens       string `json:"delegate_tokens" yaml:"delegate_tokens"`
	PrincipalCostUSD     string `json:"principal_cost_usd" yaml:"principal_cost_usd"`
	DelegateCostUSD      string `json:"delegate_cost_usd" yaml:"delegate_cost_usd"`
	TotalCostUSD         string `json:"total_cost_usd" yaml:"total_cost_usd"`
	CommunicationBytes   string `json:"communication_bytes" yaml:"communication_bytes"`
}

func boundedTaskStatus(task delegation.Task) taskStatusSummary {
	view := taskStatusSummary{ID: task.ID, Status: task.Status, Role: task.Role, Deadline: task.Deadline, AttemptCount: len(task.Attempts)}
	view.Revision, view.Workspace = task.Revision, task.Source.Workspace
	view.Plan, view.Phase = task.PlanReference, task.PlanPhase
	view.NextAction = principalNextAction(task)
	view.WorkerHealth = "unrecorded"
	if task.Worker != nil {
		view.WorkerHealth = "unknown"
	}
	view.ProviderQuota = "unknown"
	var limits struct {
		MaxAttempts int `json:"max_attempts"`
	}
	if json.Unmarshal(task.ConfigSnapshot, &limits) == nil {
		view.MaxAttempts = limits.MaxAttempts
	}
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
	var firstStart, lastFinish time.Time
	for _, attempt := range task.Attempts {
		if !attempt.StartedAt.IsZero() {
			if firstStart.IsZero() || attempt.StartedAt.Before(firstStart) {
				firstStart = attempt.StartedAt
			}
			end := attempt.FinishedAt
			if end.IsZero() {
				end = time.Now()
			} else if end.After(lastFinish) {
				lastFinish = end
			}
			elapsed += end.Sub(attempt.StartedAt)
		}
	}
	metrics := taskMetricsSummary{AttemptElapsedMS: elapsed.Milliseconds(), PrincipalTokens: learnProvenanceUnknown, DelegateTokens: learnProvenanceUnknown, PrincipalCostUSD: learnProvenanceUnknown, DelegateCostUSD: learnProvenanceUnknown, TotalCostUSD: learnProvenanceUnknown, CommunicationBytes: learnProvenanceUnknown}
	if !task.CreatedAt.IsZero() && !firstStart.IsZero() && firstStart.After(task.CreatedAt) {
		metrics.PreparationElapsedMS = firstStart.Sub(task.CreatedAt).Milliseconds()
	}
	acceptedAt := time.Time{}
	for _, decision := range task.PrincipalDecisions {
		if decision.Action == delegation.PrincipalDecisionAccepted && decision.AcceptedAt.After(acceptedAt) {
			acceptedAt = decision.AcceptedAt
		}
	}
	if !lastFinish.IsZero() && !acceptedAt.IsZero() && acceptedAt.After(lastFinish) {
		metrics.ReviewElapsedMS = acceptedAt.Sub(lastFinish).Milliseconds()
	}
	if !acceptedAt.IsZero() && task.Integration != nil && task.Integration.Applied.After(acceptedAt) {
		metrics.IntegrationElapsedMS = task.Integration.Applied.Sub(acceptedAt).Milliseconds()
	}
	view.Metrics = metrics
	if len(view.Attempts) > 3 {
		view.Attempts = append([]delegation.Attempt(nil), view.Attempts[len(view.Attempts)-3:]...)
	}
	if len(view.Artifacts) > 5 {
		view.Artifacts = append([]delegation.Artifact(nil), view.Artifacts[len(view.Artifacts)-5:]...)
	}
	return view
}

func probeTaskStatus(ctx context.Context, store *delegation.Store, status *taskStatusSummary) {
	if status.WorkerHealth == "unrecorded" {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if store.ProbeWorker(probeCtx, status.ID) == nil {
		status.WorkerHealth = taskWorkerHealthRunning
	}
}

func principalNextAction(task delegation.Task) string {
	if task.Status == delegation.TaskResultReady && task.Verification != nil && (task.Verification.Status == delegation.VerificationFailed || task.Verification.Status == delegation.VerificationInterrupted || task.Verification.Status == delegation.VerificationStale) {
		return "review declared checks before accepting, correcting, or discarding the result"
	}
	if task.Status == delegation.TaskAccepted && task.Integration != nil {
		return "verify the applied changes and update the linked plan"
	}
	switch task.Status {
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
		return "inspect task status and choose the next action"
	}
}

func selectedVerificationChecks(cfg *ProjectConfig) ([]delegation.VerificationCheckConfig, error) {
	if cfg == nil || len(cfg.Delegation.VerifyOnResult) == 0 {
		return nil, nil
	}
	if err := validateGateSteps(cfg.Gate); err != nil {
		return nil, err
	}
	steps := make(map[string]GateStep, len(cfg.Gate))
	for _, step := range cfg.Gate {
		steps[step.Name] = step
	}
	seen := map[string]bool{}
	checks := make([]delegation.VerificationCheckConfig, 0, len(cfg.Delegation.VerifyOnResult))
	for _, name := range cfg.Delegation.VerifyOnResult {
		name = strings.TrimSpace(name)
		step, ok := steps[name]
		if !ok || seen[name] {
			return nil, fmt.Errorf("delegation.verify_on_result contains unknown or duplicate gate name %q", name)
		}
		if strings.TrimSpace(step.Run) == "" || strings.TrimSpace(step.Exclude) != "" {
			return nil, fmt.Errorf("delegation.verify_on_result gate %q must declare a runnable command and no exclusion", name)
		}
		seen[name] = true
		checks = append(checks, delegation.VerificationCheckConfig{Name: step.Name, Run: step.Run})
	}
	return checks, nil
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
		data, err := store.ReadArtifact(task.ID, "worker.log")
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
				return outputTask(cmd, map[string]any{"task_id": task.ID, "plan": task.PlanReference, "phase": task.PlanPhase, "artifact": artifact, "content": string(data)})
			}
		}
		return fmt.Errorf("artifact %q is not recorded for task %s", args[1], task.ID)
	}
	if getFormat(cmd) != formatText {
		return outputTask(cmd, task.Artifacts)
	}
	var out strings.Builder
	if task.PlanReference != "" {
		fmt.Fprintf(&out, "Task %s · %s · %s\n\n", task.ID, task.PlanReference, taskPlanScope(task.PlanReference, task.PlanPhase))
	}
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
var agentTaskFailCmd = &cobra.Command{Use: "fail <task-id>", Short: "Mark a task failed after confirming worker shutdown", Args: cobra.ExactArgs(1), RunE: runAgentTaskFail}
var agentTaskDiscardCmd = &cobra.Command{Use: "discard <task-id>", Short: "Discard a delivered result without accepting it", Args: cobra.ExactArgs(1), RunE: runAgentTaskDiscard}
var agentTaskInboxCmd = &cobra.Command{Use: "inbox", Short: "Read the durable task decision inbox"}
var agentTaskInboxListCmd = &cobra.Command{Use: "list", Short: "List tasks requiring principal attention", Args: cobra.NoArgs, RunE: runAgentTaskInboxList}

func init() {
	agentTaskInboxCmd.AddCommand(agentTaskInboxListCmd)
	agentTaskAcceptCmd.Flags().String("note", "", "Principal acceptance note")
	agentTaskCorrectCmd.Flags().String("instructions", "", "Correction instructions for a follow-up attempt")
	agentTaskFailCmd.Flags().String("reason", "", "Required reason for terminal failure")
	agentTaskDiscardCmd.Flags().String("reason", "", "Required reason for discarding the result")
}

func runAgentTaskInboxList(cmd *cobra.Command, _ []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	items, err := store.Inbox()
	if err != nil {
		return err
	}
	if getFormat(cmd) == formatText {
		var out strings.Builder
		if len(items) == 0 {
			out.WriteString("No task decisions are pending.\n")
		}
		for _, item := range items {
			fmt.Fprintf(&out, "%s %s — %s (revision %d)\n", item.TaskID, item.Status, strings.Join(item.Kinds, ", "), item.Revision)
			if len(item.PendingRequestIDs) > 0 {
				fmt.Fprintf(&out, "  Permission requests: %s\n", strings.Join(item.PendingRequestIDs, ", "))
			}
			fmt.Fprintf(&out, "  Next: %s\n", item.NextAction)
		}
		outputString(cmd, out.String())
		return nil
	}
	return outputTask(cmd, items)
}

func runAgentTaskFail(cmd *cobra.Command, args []string) error {
	reason := strings.TrimSpace(getStringFlag(cmd, "reason", false))
	if reason == "" {
		return fmt.Errorf("--reason is required")
	}
	store, err := taskStore()
	if err != nil {
		return err
	}
	if err := store.Fail(args[0], reason); err != nil {
		return err
	}
	return outputAction(cmd, "Task marked failed: "+args[0]+"\n", map[string]string{delegationFieldTaskID: args[0], ctxMapStatus: string(delegation.TaskFailed)})
}

func runAgentTaskDiscard(cmd *cobra.Command, args []string) error {
	reason := strings.TrimSpace(getStringFlag(cmd, "reason", false))
	if reason == "" {
		return fmt.Errorf("--reason is required")
	}
	store, err := taskStore()
	if err != nil {
		return err
	}
	if err := store.DiscardResult(args[0], reason); err != nil {
		return err
	}
	return outputAction(cmd, "Task result discarded: "+args[0]+"\n", map[string]string{delegationFieldTaskID: args[0], ctxMapStatus: string(delegation.TaskDiscarded)})
}

func runAgentTaskAccept(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	if err := store.AcceptResult(args[0], getStringFlag(cmd, "note", false)); err != nil {
		return err
	}
	hint := "sdt agent task apply preview " + args[0]
	return outputAction(cmd, "Result accepted: "+args[0]+"\nNext: "+hint+"\n", map[string]string{delegationFieldTaskID: args[0], ctxMapStatus: string(delegation.TaskAccepted), "next_command": hint})
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
	if err := ensureTaskWorkerStopped(cmd, store, task); err != nil {
		return err
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

var agentTaskApplyCmd = &cobra.Command{Use: "apply", Short: "Preview or apply accepted task changes to the principal checkout"}
var agentTaskApplyPreviewCmd = &cobra.Command{Use: "preview <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskApplyPreview}
var agentTaskApplyExecuteCmd = &cobra.Command{Use: "execute <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskApplyExecute}

func init() {
	agentTaskApplyExecuteCmd.Flags().String("digest", "", "Digest from the reviewed integration preview")
	agentTaskApplyCmd.AddCommand(agentTaskApplyPreviewCmd, agentTaskApplyExecuteCmd)
}

func runAgentTaskApplyPreview(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	_, root, err := findProjectConfigAt()
	if err != nil {
		return err
	}
	preview, err := store.PreviewIntegration(cmd.Context(), args[0], root)
	if err != nil {
		return err
	}
	return outputTask(cmd, preview)
}

func runAgentTaskApplyExecute(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	_, root, err := findProjectConfigAt()
	if err != nil {
		return err
	}
	digest := getStringFlag(cmd, "digest", false)
	result, err := store.ApplyIntegration(cmd.Context(), args[0], root, digest)
	if err != nil {
		return err
	}
	return outputAction(cmd, fmt.Sprintf("Applied accepted task %s to the working tree (%d paths); no commit created.\n", result.TaskID, len(result.Paths)), result)
}

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
		if output, err := exec.CommandContext(cmd.Context(), "git", "-C", task.Source.Workspace, "status", "--short").Output(); err == nil { //#nosec G204 -- Read-only Git status receives cwd as one argv value; no shell.
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
	if err := ensureTaskWorkerStopped(cmd, store, task); err != nil {
		return err
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
			if output, err := exec.CommandContext(cmd.Context(), "git", "-C", root, "worktree", "remove", "--force", workspace).CombinedOutput(); err != nil { //#nosec G204 -- Task path and branch were validated above; fixed Git argv without a shell.
				return fmt.Errorf("remove task worktree: %w: %s", err, strings.TrimSpace(string(output)))
			}
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		branch := "refs/heads/" + task.Source.Branch
		if err := exec.CommandContext(cmd.Context(), "git", "-C", root, "show-ref", "--verify", "--quiet", branch).Run(); err == nil { //#nosec G204 -- Branch is the validated task branch under refs/heads; fixed Git argv without a shell.
			if output, err := exec.CommandContext(cmd.Context(), "git", "-C", root, "branch", "-D", task.Source.Branch).CombinedOutput(); err != nil { //#nosec G204 -- Branch must equal sdt/delegation/<safe task id> above; fixed argv without a shell.
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
	if err := ensureTaskWorkerStopped(cmd, store, task); err != nil {
		return fmt.Errorf("task %s: %w", task.ID, err)
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
		killErr := syscall.Kill(-identity.GroupID, syscall.SIGKILL)
		if killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
			return delegation.ProcessIdentity{}, fmt.Errorf("record worker: %w; terminate unrecorded worker: %v", err, killErr)
		}
		return delegation.ProcessIdentity{}, err
	}
	return identity, nil
}

func ensureTaskWorkerStopped(cmd *cobra.Command, store *delegation.Store, task delegation.Task) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 500*time.Millisecond)
	err := store.ProbeWorker(ctx, task.ID)
	cancel()
	if err == nil {
		return fmt.Errorf("task still has a live worker; wait for it to stop before replacing or cleaning up")
	}
	if task.Worker != nil && delegation.WorkerMatches(*task.Worker) {
		return fmt.Errorf("task still has a live worker; wait for it to stop before replacing or cleaning up")
	}
	for _, attempt := range task.Attempts {
		identity := delegation.ProcessIdentity{PID: attempt.ProcessID, GroupID: attempt.ProcessID, StartToken: attempt.ProcessStart}
		if delegation.WorkerMatches(identity) {
			return fmt.Errorf("task still has a live attempt worker; wait for it to stop before replacing or cleaning up")
		}
	}
	return nil
}

func outputTaskStatus(cmd *cobra.Command, status taskStatusSummary) {
	var b strings.Builder
	fmt.Fprintf(&b, "Task %s — %s (%s)\nAttempts: %d\nDeadline: %s\n", status.ID, status.Status, status.Role, status.AttemptCount, status.Deadline.Format(time.RFC3339))
	if status.Plan != "" {
		b.WriteString(formatPlanScopeLine(status.Plan, status.Phase))
	}
	fmt.Fprintf(&b, "Next principal action: %s\n", status.NextAction)
	if status.MaxAttempts > 0 {
		fmt.Fprintf(&b, "Local attempt limit: %d\n", status.MaxAttempts)
	}
	fmt.Fprintf(&b, "Worker: %s\nProvider quota: %s\n", status.WorkerHealth, status.ProviderQuota)
	for _, request := range status.PendingPermissions {
		fmt.Fprintf(&b, "Pending permission: %s\n", request)
	}
	for _, artifact := range status.Artifacts {
		fmt.Fprintf(&b, "Artifact %s: %s\n", artifact.Name, artifact.Path)
	}
	fmt.Fprintf(&b, "Preparation: %d ms; worker attempts: %d ms; principal review: %d ms; integration: %d ms\n", status.Metrics.PreparationElapsedMS, status.Metrics.AttemptElapsedMS, status.Metrics.ReviewElapsedMS, status.Metrics.IntegrationElapsedMS)
	fmt.Fprintf(&b, "Provider tokens and costs: %s; communication bytes: %s\n", status.Metrics.DelegateTokens, status.Metrics.CommunicationBytes)
	outputString(cmd, b.String())
}

func outputTask(cmd *cobra.Command, value any) error {
	if getFormat(cmd) == formatText {
		if status, ok := value.(taskStatusSummary); ok {
			outputTaskStatus(cmd, status)
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
		case delegation.IntegrationPreview:
			return outputIntegrationPreview(cmd, rows)
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

func outputIntegrationPreview(cmd *cobra.Command, preview delegation.IntegrationPreview) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Task %s (%s)\nDigest: %s\nSource: %s\n", preview.TaskID, preview.Status, preview.Digest, preview.SourceWorkspace)
	if preview.PlanReference != "" {
		b.WriteString(formatPlanScopeLine(preview.PlanReference, preview.PlanPhase))
	}
	if preview.NoChanges {
		b.WriteString("No task changes to apply.\n")
	}
	if preview.AlreadyApplied {
		b.WriteString("These accepted changes are already applied.\n")
	}
	for _, path := range preview.Paths {
		fmt.Fprintf(&b, "Path: %s\n", path)
	}
	for _, conflict := range preview.Conflicts {
		fmt.Fprintf(&b, "Conflict: %s\n", conflict)
	}
	if preview.Patch != "" {
		b.WriteString("\n")
		b.WriteString(preview.Patch)
	}
	outputString(cmd, b.String())
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
var agentTaskPermissionResolveBatchCmd = &cobra.Command{
	Use:   "resolve-batch <task-id>",
	Short: "Resolve independent ACP permission requests with exact per-request choices",
	Long: `Input JSON or YAML with a decisions list; each request_id must map to an option_id
offered for that request. The batch shares source, rationale, and mandate flags but
records them separately on every decision. A stale or invalid choice applies none.

Example input:
  decisions:
    - request_id: request-a
      option_id: allow-once
    - request_id: request-b
      option_id: reject-once`,
	Args: cobra.ExactArgs(1), RunE: runAgentTaskPermissionResolveBatch,
}

type permissionBatchInput struct {
	Decisions []delegation.PermissionBatchDecision `json:"decisions" yaml:"decisions"`
}

func decodePermissionBatchInput(raw string) (permissionBatchInput, error) {
	var input permissionBatchInput
	if err := yaml.Unmarshal([]byte(raw), &input); err != nil {
		return permissionBatchInput{}, fmt.Errorf("decode permission batch input: %w", err)
	}
	if len(input.Decisions) == 0 {
		return permissionBatchInput{}, fmt.Errorf("permission batch input must contain at least one decision")
	}
	return input, nil
}

func init() {
	agentTaskPermissionsCmd.AddCommand(agentTaskPermissionListCmd, agentTaskPermissionResolveCmd, agentTaskPermissionResolveBatchCmd)
	agentTaskPermissionListCmd.Flags().String("source", "", "Filter decision source, including principal, user or policy")
	agentTaskPermissionListCmd.Flags().String("outcome", "", "Filter recorded disposition, such as pending, resolved or stale")
	agentTaskPermissionListCmd.Flags().Bool("escalated", false, "Show only requests carrying an escalation reason")
	agentTaskPermissionResolveCmd.Flags().String("option", "", "Exact offered permission option id")
	agentTaskPermissionResolveCmd.Flags().String("rationale", "", "Principal decision rationale")
	agentTaskPermissionResolveCmd.Flags().String("source", delegation.PermissionSourcePrincipal, "Decision source: principal or user (explicit user response)")
	agentTaskPermissionResolveCmd.Flags().String("mandate", "", "Existing mandate reference required for a principal decision")
	agentTaskPermissionResolveBatchCmd.Flags().String("rationale", "", "Rationale recorded for each permission decision")
	agentTaskPermissionResolveBatchCmd.Flags().String("source", delegation.PermissionSourcePrincipal, "Decision source: principal or user (explicit user response)")
	agentTaskPermissionResolveBatchCmd.Flags().String("mandate", "", "Existing mandate reference required for a principal decision")
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
	return outputTask(cmd, filterPermissionHistory(task.Permissions, getStringFlag(cmd, "source", false), getStringFlag(cmd, "outcome", false), getBoolFlag(cmd, "escalated", false)))
}

func runAgentTaskPermissionResolve(cmd *cobra.Command, args []string) error {
	source := getStringFlag(cmd, "source", false)
	rationale := getStringFlag(cmd, "rationale", false)
	mandate := getStringFlag(cmd, "mandate", false)
	if err := validatePermissionAuthority(source, rationale, mandate); err != nil {
		return err
	}
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
	if err := store.ResolvePermissionWithPolicy(args[0], request.AttemptID, request.RequestID, option, source, rationale, version, "explicit CLI decision", mandate); err != nil {
		return err
	}
	return outputAction(cmd, "Permission decision recorded; the worker reconciles durable state even if the wake notification is unavailable.\n", map[string]string{"task_id": args[0], "request_id": args[1], ctxMapStatus: "resolved"})
}

func runAgentTaskPermissionResolveBatch(cmd *cobra.Command, args []string) error {
	source := getStringFlag(cmd, "source", false)
	rationale := getStringFlag(cmd, "rationale", false)
	mandate := getStringFlag(cmd, "mandate", false)
	if err := validatePermissionAuthority(source, rationale, mandate); err != nil {
		return err
	}
	input, err := decodePermissionBatchInput(getInputString(cmd, nil))
	if err != nil {
		return err
	}
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
	if len(task.Attempts) == 0 {
		return fmt.Errorf("task %q has no attempt with pending permissions", args[0])
	}
	versionData, err := json.Marshal(cfg.Delegation.PermissionPolicy)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(versionData)
	version := hex.EncodeToString(digest[:])
	if err := store.ResolvePermissionBatchWithPolicy(args[0], task.Attempts[len(task.Attempts)-1].ID, input.Decisions, source, rationale, version, "explicit CLI batch decision", mandate); err != nil {
		return err
	}
	resolved, err := store.Read(args[0])
	if err != nil {
		return err
	}
	type decisionSummary struct {
		RequestID  string `json:"request_id"`
		Capability string `json:"capability"`
		Operation  string `json:"operation,omitempty"`
		Scope      string `json:"scope"`
		OptionID   string `json:"option_id"`
		OptionName string `json:"option_name"`
	}
	summaries := make([]decisionSummary, 0, len(input.Decisions))
	for _, decision := range input.Decisions {
		for _, record := range resolved.Permissions {
			if record.RequestID != decision.RequestID || record.AttemptID != task.Attempts[len(task.Attempts)-1].ID {
				continue
			}
			name := decision.OptionID
			for _, option := range record.Options {
				if option.OptionID == decision.OptionID {
					name = option.Name
					break
				}
			}
			summaries = append(summaries, decisionSummary{
				RequestID: record.RequestID, Capability: record.Capability, Operation: record.Operation,
				Scope: record.Scope, OptionID: decision.OptionID, OptionName: name,
			})
			break
		}
	}
	var message strings.Builder
	fmt.Fprintf(&message, "Recorded %d exact permission decisions; the worker reconciles durable state even if the wake notification is unavailable.\n", len(summaries))
	for _, summary := range summaries {
		fmt.Fprintf(&message, "- %q: %q / %q / %q → %q (%q)\n", summary.RequestID, summary.Capability, summary.Operation, summary.Scope, summary.OptionName, summary.OptionID)
	}
	return outputAction(cmd, message.String(), map[string]any{
		delegationFieldTaskID: args[0], "status": resolved.Status, "resolved_count": len(summaries), "decisions": summaries,
	})
}

var agentTaskAvailabilityCmd = &cobra.Command{Use: "availability", Short: "Inspect or reset provider availability exclusions"}
var agentTaskMessageCmd = &cobra.Command{Use: "message", Short: "Exchange untrusted coordination notes within one task"}
var agentTaskMessageListCmd = &cobra.Command{Use: "list <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskMessageList}
var agentTaskMessageSendCmd = &cobra.Command{Use: "send <task-id>", Args: cobra.ExactArgs(1), RunE: runAgentTaskMessageSend}
var agentTaskAvailabilityListCmd = &cobra.Command{Use: useList, RunE: runAgentTaskAvailabilityList}
var agentTaskAvailabilityResetCmd = &cobra.Command{Use: "reset <key>", Args: cobra.ExactArgs(1), RunE: runAgentTaskAvailabilityReset}

func init() {
	agentTaskAvailabilityCmd.AddCommand(agentTaskAvailabilityListCmd, agentTaskAvailabilityResetCmd)
	agentTaskMessageCmd.AddCommand(agentTaskMessageListCmd, agentTaskMessageSendCmd)
	agentTaskMessageListCmd.Flags().String("as", "", "Show messages for one task participant")
	agentTaskMessageSendCmd.Flags().String("from", "", "Task participant sending the message")
	agentTaskMessageSendCmd.Flags().String("to", "", "Task participant receiving the message")
	agentTaskMessageSendCmd.Flags().String("text", "", "Coordination note; treated as untrusted context")
	agentTaskMessageSendCmd.Flags().String("artifact", "", "Name of a retained task artifact to reference")
}

func runAgentTaskMessageList(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	messages, err := store.ListMessages(args[0], getStringFlag(cmd, "as", false))
	if err != nil {
		return err
	}
	if getFormat(cmd) == formatText {
		var out strings.Builder
		for _, message := range messages {
			fmt.Fprintf(&out, "%d %s → %s [%s]\n%s\n", message.Sequence, message.From, message.To, message.CreatedAt.Format(time.RFC3339), safeTaskMessageText(message.Text))
			if message.Artifact != "" {
				fmt.Fprintf(&out, "Artifact: %s\n", message.Artifact)
			}
		}
		outputString(cmd, out.String())
		return nil
	}
	return outputTask(cmd, messages)
}

func safeTaskMessageText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
}

func runAgentTaskMessageSend(cmd *cobra.Command, args []string) error {
	store, err := taskStore()
	if err != nil {
		return err
	}
	message, err := store.SendMessage(args[0], getStringFlag(cmd, "from", false), getStringFlag(cmd, "to", false), getStringFlag(cmd, "text", false), getStringFlag(cmd, "artifact", false))
	if err != nil {
		return err
	}
	return outputAction(cmd, fmt.Sprintf("Task message %d recorded; coordination text is untrusted context.\n", message.Sequence), message)
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
	workerTaskID := getStringFlag(cmd, "task", false)
	if err := os.Setenv(delegationWorkerEnv, workerTaskID); err != nil {
		return fmt.Errorf("mark delegated worker context: %w", err)
	}
	defer os.Unsetenv(delegationWorkerEnv) //nolint:errcheck // This is a dedicated worker process.
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
		if task.Worker != nil && task.Worker.StartToken != token {
			probeCtx, cancel := context.WithTimeout(cmd.Context(), 250*time.Millisecond)
			probeErr := store.ProbeWorker(probeCtx, taskID)
			cancel()
			if probeErr == nil || delegation.WorkerMatches(*task.Worker) {
				return fmt.Errorf("task %s is owned by another live worker", taskID)
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("worker identity was not durably assigned to task %s", taskID)
		}
		time.Sleep(20 * time.Millisecond)
	}
	notifier, err := store.Listen(cmd.Context(), taskID, identity)
	if err != nil {
		updateErr := store.Update(taskID, "worker_control_unavailable", "Worker control socket could not be initialized: "+err.Error(), func(task *delegation.Task) error {
			task.Status = delegation.TaskInterventionRequired
			return nil
		})
		if updateErr != nil {
			return fmt.Errorf("initialize worker control socket: %w (record intervention: %v)", err, updateErr)
		}
		return fmt.Errorf("initialize worker control socket: %w", err)
	}
	defer notifier.Close() //nolint:errcheck // Worker owns its subscription endpoint.
	workerCtx, cancelWorker := context.WithCancel(cmd.Context())
	defer cancelWorker()
	go func() {
		select {
		case <-notifier.CancelRequests():
			cancelWorker()
		case <-workerCtx.Done():
		}
	}()
	briefData, err := store.ReadArtifact(taskID, "brief.md")
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
	policy := func(_ context.Context, capability, operation, scope string) (delegation.PermissionRule, error) {
		live := loadProjectConfig(filepath.Join(root, sdtConfigFile))
		if live == nil {
			return delegation.PermissionRule{}, fmt.Errorf("current project permission policy is unavailable")
		}
		if err := validateDelegationConfig(live.Delegation); err != nil {
			return delegation.PermissionRule{}, fmt.Errorf("current delegation policy is invalid: %w", err)
		}
		seen := map[string]bool{}
		for _, rule := range live.Delegation.PermissionPolicy {
			key := strings.Join([]string{rule.Capability, rule.Operation, rule.Scope}, "\x00")
			if strings.TrimSpace(rule.Capability) == "" || seen[key] {
				return delegation.PermissionRule{}, fmt.Errorf("current permission policy is invalid")
			}
			seen[key] = true
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
		task, err := store.Read(taskID)
		if err != nil {
			return delegation.PermissionRule{}, err
		}
		for i, rule := range live.Delegation.PermissionPolicy {
			if rule.Capability == capability && rule.Operation == operation && rule.Scope == scope {
				validFor, err := time.ParseDuration(rule.ValidFor)
				if err != nil || validFor <= 0 {
					return delegation.PermissionRule{}, fmt.Errorf("current permission mandate has invalid validity")
				}
				expiresAt := task.CreatedAt.Add(validFor)
				if !task.Deadline.IsZero() && task.Deadline.Before(expiresAt) {
					expiresAt = task.Deadline
				}
				decision := rule.Decision
				if !expiresAt.After(time.Now()) {
					decision = permissionDecisionAsk
				}
				return delegation.PermissionRule{Capability: capability, Operation: operation, Scope: scope, Decision: decision, Version: version, Mandate: fmt.Sprintf("delegation.permission_policy[%d]", i), ExpiresAt: expiresAt}, nil
			}
		}
		return delegation.PermissionRule{Capability: capability, Operation: operation, Scope: scope, Decision: permissionDecisionAsk, Version: version}, nil
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
		runErr := delegation.RunACPTask(workerCtx, delegation.ACPTaskOptions{
			Store: store, TaskID: taskID, Workspace: workspace, Brief: string(briefData),
			Candidates: candidates, MaxAttempts: snap.MaxAttempts, Policy: policy, WorkerIdentity: identity, Trace: snap.Trace,
			Notifier: notifier,
		})
		if errors.Is(workerCtx.Err(), context.Canceled) {
			return nil
		}
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
		case <-workerCtx.Done():
			timer.Stop()
			return cmd.Context().Err()
		case <-timer.C:
		}
	}
}

var agentTaskWaitCmd = &cobra.Command{
	Use: "wait <task-id> [task-id...]", Short: "Wait for delegated tasks through worker events",
	Args: cobra.MinimumNArgs(1), RunE: runAgentTaskWait,
}

func init() {
	agentTaskWaitCmd.Flags().Duration("timeout", time.Minute, "Maximum time to wait")
	agentTaskWaitCmd.Flags().Bool("any", false, "Wait for any supplied task to require attention")
	agentTaskWaitCmd.Flags().Bool(delegation.WaitModeAll, false, "Wait for all supplied tasks to finish; return early for intervention")
	agentTaskWaitCmd.MarkFlagsMutuallyExclusive("any", delegation.WaitModeAll)
}

func runAgentTaskWait(cmd *cobra.Command, args []string) error {
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil || timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
	}
	store, err := taskStore()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	mode := "one"
	if getBoolFlag(cmd, "any", false) {
		mode = "any"
	}
	if getBoolFlag(cmd, delegation.WaitModeAll, false) {
		mode = delegation.WaitModeAll
	}
	result, waitErr := store.Wait(ctx, args, mode)
	if len(result.Tasks) == 0 {
		return waitErr
	}
	statuses := make([]taskStatusSummary, 0, len(result.Tasks))
	for _, task := range result.Tasks {
		statuses = append(statuses, boundedTaskStatus(task))
		probeTaskStatus(cmd.Context(), store, &statuses[len(statuses)-1])
	}
	if err := outputTask(cmd, struct {
		delegation.WaitSnapshot
		Tasks []taskStatusSummary `json:"tasks"`
	}{result, statuses}); err != nil {
		return err
	}
	return waitErr
}
