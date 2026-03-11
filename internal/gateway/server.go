package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
)

type Server struct {
	app *platform.App
}

func NewServer() *Server {
	return NewServerWithApp(platform.NewAppWithMemory())
}

func NewServerWithApp(app *platform.App) *Server {
	return &Server{app: app}
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
	providers, err := s.app.ListProviders()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (s *Server) handleListListings(w http.ResponseWriter) {
	listings, err := s.app.ListListings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"listings": listings})
}

func (s *Server) handleListOrders(w http.ResponseWriter) {
	orders, err := s.app.ListOrders()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	order, err := s.app.GetOrder(orderID)
	if err != nil {
		if err.Error() == "order not found" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

	input := platform.CreateOrderInput{
		BuyerOrgID:    payload.BuyerOrgID,
		ProviderOrgID: payload.ProviderOrgID,
		Title:         payload.Title,
		FundingMode:   core.FundingMode(payload.FundingMode),
		CreditLineID:  payload.CreditLineID,
		Milestones:    make([]platform.CreateMilestoneInput, 0, len(payload.Milestones)),
	}
	for _, milestone := range payload.Milestones {
		input.Milestones = append(input.Milestones, platform.CreateMilestoneInput{
			ID:             milestone.ID,
			Title:          milestone.Title,
			BasePriceCents: milestone.BasePriceCents,
			BudgetCents:    milestone.BudgetCents,
		})
	}

	order, err := s.app.CreateOrder(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

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

	order, entry, err := s.app.SettleMilestone(orderID, platform.SettleMilestoneInput{
		MilestoneID: milestoneID,
		Summary:     payload.Summary,
		Source:      payload.Source,
		OccurredAt:  time.Now().UTC(),
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}

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

	order, charge, err := s.app.RecordUsageCharge(orderID, platform.RecordUsageChargeInput{
		MilestoneID: milestoneID,
		Kind:        payload.Kind,
		AmountCents: payload.AmountCents,
		ProofRef:    payload.ProofRef,
	})
	if err != nil {
		writeGatewayError(w, err)
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

	order, refund, recovery, err := s.app.OpenDispute(orderID, platform.OpenDisputeInput{
		MilestoneID: payload.MilestoneID,
		Reason:      payload.Reason,
		RefundCents: payload.RefundCents,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"order": order, "refundEntry": refund, "recoveryEntry": recovery})
}

func (s *Server) handleCreditDecision(w http.ResponseWriter, r *http.Request) {
	var history core.CreditHistory
	if err := json.NewDecoder(r.Body).Decode(&history); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"decision": s.app.DecideCredit(history)})
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

	message, err := s.app.CreateMessage(payload.OrderID, payload.Author, payload.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"message": message})
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

func writeGatewayError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "order not found":
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	}
}
