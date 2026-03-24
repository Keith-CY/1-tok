package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chenyu/1-tok/internal/carrier"
	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/demoenv"
	"github.com/chenyu/1-tok/internal/httputil"
	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/notifications"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/ratelimit"
	"github.com/chenyu/1-tok/internal/release"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/chenyu/1-tok/internal/serviceauth"
	"github.com/chenyu/1-tok/internal/usageproof"
	"github.com/chenyu/1-tok/internal/validation"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Sentinel startup errors returned by NewServerWithOptionsE.
var (
	ErrIAMUpstreamRequired    = errors.New("IAM_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
	ErrExecutionTokenRequired = errors.New("API_GATEWAY_EXECUTION_TOKEN or API_GATEWAY_EXECUTION_TOKENS is required when ONE_TOK_REQUIRE_EXTERNALS=true")
)

type Server struct {
	app             *platform.App
	auth            iamclient.Client
	executionTokens serviceauth.TokenSet
	rateLimiter     ratelimit.Limiter
	carrier         *carrier.Service
	carrierAwardExecutor carrierAwardExecutor
	webhooks        *notifications.Registry
	evidence        *carrier.EvidenceStore
	ledger          *carrier.EventLedger
	demoConfig      demoenv.Config
	demoPrepare     func(context.Context) (release.DemoRunSummary, error)
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
	App                           *platform.App
	IAM                           iamclient.Client
	ExecutionToken                string
	ExecutionTokens               serviceauth.TokenSet
	RateLimiter                   ratelimit.Limiter
	Carrier                       *carrier.Service
	CarrierAwardExecutor          carrierAwardExecutor
	ProviderSettlementProvisioner platform.ProviderSettlementProvisioner
	DemoConfig                    *demoenv.Config
	DemoPrepare                   func(context.Context) (release.DemoRunSummary, error)
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
		if options.IAM == nil || iamclient.IsNoop(options.IAM) {
			return nil, ErrIAMUpstreamRequired
		}
		if options.ExecutionTokens.Empty() {
			return nil, ErrExecutionTokenRequired
		}
	}
	carrierSvc := options.Carrier
	if carrierSvc == nil {
		carrierSvc = carrier.NewService()
	}
	if options.ProviderSettlementProvisioner != nil {
		options.App.SetProviderSettlementProvisioner(options.ProviderSettlementProvisioner)
	}
	carrierAwardExecutor := options.CarrierAwardExecutor
	if carrierAwardExecutor == nil {
		carrierAwardExecutor = newCarrierOrderAutoExecutor(options.App, carrierSvc)
	}
	webhookSvc := notifications.NewWebhookService()
	registry := notifications.NewRegistry(webhookSvc)
	demoConfig := demoenv.ConfigFromEnv()
	if options.DemoConfig != nil {
		demoConfig = *options.DemoConfig
	}
	demoPrepare := options.DemoPrepare
	if demoPrepare == nil {
		demoPrepare = func(ctx context.Context) (release.DemoRunSummary, error) {
			return release.RunDemoPrepare(ctx, release.DemoRunConfigFromEnv())
		}
	}
	// Wire notifications to the app via adapter
	options.App.SetNotifier(&webhookNotifierAdapter{svc: webhookSvc})
	return &Server{
		app:             options.App,
		auth:            options.IAM,
		executionTokens: options.ExecutionTokens,
		rateLimiter:     options.RateLimiter,
		carrier:         carrierSvc,
		carrierAwardExecutor: carrierAwardExecutor,
		webhooks:        registry,
		evidence:        carrier.NewEvidenceStore(),
		ledger:          carrier.NewEventLedger(),
		demoConfig:      demoConfig,
		demoPrepare:     demoPrepare,
	}, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/system":
		s.handleSystemInfo(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/system/ratelimits":
		s.handleRateLimitConfig(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/system/stale-jobs":
		s.handleStaleJobs(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/system/exposure":
		s.handleFiberExposure(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ops/demo/status":
		s.handleDemoStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ops/demo/prepare":
		s.handleDemoPrepare(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ops/demo/warmup":
		s.handleDemoWarmup(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/provider-applications":
		s.handleSubmitApplication(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/provider-applications":
		s.handleListApplications(w, r)
	case r.Method == http.MethodPost && isApplicationReviewPath(r.URL.Path):
		s.handleReviewApplication(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/carrier-bindings":
		s.handleRegisterCarrierBinding(w, r)
	case r.Method == http.MethodGet && isCarrierBindingPath(r.URL.Path):
		s.handleGetCarrierBinding(w, r)
	case r.Method == http.MethodPost && isCarrierBindingVerifyPath(r.URL.Path):
		s.handleVerifyCarrierBinding(w, r)
	case r.Method == http.MethodPost && isCarrierBindingSuspendPath(r.URL.Path):
		s.handleSuspendCarrierBinding(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/provider-settlement-bindings":
		s.handleRegisterProviderSettlementBinding(w, r)
	case r.Method == http.MethodGet && isProviderSettlementBindingPoolPath(r.URL.Path):
		s.handleGetProviderSettlementPool(w, r)
	case r.Method == http.MethodGet && isProviderSettlementBindingPath(r.URL.Path):
		s.handleGetProviderSettlementBinding(w, r)
	case r.Method == http.MethodPost && isProviderSettlementBindingVerifyPath(r.URL.Path):
		s.handleVerifyProviderSettlementBinding(w, r)
	case r.Method == http.MethodPost && isProviderSettlementBindingSuspendPath(r.URL.Path):
		s.handleSuspendProviderSettlementBinding(w, r)
	case r.Method == http.MethodPost && isProviderSettlementBindingDisconnectPath(r.URL.Path):
		s.handleDisconnectProviderSettlementBinding(w, r)
	case r.Method == http.MethodPost && isProviderSettlementBindingRecoverPath(r.URL.Path):
		s.handleRecoverProviderSettlementBinding(w, r)
	case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/carrier/callback" || r.URL.Path == "/api/v1/carrier/callbacks/events"):
		s.handleCarrierCallback(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/credit-limits":
		s.handleSetCreditLimit(w, r)
	case r.Method == http.MethodGet && isGetCreditLimitPath(r.URL.Path):
		s.handleGetCreditLimit(w, r)
	case r.Method == http.MethodPost && isJobEvidencePath(r.URL.Path):
		s.handleSubmitEvidence(w, r)
	case r.Method == http.MethodGet && isJobEvidencePath(r.URL.Path):
		s.handleGetEvidence(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/stats":
		s.handleMarketplaceStats(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/leaderboard":
		s.handleLeaderboard(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/export/orders":
		s.handleExportOrders(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/export/disputes":
		s.handleExportDisputes(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/export/rfqs":
		s.handleExportRFQs(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/export/applications":
		s.handleExportApplications(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders/batch-status":
		s.handleBatchOrderStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/listings":
		s.handleCreateListing(w, r)
	case r.Method == http.MethodPost && isOrderTopUpPath(r.URL.Path):
		s.handleTopUpMilestone(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/providers":
		s.handleListProviders(w, r)
	case r.Method == http.MethodGet && isProviderPath(r.URL.Path):
		s.handleGetProvider(w, r)
	case r.Method == http.MethodGet && isProviderRevenuePath(r.URL.Path):
		s.handleProviderRevenue(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/listings":
		s.handleListListings(w, r)
	case r.Method == http.MethodGet && isListingPath(r.URL.Path):
		s.handleGetListing(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/rfqs":
		s.handleListRFQs(w, r)
	case r.Method == http.MethodGet && isRFQDetailPath(r.URL.Path):
		s.handleGetRFQ(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/disputes":
		s.handleListDisputes(w, r)
	case r.Method == http.MethodGet && isDisputeDetailPath(r.URL.Path):
		s.handleGetDispute(w, r)
	case r.Method == http.MethodGet && isRFQBidsPath(r.URL.Path):
		s.handleListRFQBids(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orders":
		s.handleListOrders(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs":
		s.handleCreateRFQ(w, r)
	case r.Method == http.MethodPost && isRFQBidsPath(r.URL.Path):
		s.handleCreateBid(w, r)
	case r.Method == http.MethodPost && isRFQAwardPath(r.URL.Path):
		s.handleAwardRFQ(w, r)
	case r.Method == http.MethodGet && isRFQMessagesPath(r.URL.Path):
		s.handleListRFQMessages(w, r)
	case r.Method == http.MethodPost && isRFQMessagesPath(r.URL.Path):
		s.handleCreateRFQMessage(w, r)
	case r.Method == http.MethodGet && isOrderRatingPath(r.URL.Path):
		s.handleGetOrderRating(w, r)
	case r.Method == http.MethodGet && isOrderMessagesPath(r.URL.Path):
		s.handleListOrderMessages(w, r)
	case r.Method == http.MethodGet && isOrderBudgetPath(r.URL.Path):
		s.handleOrderBudget(w, r)
	case r.Method == http.MethodGet && isOrderTimelinePath(r.URL.Path):
		s.handleOrderTimeline(w, r)
	case r.Method == http.MethodGet && isOrderProviderSettlementReservationPath(r.URL.Path):
		s.handleGetOrderProviderSettlementReservation(w, r)
	case r.Method == http.MethodGet && isBindCarrierPath(r.URL.Path):
		s.handleGetBinding(w, r)
	case r.Method == http.MethodGet && isCreateJobPath(r.URL.Path):
		s.handleListJobs(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/orders/"):
		s.handleGetOrder(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
		s.handleCreateOrder(w, r)
	case r.Method == http.MethodPost && isOrderSettlePath(r.URL.Path):
		s.handleSettleMilestone(w, r)
	case r.Method == http.MethodPost && isOrderUsagePath(r.URL.Path):
		s.handleRecordUsage(w, r)
	case r.Method == http.MethodPost && isOrderDisputesPath(r.URL.Path):
		s.handleCreateDispute(w, r)
	case r.Method == http.MethodPost && isDisputeResolvePath(r.URL.Path):
		s.handleResolveDispute(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/credits/decision":
		s.handleCreditDecision(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/messages":
		s.handleCreateMessage(w, r)
	case r.Method == http.MethodPost && isOrderRatingPath(r.URL.Path):
		s.handleRateOrder(w, r)
	case r.Method == http.MethodPost && isBindCarrierPath(r.URL.Path):
		s.handleBindCarrier(w, r)
	case r.Method == http.MethodPost && isCreateJobPath(r.URL.Path):
		s.handleCreateJob(w, r)
	case r.Method == http.MethodGet && isJobPath(r.URL.Path):
		s.handleGetJob(w, r)
	case r.Method == http.MethodPatch && isJobActionPath(r.URL.Path, "start"):
		s.handleStartJob(w, r)
	case r.Method == http.MethodPatch && isJobActionPath(r.URL.Path, "complete"):
		s.handleCompleteJob(w, r)
	case r.Method == http.MethodPatch && isJobActionPath(r.URL.Path, "fail"):
		s.handleFailJob(w, r)
	case r.Method == http.MethodPost && isJobActionPath(r.URL.Path, "progress"):
		s.handleJobProgress(w, r)
	case r.Method == http.MethodPost && isJobActionPath(r.URL.Path, "heartbeat"):
		s.handleJobHeartbeat(w, r)
	case r.Method == http.MethodPost && isJobActionPath(r.URL.Path, "cancel"):
		s.handleCancelJob(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/webhooks":
		s.handleListWebhooks(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/webhooks":
		s.handleRegisterWebhook(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/notifications/"):
		s.handleListNotifications(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/webhooks/"):
		s.handleUnregisterWebhook(w, r)
	default:
		httputil.WriteError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "route not found")
	}
}
func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	input := platform.SearchProvidersInput{
		Capability: q.Get("capability"),
		Tier:       q.Get("tier"),
	}
	if v := q.Get("minRating"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			input.MinRating = parsed
		}
	}
	providers, err := s.app.SearchProviders(input)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	page := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"providers": httputil.Apply(providers, page), "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "total": len(providers)}})
}
func (s *Server) handleListListings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	input := platform.ListListingsInput{
		Query:         q.Get("q"),
		Category:      q.Get("category"),
		Tags:          q["tag"],
		ProviderOrgID: q.Get("providerOrgId"),
	}
	if v := q.Get("minPrice"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			input.MinPriceCents = parsed
		}
	}
	if v := q.Get("maxPrice"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			input.MaxPriceCents = parsed
		}
	}
	listings, err := s.app.SearchListings(input)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Sort support
	if sortBy := q.Get("sort"); sortBy != "" {
		switch sortBy {
		case "price_asc":
			slices.SortFunc(listings, func(a, b platform.Listing) int {
				return int(a.BasePriceCents - b.BasePriceCents)
			})
		case "price_desc":
			slices.SortFunc(listings, func(a, b platform.Listing) int {
				return int(b.BasePriceCents - a.BasePriceCents)
			})
		case "title":
			slices.SortFunc(listings, func(a, b platform.Listing) int {
				if a.Title < b.Title {
					return -1
				}
				if a.Title > b.Title {
					return 1
				}
				return 0
			})
		}
	}
	page := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"listings": httputil.Apply(listings, page), "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "total": len(listings)}})
}
func (s *Server) handleListRFQs(w http.ResponseWriter, r *http.Request) {
	rfqs, err := s.app.ListRFQs()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		rfqs, err = filterRFQsForActor(rfqs, actor)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	// Apply query filters
	if status := r.URL.Query().Get("status"); status != "" {
		filtered := make([]platform.RFQ, 0)
		for _, rfq := range rfqs {
			if string(rfq.Status) == status {
				filtered = append(filtered, rfq)
			}
		}
		rfqs = filtered
	}
	page := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"rfqs": httputil.Apply(rfqs, page), "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "total": len(rfqs)}})
}
func (s *Server) handleListDisputes(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	disputes, err := s.app.ListDisputes()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filtered := make([]platform.Dispute, 0)
		for _, d := range disputes {
			if string(d.Status) == status {
				filtered = append(filtered, d)
			}
		}
		disputes = filtered
	}
	page := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"disputes": httputil.Apply(disputes, page), "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "total": len(disputes)}})
}
func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.app.ListOrders()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		orders, err = filterOrdersForActor(orders, actor)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	// Apply query filters
	q := r.URL.Query()
	if status := q.Get("status"); status != "" {
		filtered := make([]*core.Order, 0)
		for _, o := range orders {
			if string(o.Status) == status {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}
	page := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"orders": httputil.Apply(orders, page), "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "total": len(orders)}})
}
func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromPath(r.URL.Path)
	if err != nil {
		writeCallbackError(w, http.StatusBadRequest, httputil.ErrCodeBadRequest, err.Error())
		return
	}
	order, err := s.app.GetOrder(orderID)
	if err != nil {
		if errors.Is(err, core.ErrOrderNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}

	resp := map[string]any{"order": order}
	if budgetWall, _ := s.app.GetBudgetWallInfo(order.ID); budgetWall != nil {
		resp["budgetWall"] = budgetWall
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
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
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if verr := validation.New().
		Required("fundingMode", payload.FundingMode).
		Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	if len(payload.Milestones) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "at least one milestone is required")
		return
	}
	if !core.IsValidFundingMode(core.FundingMode(payload.FundingMode)) {
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "fundingMode must be 'prepaid' or 'credit'")
		return
	}
	if payload.FundingMode == "credit" && payload.CreditLineID == "" {
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "creditLineId is required for credit funding mode")
		return
	}
	buyerOrgID, err := s.resolveBuyerOrg(r, payload.BuyerOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/orders",
		OrgID:  buyerOrgID,
		UserID: actorUserID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateOrder, ratelimit.Meta{
		IP:     ratelimit.ClientIP(r),
		OrgID:  buyerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}
	if buyerOrgID == "" || payload.ProviderOrgID == "" || len(payload.Milestones) == 0 {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
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
		writeCallbackError(w, http.StatusBadRequest, httputil.ErrCodeBadRequest, err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"order": order})
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
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if verr := validation.New().
		Required("title", payload.Title).
		Required("category", payload.Category).
		Required("scope", payload.Scope).
		Positive("budgetCents", payload.BudgetCents).
		Required("responseDeadlineAt", payload.ResponseDeadlineAt).
		Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	buyerOrgID, err := s.resolveBuyerOrg(r, payload.BuyerOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/rfqs",
		OrgID:  buyerOrgID,
		UserID: actorUserID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreateRFQ, ratelimit.Meta{
		IP:     ratelimit.ClientIP(r),
		OrgID:  buyerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}
	responseDeadlineAt, err := time.Parse(time.RFC3339, payload.ResponseDeadlineAt)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid responseDeadlineAt"})
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
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"rfq": rfq})
}
func (s *Server) handleListRFQBids(w http.ResponseWriter, r *http.Request) {
	rfqID, err := rfqIDFromBidsPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	bids, err := s.app.ListRFQBids(rfqID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		rfq, err := s.app.GetRFQ(rfqID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		bids, err = filterBidsForActor(rfq, bids, actor)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	page := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"bids": httputil.Apply(bids, page), "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "total": len(bids)}})
}
func (s *Server) handleCreateBid(w http.ResponseWriter, r *http.Request) {
	rfqID, err := rfqIDFromBidsPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		ProviderOrgID      string `json:"providerOrgId"`
		Message            string `json:"message"`
		QuoteCents         int64  `json:"quoteCents"`
		ExecutionProfileID string `json:"executionProfileId"`
		Milestones         []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			BasePriceCents int64  `json:"basePriceCents"`
			BudgetCents    int64  `json:"budgetCents"`
		} `json:"milestones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if verr := validation.New().
		Required("message", payload.Message).
		Positive("quoteCents", payload.QuoteCents).
		Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	providerOrgID, err := s.resolveProviderOrg(r, payload.ProviderOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
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
		IP:     ratelimit.ClientIP(r),
		OrgID:  providerOrgID,
		UserID: actorUserID,
	}); blocked {
		return
	}
	input := platform.CreateBidInput{
		ProviderOrgID:      providerOrgID,
		Message:            payload.Message,
		QuoteCents:         payload.QuoteCents,
		ExecutionProfileID: payload.ExecutionProfileID,
		Milestones:         make([]platform.BidMilestoneInput, 0, len(payload.Milestones)),
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
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"bid": bid})
}
func (s *Server) handleAwardRFQ(w http.ResponseWriter, r *http.Request) {
	rfqID, err := rfqIDFromAwardPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		BidID        string `json:"bidId"`
		FundingMode  string `json:"fundingMode"`
		CreditLineID string `json:"creditLineId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if payload.FundingMode == "credit" && payload.CreditLineID == "" {
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "creditLineId is required for credit funding mode")
		return
	}
	rfq, err := s.app.GetRFQ(rfqID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	buyerOrgID, err := s.resolveBuyerOrg(r, rfq.BuyerOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	if buyerOrgID != rfq.BuyerOrgID {
		httputil.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "buyer org mismatch"})
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
		IP:     ratelimit.ClientIP(r),
		OrgID:  buyerOrgID,
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
	s.dispatchCarrierExecution(awardedRFQ, order)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"rfq": awardedRFQ, "order": order})
}
func (s *Server) resolveBuyerOrg(r *http.Request, requestedBuyerOrgID string) (string, error) {
	if s.auth == nil || iamclient.IsNoop(s.auth) {
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
			return "", fmt.Errorf("buyer org mismatch: %w", platform.ErrOrgMismatch)
		}
		return membership.OrganizationID, nil
	}
	return "", fmt.Errorf("buyer membership is required: %w", platform.ErrMembershipRequired)
}
func (s *Server) resolveProviderOrg(r *http.Request, requestedProviderOrgID string) (string, error) {
	if s.auth == nil || iamclient.IsNoop(s.auth) {
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
			return "", fmt.Errorf("provider org mismatch: %w", platform.ErrOrgMismatch)
		}
		return membership.OrganizationID, nil
	}
	return "", fmt.Errorf("provider membership is required: %w", platform.ErrMembershipRequired)
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
	if s.auth == nil || iamclient.IsNoop(s.auth) {
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
	return "", fmt.Errorf("ops membership is required: %w", platform.ErrMembershipRequired)
}

type actorContextKey struct{}

func (s *Server) authenticatedActor(r *http.Request) (iamclient.Actor, error) {
	if s.auth == nil || iamclient.IsNoop(s.auth) {
		return iamclient.Actor{}, nil
	}
	// Return cached actor if available
	if cached, ok := r.Context().Value(actorContextKey{}).(iamclient.Actor); ok {
		return cached, nil
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return iamclient.Actor{}, iamclient.ErrUnauthorized
	}
	actor, err := s.auth.GetActor(r.Context(), token)
	if err != nil {
		return iamclient.Actor{}, err
	}
	// Cache in context for subsequent calls in same request
	ctx := context.WithValue(r.Context(), actorContextKey{}, actor)
	*r = *r.WithContext(ctx)
	return actor, nil
}
func (s *Server) handleSettleMilestone(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeExecutionMutation(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	orderID, milestoneID, err := orderMilestoneFromSettlePath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		MilestoneID string `json:"milestoneId"`
		Summary     string `json:"summary"`
		Source      string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
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
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"order": order, "ledgerEntry": entry})
}
func (s *Server) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeExecutionMutation(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	orderID, milestoneID, err := orderMilestoneFromUsagePath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		Kind           core.UsageChargeKind `json:"kind"`
		AmountCents    int64                `json:"amountCents"`
		ProofRef       string               `json:"proofRef"`
		ProofSignature string               `json:"proofSignature"`
		ProofTimestamp string               `json:"proofTimestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	order, charge, err := s.app.RecordUsageCharge(orderID, platform.RecordUsageChargeInput{
		MilestoneID:    milestoneID,
		Kind:           payload.Kind,
		AmountCents:    payload.AmountCents,
		ProofRef:       payload.ProofRef,
		ProofSignature: payload.ProofSignature,
		ProofTimestamp: payload.ProofTimestamp,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"order": order, "usageCharge": charge})
}
func (s *Server) handleCreateDispute(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromDisputePath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		MilestoneID string `json:"milestoneId"`
		Reason      string `json:"reason"`
		RefundCents int64  `json:"refundCents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if verr := validation.New().
		Required("milestoneId", payload.MilestoneID).
		Required("reason", payload.Reason).
		Positive("refundCents", payload.RefundCents).
		Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		order, err := s.app.GetOrder(orderID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		buyerOrgID, err := s.resolveBuyerOrg(r, order.BuyerOrgID)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		if buyerOrgID != order.BuyerOrgID {
			httputil.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "buyer org mismatch"})
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
			OrgID:  order.BuyerOrgID,
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
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"order": order, "refundEntry": refund, "recoveryEntry": recovery})
}
func (s *Server) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	disputeID, err := disputeIDFromResolvePath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(payload.Resolution) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "resolution is required"})
		return
	}
	resolvedBy, err := s.resolveOpsUser(r)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/disputes/:id/resolve",
		UserID: resolvedBy,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayResolveDisp, ratelimit.Meta{
		OrgID:  "ops",
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
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"dispute": dispute, "order": order})
}
func (s *Server) handleCreditDecision(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	actorUserID := s.actorUserID(r)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/api/v1/credits/decision",
		UserID: actorUserID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyGatewayCreditDec, ratelimit.Meta{
		OrgID:  "ops",
		UserID: actorUserID,
	}); blocked {
		return
	}
	var history core.CreditHistory
	if err := json.NewDecoder(r.Body).Decode(&history); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"decision": s.app.DecideCredit(history)})
}
func (s *Server) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		OrderID string `json:"orderId"`
		Author  string `json:"author"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	validator := validation.New().
		Required("orderId", payload.OrderID).
		Required("body", payload.Body)
	if s.auth == nil || iamclient.IsNoop(s.auth) {
		validator = validator.Required("author", payload.Author)
	}
	if verr := validator.Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		order, err := s.app.GetOrder(payload.OrderID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			httputil.WriteAuthError(w, err)
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
			OrgID:  orgID,
			UserID: actor.UserID,
		}); blocked {
			return
		}
	}
	message, err := s.app.CreateMessage(payload.OrderID, payload.Author, payload.Body)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"message": message})
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
func writeGatewayError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrOrderNotFound),
		errors.Is(err, core.ErrMilestoneNotFound),
		errors.Is(err, platform.ErrRFQNotFound),
		errors.Is(err, platform.ErrBidNotFound),
		errors.Is(err, platform.ErrDisputeNotFound),
		errors.Is(err, platform.ErrProviderCarrierBindingNotFound),
		errors.Is(err, platform.ErrProviderSettlementBindingNotFound):
		httputil.WriteError(w, http.StatusNotFound, httputil.ErrCodeNotFound, err.Error())
	case errors.Is(err, platform.ErrProviderSuspended):
		httputil.WriteError(w, http.StatusForbidden, httputil.ErrCodeForbidden, err.Error())
	case errors.Is(err, platform.ErrMissingRequiredFields),
		errors.Is(err, platform.ErrInvalidScore),
		errors.Is(err, platform.ErrBidIDRequired),
		errors.Is(err, platform.ErrDeadlineRequired),
		errors.Is(err, platform.ErrDeadlineInPast),
		errors.Is(err, platform.ErrMilestonesRequired):
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, err.Error())
	case errors.Is(err, platform.ErrRFQNotOpenForBids),
		errors.Is(err, platform.ErrRFQNotOpenForAward),
		errors.Is(err, platform.ErrBidNotBelongToRFQ),
		errors.Is(err, platform.ErrOrderNotCompleted),
		errors.Is(err, platform.ErrOrderAlreadyRated),
		errors.Is(err, platform.ErrProviderSettlementBindingRequired),
		errors.Is(err, platform.ErrProviderSettlementPoolUnavailable),
		errors.Is(err, platform.ErrProviderSettlementProvisionerMissing):
		httputil.WriteError(w, http.StatusConflict, httputil.ErrCodeConflict, err.Error())
	default:
		httputil.WriteError(w, http.StatusConflict, httputil.ErrCodeConflict, err.Error())
	}
}
func (s *Server) actorUserID(r *http.Request) string {
	if s.auth == nil || iamclient.IsNoop(s.auth) {
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
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "rate limiter unavailable"})
		return true
	}
	if decision.Allowed {
		return false
	}
	ratelimit.WriteHeaders(w, time.Now().UTC(), decision)
	observability.CaptureMessage(r.Context(), "rate limit exceeded")
	httputil.WriteJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
	return true
}
func (s *Server) authorizeExecutionMutation(r *http.Request) error {
	if s.executionTokens.Empty() {
		return nil
	}
	if s.executionTokens.MatchesRequest(r) {
		return nil
	}
	return serviceauth.ErrInvalidServiceToken
}
func filterOrdersForActor(orders []*core.Order, actor iamclient.Actor) ([]*core.Order, error) {
	if len(actor.Memberships) == 0 {
		return nil, platform.ErrMembershipRequired
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
		return nil, platform.ErrMembershipRequired
	}
	return filtered, nil
}
func authorizeOrderForActor(order *core.Order, actor iamclient.Actor) error {
	if order == nil {
		return errors.New("order is required")
	}
	if len(actor.Memberships) == 0 {
		return platform.ErrMembershipRequired
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
	return platform.ErrMembershipRequired
}
func filterRFQsForActor(rfqs []platform.RFQ, actor iamclient.Actor) ([]platform.RFQ, error) {
	if len(actor.Memberships) == 0 {
		return nil, platform.ErrMembershipRequired
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
		return nil, platform.ErrMembershipRequired
	}
	return filtered, nil
}
func filterBidsForActor(rfq platform.RFQ, bids []platform.Bid, actor iamclient.Actor) ([]platform.Bid, error) {
	if len(actor.Memberships) == 0 {
		return nil, platform.ErrMembershipRequired
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
		return nil, platform.ErrMembershipRequired
	}
	filtered := make([]platform.Bid, 0, len(bids))
	for _, bid := range bids {
		if _, ok := providerOrgIDs[bid.ProviderOrgID]; ok {
			filtered = append(filtered, bid)
		}
	}
	return filtered, nil
}

// Route predicates — used in ServeHTTP to avoid fragile HasSuffix matching.
// Each function validates that the path matches the expected structure.
func isRFQBidsPath(path string) bool {
	_, err := rfqIDFromBidsPath(path)
	return err == nil
}
func isRFQAwardPath(path string) bool {
	_, err := rfqIDFromAwardPath(path)
	return err == nil
}
func isOrderSettlePath(path string) bool {
	_, _, err := orderMilestoneFromSettlePath(path)
	return err == nil
}
func isOrderUsagePath(path string) bool {
	_, _, err := orderMilestoneFromUsagePath(path)
	return err == nil
}
func isOrderDisputesPath(path string) bool {
	_, err := orderIDFromDisputePath(path)
	return err == nil
}
func isDisputeResolvePath(path string) bool {
	_, err := disputeIDFromResolvePath(path)
	return err == nil
}
func isOrderRatingPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "rating"
}
func orderIDFromRatingPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[4] != "rating" {
		return "", errors.New("invalid rating path")
	}
	return parts[3], nil
}
func (s *Server) handleRateOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromRatingPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		order, err := s.app.GetOrder(orderID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	var payload struct {
		Score   int    `json:"score"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if verr := validation.New().
		Range("score", int64(payload.Score), 1, 5).
		Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	rating, err := s.app.RateOrder(orderID, platform.RateOrderInput{
		Score:   payload.Score,
		Comment: payload.Comment,
	})
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"rating": rating})
}
func isRFQMessagesPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "rfqs" && parts[4] == "messages"
}
func rfqIDFromMessagesPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[4] != "messages" {
		return "", errors.New("invalid rfq messages path")
	}
	return parts[3], nil
}
func (s *Server) handleListRFQMessages(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		// Verify actor is RFQ participant (buyer or bidding provider)
		rfqID, _ := rfqIDFromMessagesPath(r.URL.Path)
		rfq, err := s.app.GetRFQ(rfqID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if err := s.authorizeRFQForActor(rfq, actor); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	rfqID, err := rfqIDFromMessagesPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	messages, err := s.app.ListRFQMessages(rfqID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"messages": messages})
}
func (s *Server) handleCreateRFQMessage(w http.ResponseWriter, r *http.Request) {
	var actorID string
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		actorID = actor.UserID
		// Verify actor is RFQ participant
		rfqID, _ := rfqIDFromMessagesPath(r.URL.Path)
		rfq, err := s.app.GetRFQ(rfqID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if err := s.authorizeRFQForActor(rfq, actor); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	rfqID, err := rfqIDFromMessagesPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var payload struct {
		Author string `json:"author"`
		Body   string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	author := payload.Author
	if actorID != "" {
		author = actorID // Use authenticated actor instead of payload
	}
	if verr := validation.New().
		Required("body", payload.Body).
		Required("author", author).
		Build(); verr != nil {
		httputil.WriteErrorWithDetails(w, http.StatusBadRequest, httputil.ErrCodeValidation, "validation failed", verr.Fields)
		return
	}
	message, err := s.app.CreateRFQMessage(rfqID, author, payload.Body)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"message": message})
}
func isOrderMessagesPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "messages"
}
func (s *Server) handleListOrderMessages(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	orderID := parts[3]
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		order, err := s.app.GetOrder(orderID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	messages, err := s.app.ListOrderMessages(orderID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"messages": messages})
}
func isProviderPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "providers"
}
func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerID := parts[3]
	provider, err := s.app.GetProvider(providerID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"provider": provider})
}
func isListingPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "listings"
}
func (s *Server) handleGetListing(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	listing, err := s.app.GetListing(parts[3])
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"listing": listing})
}
func isRFQDetailPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// /api/v1/rfqs/:id — exactly 4 parts, no sub-resources
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "rfqs"
}
func (s *Server) handleGetRFQ(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	rfqID := parts[3]
	rfq, err := s.app.GetRFQ(rfqID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"rfq": rfq})
}
func isDisputeDetailPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "disputes"
}
func (s *Server) handleGetDispute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	disputeID := parts[3]
	dispute, err := s.app.GetDispute(disputeID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	// Collect evidence for the disputed order's jobs
	var evidence []any
	order, orderErr := s.app.GetOrder(dispute.OrderID)
	if orderErr == nil {
		for _, ms := range order.Milestones {
			if ms.ID == dispute.MilestoneID {
				binding, bindErr := s.carrier.GetBinding(order.ID, ms.ID)
				if bindErr == nil {
					jobs, _ := s.carrier.ListJobs(binding.ID)
					for _, job := range jobs {
						if ev, evErr := s.evidence.Get(job.ID); evErr == nil {
							evidence = append(evidence, ev)
						}
					}
				}
			}
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"dispute": dispute, "evidence": evidence})
}
func (s *Server) handleGetOrderRating(w http.ResponseWriter, r *http.Request) {
	orderID, err := orderIDFromRatingPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rating, err := s.app.GetOrderRating(orderID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"rating": rating})
}
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		if _, err := s.authenticatedActor(r); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"webhooks": s.webhooks.List()})
}
func (s *Server) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		if _, err := s.authenticatedActor(r); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	var payload struct {
		Target string `json:"target"`
		URL    string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if payload.Target == "" || payload.URL == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "target and url required"})
		return
	}
	s.webhooks.Register(payload.Target, payload.URL)
	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}
