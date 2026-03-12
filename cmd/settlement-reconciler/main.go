package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenyu/1-tok/internal/services/settlement"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reconciler := settlement.NewReconciler(settlement.ReconcilerOptions{})
	if envBool("SETTLEMENT_RECONCILER_ONCE") {
		summary, err := reconciler.Sync(ctx)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("settlement reconciler synced invoices=%d withdrawals=%d", summary.InvoiceUpdates, summary.WithdrawalUpdates)
		return
	}

	if err := settlement.RunReconcilerLoop(ctx, reconciler, reconcileInterval(), log.Default()); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func reconcileInterval() time.Duration {
	value := strings.TrimSpace(os.Getenv("SETTLEMENT_RECONCILER_INTERVAL"))
	if value == "" {
		return 30 * time.Second
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 30 * time.Second
}

func envBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
