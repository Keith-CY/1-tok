package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/ratelimit"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

// Sentinel startup errors returned by NewServerWithOptionsE.
var (
	ErrIAMUpstreamRequired      = errors.New("IAM_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
	ErrExecutionTokenRequired   = errors.New("API_GATEWAY_EXECUTION_TOKEN or API_GATEWAY_EXECUTION_TOKENS is required when ONE_TOK_REQUIRE_EXTERNALS=true")
)

type Server struct {
	app             *platform.App
	auth            iamclient.Client
	executionTokens serviceauth.TokenSet
	rateLimiter     ratelimit.Limiter
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		App: platform.NewAppWithMemory(),
		IAM: iamclient.NewClientFromEnv(),
	})
}

func NewServerWithApp(app *platform.App) *Server {
	return NewServerWithOptions(Options{
		App: app,
		IAM: iamclient.NewClientFromEnv(),
	})
}

type Options struct {
	App             *platform.App
	IAM             iamclient.Client
	ExecutionToken  string
	ExecutionTokens serviceauth.TokenSet
	RateLimiter     ratelimit.Limiter
}

func NewServerWithOptions(options Options) *Server {
	server, err := NewServerWithOptionsE(options)
	if err != nil {
		panic(fmt.Sprintf("gateway: %v", err))
	}
	return server
}

