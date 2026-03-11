package postgres

import (
	"os"
	"slices"
	"testing"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
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

func TestProviderRepositoryRoundTrip(t *testing.T) {
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

	repo := NewProviderRepository(db)
	provider := platform.ProviderProfile{
		ID:             "provider_persist",
		Name:           "Persistent Carrier",
		Capabilities:   []string{"carrier", "run_shell"},
		ReputationTier: "silver",
	}

	if err := repo.Upsert(provider); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}

	providers, err := repo.List()
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}

	if !slices.ContainsFunc(providers, func(candidate platform.ProviderProfile) bool {
		return candidate.ID == provider.ID && candidate.Name == provider.Name
	}) {
		t.Fatalf("expected provider %+v in %+v", provider, providers)
	}
}

func TestListingRepositoryRoundTrip(t *testing.T) {
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

	providers := NewProviderRepository(db)
	if err := providers.Upsert(platform.ProviderProfile{
		ID:             "provider_for_listing",
		Name:           "Listing Provider",
		Capabilities:   []string{"carrier"},
		ReputationTier: "gold",
	}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}

	repo := NewListingRepository(db)
	listing := platform.Listing{
		ID:             "listing_persist",
		ProviderOrgID:  "provider_for_listing",
		Title:          "Persistent Listing",
		Category:       "agent-ops",
		BasePriceCents: 2400,
		Tags:           []string{"fiber", "carrier"},
	}

	if err := repo.Upsert(listing); err != nil {
		t.Fatalf("upsert listing: %v", err)
	}

	listings, err := repo.List()
	if err != nil {
		t.Fatalf("list listings: %v", err)
	}

	if !slices.ContainsFunc(listings, func(candidate platform.Listing) bool {
		return candidate.ID == listing.ID && candidate.ProviderOrgID == listing.ProviderOrgID
	}) {
		t.Fatalf("expected listing %+v in %+v", listing, listings)
	}
}

func TestSeedCatalogInsertsDefaultProvidersAndListings(t *testing.T) {
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

	if err := SeedCatalog(db); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	providers, err := NewProviderRepository(db).List()
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if !slices.ContainsFunc(providers, func(candidate platform.ProviderProfile) bool {
		return candidate.ID == "provider_1"
	}) {
		t.Fatalf("expected seeded provider_1 in %+v", providers)
	}

	listings, err := NewListingRepository(db).List()
	if err != nil {
		t.Fatalf("list listings: %v", err)
	}
	if !slices.ContainsFunc(listings, func(candidate platform.Listing) bool {
		return candidate.ID == "listing_1"
	}) {
		t.Fatalf("expected seeded listing_1 in %+v", listings)
	}
}
