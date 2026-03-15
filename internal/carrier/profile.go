package carrier

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
	MaxTimeout  string            `json:"maxTimeout,omitempty"` // e.g., "30m"
}

// ProfileRegistry manages execution profiles.
type ProfileRegistry struct {
	profiles map[string]ExecutionProfile
}

// NewProfileRegistry creates a new registry.
func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{profiles: make(map[string]ExecutionProfile)}
}

// Register adds or updates a profile.
func (r *ProfileRegistry) Register(profile ExecutionProfile) {
	r.profiles[profile.ID] = profile
}

// Get returns a profile by ID.
func (r *ProfileRegistry) Get(id string) (ExecutionProfile, bool) {
	p, ok := r.profiles[id]
	return p, ok
}

// List returns all profiles for a carrier.
func (r *ProfileRegistry) List(carrierID string) []ExecutionProfile {
	result := make([]ExecutionProfile, 0)
	for _, p := range r.profiles {
		if carrierID == "" || p.CarrierID == carrierID {
			result = append(result, p)
		}
	}
	return result
}

// Delete removes a profile.
func (r *ProfileRegistry) Delete(id string) {
	delete(r.profiles, id)
}
