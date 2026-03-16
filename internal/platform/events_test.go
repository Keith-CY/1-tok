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
		ResponseDeadlineAt: time.Date(2099, 3, 15, 12, 0, 0, 0, time.UTC),
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

func TestAppCreateBidPublishesSubmittedEvent(t *testing.T) {
	publisher := &spyPublisher{}
	app := NewAppWithMemory()
	app.publisher = publisher

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures and propose a fix plan.",
		BudgetCents:        8_000,
		ResponseDeadlineAt: time.Date(2099, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	_, err = app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "Carrier-ready response",
		QuoteCents:    7_200,
		Milestones: []BidMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 3000,
				BudgetCents:    3600,
			},
		},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	if len(publisher.subjects) != 2 {
		t.Fatalf("expected two events, got %d", len(publisher.subjects))
	}

	if publisher.subjects[1] != "market.bid.submitted" {
		t.Fatalf("expected bid submitted subject, got %s", publisher.subjects[1])
	}
}

func TestAppAwardRFQPublishesAwardedEvent(t *testing.T) {
	publisher := &spyPublisher{}
	app := NewAppWithMemory()
	app.publisher = publisher

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures and propose a fix plan.",
		BudgetCents:        8_000,
		ResponseDeadlineAt: time.Date(2099, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	bid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "Carrier-ready response",
		QuoteCents:    7_200,
		Milestones: []BidMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 3000,
				BudgetCents:    3600,
			},
		},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	_, _, err = app.AwardRFQ(rfq.ID, AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: core.FundingModeCredit,
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}

	if publisher.subjects[len(publisher.subjects)-1] != "market.rfq.awarded" {
		t.Fatalf("expected rfq awarded subject, got %s", publisher.subjects[len(publisher.subjects)-1])
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

func TestCreateMessage_PublishesEvent(t *testing.T) {
	app := NewAppWithMemory()
	publisher := &spyPublisher{}
	app.publisher = publisher

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Msg pub", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2099, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	publisher.subjects = nil
	_, err := app.CreateMessage(order.ID, "buyer", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range publisher.subjects {
		if s == "market.message.created" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected market.message.created, got %v", publisher.subjects)
	}
}

func TestRecordUsageCharge_PublishesEvent(t *testing.T) {
	app := NewAppWithMemory()
	publisher := &spyPublisher{}
	app.publisher = publisher

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Usage pub", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2099, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	publisher.subjects = nil
	_, _, err := app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: "token", AmountCents: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range publisher.subjects {
		if s == "market.usage.recorded" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected market.usage.recorded, got %v", publisher.subjects)
	}
}
