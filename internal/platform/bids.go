package platform

import (
	"slices"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

type BidStatus string

const (
	BidStatusOpen     BidStatus = "open"
	BidStatusAwarded  BidStatus = "awarded"
	BidStatusRejected BidStatus = "rejected"
)

type BidMilestone struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	BasePriceCents int64  `json:"basePriceCents"`
	BudgetCents    int64  `json:"budgetCents"`
}

type Bid struct {
	ID            string         `json:"id"`
	RFQID         string         `json:"rfqId"`
	ProviderOrgID string         `json:"providerOrgId"`
	Message       string         `json:"message"`
	QuoteCents    int64          `json:"quoteCents"`
	Status        BidStatus      `json:"status"`
	Milestones    []BidMilestone `json:"milestones"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type BidMilestoneInput struct {
	ID             string
	Title          string
	BasePriceCents int64
	BudgetCents    int64
}

type CreateBidInput struct {
	ProviderOrgID string
	Message       string
	QuoteCents    int64              // optional: defaults to RFQ budget
	Milestones    []BidMilestoneInput // optional: defaults to RFQ default milestones
}

type AwardRFQInput struct {
	BidID        string
	FundingMode  core.FundingMode
	CreditLineID string
}

func (a *App) ListRFQBids(rfqID string) ([]Bid, error) {
	return a.bids.ListByRFQ(rfqID)
}

func (a *App) CreateBid(rfqID string, input CreateBidInput) (Bid, error) {
	rfq, err := a.rfqs.Get(rfqID)
	if err != nil {
		return Bid{}, err
	}
	if rfq.Status != RFQStatusOpen {
		return Bid{}, ErrRFQNotOpenForBids
	}
	if input.ProviderOrgID == "" || input.Message == "" {
		return Bid{}, ErrMissingRequiredFields
	}

	// Default to RFQ budget and milestones when the provider does not supply their own.
	quoteCents := input.QuoteCents
	if quoteCents <= 0 {
		quoteCents = rfq.BudgetCents
	}

	milestones := input.Milestones
	if len(milestones) == 0 && len(rfq.DefaultMilestones) > 0 {
		milestones = make([]BidMilestoneInput, 0, len(rfq.DefaultMilestones))
		for _, m := range rfq.DefaultMilestones {
			milestones = append(milestones, BidMilestoneInput{
				ID:             m.ID,
				Title:          m.Title,
				BasePriceCents: m.BasePriceCents,
				BudgetCents:    m.BudgetCents,
			})
		}
	}

	if len(milestones) == 0 {
		return Bid{}, ErrMilestonesRequired
	}

	bidID, err := a.bids.NextID()
	if err != nil {
		return Bid{}, err
	}

	now := a.now()
	bid := Bid{
		ID:            bidID,
		RFQID:         rfqID,
		ProviderOrgID: input.ProviderOrgID,
		Message:       input.Message,
		QuoteCents:    quoteCents,
		Status:        BidStatusOpen,
		Milestones:    make([]BidMilestone, 0, len(milestones)),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	for _, milestone := range milestones {
		bid.Milestones = append(bid.Milestones, BidMilestone{
			ID:             milestone.ID,
			Title:          milestone.Title,
			BasePriceCents: milestone.BasePriceCents,
			BudgetCents:    milestone.BudgetCents,
		})
	}

	if err := a.bids.Save(bid); err != nil {
		return Bid{}, err
	}

	if err := a.publish("market.bid.submitted", map[string]any{
		"bidId":         bid.ID,
		"rfqId":         bid.RFQID,
		"providerOrgId": bid.ProviderOrgID,
		"quoteCents":    bid.QuoteCents,
	}); err != nil {
		return Bid{}, err
	}

	return bid, nil
}

func (a *App) AwardRFQ(rfqID string, input AwardRFQInput) (RFQ, *core.Order, error) {
	rfq, err := a.rfqs.Get(rfqID)
	if err != nil {
		return RFQ{}, nil, err
	}
	if rfq.Status != RFQStatusOpen {
		return RFQ{}, nil, ErrRFQNotOpenForAward
	}
	if input.BidID == "" {
		return RFQ{}, nil, ErrBidIDRequired
	}

	bid, err := a.bids.Get(input.BidID)
	if err != nil {
		return RFQ{}, nil, err
	}
	if bid.RFQID != rfqID {
		return RFQ{}, nil, ErrBidNotBelongToRFQ
	}

	bids, err := a.bids.ListByRFQ(rfqID)
	if err != nil {
		return RFQ{}, nil, err
	}
	for _, candidate := range bids {
		candidate.UpdatedAt = a.now()
		if candidate.ID == bid.ID {
			candidate.Status = BidStatusAwarded
		} else {
			candidate.Status = BidStatusRejected
		}
		if err := a.bids.Save(candidate); err != nil {
			return RFQ{}, nil, err
		}
	}

	orderInput := CreateOrderInput{
		BuyerOrgID:    rfq.BuyerOrgID,
		ProviderOrgID: bid.ProviderOrgID,
		Title:         rfq.Title,
		FundingMode:   input.FundingMode,
		CreditLineID:  input.CreditLineID,
		Milestones:    make([]CreateMilestoneInput, 0, len(bid.Milestones)),
	}
	for _, milestone := range bid.Milestones {
		orderInput.Milestones = append(orderInput.Milestones, CreateMilestoneInput{
			ID:             milestone.ID,
			Title:          milestone.Title,
			BasePriceCents: milestone.BasePriceCents,
			BudgetCents:    milestone.BudgetCents,
		})
	}

	order, err := a.CreateOrder(orderInput)
	if err != nil {
		return RFQ{}, nil, err
	}

	rfq.Status = RFQStatusAwarded
	rfq.AwardedBidID = bid.ID
	rfq.AwardedProviderOrgID = bid.ProviderOrgID
	rfq.OrderID = order.ID
	rfq.UpdatedAt = a.now()
	if err := a.rfqs.Save(rfq); err != nil {
		return RFQ{}, nil, err
	}

	if err := a.publish("market.rfq.awarded", map[string]any{
		"rfqId":              rfq.ID,
		"bidId":              bid.ID,
		"providerOrgId":      bid.ProviderOrgID,
		"orderId":            order.ID,
		"fundingMode":        order.FundingMode,
		"responseDeadlineAt": rfq.ResponseDeadlineAt,
	}); err != nil {
		return RFQ{}, nil, err
	}

	a.notify("rfq.awarded", rfq.BuyerOrgID, map[string]any{"rfqId": rfq.ID, "orderId": order.ID})
	a.notify("rfq.awarded", bid.ProviderOrgID, map[string]any{"rfqId": rfq.ID, "orderId": order.ID})

	return rfq, order, nil
}

func cloneBid(bid Bid) Bid {
	cloned := bid
	cloned.Milestones = slices.Clone(bid.Milestones)
	return cloned
}