// NewServerWithOptionsE is the error-returning variant of NewServerWithOptions.
// Prefer this in entrypoints where you want to log.Fatal instead of panic.
func NewServerWithOptionsE(options Options) (*Server, error) {
	if options.App == nil {
		options.App = platform.NewAppWithMemory()
	}
	if options.IAM == nil {
		options.IAM = iamclient.NewClientFromEnv()
	}
	if options.RateLimiter == nil {
		limiter, err := ratelimit.NewServiceFromEnv()
		if err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
		options.RateLimiter = limiter
	}
	if options.ExecutionTokens.Empty() {
		if options.ExecutionToken != "" {
			options.ExecutionTokens = serviceauth.NewTokenSet(options.ExecutionToken)
		} else {
			options.ExecutionTokens = serviceauth.FromEnv("API_GATEWAY_EXECUTION_TOKENS", "API_GATEWAY_EXECUTION_TOKEN")
		}
	}
	if runtimeconfig.RequireExternalDependencies() {
		if options.IAM == nil {
			return nil, ErrIAMUpstreamRequired
		}
		if options.ExecutionTokens.Empty() {
			return nil, ErrExecutionTokenRequired
		}
	}

	return &Server{
		app:             options.App,
		auth:            options.IAM,
		executionTokens: options.ExecutionTokens,
		rateLimiter:     options.RateLimiter,
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/providers":
		s.handleListProviders(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/listings":
		s.handleListListings(w)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/rfqs":
		s.handleListRFQs(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/disputes":
		s.handleListDisputes(w, r)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/bids"):
		s.handleListRFQBids(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orders":
		s.handleListOrders(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs":
		s.handleCreateRFQ(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/bids"):
		s.handleCreateBid(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/award"):
		s.handleAwardRFQ(w, r)
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
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/resolve"):
		s.handleResolveDispute(w, r)
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

func (s *Server) handleListRFQs(w http.ResponseWriter, r *http.Request) {
	rfqs, err := s.app.ListRFQs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if s.auth != nil {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			writeAuthError(w, err)
			return
		}

		rfqs, err = filterRFQsForActor(rfqs, actor)
		if err != nil {
			writeAuthError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"rfqs": rfqs})
}

func (s *Server) handleListDisputes(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		writeAuthError(w, err)
		return
	}

	disputes, err := s.app.ListDisputes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"disputes": disputes})
}

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.app.ListOrders()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if s.auth != nil {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			writeAuthError(w, err)
			return
		}

		orders, err = filterOrdersForActor(orders, actor)
		if err != nil {
			writeAuthError(w, err)
			return
		}
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
		if errors.Is(err, core.ErrOrderNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if s.auth != nil {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			writeAuthError(w, err)
			return
		}
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

	buyerOrgID, err := s.resolveBuyerOrg(r, payload.BuyerOrgID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/orders",
		OrgID:  buyerOrgID,
		UserID: actorUserID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateOrder, ratelimit.Meta{
		IP:    ratelimit.ClientIP(r),
		OrgID: buyerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}

	if buyerOrgID == "" || payload.ProviderOrgID == "" || len(payload.Milestones) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	input := platform.CreateOrderInput{
		BuyerOrgID:    buyerOrgID,
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

func (s *Server) handleCreateRFQ(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BuyerOrgID         string `json:"buyerOrgId"`
		Title              string `json:"title"`
		Category           string `json:"category"`
		Scope              string `json:"scope"`
		BudgetCents        int64  `json:"budgetCents"`
		ResponseDeadlineAt string `json:"responseDeadlineAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	buyerOrgID, err := s.resolveBuyerOrg(r, payload.BuyerOrgID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/rfqs",
		OrgID:  buyerOrgID,
		UserID: actorUserID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateRFQ, ratelimit.Meta{
		IP:    ratelimit.ClientIP(r),
		OrgID: buyerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}

	responseDeadlineAt, err := time.Parse(time.RFC3339, payload.ResponseDeadlineAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid responseDeadlineAt"})
		return
	}

	rfq, err := s.app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         buyerOrgID,
		Title:              payload.Title,
		Category:           payload.Category,
		Scope:              payload.Scope,
		BudgetCents:        payload.BudgetCents,
		ResponseDeadlineAt: responseDeadlineAt,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"rfq": rfq})
}

func (s *Server) handleListRFQBids(w http.ResponseWriter, r *http.Request) {
	rfqID, err := rfqIDFromBidsPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	bids, err := s.app.ListRFQBids(rfqID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	if s.auth != nil {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			writeAuthError(w, err)
			return
		}

		rfq, err := s.app.GetRFQ(rfqID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}

		bids, err = filterBidsForActor(rfq, bids, actor)
		if err != nil {
			writeAuthError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"bids": bids})
}

func (s *Server) handleCreateBid(w http.ResponseWriter, r *http.Request) {
	rfqID, err := rfqIDFromBidsPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		ProviderOrgID string `json:"providerOrgId"`
		Message       string `json:"message"`
		QuoteCents    int64  `json:"quoteCents"`
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

	providerOrgID, err := s.resolveProviderOrg(r, payload.ProviderOrgID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/rfqs/:id/bids",
		OrgID:  providerOrgID,
		UserID: actorUserID,
		RFQID:  rfqID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateBid, ratelimit.Meta{
		IP:    ratelimit.ClientIP(r),
		OrgID: providerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}

	input := platform.CreateBidInput{
		ProviderOrgID: providerOrgID,
		Message:       payload.Message,
		QuoteCents:    payload.QuoteCents,
		Milestones:    make([]platform.BidMilestoneInput, 0, len(payload.Milestones)),
	}
	for _, milestone := range payload.Milestones {
		input.Milestones = append(input.Milestones, platform.BidMilestoneInput{
			ID:             milestone.ID,
			Title:          milestone.Title,
			BasePriceCents: milestone.BasePriceCents,
			BudgetCents:    milestone.BudgetCents,
		})
	}

	bid, err := s.app.CreateBid(rfqID, input)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"bid": bid})
}

func (s *Server) handleAwardRFQ(w http.ResponseWriter, r *http.Request) {
	rfqID, err := rfqIDFromAwardPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		BidID        string `json:"bidId"`
		FundingMode  string `json:"fundingMode"`
		CreditLineID string `json:"creditLineId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	rfq, err := s.app.GetRFQ(rfqID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	buyerOrgID, err := s.resolveBuyerOrg(r, rfq.BuyerOrgID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	if buyerOrgID != rfq.BuyerOrgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "buyer org mismatch"})
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/rfqs/:id/award",
		OrgID:  buyerOrgID,
		UserID: actorUserID,
		RFQID:  rfqID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayAwardRFQ, ratelimit.Meta{
		IP:    ratelimit.ClientIP(r),
		OrgID: buyerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}

	awardedRFQ, order, err := s.app.AwardRFQ(rfqID, platform.AwardRFQInput{
		BidID:        payload.BidID,
		FundingMode:  core.FundingMode(payload.FundingMode),
		CreditLineID: payload.CreditLineID,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"rfq": awardedRFQ, "order": order})
}

func (s *Server) resolveBuyerOrg(r *http.Request, requestedBuyerOrgID string) (string, error) {
	if s.auth == nil {
		return requestedBuyerOrgID, nil
	}

	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return "", iamclient.ErrUnauthorized
	}

	actor, err := s.auth.GetActor(r.Context(), token)
	if err != nil {
		return "", err
	}

	for _, membership := range actor.Memberships {
		if membership.OrganizationKind != "buyer" {
			continue
		}
		if !isBuyerRole(membership.Role) {
			continue
		}
		if requestedBuyerOrgID != "" && requestedBuyerOrgID != membership.OrganizationID {
			return "", errors.New("buyer org mismatch")
		}
		return membership.OrganizationID, nil
	}

	return "", errors.New("buyer membership is required")
}

func (s *Server) resolveProviderOrg(r *http.Request, requestedProviderOrgID string) (string, error) {
	if s.auth == nil {
		return requestedProviderOrgID, nil
	}

	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return "", iamclient.ErrUnauthorized
	}

	actor, err := s.auth.GetActor(r.Context(), token)
	if err != nil {
		return "", err
	}

	for _, membership := range actor.Memberships {
		if membership.OrganizationKind != "provider" {
			continue
		}
		if !isProviderRole(membership.Role) {
			continue
		}
		if requestedProviderOrgID != "" && requestedProviderOrgID != membership.OrganizationID {
			return "", errors.New("provider org mismatch")
		}
		return membership.OrganizationID, nil
	}

	return "", errors.New("provider membership is required")
}

func bearerToken(header string) (string, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	return token, token != ""
}

func isBuyerRole(role string) bool {
	switch role {
	case "org_owner", "procurement", "operator":
		return true
	default:
		return false
	}
}

func isProviderRole(role string) bool {
	switch role {
	case "org_owner", "sales", "delivery_operator":
		return true
	default:
		return false
	}
}

func isOpsRole(role string) bool {
	switch role {
	case "ops_reviewer", "risk_admin", "finance_admin", "super_admin":
		return true
	default:
		return false
	}
}

func (s *Server) resolveOpsUser(r *http.Request) (string, error) {
	if s.auth == nil {
		return "", nil
	}

	actor, err := s.authenticatedActor(r)
	if err != nil {
		return "", err
	}

	for _, membership := range actor.Memberships {
		if membership.OrganizationKind != "ops" {
			continue
		}
		if !isOpsRole(membership.Role) {
			continue
		}
		return actor.UserID, nil
	}

	return "", errors.New("ops membership is required")
}

func (s *Server) authenticatedActor(r *http.Request) (iamclient.Actor, error) {
	if s.auth == nil {
		return iamclient.Actor{}, nil
	}

	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return iamclient.Actor{}, iamclient.ErrUnauthorized
	}

	return s.auth.GetActor(r.Context(), token)
}

func (s *Server) handleSettleMilestone(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeExecutionMutation(r); err != nil {
		writeAuthError(w, err)
		return
	}

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
	if err := s.authorizeExecutionMutation(r); err != nil {
		writeAuthError(w, err)
		return
	}

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

	if s.auth != nil {
		order, err := s.app.GetOrder(orderID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}

		buyerOrgID, err := s.resolveBuyerOrg(r, order.BuyerOrgID)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if buyerOrgID != order.BuyerOrgID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "buyer org mismatch"})
			return
		}
		actorUserID := s.actorUserID(r)
		r = observability.WithRequestTags(r, observability.RequestTags{
			Route:   "/api/v1/orders/:id/disputes",
			OrgID:   order.BuyerOrgID,
			UserID:  actorUserID,
			OrderID: orderID,
		})
		if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateDisp, ratelimit.Meta{
			OrgID: order.BuyerOrgID,
			UserID: actorUserID,
		}); blocked {
			return
		}
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

func (s *Server) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	disputeID, err := disputeIDFromResolvePath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(payload.Resolution) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "resolution is required"})
		return
	}

	resolvedBy, err := s.resolveOpsUser(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/disputes/:id/resolve",
		UserID: resolvedBy,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayResolveDisp, ratelimit.Meta{
		OrgID: "ops",
		UserID: resolvedBy,
	}); blocked {
		return
	}

	dispute, order, err := s.app.ResolveDispute(disputeID, platform.ResolveDisputeInput{
		Resolution: payload.Resolution,
		ResolvedBy: resolvedBy,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"dispute": dispute, "order": order})
}

