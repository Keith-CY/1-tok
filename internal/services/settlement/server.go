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
	"github.com/chenyu/1-tok/internal/services/proxy"
)

type Server struct {
	inner   http.Handler
	fiber   fiberclient.InvoiceClient
	funding FundingRecordRepository
}

type Options struct {
	Upstream string
	Fiber    fiberclient.InvoiceClient
	Funding  FundingRecordRepository
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		Upstream: upstream(),
		Fiber:    fiberclient.NewClientFromEnv(),
	})
}

func NewServerWithOptions(options Options) *Server {
	if options.Upstream == "" {
		options.Upstream = upstream()
	}
	if options.Fiber == nil {
		options.Fiber = fiberclient.NewClientFromEnv()
	}
	if options.Funding == nil {
		options.Funding = loadFundingRecordRepository()
	}

	return &Server{
		inner: proxy.NewSingleHost(options.Upstream, func(req *http.Request) {
			req.URL.Path = "/api/v1" + req.URL.Path[3:]
		}),
		fiber:   options.Fiber,
		funding: options.Funding,
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
	records, err := s.funding.List(FundingRecordFilter{
		Kind:          FundingRecordKind(r.URL.Query().Get("kind")),
		OrderID:       r.URL.Query().Get("orderId"),
		ProviderOrgID: r.URL.Query().Get("providerOrgId"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) handleSettledFeed(w http.ResponseWriter, r *http.Request) {
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
	userID := strings.TrimSpace(r.URL.Query().Get("providerOrgId"))
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
	if payload.ProviderOrgID == "" || payload.Asset == "" || payload.Amount == "" || payload.Destination.Kind == "" {
		return withdrawalRequest{}, errors.New("missing required fields")
	}
	return payload, nil
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

func upstream() string {
	if value := os.Getenv("API_GATEWAY_UPSTREAM"); value != "" {
		return value
	}
	return "http://127.0.0.1:8080"
}
