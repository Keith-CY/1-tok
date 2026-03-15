package core

import (
	"errors"
	"fmt"
	"time"
)

type FundingMode string

const (
	FundingModePrepaid FundingMode = "prepaid"
	FundingModeCredit  FundingMode = "credit"
)

type OrderStatus string

const (
	OrderStatusDraft          OrderStatus = "draft"
	OrderStatusRunning        OrderStatus = "running"
	OrderStatusAwaitingBudget OrderStatus = "awaiting_budget"
	OrderStatusCompleted      OrderStatus = "completed"
	OrderStatusFailed         OrderStatus = "failed"
)

type MilestoneState string

const (
	MilestoneStatePending MilestoneState = "pending"
	MilestoneStateRunning MilestoneState = "running"
	MilestoneStatePaused  MilestoneState = "paused"
	MilestoneStateSettled MilestoneState = "settled"
)

type DisputeStatus string

const (
	DisputeStatusNone     DisputeStatus = "none"
	DisputeStatusOpen     DisputeStatus = "open"
	DisputeStatusResolved DisputeStatus = "resolved"
)

type UsageChargeKind string

const (
	UsageChargeKindStep        UsageChargeKind = "step"
	UsageChargeKindToken       UsageChargeKind = "token"
	UsageChargeKindExternalAPI UsageChargeKind = "external_api"
)

type LedgerEntryKind string

const (
	LedgerEntryKindPlatformExposure   LedgerEntryKind = "platform_exposure"
	LedgerEntryKindProviderPayout     LedgerEntryKind = "provider_payout"
	LedgerEntryKindBuyerReimbursement LedgerEntryKind = "buyer_reimbursement"
	LedgerEntryKindProviderRecovery   LedgerEntryKind = "provider_recovery"
)

type Order struct {
	ID             string      `json:"id"`
	BuyerOrgID     string      `json:"buyerOrgId"`
	ProviderOrgID  string      `json:"providerOrgId"`
	FundingMode    FundingMode `json:"fundingMode"`
	CreditLineID   string      `json:"creditLineId,omitempty"`
	PlatformWallet string      `json:"platformWallet,omitempty"`
	Status         OrderStatus `json:"status"`
	Milestones     []Milestone `json:"milestones"`
}

type Milestone struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	BasePriceCents int64          `json:"basePriceCents"`
	BudgetCents    int64          `json:"budgetCents"`
	SettledCents   int64          `json:"settledCents"`
	Summary        string         `json:"summary,omitempty"`
	State          MilestoneState `json:"state"`
	DisputeStatus  DisputeStatus  `json:"disputeStatus"`
	UsageCharges   []UsageCharge  `json:"usageCharges"`
	SettledAt      *time.Time     `json:"settledAt,omitempty"`
}

type UsageCharge struct {
	Kind           UsageChargeKind `json:"kind"`
	AmountCents    int64           `json:"amountCents"`
	ProofRef       string          `json:"proofRef,omitempty"`
	ProofSignature string          `json:"proofSignature,omitempty"`
	ProofTimestamp string          `json:"proofTimestamp,omitempty"`
}

