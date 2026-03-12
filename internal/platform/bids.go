package platform

import (
	"errors"
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
	QuoteCents    int64
	Milestones    []BidMilestoneInput
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
		return Bid{}, errors.New("rfq is not open for bids")
	}
	if input.ProviderOrgID == "" || input.Message == "" || input.QuoteCents <= 0 || len(input.Milestones) == 0 {
		return Bid{}, errors.New("missing required fields")
	}

	bidID, err := a.bids.NextID()
	if err != nil {
		return Bid{}, err
	}

	now := time.Now().UTC()
	bid := Bid{
		ID:            bidID,
		RFQID:         rfqID,
		ProviderOrgID: input.ProviderOrgID,
		Message:       input.Message,
		QuoteCents:    input.QuoteCents,
		Status:        BidStatusOpen,
		Milestones:    make([]BidMilestone, 0, len(input.Milestones)),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	for _, milestone := range input.Milestones {
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
		return RFQ{}, nil, errors.New("rfq is not open for award")
	}
	if input.BidID == "" {
		return RFQ{}, nil, errors.New("bid id is required")
	}

	bid, err := a.bids.Get(input.BidID)
	if err != nil {
		return RFQ{}, nil, err
	}
	if bid.RFQID != rfqID {
		return RFQ{}, nil, errors.New("bid does not belong to rfq")
	}

	bids, err := a.bids.ListByRFQ(rfqID)
	if err != nil {
		return RFQ{}, nil, err
	}
	for _, candidate := range bids {
		candidate.UpdatedAt = time.Now().UTC()
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
	rfq.UpdatedAt = time.Now().UTC()
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

	return rfq, order, nil
}

func cloneBid(bid Bid) Bid {
	cloned := bid
	cloned.Milestones = slices.Clone(bid.Milestones)
	return cloned
}
