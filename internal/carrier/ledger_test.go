package carrier

import "testing"

func TestLedger_Record(t *testing.T) {
	l := NewEventLedger()
	ev, replay, err := l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", `{"action":"continue"}`)
	if err != nil { t.Fatal(err) }
	if replay { t.Error("should not be replay") }
	if ev.EventID != "evt_a" { t.Errorf("eventId = %s", ev.EventID) }
}

func TestLedger_Replay(t *testing.T) {
	l := NewEventLedger()
	l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", `{"action":"continue"}`)

	// Replay same eventId
	ev, replay, err := l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", `{"action":"continue"}`)
	if err != nil { t.Fatal(err) }
	if !replay { t.Error("expected replay") }
	if ev.DecisionJSON != `{"action":"continue"}` {
		t.Errorf("expected previous decision, got %s", ev.DecisionJSON)
	}
}

func TestLedger_Reorder(t *testing.T) {
	l := NewEventLedger()
	l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "{}")
	l.Record("exec_1", "evt_b", 2, "usage.reported", "", "{}", "{}")

	// Try sequence 1 again with different eventId
	_, _, err := l.Record("exec_1", "evt_c", 1, "execution.heartbeat", "", "{}", "{}")
	if err == nil { t.Error("expected reorder error") }
}

func TestLedger_Gap(t *testing.T) {
	l := NewEventLedger()
	l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "{}")

	// Skip sequence 2
	_, _, err := l.Record("exec_1", "evt_c", 3, "execution.completed", "", "{}", "{}")
	if err == nil { t.Error("expected gap error") }
}

func TestLedger_Sequential(t *testing.T) {
	l := NewEventLedger()
	l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "{}")
	l.Record("exec_1", "evt_b", 2, "usage.reported", "", "{}", "{}")
	l.Record("exec_1", "evt_c", 3, "execution.completed", "", "{}", "{}")

	events := l.List("exec_1")
	if len(events) != 3 { t.Errorf("expected 3 events, got %d", len(events)) }
	if l.LastSequence("exec_1") != 3 { t.Errorf("lastSeq = %d", l.LastSequence("exec_1")) }
}

func TestLedger_MultipleExecutions(t *testing.T) {
	l := NewEventLedger()
	l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "{}")
	l.Record("exec_2", "evt_b", 1, "execution.started", "", "{}", "{}")

	if len(l.List("exec_1")) != 1 { t.Error("exec_1 should have 1 event") }
	if len(l.List("exec_2")) != 1 { t.Error("exec_2 should have 1 event") }
}

func TestLedger_FirstEvent(t *testing.T) {
	l := NewEventLedger()
	// First event with sequence 1 should work
	_, _, err := l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "{}")
	if err != nil { t.Fatal(err) }
}

func TestLedger_UpdateDecision(t *testing.T) {
	l := NewEventLedger()
	l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "")

	l.UpdateDecision("evt_a", `{"type":"continue"}`)

	// Replay should return updated decision
	ev, replay, _ := l.Record("exec_1", "evt_a", 1, "execution.started", "", "{}", "")
	if !replay { t.Error("expected replay") }
	if ev.DecisionJSON != `{"type":"continue"}` {
		t.Errorf("decision = %s", ev.DecisionJSON)
	}
}

func TestLedger_UpdateDecision_NotFound(t *testing.T) {
	l := NewEventLedger()
	// Should not panic
	l.UpdateDecision("nonexistent", `{"type":"cancel"}`)
}
