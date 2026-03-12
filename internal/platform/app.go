package platform

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

type ProviderProfile struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Capabilities   []string `json:"capabilities"`
	ReputationTier string   `json:"reputationTier"`
}

type Listing struct {
	ID             string   `json:"id"`
	ProviderOrgID  string   `json:"providerOrgId"`
	Title          string   `json:"title"`
	Category       string   `json:"category"`
	BasePriceCents int64    `json:"basePriceCents"`
	Tags           []string `json:"tags"`
}

type RFQStatus string

const (
	RFQStatusOpen    RFQStatus = "open"
	RFQStatusAwarded RFQStatus = "awarded"
	RFQStatusClosed  RFQStatus = "closed"
)

type RFQ struct {
	ID                   string    `json:"id"`
	BuyerOrgID           string    `json:"buyerOrgId"`
	Title                string    `json:"title"`
	Category             string    `json:"category"`
	Scope                string    `json:"scope"`
	BudgetCents          int64     `json:"budgetCents"`
	Status               RFQStatus `json:"status"`
	AwardedBidID         string    `json:"awardedBidId,omitempty"`
	AwardedProviderOrgID string    `json:"awardedProviderOrgId,omitempty"`
	OrderID              string    `json:"orderId,omitempty"`
	ResponseDeadlineAt   time.Time `json:"responseDeadlineAt"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type Message struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type Dispute struct {
	ID          string             `json:"id"`
	OrderID     string             `json:"orderId"`
	MilestoneID string             `json:"milestoneId"`
	Reason      string             `json:"reason"`
	RefundCents int64              `json:"refundCents"`
	Status      core.DisputeStatus `json:"status"`
	Resolution  string             `json:"resolution,omitempty"`
	ResolvedBy  string             `json:"resolvedBy,omitempty"`
	ResolvedAt  *time.Time         `json:"resolvedAt,omitempty"`
	CreatedAt   time.Time          `json:"createdAt"`
}

type CreateMilestoneInput struct {
	ID             string
	Title          string
	BasePriceCents int64
	BudgetCents    int64
}

type CreateOrderInput struct {
	BuyerOrgID    string
	ProviderOrgID string
	Title         string
	FundingMode   core.FundingMode
	CreditLineID  string
	Milestones    []CreateMilestoneInput
}

type CreateRFQInput struct {
	BuyerOrgID         string
	Title              string
	Category           string
	Scope              string
	BudgetCents        int64
	ResponseDeadlineAt time.Time
}

type SettleMilestoneInput = core.SettleMilestoneInput
type RecordUsageChargeInput = core.RecordUsageChargeInput
type OpenDisputeInput = core.OpenDisputeInput

type ResolveDisputeInput struct {
	Resolution string
	ResolvedBy string
}

type OrderRepository interface {
	NextID() (string, error)
	Save(order *core.Order) error
	Get(id string) (*core.Order, error)
	List() ([]*core.Order, error)
}

type ProviderRepository interface {
	List() ([]ProviderProfile, error)
}

type ListingRepository interface {
	List() ([]Listing, error)
}

type RFQRepository interface {
	NextID() (string, error)
	Get(id string) (RFQ, error)
	Save(rfq RFQ) error
	List() ([]RFQ, error)
}

type BidRepository interface {
	NextID() (string, error)
	Get(id string) (Bid, error)
	Save(bid Bid) error
	ListByRFQ(rfqID string) ([]Bid, error)
}

type MessageRepository interface {
	NextID() (string, error)
	Save(message Message) error
}

type DisputeRepository interface {
	NextID() (string, error)
	Get(id string) (Dispute, error)
	Save(dispute Dispute) error
	List() ([]Dispute, error)
}

type App struct {
	orders       OrderRepository
	providers    ProviderRepository
	listings     ListingRepository
	rfqs         RFQRepository
	bids         BidRepository
	messages     MessageRepository
	disputes     DisputeRepository
	creditEngine core.CreditDecisionEngine
	publisher    EventPublisher
}

type EventPublisher interface {
	Publish(subject string, payload any) error
}

func DefaultProviderProfiles() []ProviderProfile {
	return []ProviderProfile{
		{
			ID:             "provider_1",
			Name:           "Atlas Ops",
			Capabilities:   []string{"carrier", "diagnostics", "token_metering"},
			ReputationTier: "gold",
		},
	}
}

func DefaultListings() []Listing {
	return []Listing{
		{
			ID:             "listing_1",
			ProviderOrgID:  "provider_1",
			Title:          "Managed Agent Operations",
			Category:       "agent-ops",
			BasePriceCents: 1500,
			Tags:           []string{"carrier-compatible", "milestone-ready"},
		},
	}
}

func NewAppWithMemory() *App {
	memory := newMemoryStores()
	return &App{
		orders:    memory.orders,
		providers: memory.providers,
		listings:  memory.listings,
		rfqs:      memory.rfqs,
		bids:      memory.bids,
		messages:  memory.messages,
		disputes:  memory.disputes,
		publisher: noopPublisher{},
		creditEngine: core.CreditDecisionEngine{
			BaseLimitCents:        50_000,
			MaxLimitCents:         500_000,
			DisputePenaltyCents:   75_000,
			FailurePenaltyCents:   50_000,
			ConsumptionMultiplier: 2,
		},
	}
}

func NewAppWithStorage(
	orders OrderRepository,
	providers ProviderRepository,
	listings ListingRepository,
	rfqs RFQRepository,
	bids BidRepository,
	messages MessageRepository,
	disputes DisputeRepository,
) *App {
	memory := newMemoryStores()
	if providers == nil {
		providers = memory.providers
	}
	if listings == nil {
		listings = memory.listings
	}
	if rfqs == nil {
		rfqs = memory.rfqs
	}
	if bids == nil {
		bids = memory.bids
	}
	return NewApp(orders, providers, listings, rfqs, bids, messages, disputes)
}

func NewApp(
	orders OrderRepository,
	providers ProviderRepository,
	listings ListingRepository,
	rfqs RFQRepository,
	bids BidRepository,
	messages MessageRepository,
	disputes DisputeRepository,
) *App {
	return &App{
		orders:    orders,
		providers: providers,
		listings:  listings,
		rfqs:      rfqs,
		bids:      bids,
		messages:  messages,
		disputes:  disputes,
		publisher: noopPublisher{},
		creditEngine: core.CreditDecisionEngine{
			BaseLimitCents:        50_000,
			MaxLimitCents:         500_000,
			DisputePenaltyCents:   75_000,
			FailurePenaltyCents:   50_000,
			ConsumptionMultiplier: 2,
		},
	}
}

func (a *App) SetPublisher(publisher EventPublisher) {
	if publisher == nil {
		a.publisher = noopPublisher{}
		return
	}
	a.publisher = publisher
}

func (a *App) ListProviders() ([]ProviderProfile, error) {
	return a.providers.List()
}

func (a *App) ListListings() ([]Listing, error) {
	return a.listings.List()
}

func (a *App) ListRFQs() ([]RFQ, error) {
	return a.rfqs.List()
}

func (a *App) GetRFQ(id string) (RFQ, error) {
	return a.rfqs.Get(id)
}

func (a *App) ListOrders() ([]*core.Order, error) {
	return a.orders.List()
}

func (a *App) ListDisputes() ([]Dispute, error) {
	return a.disputes.List()
}

func (a *App) GetOrder(id string) (*core.Order, error) {
	return a.orders.Get(id)
}

func (a *App) CreateRFQ(input CreateRFQInput) (RFQ, error) {
	if input.BuyerOrgID == "" || input.Title == "" || input.Category == "" || input.Scope == "" || input.BudgetCents <= 0 {
		return RFQ{}, errors.New("missing required fields")
	}
	if input.ResponseDeadlineAt.IsZero() {
		return RFQ{}, errors.New("response deadline is required")
	}

	rfqID, err := a.rfqs.NextID()
	if err != nil {
		return RFQ{}, err
	}

	now := time.Now().UTC()
	rfq := RFQ{
		ID:                 rfqID,
		BuyerOrgID:         input.BuyerOrgID,
		Title:              input.Title,
		Category:           input.Category,
		Scope:              input.Scope,
		BudgetCents:        input.BudgetCents,
		Status:             RFQStatusOpen,
		ResponseDeadlineAt: input.ResponseDeadlineAt.UTC(),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := a.rfqs.Save(rfq); err != nil {
		return RFQ{}, err
	}

	if err := a.publish("market.rfq.created", map[string]any{
		"rfqId":              rfq.ID,
		"buyerOrgId":         rfq.BuyerOrgID,
		"category":           rfq.Category,
		"budgetCents":        rfq.BudgetCents,
		"responseDeadlineAt": rfq.ResponseDeadlineAt,
	}); err != nil {
		return RFQ{}, err
	}

	return rfq, nil
}

func (a *App) CreateOrder(input CreateOrderInput) (*core.Order, error) {
	if input.BuyerOrgID == "" || input.ProviderOrgID == "" || len(input.Milestones) == 0 {
		return nil, errors.New("missing required fields")
	}

	orderID, err := a.orders.NextID()
	if err != nil {
		return nil, err
	}

	order := &core.Order{
		ID:             orderID,
		BuyerOrgID:     input.BuyerOrgID,
		ProviderOrgID:  input.ProviderOrgID,
		FundingMode:    input.FundingMode,
		CreditLineID:   input.CreditLineID,
		PlatformWallet: "platform_main",
		Status:         core.OrderStatusRunning,
		Milestones:     make([]core.Milestone, 0, len(input.Milestones)),
	}

	for index, milestone := range input.Milestones {
		state := core.MilestoneStatePending
		if index == 0 {
			state = core.MilestoneStateRunning
		}

		order.Milestones = append(order.Milestones, core.Milestone{
			ID:             milestone.ID,
			Title:          milestone.Title,
			BasePriceCents: milestone.BasePriceCents,
			BudgetCents:    milestone.BudgetCents,
			State:          state,
			DisputeStatus:  core.DisputeStatusNone,
		})
	}

	if err := a.orders.Save(order); err != nil {
		return nil, err
	}

	if err := a.publish("market.order.created", map[string]any{
		"orderId":       order.ID,
		"buyerOrgId":    order.BuyerOrgID,
		"providerOrgId": order.ProviderOrgID,
		"fundingMode":   order.FundingMode,
	}); err != nil {
		return nil, err
	}

	return a.orders.Get(order.ID)
}

func (a *App) SettleMilestone(orderID string, input SettleMilestoneInput) (*core.Order, core.LedgerEntry, error) {
	order, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.LedgerEntry{}, err
	}

	entry, err := order.SettleMilestone(input)
	if err != nil {
		return nil, core.LedgerEntry{}, err
	}

	advanceNextMilestone(order, input.MilestoneID)
	if err := a.orders.Save(order); err != nil {
		return nil, core.LedgerEntry{}, err
	}

	if err := a.publish("market.milestone.settled", map[string]any{
		"orderId":     order.ID,
		"milestoneId": input.MilestoneID,
		"ledgerKind":  entry.Kind,
		"amountCents": entry.AmountCents,
		"occurredAt":  input.OccurredAt,
		"fundingMode": order.FundingMode,
	}); err != nil {
		return nil, core.LedgerEntry{}, err
	}

	updated, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.LedgerEntry{}, err
	}

	return updated, entry, nil
}

func (a *App) RecordUsageCharge(orderID string, input RecordUsageChargeInput) (*core.Order, core.UsageCharge, error) {
	order, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}

	charge, err := order.RecordUsageCharge(input)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}

	if err := a.orders.Save(order); err != nil {
		return nil, core.UsageCharge{}, err
	}

	if err := a.publish("market.usage.recorded", map[string]any{
		"orderId":     order.ID,
		"milestoneId": input.MilestoneID,
		"kind":        input.Kind,
		"amountCents": input.AmountCents,
		"proofRef":    input.ProofRef,
		"orderStatus": order.Status,
	}); err != nil {
		return nil, core.UsageCharge{}, err
	}

	updated, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}

	return updated, charge, nil
}

func (a *App) OpenDispute(orderID string, input OpenDisputeInput) (*core.Order, core.LedgerEntry, core.LedgerEntry, error) {
	order, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	refund, recovery, err := order.OpenDispute(input)
	if err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	disputeID, err := a.disputes.NextID()
	if err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	if err := a.disputes.Save(Dispute{
		ID:          disputeID,
		OrderID:     orderID,
		MilestoneID: input.MilestoneID,
		Reason:      input.Reason,
		RefundCents: input.RefundCents,
		Status:      core.DisputeStatusOpen,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	if err := a.orders.Save(order); err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	if err := a.publish("market.dispute.opened", map[string]any{
		"orderId":     order.ID,
		"milestoneId": input.MilestoneID,
		"reason":      input.Reason,
		"refundCents": input.RefundCents,
	}); err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	updated, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	return updated, refund, recovery, nil
}

func (a *App) ResolveDispute(disputeID string, input ResolveDisputeInput) (Dispute, *core.Order, error) {
	dispute, err := a.disputes.Get(disputeID)
	if err != nil {
		return Dispute{}, nil, err
	}

	order, err := a.orders.Get(dispute.OrderID)
	if err != nil {
		return Dispute{}, nil, err
	}

	if err := order.ResolveDispute(core.ResolveDisputeInput{
		MilestoneID: dispute.MilestoneID,
	}); err != nil {
		return Dispute{}, nil, err
	}

	resolvedAt := time.Now().UTC()
	dispute.Status = core.DisputeStatusResolved
	dispute.Resolution = input.Resolution
	dispute.ResolvedBy = input.ResolvedBy
	dispute.ResolvedAt = &resolvedAt

	if err := a.disputes.Save(dispute); err != nil {
		return Dispute{}, nil, err
	}

	if err := a.orders.Save(order); err != nil {
		return Dispute{}, nil, err
	}

	if err := a.publish("market.dispute.resolved", map[string]any{
		"disputeId":   dispute.ID,
		"orderId":     dispute.OrderID,
		"milestoneId": dispute.MilestoneID,
		"resolvedBy":  dispute.ResolvedBy,
		"resolution":  dispute.Resolution,
	}); err != nil {
		return Dispute{}, nil, err
	}

	resolvedDispute, err := a.disputes.Get(disputeID)
	if err != nil {
		return Dispute{}, nil, err
	}

	updated, err := a.orders.Get(dispute.OrderID)
	if err != nil {
		return Dispute{}, nil, err
	}

	return resolvedDispute, updated, nil
}

func (a *App) CreateMessage(orderID, author, body string) (Message, error) {
	messageID, err := a.messages.NextID()
	if err != nil {
		return Message{}, err
	}

	message := Message{
		ID:        messageID,
		OrderID:   orderID,
		Author:    author,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}

	if err := a.messages.Save(message); err != nil {
		return Message{}, err
	}

	if err := a.publish("market.message.created", map[string]any{
		"messageId": message.ID,
		"orderId":   message.OrderID,
		"author":    message.Author,
	}); err != nil {
		return Message{}, err
	}

	return message, nil
}

func (a *App) DecideCredit(history core.CreditHistory) core.CreditDecision {
	return a.creditEngine.Decide(history)
}

func advanceNextMilestone(order *core.Order, settledMilestoneID string) {
	foundSettled := false
	for index := range order.Milestones {
		milestone := &order.Milestones[index]
		if milestone.ID == settledMilestoneID {
			foundSettled = true
			continue
		}

		if foundSettled && milestone.State == core.MilestoneStatePending {
			milestone.State = core.MilestoneStateRunning
			return
		}
	}
}

type memoryStores struct {
	orders    *memoryOrderRepository
	providers *memoryProviderRepository
	listings  *memoryListingRepository
	rfqs      *memoryRFQRepository
	bids      *memoryBidRepository
	messages  *memoryMessageRepository
	disputes  *memoryDisputeRepository
}

func newMemoryStores() *memoryStores {
	return &memoryStores{
		orders: &memoryOrderRepository{
			data: map[string]*core.Order{},
		},
		providers: &memoryProviderRepository{
			data: DefaultProviderProfiles(),
		},
		listings: &memoryListingRepository{
			data: DefaultListings(),
		},
		rfqs:     &memoryRFQRepository{},
		bids:     &memoryBidRepository{},
		messages: &memoryMessageRepository{},
		disputes: &memoryDisputeRepository{},
	}
}

type memoryOrderRepository struct {
	mu   sync.RWMutex
	seq  int
	data map[string]*core.Order
}

func (r *memoryOrderRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("ord_%d", r.seq), nil
}

func (r *memoryOrderRepository) Save(order *core.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[order.ID] = cloneOrder(order)
	return nil
}

func (r *memoryOrderRepository) Get(id string) (*core.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	order, ok := r.data[id]
	if !ok {
		return nil, errors.New("order not found")
	}
	return cloneOrder(order), nil
}

func (r *memoryOrderRepository) List() ([]*core.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	orders := make([]*core.Order, 0, len(r.data))
	for _, order := range r.data {
		orders = append(orders, cloneOrder(order))
	}
	slices.SortFunc(orders, func(a, b *core.Order) int {
		return compareStrings(a.ID, b.ID)
	})
	return orders, nil
}

type memoryProviderRepository struct {
	data []ProviderProfile
}

func (r *memoryProviderRepository) List() ([]ProviderProfile, error) {
	return slices.Clone(r.data), nil
}

type memoryListingRepository struct {
	data []Listing
}

func (r *memoryListingRepository) List() ([]Listing, error) {
	return slices.Clone(r.data), nil
}

type memoryRFQRepository struct {
	mu   sync.Mutex
	seq  int
	data []RFQ
}

func (r *memoryRFQRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("rfq_%d", r.seq), nil
}

func (r *memoryRFQRepository) Get(id string) (RFQ, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rfq := range r.data {
		if rfq.ID == id {
			return rfq, nil
		}
	}
	return RFQ{}, errors.New("rfq not found")
}

func (r *memoryRFQRepository) Save(rfq RFQ) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range r.data {
		if r.data[index].ID == rfq.ID {
			r.data[index] = rfq
			return nil
		}
	}
	r.data = append(r.data, rfq)
	return nil
}

func (r *memoryRFQRepository) List() ([]RFQ, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rfqs := slices.Clone(r.data)
	slices.SortFunc(rfqs, func(a, b RFQ) int {
		return compareStrings(a.ID, b.ID)
	})
	return rfqs, nil
}

type memoryBidRepository struct {
	mu   sync.Mutex
	seq  int
	data []Bid
}

func (r *memoryBidRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("bid_%d", r.seq), nil
}

func (r *memoryBidRepository) Get(id string) (Bid, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, bid := range r.data {
		if bid.ID == id {
			return cloneBid(bid), nil
		}
	}
	return Bid{}, errors.New("bid not found")
}

func (r *memoryBidRepository) Save(bid Bid) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range r.data {
		if r.data[index].ID == bid.ID {
			r.data[index] = cloneBid(bid)
			return nil
		}
	}
	r.data = append(r.data, cloneBid(bid))
	return nil
}

func (r *memoryBidRepository) ListByRFQ(rfqID string) ([]Bid, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bids := make([]Bid, 0)
	for _, bid := range r.data {
		if bid.RFQID != rfqID {
			continue
		}
		bids = append(bids, cloneBid(bid))
	}
	slices.SortFunc(bids, func(a, b Bid) int {
		return compareStrings(a.ID, b.ID)
	})
	return bids, nil
}

type memoryMessageRepository struct {
	mu   sync.Mutex
	seq  int
	data []Message
}

func (r *memoryMessageRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("msg_%d", r.seq), nil
}

func (r *memoryMessageRepository) Save(message Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, message)
	return nil
}

type memoryDisputeRepository struct {
	mu   sync.Mutex
	seq  int
	data []Dispute
}

func (r *memoryDisputeRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("disp_%d", r.seq), nil
}

func (r *memoryDisputeRepository) Get(id string) (Dispute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, dispute := range r.data {
		if dispute.ID == id {
			return dispute, nil
		}
	}
	return Dispute{}, errors.New("dispute not found")
}

func (r *memoryDisputeRepository) Save(dispute Dispute) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, candidate := range r.data {
		if candidate.ID == dispute.ID {
			r.data[i] = dispute
			return nil
		}
	}
	r.data = append(r.data, dispute)
	return nil
}

func (r *memoryDisputeRepository) List() ([]Dispute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	disputes := slices.Clone(r.data)
	slices.SortFunc(disputes, func(a, b Dispute) int {
		return compareStrings(a.ID, b.ID)
	})
	return disputes, nil
}

func cloneOrder(order *core.Order) *core.Order {
	if order == nil {
		return nil
	}

	cloned := *order
	cloned.Milestones = make([]core.Milestone, len(order.Milestones))
	for i, milestone := range order.Milestones {
		clonedMilestone := milestone
		clonedMilestone.UsageCharges = slices.Clone(milestone.UsageCharges)
		cloned.Milestones[i] = clonedMilestone
	}

	return &cloned
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (a *App) publish(subject string, payload any) error {
	return a.publisher.Publish(subject, payload)
}

type noopPublisher struct{}

func (noopPublisher) Publish(string, any) error {
	return nil
}