func (s *Server) handleCreditDecision(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		writeAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/credits/decision",
		UserID: actorUserID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreditDec, ratelimit.Meta{
		OrgID: "ops",
		UserID: actorUserID,
	}); blocked {
		return
	}

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

	if s.auth != nil {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		order, err := s.app.GetOrder(payload.OrderID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			writeAuthError(w, err)
			return
		}
		payload.Author = actor.UserID
		orgID := order.BuyerOrgID
		for _, membership := range actor.Memberships {
			if membership.OrganizationID == order.ProviderOrgID {
				orgID = order.ProviderOrgID
				break
			}
		}
		r = observability.WithRequestTags(r, observability.RequestTags{
			Route:   "/api/v1/messages",
			OrgID:   orgID,
			UserID:  actor.UserID,
			OrderID: payload.OrderID,
		})
		if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateMsg, ratelimit.Meta{
			OrgID: orgID,
			UserID: actor.UserID,
		}); blocked {
			return
		}
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

func rfqIDFromBidsPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[2] != "rfqs" || parts[4] != "bids" {
		return "", errors.New("invalid bid path")
	}
	return parts[3], nil
}

func rfqIDFromAwardPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[2] != "rfqs" || parts[4] != "award" {
		return "", errors.New("invalid award path")
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

func disputeIDFromResolvePath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[2] != "disputes" || parts[4] != "resolve" {
		return "", errors.New("invalid dispute resolution path")
	}
	return parts[3], nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeGatewayError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrOrderNotFound),
		errors.Is(err, core.ErrMilestoneNotFound),
		errors.Is(err, platform.ErrRFQNotFound),
		errors.Is(err, platform.ErrBidNotFound),
		errors.Is(err, platform.ErrDisputeNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, iamclient.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case err != nil && err.Error() == "invalid service token":
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case strings.Contains(err.Error(), "mismatch"), strings.Contains(err.Error(), "required"):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
}