func (s *Server) handleUnregisterWebhook(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	target := parts[3]
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		// Only allow unregister if actor belongs to the target org
		if !actorBelongsToOrg(actor, target) {
			httputil.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "not authorized for this target"})
			return
		}
	}
	s.webhooks.Unregister(target)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "unregistered"})
}
func (s *Server) handleMarketplaceStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.app.GetMarketplaceStats()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, stats)
}
func isOrderBudgetPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "budget"
}
func (s *Server) handleOrderBudget(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	orderID := parts[3]
	budget, err := s.app.GetOrderBudget(orderID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, budget)
}
func isOrderProviderSettlementReservationPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "provider-settlement-reservation"
}
func (s *Server) handleGetOrderProviderSettlementReservation(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	orderID := parts[3]
	order, err := s.app.GetOrder(orderID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		if err := authorizeOrderForActor(order, actor); err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
	}
	reservation, err := s.app.GetProviderSettlementReservation(orderID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"reservation": reservation})
}
func isProviderRevenuePath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "providers" && parts[4] == "revenue"
}
func (s *Server) handleProviderRevenue(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerID := parts[3]
	revenue, err := s.app.GetProviderRevenue(providerID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, revenue)
}
func isOrderTimelinePath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "timeline"
}
func (s *Server) handleOrderTimeline(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	orderID := parts[3]
	timeline, err := s.app.GetOrderTimeline(orderID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"timeline": timeline})
}
func (s *Server) handleBatchOrderStatus(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		OrderIDs []string `json:"orderIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	statuses, err := s.app.BatchOrderStatus(payload.OrderIDs)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"orders": statuses})
}
func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	target := parts[3]
	if s.auth != nil && !iamclient.IsNoop(s.auth) {
		actor, err := s.authenticatedActor(r)
		if err != nil {
			httputil.WriteAuthError(w, err)
			return
		}
		// Verify actor belongs to the target org
		if !actorBelongsToOrg(actor, target) {
			httputil.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "not authorized for this target"})
			return
		}
	}
	notifications, err := s.app.ListNotifications(target)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

// webhookNotifierAdapter bridges platform.Notifier (string event) to notifications.WebhookService (EventType).
type webhookNotifierAdapter struct {
	svc *notifications.WebhookService
}

func (a *webhookNotifierAdapter) Send(event string, target string, payload map[string]any) error {
	return a.svc.Send(notifications.EventType(event), target, payload)
}
func (s *Server) handleExportOrders(w http.ResponseWriter, r *http.Request) {
	csv, err := s.app.ExportOrdersCSV()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=orders.csv")
	w.Write([]byte(csv))
}
func (s *Server) handleExportDisputes(w http.ResponseWriter, r *http.Request) {
	csv, err := s.app.ExportDisputesCSV()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=disputes.csv")
	w.Write([]byte(csv))
}
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]any{
		"version":   httputil.APIVersion,
		"endpoints": 49,
		"packages": map[string]string{
			"carrier":        "async execution protocol",
			"usageproof":     "HMAC usage proof verification",
			"reconciliation": "settlement deviation detection",
			"notifications":  "webhook + in-memory delivery",
			"discord":        "bot command handler",
		},
		"middleware": []string{
			"SecurityHeaders", "RequestID", "VersionHeader",
			"CORS", "Gzip", "Timeout", "AccessLog",
			"RateLimitHeaders", "LimitBody",
		},
	}
	httputil.WriteJSON(w, http.StatusOK, info)
}
func (s *Server) handleRateLimitConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]any{
		"policies": map[string]any{
			"iam.login.ip":         map[string]any{"limit": 5, "window": "1m"},
			"iam.signup.ip":        map[string]any{"limit": 3, "window": "1m"},
			"gateway.create_rfq":   map[string]any{"limit": 10, "window": "1m"},
			"gateway.create_bid":   map[string]any{"limit": 20, "window": "1m"},
			"gateway.create_order": map[string]any{"limit": 10, "window": "1m"},
		},
		"enforcement": s.rateLimiter != nil,
	}
	httputil.WriteJSON(w, http.StatusOK, config)
}
func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := s.app.GetProviderLeaderboard()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"leaderboard": entries})
}
func isApplicationReviewPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "provider-applications" && parts[4] == "review"
}
func (s *Server) handleSubmitApplication(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		OrgID        string   `json:"orgId"`
		Name         string   `json:"name"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	app, err := s.app.SubmitProviderApplication(payload.OrgID, payload.Name, payload.Capabilities)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"application": app})
}
func (s *Server) handleListApplications(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	apps := s.app.ListProviderApplications(status)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"applications": apps})
}
func (s *Server) handleReviewApplication(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	appID := parts[3]
	var payload struct {
		ReviewedBy string `json:"reviewedBy"`
		Note       string `json:"note"`
		Approve    bool   `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	app, err := s.app.ReviewProviderApplication(appID, payload.ReviewedBy, payload.Note, payload.Approve)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"application": app})
}
func (s *Server) handleCarrierCallback(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	event, isIntegrationCallback, err := parseCarrierCallback(rawBody)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if isIntegrationCallback {
		if sig := strings.TrimSpace(r.Header.Get("X-Carrier-Signature")); sig != "" {
			callbackSecret, resolveErr := s.resolveCarrierCallbackSecret(r, event)
			if resolveErr != nil {
				writeCallbackError(w, http.StatusUnauthorized, httputil.ErrCodeUnauthorized, resolveErr.Error())
				return
			}
			if verifyErr := carrier.VerifyIntegrationCallbackBody(callbackSecret, rawBody, sig); verifyErr != nil {
				writeCallbackError(w, http.StatusUnauthorized, httputil.ErrCodeUnauthorized, verifyErr.Error())
				return
			}
		}
	} else {
		s.applyCarrierCallbackAuthHeaders(&event, r)

		// Verify callback signature (binding/provider secret preferred, then env fallback)
		callbackSecret, err := s.resolveCarrierCallbackSecret(r, event)
		if err != nil {
			writeCallbackError(w, http.StatusUnauthorized, httputil.ErrCodeUnauthorized, err.Error())
			return
		}
		if err := carrier.VerifyCallback(callbackSecret, event); err != nil {
			writeCallbackError(w, http.StatusUnauthorized, httputil.ErrCodeUnauthorized, err.Error())
			return
		}
	}

	// Normalize legacy snake_case event names to canonical dot-separated
	event.Type = carrier.NormalizeEventName(event.Type)
	// Ledger: check for replay/gap/reorder
	eventID, _ := event.Payload["eventId"].(string)
	sequence, _ := event.Payload["sequence"].(float64)
	if eventID != "" {
		ledgerEvent, replay, ledgerErr := s.ledger.Record(
			event.JobID, eventID, int64(sequence),
			event.Type, "", "", "",
		)
		if ledgerErr != nil {
			// Gap or reorder — tell Carrier to redeliver
			httputil.WriteJSON(w, http.StatusConflict, map[string]any{
				"error": ledgerErr.Error(), "accepted": false, "code": httputil.ErrCodeConflict,
			})
			return
		}
		if replay {
			// Return previous decision
			httputil.WriteJSON(w, http.StatusOK, map[string]any{
				"accepted": true, "replay": true, "decision": ledgerEvent.DecisionJSON,
			})
			return
		}
	}

	// Process callback based on type
	recommendedAction := RecommendedAction{Type: "continue"}
	switch event.Type {
	case "job.started", "execution.started", "milestone.started":
		if _, err := s.carrier.StartJob(event.JobID); err != nil {
			writeGatewayError(w, err)
			return
		}
	case "job.completed", "execution.completed":
		output, _ := event.Payload["output"].(string)
		if _, err := s.carrier.CompleteJob(event.JobID, output); err != nil {
			writeGatewayError(w, err)
			return
		}
	case "milestone.ready":
		// Milestone-ready event often carries end-of-work artifacts. Persist evidence first, then mark completion.
		if _, err := s.recordArtifactFromCallback(event); err != nil {
			if !isEvidenceAlreadySubmittedError(err) {
				writeGatewayError(w, err)
				return
			}
		}
		if _, err := s.carrier.CompleteJob(event.JobID, eventPayloadString(event.Payload, "output")); err != nil {
			writeGatewayError(w, err)
			return
		}
		job, err := s.carrier.GetJob(event.JobID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		binding, err := s.carrier.GetBindingByID(job.BindingID)
		if err != nil {
			writeGatewayError(w, err)
			return
		}
		if _, _, err := s.app.SettleMilestone(binding.OrderID, platform.SettleMilestoneInput{
			MilestoneID: job.MilestoneID,
			Summary:     eventPayloadString(event.Payload, "summary"),
			Source:      "carrier",
			OccurredAt:  time.Now().UTC(),
		}); err != nil {
			writeGatewayError(w, err)
			return
		}
	case "job.failed", "execution.failed":
		errMsg, _ := event.Payload["error"].(string)
		if _, err := s.carrier.FailJob(event.JobID, errMsg); err != nil {
			writeGatewayError(w, err)
			return
		}
	case "heartbeat", "execution.heartbeat":
		if err := s.carrier.Heartbeat(event.BindingID); err != nil {
			writeGatewayError(w, err)
			return
		}
	case "execution.pause_requested", "execution.paused":
		if _, err := s.carrier.CancelJob(event.JobID); err != nil {
			writeGatewayError(w, err)
			return
		}
		recommendedAction = RecommendedAction{Type: "pause", Reason: "pause requested by carrier"}
	case "execution.resumed":
		// Resumption is best-effort and currently maps to continue.
		_ = event.JobID
	case "usage.reported":
		if _, _, err := s.recordUsageFromCallback(event); err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		recommendedAction = eventUsageBasedAction(event.Payload)
	case "artifact.ready":
		if _, err := s.recordArtifactFromCallback(event); err != nil {
			if !isEvidenceAlreadySubmittedError(err) {
				writeGatewayError(w, err)
				return
			}
		}
		recommendedAction = eventBudgetRecommendedAction(event.Payload)
	case "budget.low":
		// Best effort: parse requested recommendation if provided by carrier.
		recommendedAction = eventBudgetRecommendedAction(event.Payload)
	default:
		writeCallbackError(w, http.StatusBadRequest, httputil.ErrCodeBadRequest, "unknown callback type")
		return
	}

	// Build callback response with recommendedAction
	response := s.buildCallbackResponse(event)
	if recommendedAction.Type != "" {
		response.RecommendedAction = recommendedAction
		if recommendedAction.Type != "continue" {
			response.ContinueAllowed = false
		}
	}

	// Store decision in ledger for replay
	if eventID != "" {
		decisionBytes, _ := json.Marshal(response.RecommendedAction)
		s.ledger.UpdateDecision(eventID, string(decisionBytes))
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

func writeCallbackError(w http.ResponseWriter, status int, code string, message string) {
	httputil.WriteJSON(w, status, map[string]any{"error": message, "code": code})
}

func (s *Server) applyCarrierCallbackAuthHeaders(event *carrier.CallbackEvent, r *http.Request) {
	if event == nil {
		return
	}

	if sig := strings.TrimSpace(r.Header.Get("X-One-Tok-Signature")); sig != "" {
		event.Signature = sig
	}

	if ts := strings.TrimSpace(r.Header.Get("X-One-Tok-Timestamp")); ts != "" {
		event.Timestamp = ts
	}
}

func (s *Server) resolveCarrierCallbackSecret(r *http.Request, event carrier.CallbackEvent) (string, error) {
	if overrideSecret := strings.TrimSpace(r.Header.Get("X-One-Tok-Callback-Secret")); overrideSecret != "" {
		return overrideSecret, nil
	}

	overrideKeyID := strings.TrimSpace(r.Header.Get("X-One-Tok-Callback-Key-Id"))
	if overrideKeyID == "" {
		overrideKeyID = strings.TrimSpace(r.Header.Get("X-One-Tok-Key-Id"))
	}
	if overrideKeyID == "" {
		overrideKeyID = strings.TrimSpace(r.Header.Get("X-Carrier-Key-Id"))
	}
	if event.BindingID == "" {
		if overrideKeyID != "" {
			return "", fmt.Errorf("callback key id provided but no binding secret configured")
		}
		return os.Getenv("CARRIER_CALLBACK_SECRET"), nil
	}

	binding, err := s.carrier.GetBindingByID(event.BindingID)
	if err != nil {
		if overrideKeyID != "" {
			return "", fmt.Errorf("callback key id provided but no binding secret configured")
		}
		return os.Getenv("CARRIER_CALLBACK_SECRET"), nil
	}

	order, err := s.app.GetOrder(binding.OrderID)
	if err != nil {
		if overrideKeyID != "" {
			return "", fmt.Errorf("callback key id provided but no binding secret configured")
		}
		return os.Getenv("CARRIER_CALLBACK_SECRET"), nil
	}

	providerBinding, err := s.app.GetProviderCarrierBinding(order.ProviderOrgID)
	if err != nil {
		if overrideKeyID != "" {
			return "", fmt.Errorf("callback key id provided but no binding secret configured")
		}
		return os.Getenv("CARRIER_CALLBACK_SECRET"), nil
	}

	if overrideKeyID != "" && providerBinding.CallbackKeyID != "" && overrideKeyID != providerBinding.CallbackKeyID {
		return "", fmt.Errorf("unknown callback key id")
	}

	secret := strings.TrimSpace(providerBinding.CallbackSecret)
	if secret == "" && overrideKeyID != "" {
		return "", fmt.Errorf("callback key id provided but no binding secret configured")
	}

	if secret != "" {
		return secret, nil
	}

	return os.Getenv("CARRIER_CALLBACK_SECRET"), nil
}

func parseCarrierCallback(rawBody []byte) (carrier.CallbackEvent, bool, error) {
	var integrationEnvelope carrier.IntegrationCallbackEnvelope
	if err := json.Unmarshal(rawBody, &integrationEnvelope); err == nil {
		if strings.TrimSpace(integrationEnvelope.EventType) != "" ||
			strings.TrimSpace(integrationEnvelope.EventID) != "" ||
			strings.TrimSpace(integrationEnvelope.CarrierExecutionID) != "" {
			return integrationEnvelope.ToCallbackEvent(), true, nil
		}
	}

	var event carrier.CallbackEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		return carrier.CallbackEvent{}, false, err
	}
	return event, false, nil
}

func (s *Server) handleCreateListing(w http.ResponseWriter, r *http.Request) {
	var payload platform.CreateListingInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	listing, err := s.app.CreateListing(payload)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"listing": listing})
}
func isOrderTopUpPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "top-up"
}
func (s *Server) handleTopUpMilestone(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	orderID := parts[3]
	var payload platform.TopUpInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	order, err := s.app.TopUpMilestone(orderID, payload)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"order": order})
}
func isCarrierBindingPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "carrier-bindings"
}
func isCarrierBindingVerifyPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[2] == "carrier-bindings" && parts[4] == "verify"
}
func isCarrierBindingSuspendPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[2] == "carrier-bindings" && parts[4] == "suspend"
}
func isProviderSettlementBindingPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "provider-settlement-bindings"
}
func isProviderSettlementBindingPoolPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "provider-settlement-bindings" && parts[4] == "pool"
}
func isProviderSettlementBindingVerifyPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[2] == "provider-settlement-bindings" && parts[4] == "verify"
}
func isProviderSettlementBindingSuspendPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[2] == "provider-settlement-bindings" && parts[4] == "suspend"
}
func isProviderSettlementBindingDisconnectPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[2] == "provider-settlement-bindings" && parts[4] == "disconnect"
}
func isProviderSettlementBindingRecoverPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[2] == "provider-settlement-bindings" && parts[4] == "recover"
}
func (s *Server) handleRegisterCarrierBinding(w http.ResponseWriter, r *http.Request) {
	var input platform.ProviderCarrierBinding
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	binding, err := s.app.RegisterCarrierBinding(input)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeCarrierBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"binding": binding})
}
func sanitizeCarrierBindingForPublic(b platform.ProviderCarrierBinding) platform.ProviderCarrierBinding {
	b.IntegrationToken = ""
	b.CallbackSecret = ""
	return b
}

func sanitizeProviderSettlementBindingForPublic(b platform.ProviderSettlementBinding) platform.ProviderSettlementBinding {
	b.OwnershipProof = ""
	return b
}

func (s *Server) handleGetCarrierBinding(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerOrgID := parts[3]
	binding, err := s.app.GetProviderCarrierBinding(providerOrgID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeCarrierBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding})
}
func (s *Server) handleVerifyCarrierBinding(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	bindingID := parts[3]
	binding, err := s.app.VerifyCarrierBinding(bindingID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeCarrierBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding})
}

func (s *Server) handleSuspendCarrierBinding(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	bindingID := parts[3]
	binding, err := s.app.SuspendCarrierBinding(bindingID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeCarrierBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding})
}

func (s *Server) handleRegisterProviderSettlementBinding(w http.ResponseWriter, r *http.Request) {
	var input platform.ProviderSettlementBinding
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	binding, err := s.app.RegisterProviderSettlementBinding(input)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeProviderSettlementBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"binding": binding})
}

func (s *Server) handleGetProviderSettlementBinding(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerOrgID := parts[3]
	binding, err := s.app.GetProviderSettlementBinding(providerOrgID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeProviderSettlementBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding})
}

func (s *Server) handleVerifyProviderSettlementBinding(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	bindingID := parts[3]
	binding, err := s.app.VerifyProviderSettlementBinding(bindingID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeProviderSettlementBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding})
}

func (s *Server) handleSuspendProviderSettlementBinding(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	bindingID := parts[3]
	binding, err := s.app.SuspendProviderSettlementBinding(bindingID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	binding = sanitizeProviderSettlementBindingForPublic(binding)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding})
}

func (s *Server) handleDisconnectProviderSettlementBinding(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeExecutionMutation(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerOrgID := parts[3]
	var payload struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
	}
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "provider settlement disconnected"
	}
	if err := s.app.ReportProviderSettlementDisconnect(providerOrgID, reason); err != nil {
		writeGatewayError(w, err)
		return
	}
	pool, err := s.app.GetProviderSettlementPool(providerOrgID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"pool": pool})
}

func (s *Server) handleRecoverProviderSettlementBinding(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeExecutionMutation(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerOrgID := parts[3]
	pool, err := s.app.RecoverProviderSettlement(providerOrgID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"pool": pool})
}

func (s *Server) handleGetProviderSettlementPool(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	providerOrgID := parts[3]
	pool, err := s.app.GetProviderSettlementPool(providerOrgID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"pool": pool})
}

// authorizeRFQForActor verifies the actor is a participant in the RFQ.
// Participants: buyer org, awarded provider, bidding providers, or ops roles.
func (s *Server) authorizeRFQForActor(rfq platform.RFQ, actor iamclient.Actor) error {
	// Ops roles can access all RFQs
	for _, m := range actor.Memberships {
		if isOpsRole(m.Role) {
			return nil
		}
	}
	// Buyer org match (buyer roles)
	for _, m := range actor.Memberships {
		if m.OrganizationID == rfq.BuyerOrgID && isBuyerRole(m.Role) {
			return nil
		}
	}
	// Awarded provider match
	if rfq.AwardedProviderOrgID != "" {
		for _, m := range actor.Memberships {
			if m.OrganizationID == rfq.AwardedProviderOrgID && isProviderRole(m.Role) {
				return nil
			}
		}
	}
	// Bidding providers — check if any of actor's orgs have submitted a bid
	bids, err := s.app.ListBids(rfq.ID)
	if err == nil {
		for _, bid := range bids {
			for _, m := range actor.Memberships {
				if m.OrganizationID == bid.ProviderOrgID && isProviderRole(m.Role) {
					return nil
				}
			}
		}
	}
	return platform.ErrOrgMismatch
}
func actorBelongsToOrg(actor iamclient.Actor, orgID string) bool {
	for _, m := range actor.Memberships {
		if m.OrganizationID == orgID {
			return true
		}
	}
	// Ops can access any org
	for _, m := range actor.Memberships {
		if isOpsRole(m.Role) {
			return true
		}
	}
	return false
}

func (s *Server) handleStaleJobs(w http.ResponseWriter, r *http.Request) {
	stale := s.carrier.ReconcileStaleJobs()
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"staleJobs": stale, "count": len(stale)})
}

func isGetCreditLimitPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "credit-limits"
}

func (s *Server) handleSetCreditLimit(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BuyerOrgID string `json:"buyerOrgId"`
		LimitCents int64  `json:"limitCents"`
		SetBy      string `json:"setBy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	limit := s.app.SetCreditLimit(payload.BuyerOrgID, payload.LimitCents, payload.SetBy)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"creditLimit": limit})
}

func (s *Server) handleGetCreditLimit(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	buyerOrgID := parts[3]
	limit, ok := s.app.GetCreditLimit(buyerOrgID)
	if !ok {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "no credit limit set"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"creditLimit": limit})
}

