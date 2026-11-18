package delegation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AvailabilityState struct {
	Revision   uint64                  `json:"revision"`
	Exclusions []AvailabilityExclusion `json:"exclusions,omitempty"`
	Probes     []RecoveryProbe         `json:"probes,omitempty"`
}

type AvailabilityExclusion struct {
	Key       string      `json:"key"`
	Agent     string      `json:"agent"`
	Group     string      `json:"group,omitempty"`
	Kind      FailureKind `json:"kind"`
	Reason    string      `json:"reason"`
	Since     time.Time   `json:"since"`
	Until     time.Time   `json:"until"`
	ProbeTask string      `json:"probe_task,omitempty"`
}

type RecoveryProbe struct {
	Key       string    `json:"key"`
	TaskID    string    `json:"task_id"`
	StartedAt time.Time `json:"started_at"`
}

func validAvailabilityKey(key string) bool {
	for _, prefix := range []string{"entry:", "group:"} {
		if strings.HasPrefix(key, prefix) {
			value := strings.TrimPrefix(key, prefix)
			return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\x00\r\n")
		}
	}
	return false
}

// RecordAvailabilityFailure persists only a known quota or unavailability signal.
func (s *Store) RecordAvailabilityFailure(agent, group string, signal FailureSignal, cooldown time.Duration, now time.Time) (AvailabilityExclusion, error) {
	if !signal.Recoverable() {
		return AvailabilityExclusion{}, fmt.Errorf("failure %q is not recoverable", signal.Kind)
	}
	if strings.TrimSpace(agent) == "" {
		return AvailabilityExclusion{}, fmt.Errorf("agent name is required")
	}
	exclusion, err := availabilityExpiry(agent, group, signal, cooldown, now)
	if err != nil {
		return AvailabilityExclusion{}, err
	}
	err = s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		upsertExclusion(&state, exclusion)
		state.Revision++
		return s.writeAvailability(state)
	})
	return exclusion, err
}

func (s *Store) Availability(now time.Time) ([]AvailabilityExclusion, error) {
	state, err := s.readAvailability()
	if err != nil {
		return nil, err
	}
	active := make([]AvailabilityExclusion, 0, len(state.Exclusions))
	for _, exclusion := range state.Exclusions {
		if exclusion.Until.After(now) {
			active = append(active, exclusion)
		}
	}
	return active, nil
}

func (s *Store) ResetAvailability(key string) error {
	if !validAvailabilityKey(key) {
		return fmt.Errorf("invalid availability key %q", key)
	}
	return s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		for _, probe := range state.Probes {
			if probe.Key == key {
				return fmt.Errorf("cannot reset availability while task %q owns the recovery probe", probe.TaskID)
			}
		}
		filtered := state.Exclusions[:0]
		for _, exclusion := range state.Exclusions {
			if exclusion.Key != key {
				filtered = append(filtered, exclusion)
			}
		}
		if len(filtered) == len(state.Exclusions) {
			return nil
		}
		state.Exclusions = filtered
		state.Revision++
		return s.writeAvailability(state)
	})
}

func (s *Store) RecoveryProbes(taskID string) ([]RecoveryProbe, error) {
	state, err := s.readAvailability()
	if err != nil {
		return nil, err
	}
	var owned []RecoveryProbe
	for _, probe := range state.Probes {
		if probe.TaskID == taskID {
			owned = append(owned, probe)
		}
	}
	return owned, nil
}

func (s *Store) CandidateAvailable(agent, group, taskID string, now time.Time) (bool, error) {
	state, err := s.readAvailability()
	if err != nil {
		return false, err
	}
	for _, exclusion := range state.Exclusions {
		if exclusion.ProbeTask != "" {
			if exclusion.ProbeTask == taskID {
				continue
			}
			if exclusion.Key == "entry:"+agent || (group != "" && exclusion.Key == "group:"+group) {
				return false, nil
			}
		}
		if !exclusion.Until.After(now) {
			continue
		}
		if exclusion.Key == "entry:"+agent || (group != "" && exclusion.Key == "group:"+group) {
			return false, nil
		}
	}
	return true, nil
}

// ClaimCandidate atomically excludes active failures and claims every expired key as one
// recovery probe, so an entry and its shared group cannot be probed by different tasks.
//
//nolint:gocognit // This function preserves a multi-step state transition as one auditable unit.
func (s *Store) ClaimCandidate(agent, group, taskID string, now time.Time) (bool, []string, error) {
	if strings.TrimSpace(agent) == "" || !safeID.MatchString(taskID) {
		return false, nil, fmt.Errorf("agent and safe task id are required")
	}
	var available bool
	var probes []string
	err := s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		for i := range state.Exclusions {
			exclusion := &state.Exclusions[i]
			matches := exclusion.Key == "entry:"+agent || (group != "" && exclusion.Key == "group:"+group)
			if !matches {
				continue
			}
			if exclusion.ProbeTask != "" && exclusion.ProbeTask != taskID {
				return nil
			}
			if exclusion.Until.After(now) && exclusion.ProbeTask != taskID {
				return nil
			}
			if !exclusion.Until.After(now) {
				exclusion.ProbeTask = taskID
				probes = append(probes, exclusion.Key)
			}
		}
		for _, key := range probes {
			found := false
			for _, probe := range state.Probes {
				if probe.Key == key && probe.TaskID == taskID {
					found = true
					break
				}
			}
			if !found {
				state.Probes = append(state.Probes, RecoveryProbe{Key: key, TaskID: taskID, StartedAt: now.UTC()})
			}
		}
		available = true
		if len(probes) > 0 {
			state.Revision++
			return s.writeAvailability(state)
		}
		return nil
	})
	return available, probes, err
}

