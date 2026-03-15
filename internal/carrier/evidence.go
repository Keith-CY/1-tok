package carrier

import (
	"fmt"
	"sync"
	"time"
)

// EvidencePackage contains execution artifacts and log summary for a completed job.
type EvidencePackage struct {
	ID          string           `json:"id"`
	JobID       string           `json:"jobId"`
	BindingID   string           `json:"bindingId"`
	Summary     string           `json:"summary"`
	Artifacts   []Artifact       `json:"artifacts"`
	UsageReport *UsageReport     `json:"usageReport,omitempty"`
	SubmittedAt time.Time        `json:"submittedAt"`
}

// Artifact is a reference to an execution artifact stored by the Carrier.
type Artifact struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // log, output, screenshot, trace
	URL      string `json:"url"`
	SizeBytes int64 `json:"sizeBytes,omitempty"`
}

// UsageReport is the Carrier's self-reported usage for reconciliation.
type UsageReport struct {
	TokenCount    int64  `json:"tokenCount"`
	StepCount     int64  `json:"stepCount"`
	APICallCount  int64  `json:"apiCallCount"`
	TotalCostCents int64 `json:"totalCostCents"`
}

// EvidenceStore manages evidence packages.
type EvidenceStore struct {
	mu   sync.Mutex
	seq  int
	data map[string]EvidencePackage // jobID → evidence
}

// NewEvidenceStore creates a new evidence store.
func NewEvidenceStore() *EvidenceStore {
	return &EvidenceStore{data: make(map[string]EvidencePackage)}
}

// Submit stores an evidence package for a completed job.
func (s *EvidenceStore) Submit(jobID, bindingID, summary string, artifacts []Artifact, usage *UsageReport) (EvidencePackage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[jobID]; exists {
		return EvidencePackage{}, fmt.Errorf("evidence already submitted for job %s", jobID)
	}

	s.seq++
	pkg := EvidencePackage{
		ID:          fmt.Sprintf("ev_%d", s.seq),
		JobID:       jobID,
		BindingID:   bindingID,
		Summary:     summary,
		Artifacts:   artifacts,
		UsageReport: usage,
		SubmittedAt: time.Now().UTC(),
	}
	s.data[jobID] = pkg
	return pkg, nil
}

// Get returns the evidence package for a job.
func (s *EvidenceStore) Get(jobID string) (EvidencePackage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg, ok := s.data[jobID]
	if !ok {
		return EvidencePackage{}, fmt.Errorf("no evidence for job %s", jobID)
	}
	return pkg, nil
}
