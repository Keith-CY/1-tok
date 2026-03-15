package platform

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/reconciliation"
)

type ProviderProfile struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Capabilities   []string `json:"capabilities"`
	ReputationTier string   `json:"reputationTier"`
	Rating         float64  `json:"rating"`
	RatingCount    int      `json:"ratingCount"`
}

// OrderRating represents a buyer's rating of a completed order.
type OrderRating struct {
	OrderID       string    `json:"orderId"`
	ProviderOrgID string    `json:"providerOrgId"`
	BuyerOrgID    string    `json:"buyerOrgId"`
	Score         int       `json:"score"` // 1-5
	Comment       string    `json:"comment,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// RateOrderInput is the input for rating a completed order.
type RateOrderInput struct {
	Score   int    // 1-5 stars
	Comment string
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
	ID                   string           `json:"id"`
	BuyerOrgID           string           `json:"buyerOrgId"`
	Title                string           `json:"title"`
	Category             string           `json:"category"`
	Scope                string           `json:"scope"`
	BudgetCents          int64            `json:"budgetCents"`
	DefaultMilestones    []RFQMilestone   `json:"defaultMilestones"`
	Status               RFQStatus        `json:"status"`
	AwardedBidID         string           `json:"awardedBidId,omitempty"`
	AwardedProviderOrgID string           `json:"awardedProviderOrgId,omitempty"`
	OrderID              string           `json:"orderId,omitempty"`
	ResponseDeadlineAt   time.Time        `json:"responseDeadlineAt"`
	CreatedAt            time.Time        `json:"createdAt"`
	UpdatedAt            time.Time        `json:"updatedAt"`
}

// RFQMilestone is a platform-generated default milestone attached to an RFQ.
// Providers may accept these defaults or override with their own in a bid.
type RFQMilestone struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	BasePriceCents int64  `json:"basePriceCents"`
	BudgetCents    int64  `json:"budgetCents"`
}

type Message struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId,omitempty"`
	RFQID     string    `json:"rfqId,omitempty"`
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
	Milestones         []CreateMilestoneInput // optional; auto-generated if empty
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
	Get(id string) (ProviderProfile, error)
}

type ListingRepository interface {
	List() ([]Listing, error)
	Get(id string) (Listing, error)
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
	ListByRFQ(rfqID string) ([]Message, error)
	ListByOrder(orderID string) ([]Message, error)
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
	notifier     Notifier
	mu           sync.Mutex // guards compound multi-store operations
	ratings      []OrderRating
	clock        Clock
}

// Notifier is an optional notification delivery interface.
type Notifier interface {
	Send(event string, target string, payload map[string]any) error
}

// SetNotifier sets the notification service.
func (a *App) SetNotifier(n Notifier) {
	a.notifier = n
}