func (s *Server) actorUserID(r *http.Request) string {
	if s.auth == nil {
		return ""
	}
	actor, err := s.authenticatedActor(r)
	if err != nil {
		return ""
	}
	return actor.UserID
}

func (s *Server) applyRateLimit(w http.ResponseWriter, r *http.Request, policy ratelimit.Policy, meta ratelimit.Meta) bool {
	if s.rateLimiter == nil {
		return false
	}
	decision, err := s.rateLimiter.Allow(r.Context(), policy, meta)
	if err != nil {
		observability.CaptureError(r.Context(), err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "rate limiter unavailable"})
		return true
	}
	if decision.Allowed {
		return false
	}
	ratelimit.WriteHeaders(w, time.Now().UTC(), decision)
	observability.CaptureMessage(r.Context(), "rate limit exceeded")
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
	return true
}

func (s *Server) authorizeExecutionMutation(r *http.Request) error {
	if s.executionTokens.Empty() {
		return nil
	}
	if s.executionTokens.MatchesRequest(r) {
		return nil
	}
	return errors.New("invalid service token")
}

func filterOrdersForActor(orders []*core.Order, actor iamclient.Actor) ([]*core.Order, error) {
	if len(actor.Memberships) == 0 {
		return nil, errors.New("membership is required")
	}

	buyerOrgIDs := make(map[string]struct{})
	providerOrgIDs := make(map[string]struct{})

	for _, membership := range actor.Memberships {
		switch {
		case membership.OrganizationKind == "ops" && isOpsRole(membership.Role):
			return orders, nil
		case membership.OrganizationKind == "buyer" && isBuyerRole(membership.Role):
			buyerOrgIDs[membership.OrganizationID] = struct{}{}
		case membership.OrganizationKind == "provider" && isProviderRole(membership.Role):
			providerOrgIDs[membership.OrganizationID] = struct{}{}
		}
	}

	filtered := make([]*core.Order, 0, len(orders))
	for _, order := range orders {
		if order == nil {
			continue
		}
		if _, ok := buyerOrgIDs[order.BuyerOrgID]; ok {
			filtered = append(filtered, order)
			continue
		}
		if _, ok := providerOrgIDs[order.ProviderOrgID]; ok {
			filtered = append(filtered, order)
		}
	}

	if len(filtered) == 0 && len(buyerOrgIDs) == 0 && len(providerOrgIDs) == 0 {
		return nil, errors.New("membership is required")
	}

	return filtered, nil
}

