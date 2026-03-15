// Package carrier defines the async execution protocol for Carrier integration.
package carrier

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// JobState represents the state of an execution job.
type JobState string

const (
	JobStatePending   JobState = "pending"
	JobStateRunning   JobState = "running"
	JobStateCompleted JobState = "completed"
	JobStateFailed    JobState = "failed"
	JobStateCancelled JobState = "cancelled"
)

var (
	ErrBindingExists      = errors.New("carrier already bound to this milestone")
	ErrBindingNotFound    = errors.New("carrier binding not found")
	ErrJobNotFound        = errors.New("execution job not found")
	ErrInvalidTransition  = errors.New("invalid job state transition")
	ErrCarrierStale       = errors.New("carrier heartbeat stale")
)

// HeartbeatTimeout is how long before a carrier is considered stale.
const HeartbeatTimeout = 2 * time.Minute

// Binding represents a Carrier instance bound to an order milestone.
type Binding struct {
	ID           string    `json:"id"`
	CarrierID    string    `json:"carrierId"`
	OrderID      string    `json:"orderId"`
	MilestoneID  string    `json:"milestoneId"`
	Capabilities []string  `json:"capabilities"`
	BoundAt      time.Time `json:"boundAt"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
}

// ExecutionJob represents an async execution task managed by a Carrier.
type ExecutionJob struct {
	ID           string     `json:"id"`
	BindingID    string     `json:"bindingId"`
	MilestoneID  string     `json:"milestoneId"`
	State        JobState   `json:"state"`
	Input        string     `json:"input,omitempty"`
	Output       string     `json:"output,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	Progress     *Progress  `json:"progress,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

// Progress represents execution progress.
type Progress struct {
	Step    int    `json:"step"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

// validTransitions defines allowed state transitions.
var validTransitions = map[JobState][]JobState{
	JobStatePending:   {JobStateRunning, JobStateCancelled},
	JobStateRunning:   {JobStateCompleted, JobStateFailed, JobStateCancelled},
	JobStateCompleted: {},
	JobStateFailed:    {},
	JobStateCancelled: {},
}

func canTransition(from, to JobState) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// Service manages carrier bindings and execution jobs.
type Service struct {
	mu          sync.Mutex
	bindingSeq  int
	jobSeq      int
	bindings    map[string]Binding            // id → binding
	bindingIdx  map[string]string             // "orderID|milestoneID" → binding id
	jobs        map[string]ExecutionJob       // id → job
	jobsByBinding map[string][]string         // binding id → job ids
}

// NewService creates a new carrier service.
func NewService() *Service {
	return &Service{
		bindings:      make(map[string]Binding),
		bindingIdx:    make(map[string]string),
		jobs:          make(map[string]ExecutionJob),
		jobsByBinding: make(map[string][]string),
	}
}

// Bind creates a carrier binding for a milestone.
func (s *Service) Bind(orderID, milestoneID, carrierID string, capabilities []string) (Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := orderID + "|" + milestoneID
	if _, exists := s.bindingIdx[key]; exists {
		return Binding{}, ErrBindingExists
	}

	s.bindingSeq++
	now := time.Now().UTC()
	binding := Binding{
		ID:            fmt.Sprintf("bind_%d", s.bindingSeq),
		CarrierID:     carrierID,
		OrderID:       orderID,
		MilestoneID:   milestoneID,
		Capabilities:  capabilities,
		BoundAt:       now,
		LastHeartbeat: now,
	}
	s.bindings[binding.ID] = binding
	s.bindingIdx[key] = binding.ID
	return binding, nil
}

// GetBinding retrieves a binding by order and milestone.
func (s *Service) GetBinding(orderID, milestoneID string) (Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := orderID + "|" + milestoneID
	id, ok := s.bindingIdx[key]
	if !ok {
		return Binding{}, ErrBindingNotFound
	}
	return s.bindings[id], nil
}

// Heartbeat updates the last heartbeat time for a binding.
func (s *Service) Heartbeat(bindingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bindings[bindingID]
	if !ok {
		return ErrBindingNotFound
	}
	b.LastHeartbeat = time.Now().UTC()
	s.bindings[bindingID] = b
	return nil
}

// IsStale checks if a binding's heartbeat is stale.
func (s *Service) IsStale(bindingID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bindings[bindingID]
	if !ok {
		return false, ErrBindingNotFound
	}
	return time.Since(b.LastHeartbeat) > HeartbeatTimeout, nil
}

// CreateJob creates a new execution job.
func (s *Service) CreateJob(bindingID, milestoneID, input string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.bindings[bindingID]; !ok {
		return ExecutionJob{}, ErrBindingNotFound
	}

	s.jobSeq++
	job := ExecutionJob{
		ID:          fmt.Sprintf("job_%d", s.jobSeq),
		BindingID:   bindingID,
		MilestoneID: milestoneID,
		State:       JobStatePending,
		Input:       input,
		CreatedAt:   time.Now().UTC(),
	}
	s.jobs[job.ID] = job
	s.jobsByBinding[bindingID] = append(s.jobsByBinding[bindingID], job.ID)
	return job, nil
}

// GetJob retrieves a job by ID.
func (s *Service) GetJob(jobID string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return ExecutionJob{}, ErrJobNotFound
	}
	return job, nil
}

// StartJob transitions a job from pending to running.
func (s *Service) StartJob(jobID string) (ExecutionJob, error) {
	return s.transitionJob(jobID, JobStateRunning, "", "")
}

// CompleteJob transitions a job to completed with output.
func (s *Service) CompleteJob(jobID, output string) (ExecutionJob, error) {
	return s.transitionJob(jobID, JobStateCompleted, output, "")
}

// FailJob transitions a job to failed with an error message.
func (s *Service) FailJob(jobID, errMsg string) (ExecutionJob, error) {
	return s.transitionJob(jobID, JobStateFailed, "", errMsg)
}

// CancelJob transitions a job to cancelled.
func (s *Service) CancelJob(jobID string) (ExecutionJob, error) {
	return s.transitionJob(jobID, JobStateCancelled, "", "")
}

func (s *Service) transitionJob(jobID string, to JobState, output, errMsg string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return ExecutionJob{}, ErrJobNotFound
	}
	if !canTransition(job.State, to) {
		return ExecutionJob{}, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, job.State, to)
	}
	now := time.Now().UTC()
	job.State = to
	switch to {
	case JobStateRunning:
		job.StartedAt = &now
	case JobStateCompleted:
		job.CompletedAt = &now
		job.Output = output
	case JobStateFailed:
		job.CompletedAt = &now
		job.ErrorMessage = errMsg
	case JobStateCancelled:
		job.CompletedAt = &now
	}
	s.jobs[jobID] = job
	return job, nil
}

// UpdateProgress updates the progress of a running job.
func (s *Service) UpdateProgress(jobID string, step, total int, message string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return ExecutionJob{}, ErrJobNotFound
	}
	if job.State != JobStateRunning {
		return ExecutionJob{}, fmt.Errorf("can only update progress on running jobs, current state: %s", job.State)
	}
	job.Progress = &Progress{Step: step, Total: total, Message: message}
	s.jobs[jobID] = job
	return job, nil
}

// ListJobs returns all jobs for a binding.
func (s *Service) ListJobs(bindingID string) ([]ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.jobsByBinding[bindingID]
	result := make([]ExecutionJob, 0, len(ids))
	for _, id := range ids {
		if job, ok := s.jobs[id]; ok {
			result = append(result, job)
		}
	}
	return result, nil
}

// StaleJobTimeout is the duration after which a running job with no heartbeat is considered stale.
const StaleJobTimeout = 5 * time.Minute

// StaleJob represents a job that missed its heartbeat window.
type StaleJob struct {
	Job           ExecutionJob `json:"job"`
	BindingID     string       `json:"bindingId"`
	LastHeartbeat time.Time    `json:"lastHeartbeat"`
	StaleSince    time.Duration `json:"staleSince"`
}

// ReconcileStaleJobs finds running jobs whose binding heartbeat is stale.
func (s *Service) ReconcileStaleJobs() []StaleJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stale []StaleJob
	for _, job := range s.jobs {
		if job.State != JobStateRunning {
			continue
		}
		binding, ok := s.bindings[job.BindingID]
		if !ok {
			continue
		}
		elapsed := time.Since(binding.LastHeartbeat)
		if elapsed > StaleJobTimeout {
			stale = append(stale, StaleJob{
				Job:           job,
				BindingID:     job.BindingID,
				LastHeartbeat: binding.LastHeartbeat,
				StaleSince:    elapsed,
			})
		}
	}
	return stale
}
