package platform

import (
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

func TestAppCreateOrderPublishesOrderCreatedEvent(t *testing.T) {
	publisher := &spyPublisher{}
	app := NewAppWithMemory()
	app.publisher = publisher

	_, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Agent operations",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Milestones: []CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1000,
				BudgetCents:    1400,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if len(publisher.subjects) != 1 {
		t.Fatalf("expected one event, got %d", len(publisher.subjects))
	}

	if publisher.subjects[0] != "market.order.created" {
		t.Fatalf("expected order created subject, got %s", publisher.subjects[0])
	}
}

func TestAppCreateRFQPublishesCreatedEvent(t *testing.T) {
	publisher := &spyPublisher{}
	app := NewAppWithMemory()
	app.publisher = publisher

	_, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures and propose a fix plan.",
		BudgetCents:        8_000,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	if len(publisher.subjects) != 1 {
		t.Fatalf("expected one event, got %d", len(publisher.subjects))
	}

	if publisher.subjects[0] != "market.rfq.created" {
		t.Fatalf("expected rfq created subject, got %s", publisher.subjects[0])
	}
}

func TestAppSettleMilestonePublishesSettledEvent(t *testing.T) {
	publisher := &spyPublisher{}
	app := NewAppWithMemory()
	app.publisher = publisher

	order, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Agent operations",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Milestones: []CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1000,
				BudgetCents:    1400,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	_, _, err = app.SettleMilestone(order.ID, SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("settle milestone: %v", err)
	}

	if len(publisher.subjects) != 2 {
		t.Fatalf("expected two events, got %d", len(publisher.subjects))
	}

	if publisher.subjects[1] != "market.milestone.settled" {
		t.Fatalf("expected settled event, got %s", publisher.subjects[1])
	}
}

type spyPublisher struct {
	subjects []string
}

func (s *spyPublisher) Publish(subject string, _ any) error {
	s.subjects = append(s.subjects, subject)
	return nil
}
