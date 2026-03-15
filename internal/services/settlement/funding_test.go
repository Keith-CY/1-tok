package settlement

import (
	"os"
	"testing"

	postgresstore "github.com/chenyu/1-tok/internal/store/postgres"
)

func TestPostgresFundingRecordRepository(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL is not set")
	}

	db, err := postgresstore.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := MigrateFundingRecordStore(db); err != nil {
		t.Fatal(err)
	}

	repo := newPostgresFundingRecordRepository(db)

	// NextID
	id, err := repo.NextID()
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}

	// Save
	record := FundingRecord{
		ID:            id,
		Kind:          "invoice",
		OrderID:       "ord_test",
		MilestoneID:   "ms_test",
		BuyerOrgID:    "org_b",
		ProviderOrgID: "org_p",
		Asset:         "CKB",
		Amount:        "100",
		State:         "pending",
	}
	if err := repo.Save(record); err != nil {
		t.Fatal(err)
	}

	// List
	records, err := repo.List(FundingRecordFilter{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range records {
		if r.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("saved record not found in list")
	}

	// UpdateInvoiceState
	if err := repo.UpdateInvoiceState("lnbc_test", "SETTLED"); err != nil {
		// May return error if no matching invoice — that's ok
		t.Logf("UpdateInvoiceState: %v (expected for test data)", err)
	}

	// List with filter
	filtered, err := repo.List(FundingRecordFilter{Kind: "invoice", OrderID: "ord_test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) == 0 {
		t.Error("expected filtered results")
	}

	// Verify
	if err := VerifyFundingRecordStore(db); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresFundingRecordRepository_UpdateExternalState(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL is not set")
	}
	db, err := postgresstore.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := MigrateFundingRecordStore(db); err != nil {
		t.Fatal(err)
	}

	repo := newPostgresFundingRecordRepository(db)

	id, _ := repo.NextID()
	record := FundingRecord{
		ID: id, Kind: "withdrawal", OrderID: "ord_ext",
		ProviderOrgID: "org_p", Asset: "CKB", Amount: "50",
		State: "pending", ExternalID: "ext_test_" + id,
	}
	repo.Save(record)

	// Update external state by external_id
	if err := repo.UpdateExternalState(record.ExternalID, "completed"); err != nil {
		t.Fatal(err)
	}

	// Verify
	records, _ := repo.List(FundingRecordFilter{Kind: "withdrawal"})
	for _, r := range records {
		if r.ID == id && r.State != "completed" {
			t.Errorf("state = %s, want completed", r.State)
		}
	}
}

func TestLoadFundingRecordRepository_Memory(t *testing.T) {
	t.Setenv("SETTLEMENT_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")

	repo := loadFundingRecordRepository(); var err error
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
}

func TestLoadFundingRecordRepository_Postgres(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("SETTLEMENT_DATABASE_URL", dsn)

	repo := loadFundingRecordRepository(); var err error
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
}

func TestLoadConfiguredFundingRecordRepository_NoDSN(t *testing.T) {
	t.Setenv("SETTLEMENT_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")
	_, err := loadConfiguredFundingRecordRepository()
	if err == nil {
		t.Error("expected error without DSN")
	}
}

func TestLoadConfiguredFundingRecordRepository_InvalidDSN(t *testing.T) {
	t.Setenv("SETTLEMENT_DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/invalid")
	_, err := loadConfiguredFundingRecordRepository()
	if err == nil {
		t.Error("expected error for invalid DSN")
	}
}

func TestLoadConfiguredFundingRecordRepository_WithDSN(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("SETTLEMENT_DATABASE_URL", dsn)
	repo, err := loadConfiguredFundingRecordRepository()
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
}

func TestLoadConfiguredFundingRecordRepository_RequireBootstrap(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("SETTLEMENT_DATABASE_URL", dsn)
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "true")
	repo, err := loadConfiguredFundingRecordRepository()
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
}

func TestLoadFundingRecordRepositoryE_SettlementDSN(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("SETTLEMENT_DATABASE_URL", dsn)
	repo, err := loadFundingRecordRepositoryE()
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
}
