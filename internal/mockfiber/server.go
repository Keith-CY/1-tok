package mockfiber

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type Server struct {
	mu            sync.Mutex
	invoiceSeq    int
	withdrawalSeq int
	items         []fiberclient.SettledFeedItem
	withdrawals   []fiberclient.WithdrawalStatusItem
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mock-fiber"})
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	var payload struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeRPCError(w, payload.ID, -32700, "invalid json")
		return
	}

	switch payload.Method {
	case "tip.create":
		s.handleCreate(w, payload.ID, payload.Params)
	case "withdrawal.quote":
		s.handleQuoteWithdrawal(w, payload.ID, payload.Params)
	case "withdrawal.request":
		s.handleRequestWithdrawal(w, payload.ID, payload.Params)
	case "tip.settled_feed":
		s.handleSettledFeed(w, payload.ID)
	case "dashboard.summary":
		s.handleDashboardSummary(w, payload.ID, payload.Params)
	default:
		writeRPCError(w, payload.ID, -32601, "method not found")
	}
}

func (s *Server) handleCreate(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input fiberclient.CreateInvoiceInput
	if err := json.Unmarshal(raw, &input); err != nil {
		writeRPCError(w, id, -32602, "invalid params")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.invoiceSeq++
	invoice := fmt.Sprintf("inv_mock_%d", s.invoiceSeq)
	s.items = append(s.items, fiberclient.SettledFeedItem{
		TipIntentID: fmt.Sprintf("tip_mock_%d", s.invoiceSeq),
		PostID:      input.PostID,
		Invoice:     invoice,
		Amount:      input.Amount,
		Asset:       input.Asset,
		FromUserID:  input.FromUserID,
		ToUserID:    input.ToUserID,
		Message:     input.Message,
		SettledAt:   time.Now().UTC().Format(time.RFC3339),
	})

	writeRPCResult(w, id, fiberclient.CreateInvoiceResult{Invoice: invoice})
}

func (s *Server) handleSettledFeed(w http.ResponseWriter, id any) {
	s.mu.Lock()
	items := append([]fiberclient.SettledFeedItem(nil), s.items...)
	s.mu.Unlock()

	writeRPCResult(w, id, fiberclient.SettledFeedResult{
		Items: items,
	})
}

func (s *Server) handleQuoteWithdrawal(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input fiberclient.QuotePayoutInput
	if err := json.Unmarshal(raw, &input); err != nil {
		writeRPCError(w, id, -32602, "invalid params")
		return
	}
	if input.UserID == "" || input.Asset == "" || input.Amount == "" || input.Destination.Kind == "" {
		writeRPCError(w, id, -32602, "missing required fields")
		return
	}

	writeRPCResult(w, id, fiberclient.QuotePayoutResult{
		Asset:            input.Asset,
		Amount:           input.Amount,
		MinimumAmount:    "1",
		AvailableBalance: "1000",
		LockedBalance:    "0",
		NetworkFee:       "0.1",
		ReceiveAmount:    input.Amount,
		DestinationValid: true,
	})
}

func (s *Server) handleRequestWithdrawal(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input fiberclient.RequestPayoutInput
	if err := json.Unmarshal(raw, &input); err != nil {
		writeRPCError(w, id, -32602, "invalid params")
		return
	}
	if input.UserID == "" || input.Asset == "" || input.Amount == "" || input.Destination.Kind == "" {
		writeRPCError(w, id, -32602, "missing required fields")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.withdrawalSeq++
	withdrawal := fiberclient.WithdrawalStatusItem{
		ID:     fmt.Sprintf("wd_mock_%d", s.withdrawalSeq),
		UserID: input.UserID,
		Asset:  input.Asset,
		Amount: input.Amount,
		State:  "PROCESSING",
	}
	s.withdrawals = append(s.withdrawals, withdrawal)

	writeRPCResult(w, id, fiberclient.RequestPayoutResult{
		ID:    withdrawal.ID,
		State: "PENDING",
	})
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		writeRPCError(w, id, -32602, "invalid params")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	withdrawals := make([]fiberclient.WithdrawalStatusItem, 0, len(s.withdrawals))
	for _, withdrawal := range s.withdrawals {
		if input.UserID == "" || withdrawal.UserID == input.UserID {
			withdrawals = append(withdrawals, withdrawal)
		}
	}

	writeRPCResult(w, id, map[string]any{
		"admin": map[string]any{
			"withdrawals": withdrawals,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeRPCError(w http.ResponseWriter, id any, code int, message string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
