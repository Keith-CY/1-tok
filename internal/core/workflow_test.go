package core

import (
	"testing"
	"time"
)

func TestMilestoneSettlementUsesCreditAndCreatesExposure(t *testing.T) {
	now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	order := Order{
		ID:             "ord_1",
		BuyerOrgID:     "buyer_1",
		ProviderOrgID:  "provider_1",
		FundingMode:    FundingModeCredit,
		Status:         OrderStatusRunning,
		CreditLineID:   "credit_1",
		PlatformWallet: "platform_main",
		Milestones: []Milestone{
			{
				ID:             "ms_1",
				Title:          "Plan execution",
				BasePriceCents: 1_000,
				BudgetCents:    1_500,
				State:          MilestoneStateRunning,
			},
			{
				ID:             "ms_2",
				Title:          "Verify delivery",
				BasePriceCents: 500,
				BudgetCents:    900,
				State:          MilestoneStatePending,
			},
		},
	}

	entry, err := order.SettleMilestone(SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "completed",
		Source:      "carrier",
		OccurredAt:  now,
	})
	if err != nil {
		t.Fatalf("expected settlement to succeed, got %v", err)
	}

	if order.Status != OrderStatusRunning {
		t.Fatalf("expected order to keep running for remaining milestones, got %s", order.Status)
	}

	if order.Milestones[0].State != MilestoneStateSettled {
		t.Fatalf("expected milestone settled, got %s", order.Milestones[0].State)
	}

	if entry.Kind != LedgerEntryKindPlatformExposure {
		t.Fatalf("expected exposure ledger entry, got %s", entry.Kind)
	}

	if entry.AmountCents != 1_000 {
		t.Fatalf("expected exposure amount 1000, got %d", entry.AmountCents)
	}
}

func TestUsageChargePausesOrderWhenBudgetIsExceeded(t *testing.T) {
	order := Order{
		ID:            "ord_2",
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		FundingMode:   FundingModePrepaid,
		Status:        OrderStatusRunning,
		Milestones: []Milestone{
			{
				ID:             "ms_1",
				Title:          "Run agent",
				BasePriceCents: 900,
				BudgetCents:    1_000,
				State:          MilestoneStateRunning,
				UsageCharges: []UsageCharge{
					{Kind: UsageChargeKindToken, AmountCents: 50},
				},
			},
		},
	}

	charge, err := order.RecordUsageCharge(RecordUsageChargeInput{
		MilestoneID: "ms_1",
		Kind:        UsageChargeKindExternalAPI,
		AmountCents: 60,
		ProofRef:    "evt_1",
	})
	if err != nil {
		t.Fatalf("expected charge to be accepted, got %v", err)
	}

	if charge.Kind != UsageChargeKindExternalAPI {
		t.Fatalf("expected external_api charge, got %s", charge.Kind)
	}

	if order.Status != OrderStatusAwaitingBudget {
		t.Fatalf("expected order awaiting budget, got %s", order.Status)
	}

	if order.Milestones[0].State != MilestoneStatePaused {
		t.Fatalf("expected milestone paused, got %s", order.Milestones[0].State)
	}
}

func TestDisputeCreatesRecoveryAgainstProvider(t *testing.T) {
	order := Order{
		ID:            "ord_3",
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		FundingMode:   FundingModeCredit,
		Status:        OrderStatusRunning,
		Milestones: []Milestone{
			{
				ID:             "ms_1",
				Title:          "Diagnose",
				BasePriceCents: 800,
				BudgetCents:    1_000,
				State:          MilestoneStateSettled,
				SettledCents:   800,
			},
		},
	}

	refund, recovery, err := order.OpenDispute(OpenDisputeInput{
		MilestoneID: "ms_1",
		Reason:      "bad output",
		RefundCents: 300,
	})
	if err != nil {
		t.Fatalf("expected dispute to succeed, got %v", err)
	}

	if refund.Kind != LedgerEntryKindBuyerReimbursement {
		t.Fatalf("expected buyer reimbursement, got %s", refund.Kind)
	}

	if recovery.Kind != LedgerEntryKindProviderRecovery {
		t.Fatalf("expected provider recovery, got %s", recovery.Kind)
	}

	if order.Milestones[0].DisputeStatus != DisputeStatusOpen {
		t.Fatalf("expected open dispute, got %s", order.Milestones[0].DisputeStatus)
	}
}