type LedgerEntry struct {
	Kind        LedgerEntryKind   `json:"kind"`
	OrderID     string            `json:"orderId"`
	MilestoneID string            `json:"milestoneId"`
	AmountCents int64             `json:"amountCents"`
	CreatedAt   time.Time         `json:"createdAt"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type SettleMilestoneInput struct {
	MilestoneID string
	Summary     string
	Source      string
	OccurredAt  time.Time
}

type RecordUsageChargeInput struct {
	MilestoneID    string
	Kind           UsageChargeKind
	AmountCents    int64
	ProofRef       string
	ProofSignature string
	ProofTimestamp string
}

type OpenDisputeInput struct {
	MilestoneID string
	Reason      string
	RefundCents int64
}

type ResolveDisputeInput struct {
	MilestoneID string
}

func (o *Order) SettleMilestone(input SettleMilestoneInput) (LedgerEntry, error) {
	milestone, err := o.findMilestone(input.MilestoneID)
	if err != nil {
		return LedgerEntry{}, err
	}

	if milestone.State != MilestoneStateRunning && milestone.State != MilestoneStatePaused {
		return LedgerEntry{}, fmt.Errorf("milestone %s is not payable from state %s", milestone.ID, milestone.State)
	}

	milestone.State = MilestoneStateSettled
	milestone.Summary = input.Summary
	milestone.SettledCents = milestone.BasePriceCents
	milestone.SettledAt = &input.OccurredAt

	if o.isLastMilestoneSettled() {
		o.Status = OrderStatusCompleted
	}

	entryKind := LedgerEntryKindPlatformExposure
	if o.FundingMode == FundingModePrepaid {
		entryKind = LedgerEntryKindProviderPayout
	}

	return LedgerEntry{
		Kind:        entryKind,
		OrderID:     o.ID,
		MilestoneID: milestone.ID,
		AmountCents: milestone.BasePriceCents,
		CreatedAt:   input.OccurredAt,
		Metadata: map[string]string{
			"funding_mode": string(o.FundingMode),
			"source":       input.Source,
		},
	}, nil
}

func (o *Order) RecordUsageCharge(input RecordUsageChargeInput) (UsageCharge, error) {
	milestone, err := o.findMilestone(input.MilestoneID)
	if err != nil {
		return UsageCharge{}, err
	}

	if milestone.State != MilestoneStateRunning && milestone.State != MilestoneStatePaused {
		return UsageCharge{}, fmt.Errorf("milestone %s does not accept usage from state %s", milestone.ID, milestone.State)
	}

	charge := UsageCharge{
		Kind:           input.Kind,
		AmountCents:    input.AmountCents,
		ProofRef:       input.ProofRef,
		ProofSignature: input.ProofSignature,
		ProofTimestamp: input.ProofTimestamp,
	}
	milestone.UsageCharges = append(milestone.UsageCharges, charge)

	if milestone.CurrentSpendCents() > milestone.BudgetCents {
		o.Status = OrderStatusAwaitingBudget
		milestone.State = MilestoneStatePaused
	}

	return charge, nil
}

func (o *Order) OpenDispute(input OpenDisputeInput) (LedgerEntry, LedgerEntry, error) {
	milestone, err := o.findMilestone(input.MilestoneID)
	if err != nil {
		return LedgerEntry{}, LedgerEntry{}, err
	}

	if milestone.State != MilestoneStateSettled {
		return LedgerEntry{}, LedgerEntry{}, errors.New("only settled milestones can be disputed")
	}

	milestone.DisputeStatus = DisputeStatusOpen

	now := time.Now().UTC()
	refund := LedgerEntry{
		Kind:        LedgerEntryKindBuyerReimbursement,
		OrderID:     o.ID,
		MilestoneID: milestone.ID,
		AmountCents: input.RefundCents,
		CreatedAt:   now,
		Metadata: map[string]string{
			"reason": input.Reason,
		},
	}
	recovery := LedgerEntry{
		Kind:        LedgerEntryKindProviderRecovery,
		OrderID:     o.ID,
		MilestoneID: milestone.ID,
		AmountCents: input.RefundCents,
		CreatedAt:   now,
		Metadata: map[string]string{
			"reason": input.Reason,
		},
	}

	return refund, recovery, nil
}

func (o *Order) ResolveDispute(input ResolveDisputeInput) error {
	milestone, err := o.findMilestone(input.MilestoneID)
	if err != nil {
		return err
	}

	if milestone.DisputeStatus != DisputeStatusOpen {
		return errors.New("only open disputes can be resolved")
	}

	milestone.DisputeStatus = DisputeStatusResolved
	return nil
}

func (m Milestone) CurrentSpendCents() int64 {
	total := m.BasePriceCents
	for _, charge := range m.UsageCharges {
		total += charge.AmountCents
	}

	return total
}

func (o *Order) findMilestone(id string) (*Milestone, error) {
	for i := range o.Milestones {
		if o.Milestones[i].ID == id {
			return &o.Milestones[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrMilestoneNotFound, id)
}

func (o Order) isLastMilestoneSettled() bool {
	for _, milestone := range o.Milestones {
		if milestone.State != MilestoneStateSettled {
			return false
		}
	}

	return true
}

type CreditDecisionEngine struct {
	BaseLimitCents        int64
	MaxLimitCents         int64
	DisputePenaltyCents   int64
	FailurePenaltyCents   int64
	ConsumptionMultiplier int64
}

type CreditHistory struct {
	CompletedOrders    int
	SuccessfulPayments int
	FailedPayments     int
	DisputedOrders     int
	LifetimeSpendCents int64
}

type CreditDecision struct {
	Approved              bool   `json:"approved"`
	RecommendedLimitCents int64  `json:"recommendedLimitCents"`
	Reason                string `json:"reason"`
}

func (e CreditDecisionEngine) Decide(history CreditHistory) CreditDecision {
	if history.CompletedOrders < 3 || history.SuccessfulPayments < 3 {
		return CreditDecision{
			Approved:              false,
			RecommendedLimitCents: 0,
			Reason:                "insufficient history",
		}
	}

	limit := e.BaseLimitCents + (history.LifetimeSpendCents / max64(e.ConsumptionMultiplier, 1))
	limit -= int64(history.DisputedOrders) * e.DisputePenaltyCents
	limit -= int64(history.FailedPayments) * e.FailurePenaltyCents

	if limit <= 0 {
		return CreditDecision{
			Approved:              false,
			RecommendedLimitCents: 0,
			Reason:                "risk signals exceeded threshold",
		}
	}

	if limit > e.MaxLimitCents {
		limit = e.MaxLimitCents
	}

	return CreditDecision{
		Approved:              true,
		RecommendedLimitCents: limit,
		Reason:                "approved by rule engine",
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}

	return b
}
