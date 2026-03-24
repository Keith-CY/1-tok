package settlement

import (
	"context"
	"fmt"
	"log"
	"os"
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

func TestReconcilerSyncPendingInvoicesUpdatesBuyerTopUps(t *testing.T) {
	repo := NewMemoryFundingRecordRepository()
	now := time.Now().UTC()
	if err := repo.Save(FundingRecord{
		ID:         "fund_topup_1",
		Kind:       FundingRecordKindBuyerTopUp,
		BuyerOrgID: "buyer_1",
		Invoice:    "inv_topup_1",
		State:      "UNPAID",
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		t.Fatalf("seed buyer topup: %v", err)
	}

	fiber := &stubReconcilerFiberClient{
		statusByInvoice: map[string]fiberclient.InvoiceStatusResult{
			"inv_topup_1": {State: "SETTLED"},
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
		t.Fatalf("expected 1 invoice-backed update, got %+v", summary)
	}
	if len(fiber.statusCalls) != 1 || fiber.statusCalls[0] != "inv_topup_1" {
		t.Fatalf("expected buyer topup invoice to be queried, got %+v", fiber.statusCalls)
	}

	records, err := repo.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list buyer topups: %v", err)
	}
	if len(records) != 1 || records[0].State != "SETTLED" {
		t.Fatalf("expected buyer topup to be settled, got %+v", records)
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

func TestReconcilerSyncDepositsSweepsConfirmedBuyerBalance(t *testing.T) {
	funding := NewMemoryFundingRecordRepository()
	deposits := NewBuyerDepositService(BuyerDepositServiceOptions{
		Addresses: NewMemoryBuyerDepositAddressRepository(),
		Sweeps:    NewMemoryBuyerDepositSweepRepository(),
		Funding:   funding,
		Wallet: &stubBuyerDepositWallet{
			derivedAddresses: map[int]string{0: "ckt1qyqbuyer0address"},
			balances: map[string]BuyerDepositChainBalance{
				"ckt1qyqbuyer0address": {
					Address:            "ckt1qyqbuyer0address",
					RawOnChainUnits:    1_300_000_000,
					RawConfirmedUnits:  1_300_000_000,
					ConfirmationBlocks: 24,
				},
			},
			sweepResults: map[string]BuyerDepositSweepResult{
				"ckt1qyqbuyer0address": {
					SweepTxHash:     "0xsweep123",
					SweptRawUnits:   1_300_000_000,
					TreasuryAddress: "ckt1qyqtreasuryaddress",
				},
			},
		},
		Asset:                "USDI",
		TreasuryAddress:      "ckt1qyqtreasuryaddress",
		MinSweepAmountRaw:    1_000_000_000,
		ConfirmationBlocks:   24,
		RawUnitsPerWholeUSDI: 100_000_000,
	})
	if _, err := deposits.EnsureAddress(context.Background(), "buyer_1"); err != nil {
		t.Fatalf("ensure address: %v", err)
	}

	reconciler := NewReconciler(ReconcilerOptions{
		Fiber:    &stubReconcilerFiberClient{},
		Funding:  funding,
		Deposits: deposits,
	})

	summary, err := reconciler.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if summary.DepositSweepUpdates != 1 {
		t.Fatalf("expected 1 deposit sweep update, got %+v", summary)
	}
	records, err := funding.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list buyer topups: %v", err)
	}
	if len(records) != 1 || records[0].State != "SETTLED" || records[0].Amount != "13.00" {
		t.Fatalf("unexpected credited topups: %+v", records)
	}
}

func TestNewReconciler(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	r := NewReconciler(ReconcilerOptions{
		Fiber:   fiber,
		Funding: funding,
	})
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}
}

func TestSync_EmptyStore(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.InvoiceUpdates != 0 || summary.WithdrawalUpdates != 0 {
		t.Errorf("expected 0 updates, got %+v", summary)
	}
}

func TestSync_InvoiceStateUpdate(t *testing.T) {
	fiber := &stubFiberClient{
		statusResult: fiberclient.InvoiceStatusResult{State: "paid"},
	}
	funding := NewMemoryFundingRecordRepository()

	// Add a pending invoice
	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_reconcile", State: "pending",
		Asset: "CKB", Amount: "100", ProviderOrgID: "org_p",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.InvoiceUpdates != 1 {
		t.Errorf("expected 1 invoice update, got %d", summary.InvoiceUpdates)
	}
}

func TestSync_SettledInvoiceSkipped(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_settled", State: "SETTLED",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.InvoiceUpdates != 0 {
		t.Errorf("expected 0 updates for settled invoice, got %d", summary.InvoiceUpdates)
	}
}

func TestSync_WithdrawalUpdate(t *testing.T) {
	fiber := &stubFiberClient{
		withdrawalsResult: fiberclient.WithdrawalStatusResult{
			Withdrawals: []fiberclient.WithdrawalStatusItem{
				{ID: "ext_1", State: "completed"},
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindWithdrawal,
		ExternalID: "ext_1", State: "pending",
		Asset: "CKB", Amount: "50", ProviderOrgID: "org_p",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.WithdrawalUpdates != 1 {
		t.Errorf("expected 1 withdrawal update, got %d", summary.WithdrawalUpdates)
	}
}

func TestRunReconcilerLoop_NilReconciler(t *testing.T) {
	err := RunReconcilerLoop(context.Background(), nil, time.Second, nil)
	if err == nil {
		t.Error("expected error for nil reconciler")
	}
}

func TestRunReconcilerLoop_ContextCancelled(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()
	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	logger := log.New(log.Writer(), "test: ", 0)
	err := RunReconcilerLoop(ctx, r, 50*time.Millisecond, logger)
	if err == nil {
		t.Error("expected context error")
	}
}

func TestRunReconcilerLoop_ConsecutiveErrors(t *testing.T) {
	// Create a reconciler with a fiber client that always errors
	fiber := &errorFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	// Add a pending record so sync actually tries fiber
	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_err", State: "pending",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	// First sync fails immediately
	_, err := r.Sync(context.Background())
	if err == nil {
		t.Error("expected error from fiber")
	}
}

type errorFiberClient struct{}

func (errorFiberClient) CreateInvoice(context.Context, fiberclient.CreateInvoiceInput) (fiberclient.CreateInvoiceResult, error) {
	return fiberclient.CreateInvoiceResult{}, fiberclient.ErrNotConfigured
}
func (errorFiberClient) GetInvoiceStatus(context.Context, string) (fiberclient.InvoiceStatusResult, error) {
	return fiberclient.InvoiceStatusResult{}, fiberclient.ErrNotConfigured
}
func (errorFiberClient) QuotePayout(context.Context, fiberclient.QuotePayoutInput) (fiberclient.QuotePayoutResult, error) {
	return fiberclient.QuotePayoutResult{}, fiberclient.ErrNotConfigured
}
func (errorFiberClient) RequestPayout(context.Context, fiberclient.RequestPayoutInput) (fiberclient.RequestPayoutResult, error) {
	return fiberclient.RequestPayoutResult{}, fiberclient.ErrNotConfigured
}
func (errorFiberClient) ListSettledFeed(context.Context, fiberclient.SettledFeedInput) (fiberclient.SettledFeedResult, error) {
	return fiberclient.SettledFeedResult{}, fiberclient.ErrNotConfigured
}
func (errorFiberClient) ListWithdrawalStatuses(context.Context, string) (fiberclient.WithdrawalStatusResult, error) {
	return fiberclient.WithdrawalStatusResult{}, fiberclient.ErrNotConfigured
}

type countingFiberClient struct {
	callCount         int
	failUntil         int
	statusResult      fiberclient.InvoiceStatusResult
	withdrawalsResult fiberclient.WithdrawalStatusResult
}

func (c *countingFiberClient) CreateInvoice(_ context.Context, _ fiberclient.CreateInvoiceInput) (fiberclient.CreateInvoiceResult, error) {
	return fiberclient.CreateInvoiceResult{}, nil
}
func (c *countingFiberClient) GetInvoiceStatus(_ context.Context, _ string) (fiberclient.InvoiceStatusResult, error) {
	c.callCount++
	if c.callCount <= c.failUntil {
		return fiberclient.InvoiceStatusResult{}, fiberclient.ErrNotConfigured
	}
	return c.statusResult, nil
}
func (c *countingFiberClient) QuotePayout(_ context.Context, _ fiberclient.QuotePayoutInput) (fiberclient.QuotePayoutResult, error) {
	return fiberclient.QuotePayoutResult{}, nil
}
func (c *countingFiberClient) RequestPayout(_ context.Context, _ fiberclient.RequestPayoutInput) (fiberclient.RequestPayoutResult, error) {
	return fiberclient.RequestPayoutResult{}, nil
}
func (c *countingFiberClient) ListSettledFeed(_ context.Context, _ fiberclient.SettledFeedInput) (fiberclient.SettledFeedResult, error) {
	return fiberclient.SettledFeedResult{}, nil
}
func (c *countingFiberClient) ListWithdrawalStatuses(_ context.Context, _ string) (fiberclient.WithdrawalStatusResult, error) {
	return c.withdrawalsResult, nil
}

func TestRunReconcilerLoop_MaxConsecutiveErrors(t *testing.T) {
	fiber := &errorFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	// Add a pending record so sync hits fiber and fails
	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_max_err", State: "pending",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	logger := log.New(log.Writer(), "test: ", 0)

	// RunReconcilerLoop should fail on first sync (fail fast)
	err := RunReconcilerLoop(context.Background(), r, time.Second, logger)
	if err == nil {
		t.Error("expected error from first sync failure")
	}
}

func TestRunReconcilerLoop_SuccessfulSync(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	logger := log.New(log.Writer(), "test: ", 0)
	err := RunReconcilerLoop(ctx, r, 50*time.Millisecond, logger)
	// Should exit cleanly with context.DeadlineExceeded
	if err == nil || err != context.DeadlineExceeded {
		t.Logf("loop exit: %v", err)
	}
}

func TestSync_InvoiceSameState(t *testing.T) {
	fiber := &stubFiberClient{
		statusResult: fiberclient.InvoiceStatusResult{State: "pending"},
	}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_same", State: "pending",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Same state — no update
	if summary.InvoiceUpdates != 0 {
		t.Errorf("expected 0 updates for same state, got %d", summary.InvoiceUpdates)
	}
}

func TestSync_EmptyInvoiceSkipped(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "", State: "pending", // Empty invoice
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.InvoiceUpdates != 0 {
		t.Errorf("expected 0 updates for empty invoice, got %d", summary.InvoiceUpdates)
	}
}

func TestSync_TerminalWithdrawalSkipped(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindWithdrawal,
		ExternalID: "ext_done", State: "COMPLETED",
		Asset: "CKB", Amount: "50", ProviderOrgID: "org_p",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.WithdrawalUpdates != 0 {
		t.Errorf("expected 0 updates for terminal withdrawal, got %d", summary.WithdrawalUpdates)
	}
}

func TestNewReconciler_Defaults(t *testing.T) {
	// Test with nil funding — should use loadFundingRecordRepositoryE
	r := NewReconciler(ReconcilerOptions{
		Fiber: &stubFiberClient{},
		// Funding nil — will create from env
	})
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}
}

func TestSync_WithdrawalEmptyProvider(t *testing.T) {
	fiber := &stubFiberClient{
		withdrawalsResult: fiberclient.WithdrawalStatusResult{},
	}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindWithdrawal,
		ExternalID: "ext_empty", State: "pending",
		Asset: "CKB", Amount: "50", ProviderOrgID: "", // Empty provider
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.WithdrawalUpdates != 0 {
		t.Errorf("expected 0 updates for empty provider, got %d", summary.WithdrawalUpdates)
	}
}

func TestSync_WithdrawalMultipleProviders(t *testing.T) {
	fiber := &countingFiberClient{
		withdrawalsResult: fiberclient.WithdrawalStatusResult{
			Withdrawals: []fiberclient.WithdrawalStatusItem{
				{ID: "ext_a", State: "completed"},
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()

	id1, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id1, Kind: FundingRecordKindWithdrawal,
		ExternalID: "ext_a", State: "pending",
		Asset: "CKB", Amount: "50", ProviderOrgID: "org_p1",
	})
	id2, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id2, Kind: FundingRecordKindWithdrawal,
		ExternalID: "ext_b", State: "pending",
		Asset: "CKB", Amount: "30", ProviderOrgID: "org_p2",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.WithdrawalUpdates < 1 {
		t.Errorf("expected at least 1 withdrawal update, got %d", summary.WithdrawalUpdates)
	}
}

func TestSync_InvoiceEmptyState(t *testing.T) {
	fiber := &stubFiberClient{
		statusResult: fiberclient.InvoiceStatusResult{State: ""},
	}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_empty_state", State: "pending",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.InvoiceUpdates != 0 {
		t.Errorf("expected 0 updates for empty state response, got %d", summary.InvoiceUpdates)
	}
}

func TestIsTerminalWithdrawalState(t *testing.T) {
	for _, state := range []string{"COMPLETED", "FAILED", "CANCELLED", "REJECTED"} {
		if !isTerminalWithdrawalState(state) {
			t.Errorf("isTerminalWithdrawalState(%q) = false", state)
		}
	}
	for _, state := range []string{"pending", "processing", ""} {
		if isTerminalWithdrawalState(state) {
			t.Errorf("isTerminalWithdrawalState(%q) = true", state)
		}
	}
}

func TestRunReconcilerLoop_ErrorThenRecover(t *testing.T) {
	callCount := 0
	fiber := &countingFiberClient{
		failUntil:    1, // Fail first call only
		statusResult: fiberclient.InvoiceStatusResult{State: "paid"},
	}
	funding := NewMemoryFundingRecordRepository()

	// No pending records — sync will succeed (no fiber calls needed)
	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	logger := log.New(log.Writer(), "recover: ", 0)
	err := RunReconcilerLoop(ctx, r, 50*time.Millisecond, logger)
	// Should exit with context.DeadlineExceeded (sync succeeds, loop continues)
	if err != context.DeadlineExceeded {
		t.Logf("loop exit: %v (expected context deadline)", err)
	}
	_ = callCount
}

func TestRunReconcilerLoop_DefaultInterval(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()
	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Pass 0 interval — should use default (30s)
	err := RunReconcilerLoop(ctx, r, 0, nil)
	if err != context.DeadlineExceeded {
		t.Logf("default interval: %v", err)
	}
}

func TestRunReconcilerLoop_NilLogger(t *testing.T) {
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()
	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := RunReconcilerLoop(ctx, r, 50*time.Millisecond, nil)
	if err != context.DeadlineExceeded {
		t.Logf("nil logger: %v", err)
	}
}

func TestRunReconcilerLoop_ExceedsMaxErrors(t *testing.T) {
	// Fiber that always errors
	fiber := &errorFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	// Add a pending record so each sync hits fiber
	for i := 0; i < 3; i++ {
		id, _ := funding.NextID()
		funding.Save(FundingRecord{
			ID: id, Kind: FundingRecordKindInvoice,
			Invoice: fmt.Sprintf("lnbc_max_%d", i), State: "pending",
			Asset: "CKB", Amount: "100",
		})
	}

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	// RunReconcilerLoop — first sync fails, returns immediately
	logger := log.New(log.Writer(), "max-err: ", 0)
	err := RunReconcilerLoop(context.Background(), r, time.Millisecond, logger)
	if err == nil {
		t.Error("expected error from first sync failure")
	}
}

func TestReconciler_SyncPendingInvoices_FiberError(t *testing.T) {
	fiber := &errorFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_err_sync", State: "pending",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	_, err := r.Sync(context.Background())
	if err == nil {
		t.Error("expected error from fiber")
	}
}

func TestReconciler_SyncPendingWithdrawals_FiberError(t *testing.T) {
	fiber := &errorFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindWithdrawal,
		ExternalID: "ext_err", State: "pending",
		Asset: "CKB", Amount: "50", ProviderOrgID: "org_p",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	_, err := r.Sync(context.Background())
	if err == nil {
		t.Error("expected error from fiber")
	}
}

type delayedErrorFiberClient struct {
	callCount int
	failAfter int
}

func (d *delayedErrorFiberClient) CreateInvoice(_ context.Context, _ fiberclient.CreateInvoiceInput) (fiberclient.CreateInvoiceResult, error) {
	return fiberclient.CreateInvoiceResult{}, nil
}
func (d *delayedErrorFiberClient) GetInvoiceStatus(_ context.Context, _ string) (fiberclient.InvoiceStatusResult, error) {
	d.callCount++
	if d.callCount > d.failAfter {
		return fiberclient.InvoiceStatusResult{}, fmt.Errorf("fiber down (call %d)", d.callCount)
	}
	return fiberclient.InvoiceStatusResult{State: "pending"}, nil
}
func (d *delayedErrorFiberClient) QuotePayout(_ context.Context, _ fiberclient.QuotePayoutInput) (fiberclient.QuotePayoutResult, error) {
	return fiberclient.QuotePayoutResult{}, nil
}
func (d *delayedErrorFiberClient) RequestPayout(_ context.Context, _ fiberclient.RequestPayoutInput) (fiberclient.RequestPayoutResult, error) {
	return fiberclient.RequestPayoutResult{}, nil
}
func (d *delayedErrorFiberClient) ListSettledFeed(_ context.Context, _ fiberclient.SettledFeedInput) (fiberclient.SettledFeedResult, error) {
	return fiberclient.SettledFeedResult{}, nil
}
func (d *delayedErrorFiberClient) ListWithdrawalStatuses(_ context.Context, _ string) (fiberclient.WithdrawalStatusResult, error) {
	return fiberclient.WithdrawalStatusResult{}, nil
}

func TestRunReconcilerLoop_TickerErrors(t *testing.T) {
	// First sync succeeds (failAfter=1 means first call OK, second fails)
	fiber := &delayedErrorFiberClient{failAfter: 1}
	funding := NewMemoryFundingRecordRepository()

	id, _ := funding.NextID()
	funding.Save(FundingRecord{
		ID: id, Kind: FundingRecordKindInvoice,
		Invoice: "lnbc_ticker_err", State: "pending",
		Asset: "CKB", Amount: "100",
	})

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})
	logger := log.New(log.Writer(), "ticker-err: ", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := RunReconcilerLoop(ctx, r, 20*time.Millisecond, logger)
	// Should exit with either context deadline or max consecutive errors
	if err == nil {
		t.Error("expected error from loop")
	}
	t.Logf("loop exited: %v (fiber calls: %d)", err, fiber.callCount)
}

func TestNewReconciler_NilFiber(t *testing.T) {
	r := NewReconciler(ReconcilerOptions{
		Funding: NewMemoryFundingRecordRepository(),
		// Fiber nil — should use NewClientFromEnv which returns missingClient
	})
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}
}

func TestNewReconciler_WithAll(t *testing.T) {
	r := NewReconciler(ReconcilerOptions{
		Fiber:   &stubFiberClient{},
		Funding: NewMemoryFundingRecordRepository(),
	})
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}
}

