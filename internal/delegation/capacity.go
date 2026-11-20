package delegation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type CapacityState struct {
	Revision     uint64                `json:"revision"`
	Reservations []CapacityReservation `json:"reservations,omitempty"`
}

type CapacityReservation struct {
	TaskID   string          `json:"task_id"`
	Keys     []string        `json:"keys"`
	Admitted time.Time       `json:"admitted"`
	Deadline time.Time       `json:"deadline"`
	Worker   ProcessIdentity `json:"worker,omitempty"`
}

// TryAdmit reserves every requested entry/group slot atomically. A false result
// means the task remains queued and no partial capacity was claimed.
//
//nolint:gocognit // This function preserves a multi-step state transition as one auditable unit.
func (s *Store) TryAdmit(taskID string, limits map[string]int) (bool, error) {
	if !safeID.MatchString(taskID) {
		return false, fmt.Errorf("invalid task id %q", taskID)
	}
	keys := make([]string, 0, len(limits))
	for key, limit := range limits {
		if !safeID.MatchString(key) || limit < 1 {
			return false, fmt.Errorf("invalid capacity limit %q=%d", key, limit)
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return false, fmt.Errorf("at least one capacity limit is required")
	}
	sort.Strings(keys)
	var admitted bool
	err := s.WithLock("capacity-global", func() error {
		task, err := s.Read(taskID)
		if err != nil {
			return err
		}
		if task.Status != TaskQueued {
			return fmt.Errorf("task %q must be queued to request capacity", taskID)
		}
		if !task.Deadline.IsZero() && !task.Deadline.After(time.Now()) {
			return fmt.Errorf("task %q deadline has expired", taskID)
		}
		state, err := s.readCapacity()
		if err != nil {
			return err
		}
		for _, reservation := range state.Reservations {
			if reservation.TaskID == taskID {
				admitted = equalStrings(reservation.Keys, keys)
				if !admitted {
					return fmt.Errorf("task %q already holds different capacity", taskID)
				}
				return nil
			}
		}
		for _, key := range keys {
			used := 0
			for _, reservation := range state.Reservations {
				if containsKey(reservation.Keys, key) {
					used++
				}
			}
			if used >= limits[key] {
				return nil
			}
		}
		state.Revision++
		state.Reservations = append(state.Reservations, CapacityReservation{
			TaskID: taskID, Keys: keys, Admitted: time.Now().UTC(), Deadline: task.Deadline,
		})
		if err := s.writeCapacity(state); err != nil {
			return err
		}
		admitted = true
		return nil
	})
	return admitted, err
}

func (s *Store) AttachWorker(taskID string, identity ProcessIdentity) error {
	return s.WithLock("capacity-global", func() error {
		state, err := s.readCapacity()
		if err != nil {
			return err
		}
		for i := range state.Reservations {
			if state.Reservations[i].TaskID == taskID {
				state.Reservations[i].Worker = identity
				state.Revision++
				return s.writeCapacity(state)
			}
		}
		return fmt.Errorf("task %q has no capacity reservation", taskID)
	})
}

func (s *Store) ReleaseCapacity(taskID string) error {
	return s.WithLock("capacity-global", func() error {
		state, err := s.readCapacity()
		if err != nil {
			return err
		}
		filtered := state.Reservations[:0]
		for _, reservation := range state.Reservations {
			if reservation.TaskID != taskID {
				filtered = append(filtered, reservation)
			}
		}
		if len(filtered) == len(state.Reservations) {
			return nil
		}
		state.Reservations = filtered
		state.Revision++
		return s.writeCapacity(state)
	})
}

// ReconcileCapacity releases reservations whose task deadline passed or whose
// recorded worker identity is no longer live. Unstarted reservations remain held.
func (s *Store) ReconcileCapacity(now time.Time) ([]string, error) {
	var released []string
	err := s.WithLock("capacity-global", func() error {
		state, err := s.readCapacity()
		if err != nil {
			return err
		}
		kept := state.Reservations[:0]
		for _, reservation := range state.Reservations {
			expired := !reservation.Deadline.IsZero() && !reservation.Deadline.After(now)
			workerLost, inspectErr := s.reservationWorkerLost(reservation)
			if inspectErr != nil {
				return inspectErr
			}
			if expired || workerLost {
				released = append(released, reservation.TaskID)
				continue
			}
			kept = append(kept, reservation)
		}
		if len(kept) != len(state.Reservations) {
			state.Reservations = kept
			state.Revision++
			return s.writeCapacity(state)
		}
		return nil
	})
	return released, err
}

func (s *Store) reservationWorkerLost(reservation CapacityReservation) (bool, error) {
	worker := reservation.Worker
	if worker.PID == 0 {
		task, err := s.Read(reservation.TaskID)
		if err != nil || task.Worker == nil {
			return false, nil
		}
		worker = *task.Worker
	}
	if worker.PID == 0 {
		return false, nil
	}
	stopped, err := WorkerStopped(worker)
	if err != nil {
		return false, fmt.Errorf("inspect worker for task %s before releasing capacity: %w", reservation.TaskID, err)
	}
	return stopped, nil
}

func (s *Store) readCapacity() (CapacityState, error) {
	path := filepath.Join(s.root, "capacity.json")
	data, err := os.ReadFile(path) //#nosec G304 -- fixed file below the project-owned delegation state directory
	if os.IsNotExist(err) {
		return CapacityState{}, nil
	}
	if err != nil {
		return CapacityState{}, err
	}
	var state CapacityState
	if err := json.Unmarshal(data, &state); err != nil {
		return CapacityState{}, fmt.Errorf("decode capacity state: %w", err)
	}
	return state, nil
}

func (s *Store) writeCapacity(state CapacityState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.root, "capacity.json"), append(data, '\n'))
}

func containsKey(values []string, key string) bool {
	for _, value := range values {
		if value == key {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
