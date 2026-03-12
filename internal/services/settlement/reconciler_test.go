package settlement

import (
	"context"
	"testing"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type stubReconcilerFiberClient struct {
	statusByInvoice       map[string]fiberclient.InvoiceStatusResult
	withdrawalsByUserID   map[string]fiberclient.WithdrawalStatusResult
	statusCalls           []string
	withdrawalStatusCalls []string
}

func (s *stubReconcilerFiberClient) CreateInvoice(context.Context, fiberclient.CreateInvoiceInput) (fiberclient.CreateInvoiceResult, error) {
	panic("unexpected CreateInvoice call")
}

func (s *stubReconcilerFiberClient) GetInvoiceStatus(_ context.Context, invoice string) (fiberclient.InvoiceStatusResult, error) {
	s.statusCalls = append(s.statusCalls, invoice)
	return s.statusByInvoice[invoice], nil
}

func (s *stubReconcilerFiberClient) QuotePayout(context.Context, fiberclient.QuotePayoutInput) (fiberclient.QuotePayoutResult, error) {
	panic("unexpected QuotePayout call")
}

func (s *stubReconcilerFiberClient) RequestPayout(context.Context, fiberclient.RequestPayoutInput) (fiberclient.RequestPayoutResult, error) {
	panic("unexpected RequestPayout call")
}

func (s *stubReconcilerFiberClient) ListSettledFeed(context.Context, fiberclient.SettledFeedInput) (fiberclient.SettledFeedResult, error) {
	panic("unexpected ListSettledFeed call")
}

func (s *stubReconcilerFiberClient) ListWithdrawalStatuses(_ context.Context, userID string) (fiberclient.WithdrawalStatusResult, error) {
	s.withdrawalStatusCalls = append(s.withdrawalStatusCalls, userID)
	return s.withdrawalsByUserID[userID], nil
}

func TestReconcilerSyncPendingInvoicesUpdatesUnsettledInvoices(t *testing.T) {
	repo := NewMemoryFundingRecordRepository()
	now := time.Now().UTC()
	if err := repo.Save(FundingRecord{
		ID:        "fund_invoice_1",
		Kind:      FundingRecordKindInvoice,
		Invoice:   "inv_1",
		State:     "UNPAID",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	if err := repo.Save(FundingRecord{
		ID:        "fund_invoice_2",
		Kind:      FundingRecordKindInvoice,
		Invoice:   "inv_2",
		State:     "SETTLED",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed settled invoice: %v", err)
	}

	fiber := &stubReconcilerFiberClient{
		statusByInvoice: map[string]fiberclient.InvoiceStatusResult{
			"inv_1": {State: "SETTLED"},
		},
	}

	reconciler := NewReconciler(ReconcilerOptions{
		Fiber:   fiber,
		Funding: repo,
	})

	summary, err := reconciler.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if summary.InvoiceUpdates != 1 {
		t.Fatalf("expected 1 invoice update, got %+v", summary)
	}
	if len(fiber.statusCalls) != 1 || fiber.statusCalls[0] != "inv_1" {
		t.Fatalf("expected only unsettled invoice to be queried, got %+v", fiber.statusCalls)
	}

	records, err := repo.List(FundingRecordFilter{Kind: FundingRecordKindInvoice})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	for _, record := range records {
		if record.Invoice == "inv_1" && record.State != "SETTLED" {
			t.Fatalf("expected invoice state to be updated, got %+v", record)
		}
	}
}

func TestReconcilerSyncPendingWithdrawalsUpdatesPendingProviderStatuses(t *testing.T) {
	repo := NewMemoryFundingRecordRepository()
	now := time.Now().UTC()
	if err := repo.Save(FundingRecord{
		ID:            "fund_withdrawal_1",
		Kind:          FundingRecordKindWithdrawal,
		ProviderOrgID: "provider_1",
		ExternalID:    "wd_1",
		State:         "PENDING",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("seed withdrawal: %v", err)
	}
	if err := repo.Save(FundingRecord{
		ID:            "fund_withdrawal_2",
		Kind:          FundingRecordKindWithdrawal,
		ProviderOrgID: "provider_1",
		ExternalID:    "wd_2",
		State:         "COMPLETED",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("seed completed withdrawal: %v", err)
	}

	fiber := &stubReconcilerFiberClient{
		withdrawalsByUserID: map[string]fiberclient.WithdrawalStatusResult{
			"provider_1": {
				Withdrawals: []fiberclient.WithdrawalStatusItem{
					{ID: "wd_1", State: "PROCESSING"},
				},
			},
		},
	}

	reconciler := NewReconciler(ReconcilerOptions{
		Fiber:   fiber,
		Funding: repo,
	})

	summary, err := reconciler.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if summary.WithdrawalUpdates != 1 {
		t.Fatalf("expected 1 withdrawal update, got %+v", summary)
	}
	if len(fiber.withdrawalStatusCalls) != 1 || fiber.withdrawalStatusCalls[0] != "provider_1" {
		t.Fatalf("expected one provider withdrawal sync, got %+v", fiber.withdrawalStatusCalls)
	}

	records, err := repo.List(FundingRecordFilter{Kind: FundingRecordKindWithdrawal})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	for _, record := range records {
		if record.ExternalID == "wd_1" && record.State != "PROCESSING" {
			t.Fatalf("expected pending withdrawal to be updated, got %+v", record)
		}
	}
}