// CallbackResponse is the response sent back to Carrier after processing a callback.
type CallbackResponse struct {
	Accepted          bool              `json:"accepted"`
	ContinueAllowed   bool              `json:"continueAllowed"`
	RecommendedAction RecommendedAction `json:"recommendedAction"`
}

type RecommendedAction struct {
	Type   string `json:"type"` // continue, pause, cancel
	Reason string `json:"reason,omitempty"`
}

func (s *Server) buildCallbackResponse(event carrier.CallbackEvent) CallbackResponse {
	resp := CallbackResponse{
		Accepted:          true,
		ContinueAllowed:   true,
		RecommendedAction: RecommendedAction{Type: "continue"},
	}

	if event.JobID == "" {
		return resp
	}

	job, err := s.carrier.GetJob(event.JobID)
	if err != nil || job.State == carrier.JobStateCompleted || job.State == carrier.JobStateFailed || job.State == carrier.JobStateCancelled {
		return resp
	}

	// Pause if milestone/order budget was exceeded.
	binding, err := s.carrier.GetBindingByID(job.BindingID)
	if err == nil {
		if order, err := s.app.GetOrder(binding.OrderID); err == nil {
			if order.Status == core.OrderStatusAwaitingBudget {
				resp.RecommendedAction = RecommendedAction{Type: "pause", Reason: "order budget exhausted"}
				resp.ContinueAllowed = false
			}
		}
	}

	// Check if binding is stale
	stale, _ := s.carrier.IsStale(job.BindingID)
	if stale {
		resp.RecommendedAction = RecommendedAction{Type: "cancel", Reason: "carrier heartbeat stale"}
		resp.ContinueAllowed = false
	}

	return resp
}

