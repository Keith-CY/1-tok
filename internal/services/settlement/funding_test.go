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
