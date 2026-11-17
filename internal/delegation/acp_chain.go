//go:build unix

package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sandrolain/sdt/internal/acpbridge"
)

var ErrWaitingAvailability = errors.New("delegation task is queued until an eligible agent is available")

type ACPAgentCandidate struct {
	Name              string
	Command           []string
	Model             string
	Roles             []string
	Capabilities      []string
	AvailabilityGroup string
	Concurrency       int
	GroupConcurrency  int
	Cooldown          string
}

type HandoffRenderer func(Attempt, Task) string

type ACPTaskOptions struct {
	Store           *Store
	Notifier        *Notifier
	TaskID          string
	Workspace       string
	Brief           string
	Candidates      []ACPAgentCandidate
	MaxAttempts     int
	Policy          PermissionPolicy
	WorkerIdentity  ProcessIdentity
	RenderHandoff   HandoffRenderer
	NotifierFactory func() (*Notifier, error)
	bridgeStart     func(context.Context, *acpbridge.Bridge, []string, string) (string, error)
}

// RunACPTask routes one logical task through its ordered, eligible candidate set.
// Every failed bridge is closed before a handoff is published or another candidate runs.
//
//nolint:gocognit // Routing, reservation, handoff, and retry form one ordered state machine.
//nolint:gocognit,gocyclo // This function preserves a multi-step state transition as one auditable unit.
//nolint:gocyclo
func RunACPTask(ctx context.Context, opts ACPTaskOptions) error { //nolint:gocyclo
	if opts.Store == nil || opts.TaskID == "" || opts.Workspace == "" || opts.MaxAttempts < 1 {
		return fmt.Errorf("ACP task requires store, task, workspace, and positive attempt limit")
	}
	task, err := opts.Store.Read(opts.TaskID)
	if err != nil {
		return err
	}
	if _, err := opts.Store.ReconcileCapacity(time.Now()); err != nil {
		return err
	}
	eligible := make([]ACPAgentCandidate, 0, len(opts.Candidates))
	for _, candidate := range opts.Candidates {
		if !containsString(candidate.Roles, task.Role) || !containsAll(candidate.Capabilities, task.RequiredCapabilities) {
			continue
		}
		eligible = append(eligible, candidate)
	}
	if len(eligible) == 0 {
		return opts.intervene("No configured agent matches the task role and required capabilities")
	}
	brief := opts.Brief
	var ownedNotifier *Notifier
	defer func() {
		if ownedNotifier != nil {
			_ = ownedNotifier.Close() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
		}
	}()
	for {
		task, err = opts.Store.Read(opts.TaskID)
		if err != nil {
			return err
		}
		now := time.Now()
		if !task.Deadline.IsZero() && !task.Deadline.After(now) {
			return opts.intervene("The task deadline expired before an eligible attempt could continue")
		}
		waiting := false
		attemptedAny := false
		continueChain := false
		for _, candidate := range eligible {
			task, err = opts.Store.Read(opts.TaskID)
			if err != nil {
				return err
			}
			current := currentAttemptFor(task, candidate.Name)
			if !current && len(task.Attempts) >= opts.MaxAttempts {
				return opts.intervene("The task reached its aggregate attempt limit")
			}
			available, probes, err := opts.Store.ClaimCandidate(candidate.Name, candidate.AvailabilityGroup, opts.TaskID, now)
			if err != nil {
				return err
			}
			if !available {
				waiting = true
				continue
			}
			limits := map[string]int{candidateCapacityKey("agent-", candidate.Name): candidate.Concurrency}
			if candidate.AvailabilityGroup != "" {
				if candidate.GroupConcurrency < 1 {
					return fmt.Errorf("availability group %q has no positive concurrency", candidate.AvailabilityGroup)
				}
				limits[candidateCapacityKey("group-", candidate.AvailabilityGroup)] = candidate.GroupConcurrency
			}
			admitted, err := opts.Store.TryAdmit(opts.TaskID, limits)
			if err != nil {
				_ = opts.Store.ReleaseCandidateProbes(probes, opts.TaskID) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
				return err
			}
			if !admitted {
				if err := opts.Store.ReleaseCandidateProbes(probes, opts.TaskID); err != nil {
					return err
				}
				waiting = true
				continue
			}
			attemptID := nextAttemptID(task, current)
			if err := opts.Store.BeginAgentAttempt(opts.TaskID, Attempt{ID: attemptID, Agent: candidate.Name, Model: candidate.Model}, opts.WorkerIdentity); err != nil {
				_ = opts.Store.ReleaseCapacity(opts.TaskID)                //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
				_ = opts.Store.ReleaseCandidateProbes(probes, opts.TaskID) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
				return err
			}
			if opts.WorkerIdentity.PID > 0 {
				if err := opts.Store.AttachWorker(opts.TaskID, opts.WorkerIdentity); err != nil {
					_ = opts.Store.ReleaseCapacity(opts.TaskID) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
					return err
				}
			}
			if opts.Notifier == nil && opts.NotifierFactory != nil {
				notifier, err := opts.NotifierFactory()
				if err != nil {
					_ = opts.Store.ReleaseCapacity(opts.TaskID)                //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
					_ = opts.Store.ReleaseCandidateProbes(probes, opts.TaskID) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
					return err
				}
				ownedNotifier, opts.Notifier = notifier, notifier
			}
			attemptedAny = true
			attemptOpts := ACPWorkerOptions{
				Store: opts.Store, Notifier: opts.Notifier, TaskID: opts.TaskID, AttemptID: attemptID,
				Agent: candidate.Name, Model: candidate.Model, AvailabilityGroup: candidate.AvailabilityGroup,
				Cooldown: candidate.Cooldown, Command: candidate.Command, Workspace: opts.Workspace,
				Brief: brief, Policy: opts.Policy, bridgeStart: opts.bridgeStart,
			}
			runErr := RunACPAttempt(ctx, attemptOpts)
			if err := opts.Store.ReleaseCapacity(opts.TaskID); err != nil {
				return err
			}
			if len(probes) > 0 {
				signal := FailureSignal{Kind: FailureSuccess}
				if runErr != nil {
					signal = ClassifyACPFailure(runErr)
				}
				if err := opts.Store.FinishCandidateProbes(probes, opts.TaskID, candidate.Name, candidate.AvailabilityGroup, signal, parseCooldown(candidate.Cooldown), time.Now()); err != nil {
					return err
				}
			}
			if runErr == nil {
				return nil
			}
			task, err = opts.Store.Read(opts.TaskID)
			if err != nil {
				return err
			}
			if task.Status != TaskQueued {
				return runErr
			}
			if !ClassifyACPFailure(runErr).Recoverable() {
				return runErr
			}
			attempt := task.Attempts[len(task.Attempts)-1]
			handoff := opts.renderHandoff(attempt, task)
			path := filepath.Join(opts.Store.taskDir(opts.TaskID), "artifacts", "handoff-"+attempt.ID+".md")
			if err := atomicWrite(path, []byte(handoff)); err != nil {
				stateErr := opts.intervene("The previous attempt stopped, but its handoff could not be retained")
				return fmt.Errorf("retain handoff: %v; record intervention: %w", err, stateErr)
			}
			if err := opts.Store.Update(opts.TaskID, "handoff_ready", "Previous agent stopped; handoff retained before replacement", func(task *Task) error {
				task.Artifacts = append(task.Artifacts, Artifact{Name: "handoff-" + attempt.ID, Path: path})
				return nil
			}); err != nil {
				return err
			}
			if err := opts.publishLatest(); err != nil {
				return err
			}
			brief = opts.Brief + "\n\n" + handoff
			continueChain = true
			break
		}
		if continueChain {
			continue
		}
		if task.Status == TaskQueued && (waiting || !attemptedAny) {
			_ = opts.Store.Update(opts.TaskID, "availability_wait", "No eligible agent is currently available; task remains queued", func(*Task) error { return nil }) //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
			_ = opts.publishLatest()                                                                                                                                   //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
			return ErrWaitingAvailability
		}
		return opts.intervene("No eligible configured agent remains for the task")
	}
}