func (s *Server) recordUsageFromCallback(event carrier.CallbackEvent) (*core.Order, core.UsageCharge, error) {
	job, err := s.carrier.GetJob(event.JobID)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}
	binding, err := s.carrier.GetBindingByID(job.BindingID)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}

	kind := jobInputDefaultUsageKind(event.Payload)
	amountCents, ok := getPayloadInt64(event.Payload, "amountCents")
	if !ok {
		return nil, core.UsageCharge{}, fmt.Errorf("invalid usage amount")
	}
	proofRef, proofSig, proofTS, err := parseUsageProofPayload(event.Payload)
	if err != nil {
		return nil, core.UsageCharge{}, err
	}

	if err := s.verifyUsageProofBindingSecret(&job, binding, kind, amountCents, proofRef, proofSig, proofTS); err != nil {
		return nil, core.UsageCharge{}, err
	}

	return s.app.RecordUsageCharge(binding.OrderID, platform.RecordUsageChargeInput{
		MilestoneID:    job.MilestoneID,
		Kind:           kind,
		AmountCents:    amountCents,
		ProofRef:       proofRef,
		ProofSignature: proofSig,
		ProofTimestamp: proofTS,
	})
}

func parseUsageProofPayload(payload map[string]any) (proofRef string, proofSig string, proofTS string, err error) {
	proofRef, _ = payload["proofRef"].(string)
	proofSig, _ = payload["proofSignature"].(string)
	proofTS, _ = payload["proofTimestamp"].(string)

	hasAll := strings.TrimSpace(proofRef) != "" || strings.TrimSpace(proofSig) != "" || strings.TrimSpace(proofTS) != ""
	if !hasAll {
		return "", "", "", nil
	}
	if strings.TrimSpace(proofRef) == "" || strings.TrimSpace(proofSig) == "" || strings.TrimSpace(proofTS) == "" {
		return "", "", "", fmt.Errorf("invalid usage proof payload: proofRef, proofSignature and proofTimestamp are required together")
	}
	if _, err = time.Parse(time.RFC3339, proofTS); err != nil {
		return "", "", "", fmt.Errorf("invalid usage proof payload: proofTimestamp must be RFC3339")
	}
	return strings.TrimSpace(proofRef), strings.TrimSpace(proofSig), strings.TrimSpace(proofTS), nil
}

