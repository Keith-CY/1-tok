package notifications

import (
	"testing"
)

func TestInMemoryService_Send(t *testing.T) {
	svc := NewInMemoryService()
	err := svc.Send(EventOrderCreated, "org_buyer", map[string]any{
		"orderId": "ord_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Count() != 1 {
		t.Errorf("count = %d, want 1", svc.Count())
	}
}

func TestInMemoryService_List(t *testing.T) {
	svc := NewInMemoryService()
	svc.Send(EventOrderCreated, "org_buyer", map[string]any{"orderId": "ord_1"})
	svc.Send(EventMilestoneSettled, "org_provider", map[string]any{"orderId": "ord_1", "milestoneId": "ms_1"})
	svc.Send(EventDisputeOpened, "org_buyer", map[string]any{"disputeId": "disp_1"})

	// All for org_buyer
	notifications, err := svc.List("org_buyer")
	if err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications for org_buyer, got %d", len(notifications))
	}

	// All
	all, _ := svc.List("")
	if len(all) != 3 {
		t.Errorf("expected 3 total notifications, got %d", len(all))
	}
}

func TestInMemoryService_EventTypes(t *testing.T) {
	svc := NewInMemoryService()
	events := []EventType{
		EventOrderCreated, EventMilestoneSettled, EventDisputeOpened,
		EventDisputeResolved, EventRFQAwarded, EventOrderCompleted,
		EventOrderRated, EventBudgetWallHit,
	}
	for _, event := range events {
		if err := svc.Send(event, "org_test", nil); err != nil {
			t.Fatalf("Send(%s): %v", event, err)
		}
	}
	if svc.Count() != len(events) {
		t.Errorf("count = %d, want %d", svc.Count(), len(events))
	}
}