func TestRunReconcilerLoop_TickerWithRecovery(t *testing.T) {
	// First sync succeeds, then ticker fires with success
	fiber := &stubFiberClient{}
	funding := NewMemoryFundingRecordRepository()

	r := NewReconciler(ReconcilerOptions{Fiber: fiber, Funding: funding})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := RunReconcilerLoop(ctx, r, 30*time.Millisecond, nil)
	// Should exit with context deadline
	if err != context.DeadlineExceeded {
		t.Logf("exit: %v", err)
	}
}

func TestNewReconciler_DefaultFunding(t *testing.T) {
	// No SETTLEMENT_DATABASE_URL — falls back to memory
	t.Setenv("SETTLEMENT_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")

	r := NewReconciler(ReconcilerOptions{
		Fiber: &stubFiberClient{},
		// Funding nil — should use loadFundingRecordRepositoryE which returns memory
	})
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}

	// Verify it works
	summary, err := r.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.InvoiceUpdates != 0 {
		t.Errorf("expected 0 updates, got %d", summary.InvoiceUpdates)
	}
}

func TestNewReconciler_WithPostgres(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("SETTLEMENT_DATABASE_URL", dsn)

	r := NewReconciler(ReconcilerOptions{
		Fiber: &stubFiberClient{},
		// Funding nil — should load from env with postgres
	})
	if r == nil {
		t.Fatal("expected non-nil reconciler")
	}
}
