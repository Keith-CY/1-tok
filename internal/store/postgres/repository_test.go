package postgres

import (
	"os"
	"testing"

	"github.com/chenyu/1-tok/internal/core"
)

func TestOrderRepositoryRoundTrip(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL is not set")
	}

	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repo := NewOrderRepository(db)
	orderID, err := repo.NextID()
	if err != nil {
		t.Fatalf("next id: %v", err)
	}

	order := &core.Order{
		ID:            orderID,
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Status:        core.OrderStatusRunning,
		Milestones: []core.Milestone{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1200,
				BudgetCents:    1800,
				State:          core.MilestoneStateRunning,
				DisputeStatus:  core.DisputeStatusNone,
			},
		},
	}

	if err := repo.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	stored, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}

	if stored.ID != order.ID {
		t.Fatalf("expected order id %s, got %s", order.ID, stored.ID)
	}

	orders, err := repo.List()
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}

	if len(orders) == 0 {
		t.Fatalf("expected persisted orders, got none")
	}
}
