package carrier

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrEventReplay    = errors.New("event already processed (replay)")
	ErrEventReorder   = errors.New("event sequence out of order")
	ErrEventGap       = errors.New("event sequence gap detected")
)

// ExecutionEvent is an append-only callback ledger entry.
type ExecutionEvent struct {
	ID           string         `json:"id"`
	ExecutionID  string         `json:"executionId"`
	EventID      string         `json:"eventId"`
	Sequence     int64          `json:"sequence"`
	EventType    string         `json:"eventType"`
	AttemptID    string         `json:"attemptId,omitempty"`
	PayloadJSON  string         `json:"payloadJson,omitempty"`
	DecisionJSON string         `json:"decisionJson,omitempty"`
	ReceivedAt   time.Time      `json:"receivedAt"`
}

// EventLedger manages the append-only callback event log.
type EventLedger struct {
	mu           sync.Mutex
	seq          int
	events       map[string][]ExecutionEvent  // executionID → events
	eventIndex   map[string]ExecutionEvent    // eventID → event (for replay detection)
	lastSequence map[string]int64            // executionID → last sequence
}

// NewEventLedger creates a new event ledger.
func NewEventLedger() *EventLedger {
	return &EventLedger{
		events:       make(map[string][]ExecutionEvent),
		eventIndex:   make(map[string]ExecutionEvent),
		lastSequence: make(map[string]int64),
	}
}

// Record appends an event to the ledger with replay/gap/reorder checks.
// Returns the event (with decision) and whether it was a replay.
func (l *EventLedger) Record(executionID, eventID string, sequence int64, eventType, attemptID, payloadJSON, decisionJSON string) (ExecutionEvent, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Replay detection: same eventId → return previous decision
	if prev, exists := l.eventIndex[eventID]; exists {
		return prev, true, nil
	}

	lastSeq := l.lastSequence[executionID]

	// Reorder detection: sequence <= last
	if lastSeq > 0 && sequence <= lastSeq {
		return ExecutionEvent{}, false, fmt.Errorf("%w: got %d, last was %d", ErrEventReorder, sequence, lastSeq)
	}

	// Gap detection: sequence > last + 1
	if lastSeq > 0 && sequence > lastSeq+1 {
		return ExecutionEvent{}, false, fmt.Errorf("%w: got %d, expected %d", ErrEventGap, sequence, lastSeq+1)
	}

	l.seq++
	event := ExecutionEvent{
		ID:           fmt.Sprintf("evt_%d", l.seq),
		ExecutionID:  executionID,
		EventID:      eventID,
		Sequence:     sequence,
		EventType:    eventType,
		AttemptID:    attemptID,
		PayloadJSON:  payloadJSON,
		DecisionJSON: decisionJSON,
		ReceivedAt:   time.Now().UTC(),
	}

	l.events[executionID] = append(l.events[executionID], event)
	l.eventIndex[eventID] = event
	l.lastSequence[executionID] = sequence

	return event, false, nil
}

// List returns all events for an execution in order.
func (l *EventLedger) List(executionID string) []ExecutionEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.events[executionID]
}

// LastSequence returns the last processed sequence for an execution.
func (l *EventLedger) LastSequence(executionID string) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastSequence[executionID]
}

// UpdateDecision sets the decisionJSON on an already-recorded event.
func (l *EventLedger) UpdateDecision(eventID, decisionJSON string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ev, ok := l.eventIndex[eventID]; ok {
		ev.DecisionJSON = decisionJSON
		l.eventIndex[eventID] = ev
		// Also update in the events list
		for i := range l.events[ev.ExecutionID] {
			if l.events[ev.ExecutionID][i].EventID == eventID {
				l.events[ev.ExecutionID][i].DecisionJSON = decisionJSON
				break
			}
		}
	}
}
