package platform

import (
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

func TestAppCreateOrderPersistsAndListsOrders(t *testing.T) {
	app := NewAppWithMemory()

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

	orders, err := app.ListOrders()
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("expected one order, got %d", len(orders))
	}

	if orders[0].ID != order.ID {
		t.Fatalf("expected order %s, got %s", order.ID, orders[0].ID)
	}
}

func TestAppSettleMilestoneAdvancesNextMilestone(t *testing.T) {
	app := NewAppWithMemory()
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
			{
				ID:             "ms_2",
				Title:          "Deliver",
				BasePriceCents: 500,
				BudgetCents:    900,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	updated, entry, err := app.SettleMilestone(order.ID, SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("settle milestone: %v", err)
	}

	if entry.Kind != core.LedgerEntryKindPlatformExposure {
		t.Fatalf("expected platform exposure, got %s", entry.Kind)
	}

	if updated.Milestones[1].State != core.MilestoneStateRunning {
		t.Fatalf("expected second milestone running, got %s", updated.Milestones[1].State)
	}
}