func (s *Server) verifyUsageProofBindingSecret(job *carrier.ExecutionJob, binding carrier.Binding, kind core.UsageChargeKind, amountCents int64, proofRef, proofSig, proofTS string) error {
	if proofRef == "" && proofSig == "" && proofTS == "" {
		return nil
	}

	order, err := s.app.GetOrder(binding.OrderID)
	if err != nil {
		return nil
	}

	providerBinding, err := s.app.GetProviderCarrierBinding(order.ProviderOrgID)
	if err != nil {
		return nil
	}

	secret := strings.TrimSpace(providerBinding.CallbackSecret)
	if secret == "" {
		secret = os.Getenv("CARRIER_CALLBACK_SECRET")
	}
	if secret == "" {
		return nil
	}

	return usageproof.Verify(secret, usageproof.Proof{
		ExecutionID: job.ID,
		MilestoneID: job.MilestoneID,
		Kind:        string(kind),
		AmountCents: amountCents,
		Timestamp:   proofTS,
		Signature:   proofSig,
	})
}

func jobInputDefaultUsageKind(payload map[string]any) core.UsageChargeKind {
	kind := core.UsageChargeKindExternalAPI
	if raw, ok := payload["kind"].(string); ok && raw != "" {
		kind = core.UsageChargeKind(raw)
	}
	if raw, ok := payload["usageKind"].(string); ok && raw != "" {
		kind = core.UsageChargeKind(raw)
	}
	return kind
}

