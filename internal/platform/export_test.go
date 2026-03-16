package platform

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func TestExportOrdersCSV(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Export test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	csv, err := app.ExportOrdersCSV()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(csv, "OrderID,BuyerOrgID") {
		t.Error("missing header")
	}
	if !strings.Contains(csv, "org_b") {
		t.Error("missing buyer org")
	}
}

func TestExportDisputesCSV(t *testing.T) {
	app := NewAppWithMemory()
	csv, err := app.ExportDisputesCSV()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(csv, "DisputeID,OrderID") {
		t.Error("missing header")
	}
}

func TestExportRFQsCSV(t *testing.T) {
	app := NewAppWithMemory()
	app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Export RFQ", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	csv, err := app.ExportRFQsCSV()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(csv, "RFQID") {
		t.Error("missing header")
	}
	if !strings.Contains(csv, "org_b") {
		t.Error("missing data")
	}
}

func TestExportProviderApplicationsCSV(t *testing.T) {
	app := NewAppWithMemory()
	app.SubmitProviderApplication("org_p", "Provider Inc", []string{"gpu"})
	csv := app.ExportProviderApplicationsCSV()
	if !strings.Contains(csv, "ApplicationID") {
		t.Error("missing header")
	}
	if !strings.Contains(csv, "Provider Inc") {
		t.Error("missing data")
	}
}

func TestExportOrdersCSV_EscapesCommas(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Export, test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{ProviderOrgID: "org_p", Message: "bid", QuoteCents: 5000, Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}}})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	csvText, err := app.ExportOrdersCSV()
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(csvText))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 {
		t.Fatal("expected at least one CSV data row")
	}
	if records[1][1] != "org_b" {
		t.Fatalf("buyerOrgID = %q", records[1][1])
	}
	if records[1][0] == "" {
		t.Fatal("missing order ID")
	}
}
