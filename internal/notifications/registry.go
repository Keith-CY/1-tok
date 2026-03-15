package notifications

import (
	"sync"
)

// WebhookRegistration represents a registered webhook.
type WebhookRegistration struct {
	Target string `json:"target"`
	URL    string `json:"url"`
}

// Registry manages webhook registrations and exposes them for listing.
type Registry struct {
	mu   sync.RWMutex
	regs map[string]string // target → URL
	svc  *WebhookService
}

// NewRegistry creates a registry backed by a WebhookService.
func NewRegistry(svc *WebhookService) *Registry {
	return &Registry{
		regs: make(map[string]string),
		svc:  svc,
	}
}

// Register adds a webhook and records it in the registry.
func (r *Registry) Register(target, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.regs[target] = url
	if r.svc != nil {
		r.svc.Register(target, url)
	}
}

// Unregister removes a webhook.
func (r *Registry) Unregister(target string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.regs, target)
	if r.svc != nil {
		r.svc.Unregister(target)
	}
}

// List returns all registered webhooks.
func (r *Registry) List() []WebhookRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]WebhookRegistration, 0, len(r.regs))
	for target, url := range r.regs {
		result = append(result, WebhookRegistration{Target: target, URL: url})
	}
	return result
}

// Get returns the webhook for a target.
func (r *Registry) Get(target string) (WebhookRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, ok := r.regs[target]
	if !ok {
		return WebhookRegistration{}, false
	}
	return WebhookRegistration{Target: target, URL: url}, true
}
