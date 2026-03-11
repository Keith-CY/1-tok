package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

type Store struct {
	mu         sync.RWMutex
	orderSeq   int
	messageSeq int
	disputeSeq int
	Orders     map[string]*core.Order
	Providers  []ProviderProfile
	Listings   []Listing
	Messages   []Message
	Disputes   []Dispute
}

type Server struct {
	store        *Store
	creditEngine core.CreditDecisionEngine
}

func NewServer() *Server {
	return &Server{
		store: &Store{
			Orders: map[string]*core.Order{},
			Providers: []ProviderProfile{
				{
					ID:             "provider_1",
					Name:           "Atlas Ops",
					Capabilities:   []string{"carrier", "diagnostics", "token_metering"},
					ReputationTier: "gold",
				},
			},
			Listings: []Listing{
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
		creditEngine: core.CreditDecisionEngine{
			BaseLimitCents:        50_000,
			MaxLimitCents:         500_000,
			DisputePenaltyCents:   75_000,
			FailurePenaltyCents:   50_000,
			ConsumptionMultiplier: 2,
		},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/providers":
		s.handleListProviders(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/listings":
		s.handleListListings(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orders":
		s.handleListOrders(w)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/orders/"):
		s.handleGetOrder(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
		s.handleCreateOrder(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/settle"):
		s.handleSettleMilestone(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/usage"):
		s.handleRecordUsage(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/disputes"):
		s.handleCreateDispute(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/credits/decision":
		s.handleCreditDecision(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/messages":
		s.handleCreateMessage(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
	}
}

func (s *Server) handleListProviders(w http.ResponseWriter) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{"providers": s.store.Providers})
}

func (s *Server) handleListListings(w http.ResponseWriter) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{"listings": s.store.Listings})
}

func (s *Server) handleListOrders(w http.ResponseWriter) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	orders := make([]*core.Order, 0, len(s.store.Orders))
	for _, order := range s.store.Orders {
		orders = append(orders, order)
	}

	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	order, ok := s.store.Orders[orderID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"order": order})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BuyerOrgID    string `json:"buyerOrgId"`
		ProviderOrgID string `json:"providerOrgId"`
		Title         string `json:"title"`
		FundingMode   string `json:"fundingMode"`
		CreditLineID  string `json:"creditLineId"`
		Milestones    []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			BasePriceCents int64  `json:"basePriceCents"`
			BudgetCents    int64  `json:"budgetCents"`
		} `json:"milestones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if payload.BuyerOrgID == "" || payload.ProviderOrgID == "" || len(payload.Milestones) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	s.store.orderSeq++
	orderID := fmt.Sprintf("ord_%d", s.store.orderSeq)
	order := &core.Order{
		ID:             orderID,
		BuyerOrgID:     payload.BuyerOrgID,
		ProviderOrgID:  payload.ProviderOrgID,
		FundingMode:    core.FundingMode(payload.FundingMode),
		CreditLineID:   payload.CreditLineID,
		PlatformWallet: "platform_main",
		Status:         core.OrderStatusRunning,
		Milestones:     make([]core.Milestone, 0, len(payload.Milestones)),
	}
	for index, milestone := range payload.Milestones {
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

	s.store.Orders[orderID] = order
	writeJSON(w, http.StatusCreated, map[string]any{"order": order})
}

func (s *Server) handleSettleMilestone(w http.ResponseWriter, r *http.Request) {
	orderID, milestoneID, err := orderMilestoneFromSettlePath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		MilestoneID string `json:"milestoneId"`
		Summary     string `json:"summary"`
		Source      string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	order, ok := s.store.Orders[orderID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}

	entry, err := order.SettleMilestone(core.SettleMilestoneInput{
		MilestoneID: milestoneID,
		Summary:     payload.Summary,
		Source:      payload.Source,
		OccurredAt:  time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	s.advanceNextMilestone(order, milestoneID)
	writeJSON(w, http.StatusOK, map[string]any{"order": order, "ledgerEntry": entry})
}

func (s *Server) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	orderID, milestoneID, err := orderMilestoneFromUsagePath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		Kind        core.UsageChargeKind `json:"kind"`
		AmountCents int64                `json:"amountCents"`
		ProofRef    string               `json:"proofRef"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	order, ok := s.store.Orders[orderID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}

	charge, err := order.RecordUsageCharge(core.RecordUsageChargeInput{
		MilestoneID: milestoneID,
		Kind:        payload.Kind,
		AmountCents: payload.AmountCents,
		ProofRef:    payload.ProofRef,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"order": order, "usageCharge": charge})
}

func (s *Server) handleCreateDispute(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromDisputePath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		MilestoneID string `json:"milestoneId"`
		Reason      string `json:"reason"`
		RefundCents int64  `json:"refundCents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	order, ok := s.store.Orders[orderID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}

	refund, recovery, err := order.OpenDispute(core.OpenDisputeInput{
		MilestoneID: payload.MilestoneID,
		Reason:      payload.Reason,
		RefundCents: payload.RefundCents,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	s.store.disputeSeq++
	s.store.Disputes = append(s.store.Disputes, Dispute{
		ID:          fmt.Sprintf("disp_%d", s.store.disputeSeq),
		OrderID:     orderID,
		MilestoneID: payload.MilestoneID,
		Reason:      payload.Reason,
		RefundCents: payload.RefundCents,
		CreatedAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"order":         order,
		"refundEntry":   refund,
		"recoveryEntry": recovery,
	})
}

func (s *Server) handleCreditDecision(w http.ResponseWriter, r *http.Request) {
	var history core.CreditHistory
	if err := json.NewDecoder(r.Body).Decode(&history); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"decision": s.creditEngine.Decide(history)})
}

func (s *Server) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		OrderID string `json:"orderId"`
		Author  string `json:"author"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	s.store.messageSeq++
	message := Message{
		ID:        fmt.Sprintf("msg_%d", s.store.messageSeq),
		OrderID:   payload.OrderID,
		Author:    payload.Author,
		Body:      payload.Body,
		CreatedAt: time.Now().UTC(),
	}
	s.store.Messages = append(s.store.Messages, message)

	writeJSON(w, http.StatusCreated, map[string]any{"message": message})
}

func (s *Server) advanceNextMilestone(order *core.Order, settledMilestoneID string) {
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

func orderIDFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 {
		return "", errors.New("invalid order path")
	}
	return parts[3], nil
}

func orderMilestoneFromSettlePath(path string) (string, string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 7 || parts[4] != "milestones" || parts[6] != "settle" {
		return "", "", errors.New("invalid settlement path")
	}
	return parts[3], parts[5], nil
}

func orderMilestoneFromUsagePath(path string) (string, string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 7 || parts[4] != "milestones" || parts[6] != "usage" {
		return "", "", errors.New("invalid usage path")
	}
	return parts[3], parts[5], nil
}

func orderIDFromDisputePath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[4] != "disputes" {
		return "", errors.New("invalid dispute path")
	}
	return parts[3], nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
