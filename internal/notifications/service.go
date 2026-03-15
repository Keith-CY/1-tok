// Package notifications provides a notification service for order lifecycle events.
package notifications

import (
	"sync"
	"time"
)

// EventType identifies the kind of notification.
type EventType string

const (
	EventOrderCreated       EventType = "order.created"
	EventMilestoneSettled   EventType = "milestone.settled"
	EventDisputeOpened      EventType = "dispute.opened"
	EventDisputeResolved    EventType = "dispute.resolved"
	EventRFQAwarded         EventType = "rfq.awarded"
	EventOrderCompleted     EventType = "order.completed"
	EventOrderRated         EventType = "order.rated"
	EventBudgetWallHit      EventType = "budget_wall.hit"
)

// Notification represents a single notification event.
type Notification struct {
	ID        string            `json:"id"`
	Event     EventType         `json:"event"`
	Target    string            `json:"target"` // orgId or userId
	Payload   map[string]any    `json:"payload"`
	CreatedAt time.Time         `json:"createdAt"`
	Delivered bool              `json:"delivered"`
}

// Service is the interface for notification delivery.
type Service interface {
	Send(event EventType, target string, payload map[string]any) error
	List(target string) ([]Notification, error)
}

// InMemoryService stores notifications in memory for testing.
type InMemoryService struct {
	mu   sync.Mutex
	seq  int
	data []Notification
}

// NewInMemoryService creates a new in-memory notification service.
func NewInMemoryService() *InMemoryService {
	return &InMemoryService{}
}

// Send records a notification.
func (s *InMemoryService) Send(event EventType, target string, payload map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	s.data = append(s.data, Notification{
		ID:        "notif_" + string(rune('0'+s.seq)),
		Event:     event,
		Target:    target,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
		Delivered: true,
	})
	return nil
}

// List returns all notifications for a target.
func (s *InMemoryService) List(target string) ([]Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Notification, 0)
	for _, n := range s.data {
		if n.Target == target || target == "" {
			result = append(result, n)
		}
	}
	return result, nil
}

// Count returns the total number of notifications.
func (s *InMemoryService) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}
