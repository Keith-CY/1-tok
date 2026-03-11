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

type Message struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type Dispute struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"orderId"`
	MilestoneID string    `json:"milestoneId"`
	Reason      string    `json:"reason"`
	RefundCents int64     `json:"refundCents"`
	CreatedAt   time.Time `json:"createdAt"`
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

type SettleMilestoneInput = core.SettleMilestoneInput
type RecordUsageChargeInput = core.RecordUsageChargeInput
type OpenDisputeInput = core.OpenDisputeInput

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

type MessageRepository interface {
	NextID() (string, error)
	Save(message Message) error
}

type DisputeRepository interface {
	NextID() (string, error)
	Save(dispute Dispute) error
}

type App struct {
	orders       OrderRepository
	providers    ProviderRepository
	listings     ListingRepository
	messages     MessageRepository
	disputes     DisputeRepository
	creditEngine core.CreditDecisionEngine
}

func NewAppWithMemory() *App {
	memory := newMemoryStores()
	return &App{
		orders:    memory.orders,
		providers: memory.providers,
		listings:  memory.listings,
		messages:  memory.messages,
		disputes:  memory.disputes,
		creditEngine: core.CreditDecisionEngine{
			BaseLimitCents:        50_000,
			MaxLimitCents:         500_000,
			DisputePenaltyCents:   75_000,
			FailurePenaltyCents:   50_000,
			ConsumptionMultiplier: 2,
		},
	}
}

func NewAppWithStorage(orders OrderRepository, messages MessageRepository, disputes DisputeRepository) *App {
	memory := newMemoryStores()
	return NewApp(orders, memory.providers, memory.listings, messages, disputes)
}

func NewApp(
	orders OrderRepository,
	providers ProviderRepository,
	listings ListingRepository,
	messages MessageRepository,
	disputes DisputeRepository,
) *App {
	return &App{
		orders:    orders,
		providers: providers,
		listings:  listings,
		messages:  messages,
		disputes:  disputes,
		creditEngine: core.CreditDecisionEngine{
			BaseLimitCents:        50_000,
			MaxLimitCents:         500_000,
			DisputePenaltyCents:   75_000,
			FailurePenaltyCents:   50_000,
			ConsumptionMultiplier: 2,
		},
	}
}

func (a *App) ListProviders() ([]ProviderProfile, error) {
	return a.providers.List()
}

func (a *App) ListListings() ([]Listing, error) {
	return a.listings.List()
}

func (a *App) ListOrders() ([]*core.Order, error) {
	return a.orders.List()
}

func (a *App) GetOrder(id string) (*core.Order, error) {
	return a.orders.Get(id)
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
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	if err := a.orders.Save(order); err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	updated, err := a.orders.Get(orderID)
	if err != nil {
		return nil, core.LedgerEntry{}, core.LedgerEntry{}, err
	}

	return updated, refund, recovery, nil
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
	messages  *memoryMessageRepository
	disputes  *memoryDisputeRepository
}

func newMemoryStores() *memoryStores {
	return &memoryStores{
		orders: &memoryOrderRepository{
			data: map[string]*core.Order{},
		},
		providers: &memoryProviderRepository{
			data: []ProviderProfile{
				{
					ID:             "provider_1",
					Name:           "Atlas Ops",
					Capabilities:   []string{"carrier", "diagnostics", "token_metering"},
					ReputationTier: "gold",
				},
			},
		},
		listings: &memoryListingRepository{
			data: []Listing{
				{
					ID:             "listing_1",
					ProviderOrgID:  "provider_1",
					Title:          "Managed Agent Operations",
					Category:       "agent-ops",
					BasePriceCents: 1500,
					Tags:           []string{"carrier-compatible", "milestone-ready"},
				},
			},
		},
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

func (r *memoryDisputeRepository) Save(dispute Dispute) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, dispute)
	return nil
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
