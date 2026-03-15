package discord

import "testing"

func TestFormatEvent_AllTypes(t *testing.T) {
	events := map[string]map[string]any{
		"order.created":    {"orderId": "ord_1", "buyerOrgId": "b", "providerOrgId": "p"},
		"milestone.settled": {"orderId": "ord_1", "milestoneId": "ms_1"},
		"dispute.opened":   {"orderId": "ord_1", "milestoneId": "ms_1"},
		"dispute.resolved": {"disputeId": "d_1", "resolution": "refunded"},
		"order.completed":  {"orderId": "ord_1"},
		"order.rated":      {"orderId": "ord_1", "score": 5},
		"budget_wall.hit":  {"orderId": "ord_1", "milestoneId": "ms_1"},
		"rfq.awarded":      {"rfqId": "rfq_1", "orderId": "ord_1"},
		"unknown.event":    {"key": "value"},
	}
	for event, payload := range events {
		msg := FormatEvent(event, payload)
		if msg == "" {
			t.Errorf("empty message for %s", event)
		}
	}
}

func TestPushNotifier_Notify(t *testing.T) {
	var sent string
	notifier := NewPushNotifier("channel_1", func(ch, msg string) error {
		sent = msg
		return nil
	})
	notifier.Notify("order.created", "org_1", map[string]any{"orderId": "ord_1"})
	if sent == "" {
		t.Error("expected message sent")
	}
}

func TestPushNotifier_NoChannel(t *testing.T) {
	notifier := NewPushNotifier("", nil)
	// Should not panic
	err := notifier.Notify("order.created", "org_1", nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
