package settlement

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type Reconciler struct {
	fiber    fiberclient.InvoiceClient
	funding  FundingRecordRepository
	deposits *BuyerDepositService
}

type ReconcilerOptions struct {
	Fiber    fiberclient.InvoiceClient
	Funding  FundingRecordRepository
	Deposits *BuyerDepositService
}

type ReconcileSummary struct {
	InvoiceUpdates      int
	WithdrawalUpdates   int
	DepositSweepUpdates int
}

func NewReconciler(options ReconcilerOptions) *Reconciler {
	r, err := NewReconcilerE(options)
	if err != nil {
		panic(fmt.Sprintf("reconciler: %v", err))
	}
	return r
}

// NewReconcilerE is the error-returning variant of NewReconciler.
func NewReconcilerE(options ReconcilerOptions) (*Reconciler, error) {
	if options.Fiber == nil {
		options.Fiber = fiberclient.NewClientFromEnv()
	}
	if options.Funding == nil {
		funding, err := loadFundingRecordRepositoryE()
		if err != nil {
			return nil, fmt.Errorf("funding store: %w", err)
		}
		options.Funding = funding
	}
	if options.Deposits == nil {
		deposits, err := NewBuyerDepositServiceFromEnvE(options.Funding)
		if err != nil {
			return nil, fmt.Errorf("buyer deposits: %w", err)
		}
		options.Deposits = deposits
	}

	return &Reconciler{
		fiber:    options.Fiber,
		funding:  options.Funding,
		deposits: options.Deposits,
	}, nil
}

func (r *Reconciler) Sync(ctx context.Context) (ReconcileSummary, error) {
	invoiceUpdates, err := r.syncPendingInvoices(ctx)
	if err != nil {
		return ReconcileSummary{}, err
	}

	withdrawalUpdates, err := r.syncPendingWithdrawals(ctx)
	if err != nil {
		return ReconcileSummary{}, err
	}

	depositSweepUpdates, err := r.syncBuyerDeposits(ctx)
	if err != nil {
		return ReconcileSummary{}, err
	}

	return ReconcileSummary{
		InvoiceUpdates:      invoiceUpdates,
		WithdrawalUpdates:   withdrawalUpdates,
		DepositSweepUpdates: depositSweepUpdates,
	}, nil
}

// MaxConsecutiveErrors is the number of consecutive sync failures before
// RunReconcilerLoop gives up and returns an error.
const MaxConsecutiveErrors = 5

func RunReconcilerLoop(ctx context.Context, reconciler *Reconciler, interval time.Duration, logger *log.Logger) error {
	if reconciler == nil {
		return errors.New("reconciler is required")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if logger == nil {
		logger = log.Default()
	}

	runOnce := func() error {
		summary, err := reconciler.Sync(ctx)
		if err != nil {
			return err
		}
		logger.Printf("settlement reconciler synced invoices=%d withdrawals=%d deposit_sweeps=%d", summary.InvoiceUpdates, summary.WithdrawalUpdates, summary.DepositSweepUpdates)
		return nil
	}

	// First run — fail fast if the initial sync fails.
	if err := runOnce(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveErrors := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runOnce(); err != nil {
				consecutiveErrors++
				logger.Printf("settlement reconciler error (%d/%d consecutive): %v", consecutiveErrors, MaxConsecutiveErrors, err)
				if consecutiveErrors >= MaxConsecutiveErrors {
					return fmt.Errorf("reconciler exceeded %d consecutive errors, last: %w", MaxConsecutiveErrors, err)
				}
				continue
			}
			consecutiveErrors = 0
		}
	}
}

func (r *Reconciler) syncPendingInvoices(ctx context.Context) (int, error) {
	records, err := r.funding.List(FundingRecordFilter{})
	if err != nil {
		return 0, err
	}

	updates := 0
	for _, record := range records {
		if strings.TrimSpace(record.Invoice) == "" || isSettledInvoiceState(record.State) {
			continue
		}

		status, err := r.fiber.GetInvoiceStatus(ctx, record.Invoice)
		if err != nil {
			return updates, err
		}
		if strings.TrimSpace(status.State) == "" || strings.EqualFold(status.State, record.State) {
			continue
		}
		if err := r.funding.UpdateInvoiceState(record.Invoice, status.State); err != nil {
			return updates, err
		}
		updates++
	}

	return updates, nil
}

func (r *Reconciler) syncPendingWithdrawals(ctx context.Context) (int, error) {
	records, err := r.funding.List(FundingRecordFilter{Kind: FundingRecordKindWithdrawal})
	if err != nil {
		return 0, err
	}

	providers := make(map[string]struct{})
	for _, record := range records {
		if strings.TrimSpace(record.ProviderOrgID) == "" || isTerminalWithdrawalState(record.State) {
			continue
		}
		providers[record.ProviderOrgID] = struct{}{}
	}

	updates := 0
	for providerOrgID := range providers {
		statuses, err := r.fiber.ListWithdrawalStatuses(ctx, providerOrgID)
		if err != nil {
			return updates, err
		}
		for _, item := range statuses.Withdrawals {
			if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.State) == "" {
				continue
			}
			if err := r.funding.UpdateExternalState(item.ID, item.State); err != nil {
				return updates, err
			}
			updates++
		}
	}

	return updates, nil
}

func (r *Reconciler) syncBuyerDeposits(ctx context.Context) (int, error) {
	if r.deposits == nil {
		return 0, nil
	}
	summary, err := r.deposits.SyncDeposits(ctx)
	if err != nil {
		return 0, err
	}
	return summary.SweepCount, nil
}

func isSettledInvoiceState(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "SETTLED")
}

func isTerminalWithdrawalState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "COMPLETED", "FAILED", "CANCELLED", "REJECTED":
		return true
	default:
		return false
	}
}
