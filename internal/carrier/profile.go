package carrier

import "sync"

// ExecutionProfile is a reusable execution configuration for a Carrier.
type ExecutionProfile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	CarrierID   string            `json:"carrierId"`
	Host        string            `json:"host"`
	Agent       string            `json:"agent"`
	Backend     string            `json:"backend"`
	Workspace   string            `json:"workspace"`
	Environment map[string]string `json:"environment,omitempty"`
	MaxTimeout  string            `json:"maxTimeout,omitempty"`
}

// ProfileRegistry manages execution profiles.
type ProfileRegistry struct {
	mu       sync.RWMutex
	profiles map[string]ExecutionProfile
}

func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{profiles: make(map[string]ExecutionProfile)}
}

func (r *ProfileRegistry) Register(profile ExecutionProfile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[profile.ID] = profile
}

func (r *ProfileRegistry) Get(id string) (ExecutionProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[id]
	return p, ok
}

func (r *ProfileRegistry) List(carrierID string) []ExecutionProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ExecutionProfile, 0)
	for _, p := range r.profiles {
		if carrierID == "" || p.CarrierID == carrierID {
			result = append(result, p)
		}
	}
	return result
}

func (r *ProfileRegistry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.profiles, id)
}
