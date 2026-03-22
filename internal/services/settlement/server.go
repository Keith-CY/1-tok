package settlement

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/chenyu/1-tok/internal/serviceauth"
	"github.com/chenyu/1-tok/internal/services/proxy"
	"github.com/chenyu/1-tok/internal/httputil"
)

type Server struct {
	inner         http.Handler
	fiber         fiberclient.InvoiceClient
	funding       FundingRecordRepository
	auth          iamclient.Client
	serviceTokens serviceauth.TokenSet
}

type Options struct {
	Upstream      string
	Fiber         fiberclient.InvoiceClient
	Funding       FundingRecordRepository
	Auth          iamclient.Client
	ServiceToken  string
	ServiceTokens serviceauth.TokenSet
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		Upstream: runtimeconfig.APIGatewayUpstream(),
		Fiber:    fiberclient.NewClientFromEnv(),
		Auth:     iamclient.NewClientFromEnv(),
	})
}

func NewServerWithOptions(options Options) *Server {
	s, err := NewServerWithOptionsE(options)
	if err != nil {
		panic(fmt.Sprintf("settlement: %v", err))
	}
	return s
}

// NewServerWithOptionsE is the error-returning variant of NewServerWithOptions.
func NewServerWithOptionsE(options Options) (*Server, error) {
	if options.Upstream == "" {
		if runtimeconfig.RequireExternalDependencies() && strings.TrimSpace(os.Getenv("API_GATEWAY_UPSTREAM")) == "" {
			return nil, fmt.Errorf("API_GATEWAY_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
		options.Upstream = runtimeconfig.APIGatewayUpstream()
	}
	if options.Fiber == nil {
		if runtimeconfig.RequireExternalDependencies() {
			if strings.TrimSpace(os.Getenv("FIBER_RPC_URL")) == "" {
				return nil, fmt.Errorf("FIBER_RPC_URL is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
			if strings.TrimSpace(os.Getenv("FIBER_APP_ID")) == "" {
				return nil, fmt.Errorf("FIBER_APP_ID is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
			if strings.TrimSpace(os.Getenv("FIBER_HMAC_SECRET")) == "" {
				return nil, fmt.Errorf("FIBER_HMAC_SECRET is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
		}
		options.Fiber = fiberclient.NewClientFromEnv()
	}
	if options.Funding == nil {
		funding, err := loadFundingRecordRepositoryE()
		if err != nil {
			return nil, fmt.Errorf("funding store: %w", err)
		}
		options.Funding = funding
	}
	if options.Auth == nil {
		options.Auth = iamclient.NewClientFromEnv()
	}
	if options.ServiceTokens.Empty() {
		if options.ServiceToken != "" {
			options.ServiceTokens = serviceauth.NewTokenSet(options.ServiceToken)
		} else {
			options.ServiceTokens = serviceauth.FromEnv("SETTLEMENT_SERVICE_TOKENS", "SETTLEMENT_SERVICE_TOKEN")
		}
	}
	if runtimeconfig.RequireExternalDependencies() {
		if options.Auth == nil || iamclient.IsNoop(options.Auth) {
			return nil, fmt.Errorf("IAM_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
		if options.ServiceTokens.Empty() {
			return nil, fmt.Errorf("SETTLEMENT_SERVICE_TOKEN or SETTLEMENT_SERVICE_TOKENS is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
	}

	return &Server{
		inner: proxy.NewSingleHost(options.Upstream, func(req *http.Request) {
			req.URL.Path = "/api/v1" + req.URL.Path[3:]
		}),
		fiber:         options.Fiber,
		funding:       options.Funding,
		auth:          options.Auth,
		serviceTokens: options.ServiceTokens,
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"settlement"}`))
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/invoices" {
		s.handleCreateInvoice(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/topups" {
		s.handleCreateTopUp(w, r)
		return
	}

	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/invoices/") {
		s.handleGetInvoiceStatus(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/withdrawals/quote" {
		s.handleQuoteWithdrawal(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/withdrawals" {
		s.handleRequestWithdrawal(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/provider-payouts" {
		s.handleRequestProviderPayout(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records" {
		s.handleListFundingRecords(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed" {
		s.handleSettledFeed(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/withdrawals/status" {
		s.handleWithdrawalStatuses(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/v1/orders/") {
		s.inner.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeInternalRoute(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	var payload struct {
		OrderID       string `json:"orderId"`
		MilestoneID   string `json:"milestoneId"`
		BuyerOrgID    string `json:"buyerOrgId"`
		ProviderOrgID string `json:"providerOrgId"`
		Asset         string `json:"asset"`
		Amount        string `json:"amount"`
		Memo          string `json:"memo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if payload.OrderID == "" || payload.BuyerOrgID == "" || payload.ProviderOrgID == "" || payload.Asset == "" || payload.Amount == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	postID := payload.OrderID
	if payload.MilestoneID != "" {
		postID = payload.OrderID + ":" + payload.MilestoneID
	}

	result, err := s.fiber.CreateInvoice(r.Context(), fiberclient.CreateInvoiceInput{
		PostID:     postID,
		FromUserID: payload.BuyerOrgID,
		ToUserID:   payload.ProviderOrgID,
		Asset:      payload.Asset,
		Amount:     payload.Amount,
		Message:    payload.Memo,
	})
	if err != nil {
		writeFiberError(w, err)
		return
	}

	recordID, err := s.funding.NextID()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	now := time.Now().UTC()
	if err := s.funding.Save(FundingRecord{
		ID:            recordID,
		Kind:          FundingRecordKindInvoice,
		OrderID:       payload.OrderID,
		MilestoneID:   payload.MilestoneID,
		BuyerOrgID:    payload.BuyerOrgID,
		ProviderOrgID: payload.ProviderOrgID,
		Asset:         payload.Asset,
		Amount:        payload.Amount,
		Invoice:       result.Invoice,
		State:         "UNPAID",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"invoice": result.Invoice})
}

func (s *Server) handleCreateTopUp(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		BuyerOrgID string `json:"buyerOrgId"`
		Asset      string `json:"asset"`
		Amount     string `json:"amount"`
		Memo       string `json:"memo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(payload.Amount) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	buyerOrgID, err := s.resolveBuyerOrgOrInternal(r, payload.BuyerOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	recordID, err := s.funding.NextID()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	asset := defaultSettlementAsset(payload.Asset)
	result, err := s.fiber.CreateInvoice(r.Context(), fiberclient.CreateInvoiceInput{
		PostID:     "buyer_topup:" + buyerOrgID + ":" + recordID,
		FromUserID: buyerOrgID,
		ToUserID:   marketplaceTreasuryUserID(),
		Asset:      asset,
		Amount:     strings.TrimSpace(payload.Amount),
		Message:    strings.TrimSpace(payload.Memo),
	})
	if err != nil {
		writeFiberError(w, err)
		return
	}

	now := time.Now().UTC()
	if err := s.funding.Save(FundingRecord{
		ID:         recordID,
		Kind:       FundingRecordKindBuyerTopUp,
		BuyerOrgID: buyerOrgID,
		Asset:      asset,
		Amount:     strings.TrimSpace(payload.Amount),
		Invoice:    result.Invoice,
		State:      "UNPAID",
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]string{
		"invoice":  result.Invoice,
		"recordId": recordID,
		"asset":    asset,
	})
}

func (s *Server) handleGetInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeInternalRoute(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	invoice, err := invoiceFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := s.fiber.GetInvoiceStatus(r.Context(), invoice)
	if err != nil {
		writeFiberError(w, err)
		return
	}
	if err := s.funding.UpdateInvoiceState(invoice, result.State); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"invoice": invoice,
		"state":   result.State,
	})
}

func (s *Server) handleQuoteWithdrawal(w http.ResponseWriter, r *http.Request) {
	input, err := parseWithdrawalRequest(r)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	input.ProviderOrgID, err = s.resolveProviderOrg(r, input.ProviderOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	result, err := s.fiber.QuotePayout(r.Context(), fiberclient.QuotePayoutInput{
		UserID:      input.ProviderOrgID,
		Asset:       input.Asset,
		Amount:      input.Amount,
		Destination: input.Destination,
	})
	if err != nil {
		writeFiberError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

func (s *Server) handleRequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	input, err := parseWithdrawalRequest(r)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	input.ProviderOrgID, err = s.resolveProviderOrg(r, input.ProviderOrgID)
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	result, err := s.fiber.RequestPayout(r.Context(), fiberclient.RequestPayoutInput{
		UserID:      input.ProviderOrgID,
		Asset:       input.Asset,
		Amount:      input.Amount,
		Destination: input.Destination,
	})
	if err != nil {
		writeFiberError(w, err)
		return
	}

	recordID, err := s.funding.NextID()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	now := time.Now().UTC()
	destination := map[string]string{
		"kind": input.Destination.Kind,
	}
	if input.Destination.Address != "" {
		destination["address"] = input.Destination.Address
	}
	if input.Destination.PaymentRequest != "" {
		destination["paymentRequest"] = input.Destination.PaymentRequest
	}
	if err := s.funding.Save(FundingRecord{
		ID:            recordID,
		Kind:          FundingRecordKindWithdrawal,
		ProviderOrgID: input.ProviderOrgID,
		Asset:         input.Asset,
		Amount:        input.Amount,
		ExternalID:    result.ID,
		State:         result.State,
		Destination:   destination,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result)
}

func (s *Server) handleRequestProviderPayout(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeInternalRoute(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	var payload struct {
		OrderID        string `json:"orderId"`
		MilestoneID    string `json:"milestoneId"`
		BuyerOrgID     string `json:"buyerOrgId"`
		ProviderOrgID  string `json:"providerOrgId"`
		Asset          string `json:"asset"`
		Amount         string `json:"amount"`
		PaymentRequest string `json:"paymentRequest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(payload.ProviderOrgID) == "" || strings.TrimSpace(payload.Amount) == "" || strings.TrimSpace(payload.PaymentRequest) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	asset := defaultSettlementAsset(payload.Asset)
	result, err := s.fiber.RequestPayout(r.Context(), fiberclient.RequestPayoutInput{
		UserID: marketplaceTreasuryUserID(),
		Asset:  asset,
		Amount: strings.TrimSpace(payload.Amount),
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: strings.TrimSpace(payload.PaymentRequest),
		},
	})
	if err != nil {
		writeFiberError(w, err)
		return
	}

	recordID, err := s.funding.NextID()
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	now := time.Now().UTC()
	if err := s.funding.Save(FundingRecord{
		ID:            recordID,
		Kind:          FundingRecordKindProviderPayout,
		OrderID:       strings.TrimSpace(payload.OrderID),
		MilestoneID:   strings.TrimSpace(payload.MilestoneID),
		BuyerOrgID:    strings.TrimSpace(payload.BuyerOrgID),
		ProviderOrgID: strings.TrimSpace(payload.ProviderOrgID),
		Asset:         asset,
		Amount:        strings.TrimSpace(payload.Amount),
		ExternalID:    result.ID,
		State:         result.State,
		Destination: map[string]string{
			"kind":           "PAYMENT_REQUEST",
			"paymentRequest": strings.TrimSpace(payload.PaymentRequest),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":       result.ID,
		"state":    result.State,
		"recordId": recordID,
		"asset":    asset,
	})
}

func (s *Server) handleListFundingRecords(w http.ResponseWriter, r *http.Request) {
	filter := FundingRecordFilter{
		Kind:          FundingRecordKind(r.URL.Query().Get("kind")),
		OrderID:       r.URL.Query().Get("orderId"),
		BuyerOrgID:    r.URL.Query().Get("buyerOrgId"),
		ProviderOrgID: r.URL.Query().Get("providerOrgId"),
	}
	if err := s.scopeFundingFilter(r, &filter); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	records, err := s.funding.List(filter)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) handleSettledFeed(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeInternalRoute(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	input := fiberclient.SettledFeedInput{}
	if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		input.Limit = parsed
	}
	afterSettledAt := strings.TrimSpace(r.URL.Query().Get("afterSettledAt"))
	afterID := strings.TrimSpace(r.URL.Query().Get("afterId"))
	if afterSettledAt != "" || afterID != "" {
		input.After = &fiberclient.SettledFeedCursor{
			SettledAt: afterSettledAt,
			ID:        afterID,
		}
	}

	result, err := s.fiber.ListSettledFeed(r.Context(), input)
	if err != nil {
		writeFiberError(w, err)
		return
	}

	for _, item := range result.Items {
		if strings.TrimSpace(item.Invoice) == "" {
			continue
		}
		if err := s.funding.UpdateInvoiceState(item.Invoice, "SETTLED"); err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

func (s *Server) handleWithdrawalStatuses(w http.ResponseWriter, r *http.Request) {
	userID, err := s.resolveProviderOrg(r, strings.TrimSpace(r.URL.Query().Get("providerOrgId")))
	if err != nil {
		httputil.WriteAuthError(w, err)
		return
	}
	if userID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing providerOrgId"})
		return
	}

	result, err := s.fiber.ListWithdrawalStatuses(r.Context(), userID)
	if err != nil {
		writeFiberError(w, err)
		return
	}

	for _, item := range result.Withdrawals {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		if err := s.funding.UpdateExternalState(item.ID, item.State); err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

func invoiceFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "v1" || parts[1] != "invoices" || parts[2] == "" {
		return "", errors.New("invalid invoice path")
	}
	return parts[2], nil
}

type withdrawalRequest struct {
	ProviderOrgID string                            `json:"providerOrgId"`
	Asset         string                            `json:"asset"`
	Amount        string                            `json:"amount"`
	Destination   fiberclient.WithdrawalDestination `json:"destination"`
}

func parseWithdrawalRequest(r *http.Request) (withdrawalRequest, error) {
	var payload withdrawalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return withdrawalRequest{}, errors.New("invalid json")
	}
	if payload.Asset == "" || payload.Amount == "" || payload.Destination.Kind == "" {
		return withdrawalRequest{}, errors.New("missing required fields")
	}
	return payload, nil
}

func (s *Server) scopeFundingFilter(r *http.Request, filter *FundingRecordFilter) error {
	if s.auth == nil || iamclient.IsNoop(s.auth) {
		return nil
	}

	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return iamclient.ErrUnauthorized
	}

	actor, err := s.auth.GetActor(r.Context(), token)
	if err != nil {
		return err
	}

	for _, membership := range actor.Memberships {
		if membership.OrganizationKind == "ops" && isOpsRole(membership.Role) {
			return nil
		}
		if membership.OrganizationKind == "buyer" && isBuyerTreasuryRole(membership.Role) {
			if filter.BuyerOrgID != "" && filter.BuyerOrgID != membership.OrganizationID {
				return errors.New("buyer org mismatch")
			}
			filter.BuyerOrgID = membership.OrganizationID
			filter.ProviderOrgID = ""
			return nil
		}
		if membership.OrganizationKind == "provider" && isProviderFinanceRole(membership.Role) {
			if filter.ProviderOrgID != "" && filter.ProviderOrgID != membership.OrganizationID {
				return errors.New("provider org mismatch")
			}
			filter.ProviderOrgID = membership.OrganizationID
			return nil
		}
	}

	return errors.New("buyer, provider, or ops membership is required")
}

func bearerToken(header string) (string, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	return token, token != ""
}

func isProviderFinanceRole(role string) bool {
	switch role {
	case "org_owner", "sales", "delivery_operator", "finance_viewer":
		return true
	default:
		return false
	}
}

func isBuyerTreasuryRole(role string) bool {
	switch role {
	case "org_owner", "procurement", "operator":
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
		if membership.OrganizationKind == "ops" && isOpsRole(membership.Role) {
			return requestedProviderOrgID, nil
		}
		if membership.OrganizationKind == "provider" && isProviderFinanceRole(membership.Role) {
			if requestedProviderOrgID != "" && requestedProviderOrgID != membership.OrganizationID {
				return "", errors.New("provider org mismatch")
			}
			return membership.OrganizationID, nil
		}
	}

	return "", errors.New("provider or ops membership is required")
}

func (s *Server) resolveBuyerOrgOrInternal(r *http.Request, requestedBuyerOrgID string) (string, error) {
	if (!s.serviceTokens.Empty() && s.serviceTokens.MatchesRequest(r)) || (s.serviceTokens.Empty() && (s.auth == nil || iamclient.IsNoop(s.auth))) {
		if strings.TrimSpace(requestedBuyerOrgID) == "" {
			return "", errors.New("buyer org is required")
		}
		return strings.TrimSpace(requestedBuyerOrgID), nil
	}
	return s.resolveBuyerOrg(r, requestedBuyerOrgID)
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
		if membership.OrganizationKind == "ops" && isOpsRole(membership.Role) {
			return requestedBuyerOrgID, nil
		}
		if membership.OrganizationKind == "buyer" && isBuyerTreasuryRole(membership.Role) {
			if requestedBuyerOrgID != "" && requestedBuyerOrgID != membership.OrganizationID {
				return "", errors.New("buyer org mismatch")
			}
			return membership.OrganizationID, nil
		}
	}

	return "", errors.New("buyer or ops membership is required")
}


func writeFiberError(w http.ResponseWriter, err error) {
	if errors.Is(err, fiberclient.ErrNotConfigured) {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

func (s *Server) authorizeInternalRoute(r *http.Request) error {
	if s.serviceTokens.Empty() {
		return nil
	}
	if s.serviceTokens.MatchesRequest(r) {
		return nil
	}
	return serviceauth.ErrInvalidServiceToken
}

func marketplaceTreasuryUserID() string {
	if value := strings.TrimSpace(os.Getenv("MARKETPLACE_TREASURY_USER_ID")); value != "" {
		return value
	}
	return "platform_treasury"
}

func defaultSettlementAsset(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	if env := strings.TrimSpace(os.Getenv("MARKETPLACE_SETTLEMENT_ASSET")); env != "" {
		return env
	}
	return "USDI"
}
