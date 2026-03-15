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
	bindings    []Binding
	jobs        []ExecutionJob
}

// NewService creates a new carrier service.
func NewService() *Service {
	return &Service{}
}

// Bind creates a carrier binding for a milestone.
func (s *Service) Bind(orderID, milestoneID, carrierID string, capabilities []string) (Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, b := range s.bindings {
		if b.OrderID == orderID && b.MilestoneID == milestoneID {
			return Binding{}, ErrBindingExists
		}
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
	s.bindings = append(s.bindings, binding)
	return binding, nil
}

// GetBinding retrieves a binding by order and milestone.
func (s *Service) GetBinding(orderID, milestoneID string) (Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, b := range s.bindings {
		if b.OrderID == orderID && b.MilestoneID == milestoneID {
			return b, nil
		}
	}
	return Binding{}, ErrBindingNotFound
}

// Heartbeat updates the last heartbeat time for a binding.
func (s *Service) Heartbeat(bindingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.bindings {
		if s.bindings[i].ID == bindingID {
			s.bindings[i].LastHeartbeat = time.Now().UTC()
			return nil
		}
	}
	return ErrBindingNotFound
}

// IsStale checks if a binding's heartbeat is stale.
func (s *Service) IsStale(bindingID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, b := range s.bindings {
		if b.ID == bindingID {
			return time.Since(b.LastHeartbeat) > HeartbeatTimeout, nil
		}
	}
	return false, ErrBindingNotFound
}

// CreateJob creates a new execution job.
func (s *Service) CreateJob(bindingID, milestoneID, input string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify binding exists
	found := false
	for _, b := range s.bindings {
		if b.ID == bindingID {
			found = true
			break
		}
	}
	if !found {
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
	s.jobs = append(s.jobs, job)
	return job, nil
}

// GetJob retrieves a job by ID.
func (s *Service) GetJob(jobID string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, j := range s.jobs {
		if j.ID == jobID {
			return j, nil
		}
	}
	return ExecutionJob{}, ErrJobNotFound
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

	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			if !canTransition(s.jobs[i].State, to) {
				return ExecutionJob{}, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, s.jobs[i].State, to)
			}
			now := time.Now().UTC()
			s.jobs[i].State = to
			switch to {
			case JobStateRunning:
				s.jobs[i].StartedAt = &now
			case JobStateCompleted:
				s.jobs[i].CompletedAt = &now
				s.jobs[i].Output = output
			case JobStateFailed:
				s.jobs[i].CompletedAt = &now
				s.jobs[i].ErrorMessage = errMsg
			case JobStateCancelled:
				s.jobs[i].CompletedAt = &now
			}
			return s.jobs[i], nil
		}
	}
	return ExecutionJob{}, ErrJobNotFound
}

// UpdateProgress updates the progress of a running job.
func (s *Service) UpdateProgress(jobID string, step, total int, message string) (ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			if s.jobs[i].State != JobStateRunning {
				return ExecutionJob{}, fmt.Errorf("can only update progress on running jobs, current state: %s", s.jobs[i].State)
			}
			s.jobs[i].Progress = &Progress{Step: step, Total: total, Message: message}
			return s.jobs[i], nil
		}
	}
	return ExecutionJob{}, ErrJobNotFound
}

// ListJobs returns all jobs for a binding.
func (s *Service) ListJobs(bindingID string) ([]ExecutionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]ExecutionJob, 0)
	for _, j := range s.jobs {
		if j.BindingID == bindingID {
			result = append(result, j)
		}
	}
	return result, nil
}