func TestMilestoneSettlementFromPrepaidCreatesProviderPayout(t *testing.T) {
	now := time.Date(2026, 3, 11, 13, 0, 0, 0, time.UTC)
	order := Order{
		ID:            "ord_4",
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		FundingMode:   FundingModePrepaid,
		Status:        OrderStatusRunning,
		Milestones: []Milestone{
			{
				ID:             "ms_1",
				Title:          "Deliver",
				BasePriceCents: 700,
				BudgetCents:    1200,
				State:          MilestoneStateRunning,
			},
			{
				ID:             "ms_2",
				Title:          "Closeout",
				BasePriceCents: 200,
				BudgetCents:    300,
				State:          MilestoneStatePending,
			},
		},
	}

	entry, err := order.SettleMilestone(SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  now,
	})
	if err != nil {
		t.Fatalf("expected settlement to succeed, got %v", err)
	}

	if entry.Kind != LedgerEntryKindProviderPayout {
		t.Fatalf("expected provider payout entry, got %s", entry.Kind)
	}
}

func TestMilestoneSettlementUsesReservedFundsWhenPrepaid(t *testing.T) {
	now := time.Date(2026, 3, 11, 12, 15, 0, 0, time.UTC)
	order := Order{
		ID:            "ord_4",
		BuyerOrgID:    "buyer_2",
		ProviderOrgID: "provider_1",
		FundingMode:   FundingModePrepaid,
		Status:        OrderStatusRunning,
		Milestones: []Milestone{
			{
				ID:             "ms_1",
				Title:          "Execute",
				BasePriceCents: 700,
				BudgetCents:    900,
				State:          MilestoneStateRunning,
			},
			{
				ID:             "ms_2",
				Title:          "Deliver",
				BasePriceCents: 400,
				BudgetCents:    700,
				State:          MilestoneStatePending,
			},
		},
	}

	entry, err := order.SettleMilestone(SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  now,
	})
	if err != nil {
		t.Fatalf("expected settlement to succeed, got %v", err)
	}

	if entry.Kind != LedgerEntryKindProviderPayout {
		t.Fatalf("expected provider payout entry, got %s", entry.Kind)
	}

	if entry.AmountCents != 700 {
		t.Fatalf("expected reserved capture amount 700, got %d", entry.AmountCents)
	}
}

func TestCreditDecisionUsesHistorySignals(t *testing.T) {
	engine := CreditDecisionEngine{
		BaseLimitCents:        50_000,
		MaxLimitCents:         500_000,
		DisputePenaltyCents:   75_000,
		FailurePenaltyCents:   50_000,
		ConsumptionMultiplier: 2,
	}

	decision := engine.Decide(CreditHistory{
		CompletedOrders:    24,
		SuccessfulPayments: 22,
		FailedPayments:     0,
		DisputedOrders:     1,
		LifetimeSpendCents: 240_000,
	})

	if !decision.Approved {
		t.Fatalf("expected approved decision")
	}

	if decision.RecommendedLimitCents <= 0 {
		t.Fatalf("expected positive limit, got %d", decision.RecommendedLimitCents)
	}

	if decision.RecommendedLimitCents >= engine.MaxLimitCents {
		t.Fatalf("expected capped limit below max, got %d", decision.RecommendedLimitCents)
	}
}

func TestResolveDispute(t *testing.T) {
	order := Order{
		ID: "ord_1", BuyerOrgID: "b", ProviderOrgID: "p",
		FundingMode: FundingModeCredit, Status: OrderStatusRunning,
		PlatformWallet: "w",
		Milestones: []Milestone{
			{ID: "ms_1", Title: "Work", BasePriceCents: 1000, BudgetCents: 1000,
				State: MilestoneStateSettled, DisputeStatus: DisputeStatusNone},
		},
	}

	_, _, err := order.OpenDispute(OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "quality issue", RefundCents: 500,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = order.ResolveDispute(ResolveDisputeInput{MilestoneID: "ms_1"})
	if err != nil {
		t.Fatal(err)
	}
	if order.Milestones[0].DisputeStatus != DisputeStatusResolved {
		t.Errorf("dispute status = %s, want resolved", order.Milestones[0].DisputeStatus)
	}
}

func TestResolveDispute_NoOpenDispute(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning,
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStateRunning, DisputeStatus: DisputeStatusNone},
		},
	}

	err := order.ResolveDispute(ResolveDisputeInput{MilestoneID: "ms_1"})
	if err == nil {
		t.Error("expected error when resolving non-disputed milestone")
	}
}

func TestResolveDispute_MilestoneNotFound(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning,
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStateRunning},
		},
	}

	err := order.ResolveDispute(ResolveDisputeInput{MilestoneID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent milestone")
	}
}

