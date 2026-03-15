// Package reconciliation compares real-time usage against settlement summaries
// to detect anomalies (anti-fraud layer 3).
package reconciliation

import (
	"fmt"
	"math"

	"github.com/chenyu/1-tok/internal/core"
)

// DefaultThreshold is the maximum acceptable deviation ratio (10%).
const DefaultThreshold = 0.10

// Result holds the outcome of a reconciliation check.
type Result struct {
	CumulativeCents int64    `json:"cumulativeCents"`
	SettledCents    int64    `json:"settledCents"`
	DeviationRatio  float64  `json:"deviationRatio"`
	Anomalies       []string `json:"anomalies,omitempty"`
}

// Reconcile compares the sum of usage charges against the settled amount.
// Returns anomaly flags if deviation exceeds the threshold.
func Reconcile(milestone core.Milestone, threshold float64) Result {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	cumulative := milestone.CurrentSpendCents()
	settled := milestone.SettledCents

	result := Result{
		CumulativeCents: cumulative,
		SettledCents:    settled,
	}

	if cumulative == 0 && settled == 0 {
		return result // No usage, no settlement — nothing to check
	}

	// Compute deviation ratio
	base := float64(cumulative)
	if base == 0 {
		base = float64(settled) // Avoid division by zero
	}
	result.DeviationRatio = math.Abs(float64(settled)-float64(cumulative)) / base

	if result.DeviationRatio > threshold {
		result.Anomalies = append(result.Anomalies,
			fmt.Sprintf("settlement_deviation: settled=%d cumulative=%d ratio=%.2f threshold=%.2f",
				settled, cumulative, result.DeviationRatio, threshold))
	}

	// Check for zero-usage settlement (provider submitted summary with no usage charges)
	if cumulative == 0 && settled > 0 {
		result.Anomalies = append(result.Anomalies,
			fmt.Sprintf("zero_usage_settlement: settled=%d with no usage charges", settled))
	}

	return result
}
