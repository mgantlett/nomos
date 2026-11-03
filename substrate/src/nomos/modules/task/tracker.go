// Package task defines domain models, schema constraints, and tracking mechanisms
// enforced by compiled Go SSoT contracts.
package task

import (
	"sync"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// TaskStore defines the interface for task persistence and causal DAG operations.
type TaskStore interface {
	CreateTask(task TaskRecord) error
	GetTask(key string) (*TaskRecord, error)
	UpdateTask(task TaskRecord) error
	ListTasks() []TaskRecord
	GetWavefront() ([]WavefrontCandidate, error)
	AddDependency(targetKey, blockedByKey string) error
	BindTwin(keyA, keyB string) error
}

// MemoryTaskStore provides an in-memory implementation of TaskStore.
type MemoryTaskStore struct {
	tasks map[string]TaskRecord
	mu    sync.RWMutex
}

// NewMemoryTaskStore creates a new in-memory task repository.
func NewMemoryTaskStore() *MemoryTaskStore {
	return &MemoryTaskStore{
		tasks: make(map[string]TaskRecord),
	}
}

// CreateTask registers a new task, ensuring key uniqueness and invariant compliance.
func (s *MemoryTaskStore) CreateTask(task TaskRecord) error {
	if err := ValidateTaskRecord(&task); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.Key]; exists {
		return types.NewNomosError(types.ErrCodeInvariant, "task %s already exists", task.Key)
	}

	s.tasks[task.Key] = task
	return nil
}

// GetTask retrieves a task record by its canonical key.
func (s *MemoryTaskStore) GetTask(key string) (*TaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, exists := s.tasks[key]
	if !exists {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "task %s not found", key)
	}
	cp := t
	return &cp, nil
}

// UpdateTask modifies an existing task record.
func (s *MemoryTaskStore) UpdateTask(task TaskRecord) error {
	if err := ValidateTaskRecord(&task); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.Key]; !exists {
		return types.NewNomosError(types.ErrCodeValidationFailed, "task %s not found", task.Key)
	}

	s.tasks[task.Key] = task
	return nil
}

// ListTasks returns all registered task records.
func (s *MemoryTaskStore) ListTasks() []TaskRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]TaskRecord, 0, len(s.tasks))
	for _, t := range s.tasks {
		list = append(list, t)
	}
	return list
}

// AddDependency declares that targetKey depends on blockedByKey.
func (s *MemoryTaskStore) AddDependency(targetKey, blockedByKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, exists := s.tasks[targetKey]
	if !exists {
		return types.NewNomosError(types.ErrCodeValidationFailed, "target task %s not found", targetKey)
	}
	if _, exists := s.tasks[blockedByKey]; !exists {
		return types.NewNomosError(types.ErrCodeValidationFailed, "prerequisite task %s not found", blockedByKey)
	}

	for _, dep := range target.Dependencies {
		if dep == blockedByKey {
			return nil
		}
	}

	target.Dependencies = append(target.Dependencies, blockedByKey)
	s.tasks[targetKey] = target
	return nil
}

// BindTwin establishes bilateral twin links between two tasks.
func (s *MemoryTaskStore) BindTwin(keyA, keyB string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskA, okA := s.tasks[keyA]
	if !okA {
		return types.NewNomosError(types.ErrCodeValidationFailed, "task not found: %s", keyA)
	}
	taskB, okB := s.tasks[keyB]
	if !okB {
		return types.NewNomosError(types.ErrCodeValidationFailed, "task not found: %s", keyB)
	}

	taskA.TwinKey = keyB
	taskB.TwinKey = keyA
	s.tasks[keyA] = taskA
	s.tasks[keyB] = taskB
	return nil
}

// GetWavefront returns unblocked candidate tasks ready for concurrent execution.
func (s *MemoryTaskStore) GetWavefront() ([]WavefrontCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var wavefront []WavefrontCandidate
	for _, t := range s.tasks {
		if IsTerminalStatus(t.Status) {
			continue
		}
		if isTaskUnblocked(t, s.tasks) {
			wavefront = append(wavefront, WavefrontCandidate{
				Key:         t.Key,
				Title:       t.Title,
				Plane:       t.Plane,
				Project:     t.Project,
				SchemaType:  t.SchemaType,
				Status:      t.Status,
				Tier:        t.Tier,
				TwinKey:     t.TwinKey,
				IsUnblocked: true,
			})
		}
	}
	return wavefront, nil
}

func isTaskUnblocked(t TaskRecord, all map[string]TaskRecord) bool {
	for _, depKey := range t.Dependencies {
		dep, exists := all[depKey]
		if !exists || dep.Status != StatusDone {
			return false
		}
	}
	return true
}