func candidateCapacityKey(prefix, name string) string {
	digest := sha256.Sum256([]byte(name))
	return prefix + hex.EncodeToString(digest[:])
}

func (opts ACPTaskOptions) renderHandoff(previous Attempt, task Task) string {
	if opts.RenderHandoff != nil {
		if rendered := strings.TrimSpace(opts.RenderHandoff(previous, task)); rendered != "" {
			return rendered + "\n"
		}
	}
	return fmt.Sprintf("## Verified progress\n\nNo independent verification was recorded for attempt %s.\n\n## Residual work\n\nContinue the task using the retained workspace and artifacts.\n\n## References\n\n- Attempt: %s (%s / %s)\n- Workspace: %s\n\n## Next action\n\nReview the retained changes before editing.\n", previous.ID, previous.ID, previous.Agent, previous.Model, task.Source.Workspace)
}

func (opts ACPTaskOptions) intervene(summary string) error {
	err := opts.Store.Update(opts.TaskID, "intervention_required", summary, func(task *Task) error {
		task.Status = TaskInterventionRequired
		if len(task.Attempts) > 0 {
			last := &task.Attempts[len(task.Attempts)-1]
			if last.Status == string(TaskRunning) {
				last.Status, last.FinishedAt = "intervention_required", time.Now().UTC()
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = opts.publishLatest() //nolint:errcheck // Cleanup or secondary reporting must not replace the primary result.
	return fmt.Errorf("%s", summary)
}

func (opts ACPTaskOptions) publishLatest() error {
	if opts.Notifier == nil {
		return nil
	}
	task, err := opts.Store.Read(opts.TaskID)
	if err != nil {
		return err
	}
	return opts.Notifier.Publish(task.Revision)
}

func currentAttemptFor(task Task, agent string) bool {
	if task.Status != TaskRunning || len(task.Attempts) == 0 {
		return false
	}
	last := task.Attempts[len(task.Attempts)-1]
	return last.Status == string(TaskRunning) && (last.Agent == "" || last.Agent == agent)
}

func nextAttemptID(task Task, current bool) string {
	if current {
		return task.Attempts[len(task.Attempts)-1].ID
	}
	return fmt.Sprintf("attempt-%d", len(task.Attempts)+1)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsAll(values, required []string) bool {
	for _, need := range required {
		if !containsString(values, need) {
			return false
		}
	}
	return true
}

func parseCooldown(raw string) time.Duration {
	cooldown, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return cooldown
}
