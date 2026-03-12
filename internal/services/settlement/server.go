package settlement

import (
	"encoding/json"
	"errors"
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
		Upstream: upstream(),
		Fiber:    fiberclient.NewClientFromEnv(),
		Auth:     iamclient.NewClientFromEnv(),
	})
}

func NewServerWithOptions(options Options) *Server {
	if options.Upstream == "" {
		if runtimeconfig.RequireExternalDependencies() && strings.TrimSpace(os.Getenv("API_GATEWAY_UPSTREAM")) == "" {
			panic("API_GATEWAY_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
		options.Upstream = upstream()
	}
	if options.Fiber == nil {
		if runtimeconfig.RequireExternalDependencies() {
			if strings.TrimSpace(os.Getenv("FIBER_RPC_URL")) == "" {
				panic("FIBER_RPC_URL is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
			if strings.TrimSpace(os.Getenv("FIBER_APP_ID")) == "" {
				panic("FIBER_APP_ID is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
			if strings.TrimSpace(os.Getenv("FIBER_HMAC_SECRET")) == "" {
				panic("FIBER_HMAC_SECRET is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
		}
		options.Fiber = fiberclient.NewClientFromEnv()
	}
	if options.Funding == nil {
		options.Funding = loadFundingRecordRepository()
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
		if options.Auth == nil {
			panic("IAM_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
		if options.ServiceTokens.Empty() {
			panic("SETTLEMENT_SERVICE_TOKEN or SETTLEMENT_SERVICE_TOKENS is required when ONE_TOK_REQUIRE_EXTERNALS=true")
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
	}
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
		writeAuthError(w, err)
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if payload.OrderID == "" || payload.BuyerOrgID == "" || payload.ProviderOrgID == "" || payload.Asset == "" || payload.Amount == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"invoice": result.Invoice})
}

func (s *Server) handleGetInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeInternalRoute(r); err != nil {
		writeAuthError(w, err)
		return
	}

	invoice, err := invoiceFromPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := s.fiber.GetInvoiceStatus(r.Context(), invoice)
	if err != nil {
		writeFiberError(w, err)
		return
	}
	if err := s.funding.UpdateInvoiceState(invoice, result.State); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"invoice": invoice,
		"state":   result.State,
	})
}

func (s *Server) handleQuoteWithdrawal(w http.ResponseWriter, r *http.Request) {
	input, err := parseWithdrawalRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	input.ProviderOrgID, err = s.resolveProviderOrg(r, input.ProviderOrgID)
	if err != nil {
		writeAuthError(w, err)
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

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	input, err := parseWithdrawalRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	input.ProviderOrgID, err = s.resolveProviderOrg(r, input.ProviderOrgID)
	if err != nil {
		writeAuthError(w, err)
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleListFundingRecords(w http.ResponseWriter, r *http.Request) {
	filter := FundingRecordFilter{
		Kind:          FundingRecordKind(r.URL.Query().Get("kind")),
		OrderID:       r.URL.Query().Get("orderId"),
		ProviderOrgID: r.URL.Query().Get("providerOrgId"),
	}
	if err := s.scopeFundingFilter(r, &filter); err != nil {
		writeAuthError(w, err)
		return
	}

	records, err := s.funding.List(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) handleSettledFeed(w http.ResponseWriter, r *http.Request) {
	if err := s.authorizeInternalRoute(r); err != nil {
		writeAuthError(w, err)
		return
	}

	input := fiberclient.SettledFeedInput{}
	if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
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
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleWithdrawalStatuses(w http.ResponseWriter, r *http.Request) {
	userID, err := s.resolveProviderOrg(r, strings.TrimSpace(r.URL.Query().Get("providerOrgId")))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing providerOrgId"})
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
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, result)
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
	if s.auth == nil {
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
		if membership.OrganizationKind == "provider" && isProviderFinanceRole(membership.Role) {
			if filter.ProviderOrgID != "" && filter.ProviderOrgID != membership.OrganizationID {
				return errors.New("provider org mismatch")
			}
			filter.ProviderOrgID = membership.OrganizationID
			return nil
		}
	}

	return errors.New("provider or ops membership is required")
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

func isOpsRole(role string) bool {
	switch role {
	case "ops_reviewer", "risk_admin", "finance_admin", "super_admin":
		return true
	default:
		return false
	}
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeFiberError(w http.ResponseWriter, err error) {
	if errors.Is(err, fiberclient.ErrNotConfigured) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
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

func (s *Server) authorizeInternalRoute(r *http.Request) error {
	if s.serviceTokens.Empty() {
		return nil
	}
	if s.serviceTokens.MatchesRequest(r) {
		return nil
	}
	return errors.New("invalid service token")
}

func upstream() string {
	if value := os.Getenv("API_GATEWAY_UPSTREAM"); value != "" {
		return value
	}
	return "http://127.0.0.1:8080"
}
