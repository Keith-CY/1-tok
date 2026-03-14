package platform

import "fmt"

// DefaultMilestoneSplit generates a platform-default milestone breakdown for a
// given total budget.  The split follows a three-phase pattern common to
// AI-agent service delivery:
//
//   - Setup (20%) — environment provisioning, data ingestion, configuration
//   - Execution (60%) — core agent work
//   - Delivery (20%) — validation, handoff, documentation
//
// For very small budgets (< 1000 cents = $10) a single milestone is used.
// Callers may supply their own milestones to override this default.
func DefaultMilestoneSplit(budgetCents int64) []CreateMilestoneInput {
	if budgetCents <= 0 {
		return nil
	}

	// Single milestone for small budgets
	if budgetCents < 1000 {
		return []CreateMilestoneInput{
			{ID: "ms_1", Title: "Execution", BasePriceCents: budgetCents, BudgetCents: budgetCents},
		}
	}

	setupCents := budgetCents * 20 / 100
	deliveryCents := budgetCents * 20 / 100
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