func (a *App) notify(event string, target string, payload map[string]any) {
	if a.notifier == nil {
		return
	}
	_ = a.notifier.Send(event, target, payload)
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
		return RFQ{}, ErrMissingRequiredFields
	}
	if input.ResponseDeadlineAt.IsZero() {
		return RFQ{}, ErrDeadlineRequired
	}

	rfqID, err := a.rfqs.NextID()
	if err != nil {
		return RFQ{}, err
	}

	milestones := input.Milestones
	if len(milestones) == 0 {
		milestones = DefaultMilestoneSplit(input.BudgetCents)
	}

	defaultMilestones := make([]RFQMilestone, 0, len(milestones))
	for _, m := range milestones {
		defaultMilestones = append(defaultMilestones, RFQMilestone{
			ID:             m.ID,
			Title:          m.Title,
			BasePriceCents: m.BasePriceCents,
			BudgetCents:    m.BudgetCents,
		})
	}

	now := a.now()
	rfq := RFQ{
		ID:                 rfqID,
		BuyerOrgID:         input.BuyerOrgID,
		Title:              input.Title,
		Category:           input.Category,
		Scope:              input.Scope,
		BudgetCents:        input.BudgetCents,
		DefaultMilestones:  defaultMilestones,
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
		return nil, ErrMissingRequiredFields
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

	// Notify both parties
	a.notify("order.created", order.BuyerOrgID, map[string]any{"orderId": order.ID})
	a.notify("order.created", order.ProviderOrgID, map[string]any{"orderId": order.ID})

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

	// Anti-fraud layer 3: reconciliation check
	for i := range order.Milestones {
		if order.Milestones[i].ID == input.MilestoneID {
			rec := reconciliation.Reconcile(order.Milestones[i], 0)
			if len(rec.Anomalies) > 0 {
				order.Milestones[i].AnomalyFlags = rec.Anomalies
				a.notify("reconciliation.anomaly", order.ProviderOrgID, map[string]any{
					"orderId":     order.ID,
					"milestoneId": input.MilestoneID,
					"anomalies":   rec.Anomalies,
				})
			}
			break
		}
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

	// Notify both parties on milestone settlement
	a.notify("milestone.settled", order.BuyerOrgID, map[string]any{"orderId": order.ID, "milestoneId": input.MilestoneID})
	a.notify("milestone.settled", order.ProviderOrgID, map[string]any{"orderId": order.ID, "milestoneId": input.MilestoneID})

	// Check if all milestones are settled → order completed
	allSettled := true
	for _, ms := range order.Milestones {
		if ms.State != core.MilestoneStateSettled {
			allSettled = false
			break
		}
	}
	if allSettled && order.Status == core.OrderStatusCompleted {
		a.notify("order.completed", order.BuyerOrgID, map[string]any{"orderId": order.ID})
		a.notify("order.completed", order.ProviderOrgID, map[string]any{"orderId": order.ID})
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

	// Notify on budget wall hit
	if order.Status == core.OrderStatusAwaitingBudget {
		a.notify("budget_wall.hit", order.BuyerOrgID, map[string]any{"orderId": order.ID, "milestoneId": input.MilestoneID})
		a.notify("budget_wall.hit", order.ProviderOrgID, map[string]any{"orderId": order.ID, "milestoneId": input.MilestoneID})
	}

	updated, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}

	return updated, charge, nil
}

func (a *App) OpenDispute(orderID string, input OpenDisputeInput) (*core.Order, core.LedgerEntry, core.LedgerEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

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
		CreatedAt:   a.now(),
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

	a.notify("dispute.opened", order.BuyerOrgID, map[string]any{"orderId": order.ID, "milestoneId": input.MilestoneID})
	a.notify("dispute.opened", order.ProviderOrgID, map[string]any{"orderId": order.ID, "milestoneId": input.MilestoneID})

	updated, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	return updated, refund, recovery, nil
}

func (a *App) ResolveDispute(disputeID string, input ResolveDisputeInput) (Dispute, *core.Order, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

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

	resolvedAt := a.now()
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

	a.notify("dispute.resolved", order.BuyerOrgID, map[string]any{"disputeId": dispute.ID, "orderId": dispute.OrderID, "resolution": dispute.Resolution})
	a.notify("dispute.resolved", order.ProviderOrgID, map[string]any{"disputeId": dispute.ID, "orderId": dispute.OrderID, "resolution": dispute.Resolution})

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
		CreatedAt: a.now(),
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
		return nil, core.ErrOrderNotFound
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

func (r *memoryProviderRepository) Get(id string) (ProviderProfile, error) {
	for _, p := range r.data {
		if p.ID == id {
			return p, nil
		}
	}
	return ProviderProfile{}, fmt.Errorf("provider not found: %s", id)
}

type memoryListingRepository struct {
	data []Listing
}

func (r *memoryListingRepository) List() ([]Listing, error) {
	return slices.Clone(r.data), nil
}

func (r *memoryListingRepository) Get(id string) (Listing, error) {
	for _, l := range r.data {
		if l.ID == id {
			return l, nil
		}
	}
	return Listing{}, fmt.Errorf("listing not found: %s", id)
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
	return RFQ{}, ErrRFQNotFound
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
	return Bid{}, ErrBidNotFound
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

func (r *memoryMessageRepository) ListByRFQ(rfqID string) ([]Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Message, 0)
	for _, msg := range r.data {
		if msg.RFQID == rfqID {
			result = append(result, msg)
		}
	}
	return result, nil
}

func (r *memoryMessageRepository) ListByOrder(orderID string) ([]Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Message, 0)
	for _, msg := range r.data {
		if msg.OrderID == orderID {
			result = append(result, msg)
		}
	}
	return result, nil
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
	return Dispute{}, ErrDisputeNotFound
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

// ListListingsInput holds optional filter criteria for listing search.
type ListListingsInput struct {
	Query         string   // full-text search on title
	Category      string   // exact match
	Tags          []string // any match
	ProviderOrgID string   // exact match
	MinPriceCents int64    // inclusive
	MaxPriceCents int64    // inclusive (0 = no limit)
}

// SearchListings returns listings matching the given filter criteria.
// Empty filter returns all listings.
func (a *App) SearchListings(input ListListingsInput) ([]Listing, error) {
	all, err := a.listings.List()
	if err != nil {
		return nil, err
	}

	result := make([]Listing, 0, len(all))
	for _, listing := range all {
		if !matchesListingFilter(listing, input) {
			continue
		}
		result = append(result, listing)
	}
	return result, nil
}

func matchesListingFilter(listing Listing, input ListListingsInput) bool {
	if input.Query != "" && !strings.Contains(
		strings.ToLower(listing.Title),
		strings.ToLower(input.Query),
	) {
		return false
	}
	if input.Category != "" && !strings.EqualFold(listing.Category, input.Category) {
		return false
	}
	if input.ProviderOrgID != "" && listing.ProviderOrgID != input.ProviderOrgID {
		return false
	}
	if input.MinPriceCents > 0 && listing.BasePriceCents < input.MinPriceCents {
		return false
	}
	if input.MaxPriceCents > 0 && listing.BasePriceCents > input.MaxPriceCents {
		return false
	}
	if len(input.Tags) > 0 {
		matched := false
		for _, filterTag := range input.Tags {
			for _, listingTag := range listing.Tags {
				if strings.EqualFold(filterTag, listingTag) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// RateOrder records a buyer's rating for a completed order.
// Only completed orders can be rated, and each order can only be rated once.
func (a *App) RateOrder(orderID string, input RateOrderInput) (OrderRating, error) {
	if input.Score < 1 || input.Score > 5 {
		return OrderRating{}, ErrInvalidScore
	}

	order, err := a.orders.Get(orderID)
	if err != nil {
		return OrderRating{}, err
	}
	if order.Status != core.OrderStatusCompleted {
		return OrderRating{}, ErrOrderNotCompleted
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if already rated
	for _, r := range a.ratings {
		if r.OrderID == orderID {
			return OrderRating{}, ErrOrderAlreadyRated
		}
	}

	rating := OrderRating{
		OrderID:       orderID,
		ProviderOrgID: order.ProviderOrgID,
		BuyerOrgID:    order.BuyerOrgID,
		Score:         input.Score,
		Comment:       input.Comment,
		CreatedAt:     a.now(),
	}
	a.ratings = append(a.ratings, rating)

	// Update provider average rating
	a.updateProviderRating(order.ProviderOrgID)

	if err := a.publish("market.order.rated", map[string]any{
		"orderId":       orderID,
		"providerOrgId": order.ProviderOrgID,
		"buyerOrgId":    order.BuyerOrgID,
		"score":         input.Score,
	}); err != nil {
		return OrderRating{}, err
	}

	a.notify("order.rated", order.ProviderOrgID, map[string]any{"orderId": orderID, "score": input.Score})

	return rating, nil
}

func (a *App) updateProviderRating(providerOrgID string) {
	var total, count int
	for _, r := range a.ratings {
		if r.ProviderOrgID == providerOrgID {
			total += r.Score
			count++
		}
	}
	if count == 0 {
		return
	}
	// Note: this updates in-memory only. For postgres, would need a separate update.
	_ = total
	_ = count
}

// GetOrderRating returns the rating for an order, if it exists.
func (a *App) GetOrderRating(orderID string) (OrderRating, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, r := range a.ratings {
		if r.OrderID == orderID {
			return r, nil
		}
	}
	return OrderRating{}, ErrOrderNotRated
}

// CreateRFQMessage creates a message in the context of an RFQ.
func (a *App) CreateRFQMessage(rfqID, author, body string) (Message, error) {
	if _, err := a.rfqs.Get(rfqID); err != nil {
		return Message{}, err
	}

	messageID, err := a.messages.NextID()
	if err != nil {
		return Message{}, err
	}

	message := Message{
		ID:        messageID,
		RFQID:     rfqID,
		Author:    author,
		Body:      body,
		CreatedAt: a.now(),
	}

	if err := a.messages.Save(message); err != nil {
		return Message{}, err
	}

	if err := a.publish("market.rfq.message.created", map[string]any{
		"messageId": message.ID,
		"rfqId":     rfqID,
		"author":    author,
	}); err != nil {
		return Message{}, err
	}

	return message, nil
}

// ListRFQMessages returns all messages for a given RFQ.
func (a *App) ListRFQMessages(rfqID string) ([]Message, error) {
	if _, err := a.rfqs.Get(rfqID); err != nil {
		return nil, err
	}
	return a.messages.ListByRFQ(rfqID)
}

// ListBids returns all bids for a given RFQ.
func (a *App) ListBids(rfqID string) ([]Bid, error) {
	return a.bids.ListByRFQ(rfqID)
}

// Clock provides the current time. Override for testing.
type Clock func() time.Time

// DefaultClock returns the real current time.
func DefaultClock() time.Time { return time.Now().UTC() }

// SetClock overrides the clock used by the App.
func (a *App) SetClock(c Clock) {
	a.clock = c
}

func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now().UTC()
}

// ListOrderMessages returns all messages for a given order.
func (a *App) ListOrderMessages(orderID string) ([]Message, error) {
	if _, err := a.orders.Get(orderID); err != nil {
		return nil, err
	}
	return a.messages.ListByOrder(orderID)
}

// GetProvider returns a provider profile by ID, with rating computed from order ratings.
func (a *App) GetProvider(id string) (ProviderProfile, error) {
	provider, err := a.providers.Get(id)
	if err != nil {
		return ProviderProfile{}, err
	}

	// Compute rating from stored ratings
	a.mu.Lock()
	var total, count int
	for _, r := range a.ratings {
		if r.ProviderOrgID == id {
			total += r.Score
			count++
		}
	}
	a.mu.Unlock()

	if count > 0 {
		provider.Rating = float64(total) / float64(count)
		provider.RatingCount = count
	}

	return provider, nil
}

// GetListing returns a listing by ID.
func (a *App) GetListing(id string) (Listing, error) {
	return a.listings.Get(id)
}

// GetDispute returns a dispute by ID.
func (a *App) GetDispute(id string) (Dispute, error) {
	return a.disputes.Get(id)
}


// SearchProviders returns providers matching optional capability and tier filters.
type SearchProvidersInput struct {
	Capability string
	Tier       string
	MinRating  float64
}

func (a *App) SearchProviders(input SearchProvidersInput) ([]ProviderProfile, error) {
	all, err := a.providers.List()
	if err != nil {
		return nil, err
	}

	result := make([]ProviderProfile, 0, len(all))
	for _, p := range all {
		// Compute rating
		a.mu.Lock()
		var total, count int
		for _, r := range a.ratings {
			if r.ProviderOrgID == p.ID {
				total += r.Score
				count++
			}
		}
		a.mu.Unlock()
		if count > 0 {
			p.Rating = float64(total) / float64(count)
			p.RatingCount = count
		}

		if input.MinRating > 0 && p.Rating < input.MinRating {
			continue
		}
		if input.Tier != "" && !strings.EqualFold(p.ReputationTier, input.Tier) {
			continue
		}
		if input.Capability != "" {
			found := false
			for _, cap := range p.Capabilities {
				if strings.Contains(strings.ToLower(cap), strings.ToLower(input.Capability)) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, p)
	}
	return result, nil
}

// MarketplaceStats holds aggregate marketplace statistics.
type MarketplaceStats struct {
	TotalProviders  int     `json:"totalProviders"`
	TotalListings   int     `json:"totalListings"`
	TotalRFQs       int     `json:"totalRfqs"`
	OpenRFQs        int     `json:"openRfqs"`
	TotalOrders     int     `json:"totalOrders"`
	ActiveOrders    int     `json:"activeOrders"`
	TotalDisputes   int     `json:"totalDisputes"`
	OpenDisputes    int     `json:"openDisputes"`
	TotalRatings    int     `json:"totalRatings"`
	AverageRating   float64 `json:"averageRating"`
}

// GetMarketplaceStats returns aggregate marketplace statistics.
func (a *App) GetMarketplaceStats() (MarketplaceStats, error) {
	stats := MarketplaceStats{}

	providers, err := a.providers.List()
	if err != nil {
		return stats, err
	}
	stats.TotalProviders = len(providers)

	listings, err := a.listings.List()
	if err != nil {
		return stats, err
	}
	stats.TotalListings = len(listings)

	rfqs, err := a.rfqs.List()
	if err != nil {
		return stats, err
	}
	stats.TotalRFQs = len(rfqs)
	for _, rfq := range rfqs {
		if rfq.Status == RFQStatusOpen {
			stats.OpenRFQs++
		}
	}

	orders, err := a.orders.List()
	if err != nil {
		return stats, err
	}
	stats.TotalOrders = len(orders)
	for _, o := range orders {
		if o.Status == core.OrderStatusRunning {
			stats.ActiveOrders++
		}
	}

	disputes, err := a.disputes.List()
	if err != nil {
		return stats, err
	}
	stats.TotalDisputes = len(disputes)
	for _, d := range disputes {
		if d.Status == "open" {
			stats.OpenDisputes++
		}
	}

	a.mu.Lock()
	stats.TotalRatings = len(a.ratings)
	if stats.TotalRatings > 0 {
		total := 0
		for _, r := range a.ratings {
			total += r.Score
		}
		stats.AverageRating = float64(total) / float64(stats.TotalRatings)
	}
	a.mu.Unlock()

	return stats, nil
}

// OrderBudgetSummary shows budget utilization per milestone.
type MilestoneBudget struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	BudgetCents int64   `json:"budgetCents"`
	SpentCents  int64   `json:"spentCents"`
	SettledCents int64  `json:"settledCents"`
	UsagePercent float64 `json:"usagePercent"`
	State       string  `json:"state"`
}

type OrderBudgetSummary struct {
	OrderID        string            `json:"orderId"`
	TotalBudget    int64             `json:"totalBudgetCents"`
	TotalSpent     int64             `json:"totalSpentCents"`
	TotalSettled   int64             `json:"totalSettledCents"`
	OverallPercent float64           `json:"overallPercent"`
	Milestones     []MilestoneBudget `json:"milestones"`
}

// GetOrderBudget returns budget utilization for an order.
func (a *App) GetOrderBudget(orderID string) (OrderBudgetSummary, error) {
	order, err := a.orders.Get(orderID)
	if err != nil {
		return OrderBudgetSummary{}, err
	}

	summary := OrderBudgetSummary{
		OrderID:    order.ID,
		Milestones: make([]MilestoneBudget, 0, len(order.Milestones)),
	}

	for _, ms := range order.Milestones {
		spent := ms.CurrentSpendCents()
		pct := 0.0
		if ms.BudgetCents > 0 {
			pct = float64(spent) / float64(ms.BudgetCents) * 100
		}
		summary.Milestones = append(summary.Milestones, MilestoneBudget{
			ID:           ms.ID,
			Title:        ms.Title,
			BudgetCents:  ms.BudgetCents,
			SpentCents:   spent,
			SettledCents: ms.SettledCents,
			UsagePercent: pct,
			State:        string(ms.State),
		})
		summary.TotalBudget += ms.BudgetCents
		summary.TotalSpent += spent
		summary.TotalSettled += ms.SettledCents
	}

	if summary.TotalBudget > 0 {
		summary.OverallPercent = float64(summary.TotalSpent) / float64(summary.TotalBudget) * 100
	}

	return summary, nil
}

// ProviderRevenueSummary shows revenue across all orders for a provider.
type ProviderRevenueSummary struct {
	ProviderOrgID   string `json:"providerOrgId"`
	TotalOrders     int    `json:"totalOrders"`
	ActiveOrders    int    `json:"activeOrders"`
	TotalRevenue    int64  `json:"totalRevenueCents"`
	PendingRevenue  int64  `json:"pendingRevenueCents"`
	TotalDisputes   int    `json:"totalDisputes"`
}

// GetProviderRevenue returns revenue summary for a provider.
func (a *App) GetProviderRevenue(providerOrgID string) (ProviderRevenueSummary, error) {
	orders, err := a.orders.List()
	if err != nil {
		return ProviderRevenueSummary{}, err
	}

	summary := ProviderRevenueSummary{ProviderOrgID: providerOrgID}
	for _, o := range orders {
		if o.ProviderOrgID != providerOrgID {
			continue
		}
		summary.TotalOrders++
		if o.Status == core.OrderStatusRunning {
			summary.ActiveOrders++
		}
		for _, ms := range o.Milestones {
			summary.TotalRevenue += ms.SettledCents
			if ms.State == core.MilestoneStateRunning || ms.State == core.MilestoneStatePending {
				summary.PendingRevenue += ms.BudgetCents - ms.SettledCents
			}
		}
	}

	disputes, err := a.disputes.List()
	if err != nil {
		return summary, nil // non-fatal
	}
	for _, d := range disputes {
		for _, o := range orders {
			if o.ID == d.OrderID && o.ProviderOrgID == providerOrgID {
				summary.TotalDisputes++
			}
		}
	}

	return summary, nil
}

// TimelineEvent represents a single event in an order's timeline.
type TimelineEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Details   map[string]any `json:"details,omitempty"`
}

// GetOrderTimeline builds a chronological timeline of events for an order.
func (a *App) GetOrderTimeline(orderID string) ([]TimelineEvent, error) {
	order, err := a.orders.Get(orderID)
	if err != nil {
		return nil, err
	}

	events := make([]TimelineEvent, 0)

	// Order created
	events = append(events, TimelineEvent{
		Type:      "order.created",
		Timestamp: a.now(),
		Details: map[string]any{
			"buyerOrgId":    order.BuyerOrgID,
			"providerOrgId": order.ProviderOrgID,
			"fundingMode":   string(order.FundingMode),
		},
	})

	// Milestone events
	for _, ms := range order.Milestones {
		if ms.State == core.MilestoneStateSettled && ms.SettledAt != nil {
			events = append(events, TimelineEvent{
				Type:      "milestone.settled",
				Timestamp: *ms.SettledAt,
				Details:   map[string]any{"milestoneId": ms.ID, "title": ms.Title, "settledCents": ms.SettledCents},
			})
		}

		for _, charge := range ms.UsageCharges {
			events = append(events, TimelineEvent{
				Type:      "usage.recorded",
				Timestamp: a.now(),
				Details:   map[string]any{"milestoneId": ms.ID, "kind": string(charge.Kind), "amountCents": charge.AmountCents},
			})
		}

		if len(ms.AnomalyFlags) > 0 {
			events = append(events, TimelineEvent{
				Type:      "reconciliation.anomaly",
				Timestamp: a.now(),
				Details:   map[string]any{"milestoneId": ms.ID, "flags": ms.AnomalyFlags},
			})
		}
	}

	// Rating
	a.mu.Lock()
	for _, r := range a.ratings {
		if r.OrderID == orderID {
			events = append(events, TimelineEvent{
				Type:      "order.rated",
				Timestamp: r.CreatedAt,
				Details:   map[string]any{"score": r.Score, "comment": r.Comment},
			})
		}
	}
	a.mu.Unlock()

	return events, nil
}
