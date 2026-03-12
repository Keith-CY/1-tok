package settlement

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type Reconciler struct {
	fiber   fiberclient.InvoiceClient
	funding FundingRecordRepository
}

type ReconcilerOptions struct {
	Fiber   fiberclient.InvoiceClient
	Funding FundingRecordRepository
}

type ReconcileSummary struct {
	InvoiceUpdates    int
	WithdrawalUpdates int
}

func NewReconciler(options ReconcilerOptions) *Reconciler {
	if options.Fiber == nil {
		options.Fiber = fiberclient.NewClientFromEnv()
	}
	if options.Funding == nil {
		options.Funding = loadFundingRecordRepository()
	}

	return &Reconciler{
		fiber:   options.Fiber,
		funding: options.Funding,
	}
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

	return ReconcileSummary{
		InvoiceUpdates:    invoiceUpdates,
		WithdrawalUpdates: withdrawalUpdates,
	}, nil
}

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
		logger.Printf("settlement reconciler synced invoices=%d withdrawals=%d", summary.InvoiceUpdates, summary.WithdrawalUpdates)
		return nil
	}

	if err := runOnce(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runOnce(); err != nil {
				return err
			}
		}
	}
}

func (r *Reconciler) syncPendingInvoices(ctx context.Context) (int, error) {
	records, err := r.funding.List(FundingRecordFilter{Kind: FundingRecordKindInvoice})
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