func eventPayloadString(payload map[string]any, key string) string {
	if raw, ok := payload[key].(string); ok {
		return raw
	}
	return ""
}

func recordEvidenceFromCallbackPayload(summary string, rawArtifacts any, rawUsage any) ([]carrier.Artifact, *carrier.UsageReport, error) {
	var artifacts []carrier.Artifact
	if rawArtifacts != nil {
		artifactsArray, ok := rawArtifacts.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("invalid artifacts payload")
		}
		for _, item := range artifactsArray {
			artifactJSON, err := json.Marshal(item)
			if err != nil {
				return nil, nil, err
			}
			var artifact carrier.Artifact
			if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
				return nil, nil, err
			}
			artifacts = append(artifacts, artifact)
		}
	}

	// Keep summary as provided by caller; empty summary is valid when caller intentionally omits it.
	var usage *carrier.UsageReport
	if rawUsage != nil {
		usageJSON, err := json.Marshal(rawUsage)
		if err != nil {
			return nil, nil, err
		}
		var parsed carrier.UsageReport
		if err := json.Unmarshal(usageJSON, &parsed); err != nil {
			return nil, nil, err
		}
		usage = &parsed
	}
	return artifacts, usage, nil
}

func isEvidenceAlreadySubmittedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "evidence already submitted")
}

