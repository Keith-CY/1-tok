package postgres

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/identity"
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

func TestRFQRepositoryRoundTrip(t *testing.T) {
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

	repo := NewRFQRepository(db)
	rfq := platform.RFQ{
		ID:                 "rfq_persist",
		BuyerOrgID:         "buyer_1",
		Title:              "Persistent RFQ",
		Category:           "agent-ops",
		Scope:              "Handle live triage and stabilize the carrier workflow.",
		BudgetCents:        4200,
		Status:             platform.RFQStatusOpen,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		CreatedAt:          time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC),
		UpdatedAt:          time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC),
	}

	if err := repo.Save(rfq); err != nil {
		t.Fatalf("save rfq: %v", err)
	}

	rfqs, err := repo.List()
	if err != nil {
		t.Fatalf("list rfqs: %v", err)
	}

	if !slices.ContainsFunc(rfqs, func(candidate platform.RFQ) bool {
		return candidate.ID == rfq.ID && candidate.BuyerOrgID == rfq.BuyerOrgID && candidate.Status == platform.RFQStatusOpen
	}) {
		t.Fatalf("expected rfq %+v in %+v", rfq, rfqs)
	}
}

func TestBidRepositoryRoundTrip(t *testing.T) {
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

	rfqRepo := NewRFQRepository(db)
	if err := rfqRepo.Save(platform.RFQ{
		ID:                 "rfq_bid_parent",
		BuyerOrgID:         "buyer_1",
		Title:              "Persistent RFQ",
		Category:           "agent-ops",
		Scope:              "Handle live triage and stabilize the carrier workflow.",
		BudgetCents:        4200,
		Status:             platform.RFQStatusOpen,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		CreatedAt:          time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC),
		UpdatedAt:          time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save parent rfq: %v", err)
	}

	repo := NewBidRepository(db)
	bid := platform.Bid{
		ID:            "bid_persist",
		RFQID:         "rfq_bid_parent",
		ProviderOrgID: "provider_1",
		Message:       "Persistent provider response",
		QuoteCents:    4100,
		Status:        platform.BidStatusOpen,
		Milestones: []platform.BidMilestone{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 2000,
				BudgetCents:    2400,
			},
		},
		CreatedAt: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC),
	}

	if err := repo.Save(bid); err != nil {
		t.Fatalf("save bid: %v", err)
	}

	bids, err := repo.ListByRFQ("rfq_bid_parent")
	if err != nil {
		t.Fatalf("list bids: %v", err)
	}

	if !slices.ContainsFunc(bids, func(candidate platform.Bid) bool {
		return candidate.ID == bid.ID && candidate.ProviderOrgID == bid.ProviderOrgID && candidate.Status == platform.BidStatusOpen
	}) {
		t.Fatalf("expected bid %+v in %+v", bid, bids)
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

func TestIdentityStoreRoundTrip(t *testing.T) {
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

	store := NewIdentityStore(db)
	actor, err := store.CreateSignup(identity.Signup{
		Email:            "identity-roundtrip@example.com",
		Name:             "Identity Owner",
		PasswordHash:     "hash_123",
		OrganizationName: "Identity Buyer",
		OrganizationKind: "buyer",
	})
	if err != nil && err != identity.ErrConflict {
		t.Fatalf("create signup: %v", err)
	}
	if actor.User.ID == "" {
		user, lookupErr := store.FindUserByEmail("identity-roundtrip@example.com")
		if lookupErr != nil {
			t.Fatalf("lookup user after conflict: %v", lookupErr)
		}
		actor.User = user
	}

	user, err := store.FindUserByEmail("identity-roundtrip@example.com")
	if err != nil {
		t.Fatalf("find user by email: %v", err)
	}
	if user.ID == "" || user.Email != "identity-roundtrip@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}

	session, err := store.CreateSession(identity.NewSession{
		UserID:      user.ID,
		TokenDigest: "digest-roundtrip",
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID == "" {
		t.Fatalf("expected persisted session id, got %+v", session)
	}

	authenticated, err := store.GetAuthenticatedActorBySessionDigest("digest-roundtrip")
	if err != nil {
		t.Fatalf("get actor by session digest: %v", err)
	}
	if authenticated.User.ID != user.ID {
		t.Fatalf("expected user %s, got %+v", user.ID, authenticated)
	}
	if len(authenticated.Memberships) == 0 || authenticated.Memberships[0].Organization.Kind != "buyer" {
		t.Fatalf("unexpected memberships: %+v", authenticated.Memberships)
	}
}

func TestMigrateAddsRevokedAtToLegacyIAMSessions(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL is not set")
	}

	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS iam_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_digest TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		t.Fatalf("create legacy iam_sessions: %v", err)
	}
	if _, err := db.Exec(`ALTER TABLE iam_sessions DROP COLUMN IF EXISTS revoked_at`); err != nil {
		t.Fatalf("drop revoked_at: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'iam_sessions' AND column_name = 'revoked_at'
		)
	`).Scan(&exists); err != nil {
		t.Fatalf("query columns: %v", err)
	}
	if !exists {
		t.Fatalf("expected revoked_at column to exist after migrate")
	}
}
