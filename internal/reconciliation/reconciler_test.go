package reconciliation

import (
	"testing"

	"github.com/chenyu/1-tok/internal/core"
)

func TestReconcile_NoAnomaly(t *testing.T) {
	ms := core.Milestone{
		SettledCents: 1000,
		UsageCharges: []core.UsageCharge{
			{Kind: core.UsageChargeKindToken, AmountCents: 500},
			{Kind: core.UsageChargeKindToken, AmountCents: 500},
		},
	}
	result := Reconcile(ms, DefaultThreshold)
	if len(result.Anomalies) != 0 {
		t.Errorf("expected no anomalies, got %v", result.Anomalies)
	}
	if result.DeviationRatio != 0 {
		t.Errorf("deviation = %f, want 0", result.DeviationRatio)
	}
}

func TestReconcile_SmallDeviation(t *testing.T) {
	ms := core.Milestone{
		SettledCents: 1050,
		UsageCharges: []core.UsageCharge{
			{Kind: core.UsageChargeKindToken, AmountCents: 1000},
		},
	}
	result := Reconcile(ms, DefaultThreshold)
	if len(result.Anomalies) != 0 {
		t.Errorf("5%% deviation should not flag anomaly, got %v", result.Anomalies)
	}
}

func TestReconcile_LargeDeviation(t *testing.T) {
	ms := core.Milestone{
		SettledCents: 2000,
		UsageCharges: []core.UsageCharge{
			{Kind: core.UsageChargeKindToken, AmountCents: 1000},
		},
	}
	result := Reconcile(ms, DefaultThreshold)
	if len(result.Anomalies) == 0 {
		t.Error("expected anomaly for 100% deviation")
	}
	if result.DeviationRatio < 0.5 {
		t.Errorf("deviation = %f, want >= 0.5", result.DeviationRatio)
	}
}

func TestReconcile_ZeroUsageSettlement(t *testing.T) {
	ms := core.Milestone{
		SettledCents: 5000,
		UsageCharges: nil,
	}
	result := Reconcile(ms, DefaultThreshold)
	if len(result.Anomalies) < 2 {
		t.Errorf("expected 2 anomaly flags (deviation + zero usage), got %d: %v", len(result.Anomalies), result.Anomalies)
	}
}

func TestReconcile_BothZero(t *testing.T) {
	ms := core.Milestone{SettledCents: 0}
	result := Reconcile(ms, DefaultThreshold)
	if len(result.Anomalies) != 0 {
		t.Errorf("expected no anomalies for zero/zero, got %v", result.Anomalies)
	}
}

func TestReconcile_DefaultThreshold(t *testing.T) {
	ms := core.Milestone{
		SettledCents: 2000,
		UsageCharges: []core.UsageCharge{
			{Kind: core.UsageChargeKindToken, AmountCents: 1000},
		},
	}
	result := Reconcile(ms, 0) // 0 = use default
	if len(result.Anomalies) == 0 {
		t.Error("expected anomaly with default threshold")
	}
}

func TestReconcile_UnderSettlement(t *testing.T) {
	// Settled less than usage — also suspicious
	ms := core.Milestone{
		SettledCents: 500,
		UsageCharges: []core.UsageCharge{
			{Kind: core.UsageChargeKindToken, AmountCents: 1000},
		},
	}
	result := Reconcile(ms, DefaultThreshold)
	if len(result.Anomalies) == 0 {
		t.Error("expected anomaly for under-settlement")
	}
}