func TestCreditDecision_InsufficientHistory(t *testing.T) {
	engine := CreditDecisionEngine{BaseLimitCents: 10000, MaxLimitCents: 100000}
	decision := engine.Decide(CreditHistory{CompletedOrders: 1, SuccessfulPayments: 1})

	if decision.Approved {
		t.Error("expected not approved")
	}
	if decision.Reason != "insufficient history" {
		t.Errorf("reason = %s", decision.Reason)
	}
}

func TestCreditDecision_Approved(t *testing.T) {
	engine := CreditDecisionEngine{
		BaseLimitCents:         10000,
		MaxLimitCents:          100000,
		ConsumptionMultiplier:  2,
		DisputePenaltyCents:    1000,
		FailurePenaltyCents:    2000,
	}
	decision := engine.Decide(CreditHistory{
		CompletedOrders:    5,
		SuccessfulPayments: 5,
		LifetimeSpendCents: 50000,
	})

	if !decision.Approved {
		t.Error("expected approved")
	}
	if decision.RecommendedLimitCents <= 0 {
		t.Errorf("limit = %d", decision.RecommendedLimitCents)
	}
}

func TestCreditDecision_RiskExceeded(t *testing.T) {
	engine := CreditDecisionEngine{
		BaseLimitCents:      1000,
		MaxLimitCents:       100000,
		DisputePenaltyCents: 5000,
	}
	decision := engine.Decide(CreditHistory{
		CompletedOrders:    5,
		SuccessfulPayments: 5,
		DisputedOrders:     3,
	})

	if decision.Approved {
		t.Error("expected not approved due to disputes")
	}
	if decision.Reason != "risk signals exceeded threshold" {
		t.Errorf("reason = %s", decision.Reason)
	}
}

func TestCreditDecision_CappedAtMax(t *testing.T) {
	engine := CreditDecisionEngine{
		BaseLimitCents:        100000,
		MaxLimitCents:         50000,
		ConsumptionMultiplier: 1,
	}
	decision := engine.Decide(CreditHistory{
		CompletedOrders:    10,
		SuccessfulPayments: 10,
		LifetimeSpendCents: 500000,
	})

	if !decision.Approved {
		t.Error("expected approved")
	}
	if decision.RecommendedLimitCents != 50000 {
		t.Errorf("expected capped at 50000, got %d", decision.RecommendedLimitCents)
	}
}

func TestSettleMilestone_NotRunning(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning, FundingMode: FundingModeCredit,
		PlatformWallet: "w",
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStatePending, BasePriceCents: 1000, BudgetCents: 1000},
		},
	}

	_, err := order.SettleMilestone(SettleMilestoneInput{MilestoneID: "ms_1"})
	if err == nil {
		t.Error("expected error for settling non-running milestone")
	}
}

func TestRecordUsageCharge_NotRunning(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning,
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStatePending, BasePriceCents: 1000, BudgetCents: 1000},
		},
	}

	_, err := order.RecordUsageCharge(RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: "token", AmountCents: 100,
	})
	if err == nil {
		t.Error("expected error for usage on pending milestone")
	}
}

func TestRecordUsageCharge_MilestoneNotFound(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning,
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStateRunning, BasePriceCents: 1000, BudgetCents: 1000},
		},
	}

	_, err := order.RecordUsageCharge(RecordUsageChargeInput{
		MilestoneID: "ms_nonexistent", Kind: "token", AmountCents: 100,
	})
	if err == nil {
		t.Error("expected error for nonexistent milestone")
	}
}

func TestOpenDispute_NotSettled(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning, FundingMode: FundingModeCredit,
		PlatformWallet: "w",
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStateRunning, BasePriceCents: 1000, BudgetCents: 1000},
		},
	}

	_, _, err := order.OpenDispute(OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "bad", RefundCents: 500,
	})
	if err == nil {
		t.Error("expected error for disputing non-settled milestone")
	}
}

func TestIsLastMilestoneSettled_AllSettled(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning,
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStateSettled},
			{ID: "ms_2", State: MilestoneStateSettled},
		},
	}
	if !order.isLastMilestoneSettled() {
		t.Error("expected true when all milestones settled")
	}
}

func TestIsLastMilestoneSettled_NotAll(t *testing.T) {
	order := Order{
		ID: "ord_1", Status: OrderStatusRunning,
		Milestones: []Milestone{
			{ID: "ms_1", State: MilestoneStateSettled},
			{ID: "ms_2", State: MilestoneStateRunning},
		},
	}
	if order.isLastMilestoneSettled() {
		t.Error("expected false when not all milestones settled")
	}
}