func (s *Server) recordArtifactFromCallback(event carrier.CallbackEvent) (carrier.EvidencePackage, error) {
	job, err := s.carrier.GetJob(event.JobID)
	if err != nil {
		return carrier.EvidencePackage{}, err
	}
	summary := eventPayloadString(event.Payload, "summary")
	artifacts, usage, err := recordEvidenceFromCallbackPayload(summary, event.Payload["artifacts"], event.Payload["usageReport"])
	if err != nil {
		return carrier.EvidencePackage{}, err
	}
	if _, ok := event.Payload["usageSnapshot"].(map[string]any); ok {
		usageSnapshot := event.Payload["usageSnapshot"].(map[string]any)
		if usage == nil {
			if usageSnapshot != nil {
				usageJSON, err := json.Marshal(usageSnapshot)
				if err != nil {
					return carrier.EvidencePackage{}, err
				}
				var parsed carrier.UsageReport
				if err := json.Unmarshal(usageJSON, &parsed); err != nil {
					return carrier.EvidencePackage{}, err
				}
				usage = &parsed
			}
		}
	}

	if len(artifacts) == 0 && strings.TrimSpace(summary) == "" && usage == nil {
		return carrier.EvidencePackage{}, fmt.Errorf("artifact.ready requires evidence payload")
	}

	return s.evidence.Submit(job.ID, job.BindingID, summary, artifacts, usage)
}