//nolint:gocognit,gocyclo // This function preserves a multi-step state transition as one auditable unit.
func (s *Store) FinishCandidateProbes(keys []string, taskID, agent, group string, signal FailureSignal, cooldown time.Duration, now time.Time) error {
	if len(keys) == 0 {
		return nil
	}
	return s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		keySet := map[string]bool{}
		for _, key := range keys {
			keySet[key] = true
		}
		found := map[string]bool{}
		probes := state.Probes[:0]
		for _, probe := range state.Probes {
			if probe.TaskID == taskID && keySet[probe.Key] {
				found[probe.Key] = true
				continue
			}
			probes = append(probes, probe)
		}
		for _, key := range keys {
			if !found[key] {
				return fmt.Errorf("recovery probe %q is not owned by task %q", key, taskID)
			}
		}
		state.Probes = probes
		indeterminate := signal.Kind == FailureUnknown
		for i := range state.Exclusions {
			if keySet[state.Exclusions[i].Key] && state.Exclusions[i].ProbeTask == taskID {
				if indeterminate {
					if cooldown <= 0 {
						return fmt.Errorf("indeterminate recovery probe needs a positive cooldown")
					}
					state.Exclusions[i].Kind = FailureUnavailable
					state.Exclusions[i].Reason = "recovery_probe_indeterminate"
					state.Exclusions[i].Since = now.UTC()
					state.Exclusions[i].Until = now.Add(cooldown).UTC()
				}
				state.Exclusions[i].ProbeTask = ""
			}
		}
		if !indeterminate {
			for _, key := range keys {
				state.Exclusions = filterExclusion(state.Exclusions, key, false)
			}
		}
		if signal.Recoverable() {
			exclusion, err := availabilityExpiry(agent, group, signal, cooldown, now)
			if err != nil {
				return err
			}
			upsertExclusion(&state, exclusion)
		}
		state.Revision++
		return s.writeAvailability(state)
	})
}

// ReleaseCandidateProbes returns an expired exclusion to the shared queue when
// capacity prevented a real attempt from starting; otherwise a speculative claim
// could strand the only recovery slot without contacting the provider.
func (s *Store) ReleaseCandidateProbes(keys []string, taskID string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		keySet := map[string]bool{}
		for _, key := range keys {
			keySet[key] = true
		}
		found := map[string]bool{}
		probes := state.Probes[:0]
		for _, probe := range state.Probes {
			if probe.TaskID == taskID && keySet[probe.Key] {
				found[probe.Key] = true
				continue
			}
			probes = append(probes, probe)
		}
		for _, key := range keys {
			if !found[key] {
				return fmt.Errorf("recovery probe %q is not owned by task %q", key, taskID)
			}
		}
		state.Probes = probes
		for i := range state.Exclusions {
			if keySet[state.Exclusions[i].Key] && state.Exclusions[i].ProbeTask == taskID {
				state.Exclusions[i].ProbeTask = ""
			}
		}
		state.Revision++
		return s.writeAvailability(state)
	})
}

// ClaimRecoveryProbe admits one real task as the probe after an exclusion expires.
func (s *Store) ClaimRecoveryProbe(key, taskID string, now time.Time) (bool, error) {
	if !safeID.MatchString(taskID) || !validAvailabilityKey(key) {
		return false, fmt.Errorf("invalid recovery probe identity")
	}
	claimed := false
	err := s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		for i := range state.Exclusions {
			exclusion := &state.Exclusions[i]
			if exclusion.Key != key || exclusion.Until.After(now) {
				continue
			}
			if exclusion.ProbeTask != "" {
				return nil
			}
			for _, probe := range state.Probes {
				if probe.Key == key {
					return nil
				}
			}
			exclusion.ProbeTask = taskID
			state.Probes = append(state.Probes, RecoveryProbe{Key: key, TaskID: taskID, StartedAt: now.UTC()})
			state.Revision++
			if err := s.writeAvailability(state); err != nil {
				return err
			}
			claimed = true
			return nil
		}
		return nil
	})
	return claimed, err
}

