package platform

import (
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
}

func TestDefaultMilestoneSplit_SmallBudget(t *testing.T) {
	milestones := DefaultMilestoneSplit(500)
	if len(milestones) != 1 {
		t.Fatalf("expected 1 milestone for small budget, got %d", len(milestones))
	}
	if milestones[0].BudgetCents != 500 {
		t.Errorf("expected budget 500, got %d", milestones[0].BudgetCents)
	}
}

func TestDefaultMilestoneSplit_StandardBudget(t *testing.T) {
	milestones := DefaultMilestoneSplit(10000)
	if len(milestones) != 3 {
		t.Fatalf("expected 3 milestones, got %d", len(milestones))
	}

	// 20% setup, 60% execution, 20% delivery
	if milestones[0].BudgetCents != 2000 {
		t.Errorf("setup: expected 2000, got %d", milestones[0].BudgetCents)
	}
	if milestones[1].BudgetCents != 6000 {
		t.Errorf("execution: expected 6000, got %d", milestones[1].BudgetCents)
	}
	if milestones[2].BudgetCents != 2000 {
		t.Errorf("delivery: expected 2000, got %d", milestones[2].BudgetCents)
	}

	// Ensure total equals budget (no rounding loss)
	total := milestones[0].BudgetCents + milestones[1].BudgetCents + milestones[2].BudgetCents
	if total != 10000 {
		t.Errorf("total %d != budget 10000", total)
	}
}

func TestDefaultMilestoneSplit_OddBudget(t *testing.T) {
	milestones := DefaultMilestoneSplit(9999)
	total := int64(0)
	for _, m := range milestones {
		total += m.BudgetCents
	}
	if total != 9999 {
		t.Errorf("total %d != budget 9999 (rounding loss)", total)
	}
}

func TestDefaultMilestoneSplit_ZeroBudget(t *testing.T) {
	milestones := DefaultMilestoneSplit(0)
	if milestones != nil {
		t.Errorf("expected nil for zero budget, got %v", milestones)
	}
}

func TestCreateBid_DefaultsFromRFQ(t *testing.T) {
	app := NewAppWithMemory()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "org_buyer",
		Title:              "Test RFQ",
		Category:           "ai",
		Scope:              "Build an agent",
		BudgetCents:        10000,
		ResponseDeadlineAt: fixedTime().Add(48 * 3600e9),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(rfq.DefaultMilestones) != 3 {
		t.Fatalf("expected 3 default milestones on RFQ, got %d", len(rfq.DefaultMilestones))
	}

	// Provider submits bid without milestones or quote — should inherit defaults
	bid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_provider",
		Message:       "I can do this",
	})
	if err != nil {
		t.Fatal(err)
	}

	if bid.QuoteCents != 10000 {
		t.Errorf("expected quote 10000 (from RFQ budget), got %d", bid.QuoteCents)
	}
	if len(bid.Milestones) != 3 {
		t.Fatalf("expected 3 milestones (from RFQ defaults), got %d", len(bid.Milestones))
	}
	if bid.Milestones[0].BudgetCents != 2000 {
		t.Errorf("expected first milestone budget 2000, got %d", bid.Milestones[0].BudgetCents)
	}
}

func TestCreateBid_ProviderOverride(t *testing.T) {
	app := NewAppWithMemory()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "org_buyer",
		Title:              "Test RFQ",
		Category:           "ai",
		Scope:              "Build an agent",
		BudgetCents:        10000,
		ResponseDeadlineAt: fixedTime().Add(48 * 3600e9),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Provider submits own milestones and quote
	bid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_provider",
		Message:       "Custom proposal",
		QuoteCents:    8000,
		Milestones: []BidMilestoneInput{
			{ID: "ms_a", Title: "Phase A", BasePriceCents: 5000, BudgetCents: 5000},
			{ID: "ms_b", Title: "Phase B", BasePriceCents: 3000, BudgetCents: 3000},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if bid.QuoteCents != 8000 {
		t.Errorf("expected quote 8000, got %d", bid.QuoteCents)
	}
	if len(bid.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(bid.Milestones))
	}
}

func TestCreateRFQ_CustomMilestones(t *testing.T) {
	app := NewAppWithMemory()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:  "org_buyer",
		Title:       "Custom split",
		Category:    "ai",
		Scope:       "Specific work",
		BudgetCents: 5000,
		Milestones: []CreateMilestoneInput{
			{ID: "ms_x", Title: "Research", BasePriceCents: 2000, BudgetCents: 2000},
			{ID: "ms_y", Title: "Implementation", BasePriceCents: 3000, BudgetCents: 3000},
		},
		ResponseDeadlineAt: fixedTime().Add(48 * 3600e9),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(rfq.DefaultMilestones) != 2 {
		t.Fatalf("expected 2 custom milestones, got %d", len(rfq.DefaultMilestones))
	}
	if rfq.DefaultMilestones[0].Title != "Research" {
		t.Errorf("expected title Research, got %s", rfq.DefaultMilestones[0].Title)
	}
}