func eventUsageBasedAction(payload map[string]any) RecommendedAction {
	// If carrier asks to pause after usage update, pass through explicit recommendation.
	if action, ok := payload["recommendedAction"].(string); ok && action != "" {
		return RecommendedAction{Type: action}
	}
	if pause, ok := payload["pause"].(bool); ok && pause {
		return RecommendedAction{Type: "pause", Reason: eventPayloadString(payload, "reason")}
	}
	return RecommendedAction{Type: "continue"}
}

func eventBudgetRecommendedAction(payload map[string]any) RecommendedAction {
	if action, ok := payload["recommendedAction"].(string); ok && action != "" {
		reason, _ := payload["reason"].(string)
		return RecommendedAction{Type: action, Reason: reason}
	}
	if pause, ok := payload["pause"].(bool); ok && pause {
		reason, _ := payload["reason"].(string)
		return RecommendedAction{Type: "pause", Reason: reason}
	}
	return RecommendedAction{Type: "continue"}
}

func getPayloadInt64(payload map[string]any, key string) (int64, bool) {
	if v, ok := payload[key]; ok {
		switch vv := v.(type) {
		case int:
			return int64(vv), true
		case int64:
			return vv, true
		case float64:
			return int64(vv), true
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(vv), 10, 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func (s *Server) handleFiberExposure(w http.ResponseWriter, r *http.Request) {
	exposure, err := s.app.GetFiberExposure()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, exposure)
}

func (s *Server) handleExportRFQs(w http.ResponseWriter, r *http.Request) {
	csv, err := s.app.ExportRFQsCSV()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=rfqs.csv")
	w.Write([]byte(csv))
}

func (s *Server) handleExportApplications(w http.ResponseWriter, r *http.Request) {
	csv := s.app.ExportProviderApplicationsCSV()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=applications.csv")
	w.Write([]byte(csv))
}