// FinishRecoveryProbe releases a conclusive probe and records a renewed exclusion if it
// found the provider still unavailable. Success or a non-availability outcome clears the
// expired exclusion; an unknown result never invents a longer recovery time.
func (s *Store) FinishRecoveryProbe(key, taskID, agent, group string, signal FailureSignal, cooldown time.Duration, now time.Time) error {
	return s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		probeIndex := -1
		for i := range state.Probes {
			if state.Probes[i].Key == key && state.Probes[i].TaskID == taskID {
				probeIndex = i
				break
			}
		}
		if probeIndex < 0 {
			return fmt.Errorf("recovery probe is not owned by task %q", taskID)
		}
		state.Probes = append(state.Probes[:probeIndex], state.Probes[probeIndex+1:]...)
		for i := range state.Exclusions {
			if state.Exclusions[i].Key == key && state.Exclusions[i].ProbeTask == taskID {
				state.Exclusions[i].ProbeTask = ""
			}
		}
		state.Exclusions = filterExclusion(state.Exclusions, key, signal.Recoverable())
		if signal.Recoverable() {
			exclusion, err := availabilityExpiry(agent, group, signal, cooldown, now)
			if err != nil {
				return err
			}
			upsertExclusion(&state, exclusion)
		}
		state.Revision++
		return s.writeAvailability(state)
	})
}

// ReconcileRecoveryProbe fences a crashed probe and applies one bounded cooldown.
func (s *Store) ReconcileRecoveryProbe(key, taskID, agent, group string, cooldown time.Duration, now time.Time) error {
	if cooldown <= 0 {
		return fmt.Errorf("recovery probe reconciliation needs a positive cooldown")
	}
	return s.WithLock("availability-global", func() error {
		state, err := s.readAvailability()
		if err != nil {
			return err
		}
		found := false
		probes := state.Probes[:0]
		for _, probe := range state.Probes {
			if probe.Key == key && probe.TaskID == taskID {
				found = true
				continue
			}
			probes = append(probes, probe)
		}
		if !found {
			return fmt.Errorf("recovery probe is not owned by task %q", taskID)
		}
		state.Probes = probes
		for i := range state.Exclusions {
			if state.Exclusions[i].Key == key && state.Exclusions[i].ProbeTask == taskID {
				state.Exclusions[i].ProbeTask = ""
				state.Exclusions[i].Kind = FailureUnavailable
				state.Exclusions[i].Reason = "recovery_probe_interrupted"
				state.Exclusions[i].Since = now.UTC()
				state.Exclusions[i].Until = now.Add(cooldown).UTC()
			}
		}
		state.Revision++
		return s.writeAvailability(state)
	})
}

func NextLocalMidnight(now time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.Local
	}
	local := now.In(location)
	year, month, day := local.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, location)
}

func availabilityExpiry(agent, group string, signal FailureSignal, cooldown time.Duration, now time.Time) (AvailabilityExclusion, error) {
	key, scopeGroup := "entry:"+agent, ""
	if signal.Scope == "group" && group != "" {
		key, scopeGroup = "group:"+group, group
	}
	until := signal.Until
	if until.IsZero() {
		if cooldown > 0 && signal.Recoverable() {
			until = now.Add(cooldown)
		} else if signal.Kind == FailureQuota {
			until = NextLocalMidnight(now, time.Local)
		} else {
			return AvailabilityExclusion{}, fmt.Errorf("availability expiry is not defined for %q", signal.Kind)
		}
	}
	if !until.After(now) {
		return AvailabilityExclusion{}, fmt.Errorf("availability expiry must be after failure time")
	}
	return AvailabilityExclusion{Key: key, Agent: agent, Group: scopeGroup, Kind: signal.Kind, Reason: string(signal.Kind), Since: now.UTC(), Until: until.UTC()}, nil
}

func (s *Store) readAvailability() (AvailabilityState, error) {
	data, err := os.ReadFile(s.availabilityPath()) //#nosec G304 -- fixed file below project-owned state directory
	if os.IsNotExist(err) {
		return AvailabilityState{}, nil
	}
	if err != nil {
		return AvailabilityState{}, err
	}
	var state AvailabilityState
	if err := json.Unmarshal(data, &state); err != nil {
		return AvailabilityState{}, fmt.Errorf("decode availability state: %w", err)
	}
	return state, nil
}

func (s *Store) writeAvailability(state AvailabilityState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.availabilityPath(), append(data, '\n'))
}

func (s *Store) availabilityPath() string { return filepath.Join(s.root, "availability.json") }

func upsertExclusion(state *AvailabilityState, exclusion AvailabilityExclusion) {
	for i := range state.Exclusions {
		if state.Exclusions[i].Key == exclusion.Key {
			probeTask := state.Exclusions[i].ProbeTask
			exclusion.ProbeTask = probeTask
			state.Exclusions[i] = exclusion
			return
		}
	}
	state.Exclusions = append(state.Exclusions, exclusion)
}

func filterExclusion(exclusions []AvailabilityExclusion, key string, keep bool) []AvailabilityExclusion {
	filtered := exclusions[:0]
	for _, exclusion := range exclusions {
		if exclusion.Key == key && !keep {
			continue
		}
		filtered = append(filtered, exclusion)
	}
	return filtered
}
