package platform

import "fmt"

const (
	// SmallBudgetThresholdCents is the budget below which a single milestone
	// is used instead of the standard 3-phase split ($10).
	SmallBudgetThresholdCents = 1000

	// Default milestone budget allocation percentages.
	SetupPercent    = 20
	DeliveryPercent = 20
	// ExecutionPercent is implicitly 100 - Setup - Delivery to avoid rounding loss.
)

// DefaultMilestoneSplit generates a platform-default milestone breakdown for a
// given total budget.  The split follows a three-phase pattern common to
// AI-agent service delivery:
//
//   - Setup (20%) — environment provisioning, data ingestion, configuration
//   - Execution (60%) — core agent work
//   - Delivery (20%) — validation, handoff, documentation
//
// For very small budgets (< SmallBudgetThresholdCents) a single milestone is used.
// Callers may supply their own milestones to override this default.
func DefaultMilestoneSplit(budgetCents int64) []CreateMilestoneInput {
	if budgetCents <= 0 {
		return nil
	}

	// Single milestone for small budgets
	if budgetCents < SmallBudgetThresholdCents {
		return []CreateMilestoneInput{
			{ID: "ms_1", Title: "Execution", BasePriceCents: budgetCents, BudgetCents: budgetCents},
		}
	}

	setupCents := budgetCents * SetupPercent / 100
	deliveryCents := budgetCents * DeliveryPercent / 100
	executionCents := budgetCents - setupCents - deliveryCents // remainder to avoid rounding loss

	return []CreateMilestoneInput{
		{ID: "ms_1", Title: fmt.Sprintf("Setup (%d%%)", percent(setupCents, budgetCents)), BasePriceCents: setupCents, BudgetCents: setupCents},
		{ID: "ms_2", Title: fmt.Sprintf("Execution (%d%%)", percent(executionCents, budgetCents)), BasePriceCents: executionCents, BudgetCents: executionCents},
		{ID: "ms_3", Title: fmt.Sprintf("Delivery (%d%%)", percent(deliveryCents, budgetCents)), BasePriceCents: deliveryCents, BudgetCents: deliveryCents},
	}
}

func percent(part, total int64) int64 {
	if total == 0 {
		return 0
	}
	return part * 100 / total
}