func authorizeOrderForActor(order *core.Order, actor iamclient.Actor) error {
	if order == nil {
		return errors.New("order is required")
	}
	if len(actor.Memberships) == 0 {
		return errors.New("membership is required")
	}

	for _, membership := range actor.Memberships {
		switch {
		case membership.OrganizationKind == "ops" && isOpsRole(membership.Role):
			return nil
		case membership.OrganizationKind == "buyer" && isBuyerRole(membership.Role) && membership.OrganizationID == order.BuyerOrgID:
			return nil
		case membership.OrganizationKind == "provider" && isProviderRole(membership.Role) && membership.OrganizationID == order.ProviderOrgID:
			return nil
		}
	}

	return errors.New("membership is required")
}

func filterRFQsForActor(rfqs []platform.RFQ, actor iamclient.Actor) ([]platform.RFQ, error) {
	if len(actor.Memberships) == 0 {
		return nil, errors.New("membership is required")
	}

	buyerOrgIDs := make(map[string]struct{})
	providerOrgIDs := make(map[string]struct{})

	for _, membership := range actor.Memberships {
		switch {
		case membership.OrganizationKind == "ops" && isOpsRole(membership.Role):
			return rfqs, nil
		case membership.OrganizationKind == "buyer" && isBuyerRole(membership.Role):
			buyerOrgIDs[membership.OrganizationID] = struct{}{}
		case membership.OrganizationKind == "provider" && isProviderRole(membership.Role):
			providerOrgIDs[membership.OrganizationID] = struct{}{}
		}
	}

	filtered := make([]platform.RFQ, 0, len(rfqs))
	for _, rfq := range rfqs {
		if _, ok := buyerOrgIDs[rfq.BuyerOrgID]; ok {
			filtered = append(filtered, rfq)
			continue
		}
		if len(providerOrgIDs) == 0 {
			continue
		}
		if rfq.Status == platform.RFQStatusOpen {
			filtered = append(filtered, rfq)
			continue
		}
		if _, ok := providerOrgIDs[rfq.AwardedProviderOrgID]; ok {
			filtered = append(filtered, rfq)
		}
	}

	if len(filtered) == 0 && len(buyerOrgIDs) == 0 && len(providerOrgIDs) == 0 {
		return nil, errors.New("membership is required")
	}

	return filtered, nil
}

func filterBidsForActor(rfq platform.RFQ, bids []platform.Bid, actor iamclient.Actor) ([]platform.Bid, error) {
	if len(actor.Memberships) == 0 {
		return nil, errors.New("membership is required")
	}

	providerOrgIDs := make(map[string]struct{})
	for _, membership := range actor.Memberships {
		switch {
		case membership.OrganizationKind == "ops" && isOpsRole(membership.Role):
			return bids, nil
		case membership.OrganizationKind == "buyer" && isBuyerRole(membership.Role) && membership.OrganizationID == rfq.BuyerOrgID:
			return bids, nil
		case membership.OrganizationKind == "provider" && isProviderRole(membership.Role):
			providerOrgIDs[membership.OrganizationID] = struct{}{}
		}
	}

	if len(providerOrgIDs) == 0 {
		return nil, errors.New("membership is required")
	}

	filtered := make([]platform.Bid, 0, len(bids))
	for _, bid := range bids {
		if _, ok := providerOrgIDs[bid.ProviderOrgID]; ok {
			filtered = append(filtered, bid)
		}
	}

	return filtered, nil
}
