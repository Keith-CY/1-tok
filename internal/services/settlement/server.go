package settlement

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
	"github.com/chenyu/1-tok/internal/services/proxy"
)

type Server struct {
	inner http.Handler
	fiber fiberclient.InvoiceClient
}

type Options struct {
	Upstream string
	Fiber    fiberclient.InvoiceClient
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

	return &Server{
		inner: proxy.NewSingleHost(options.Upstream, func(req *http.Request) {
			req.URL.Path = "/api/v1" + req.URL.Path[3:]
		}),
		fiber: options.Fiber,
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

	writeJSON(w, http.StatusCreated, result)
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
