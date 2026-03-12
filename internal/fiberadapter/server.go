package fiberadapter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

type Options struct {
	InvoiceRPCURL string
	PayerRPCURL   string
	InvoiceNode   rawNode
	PayerNode     rawNode
}

type Server struct {
	invoiceNode rawNode
	payerNode   rawNode

	mu              sync.Mutex
	tipSeq          int
	withdrawalSeq   int
	tips            map[string]*tipRecord
	withdrawals     []fiberclient.WithdrawalStatusItem
	withdrawalsByID map[string]*fiberclient.WithdrawalStatusItem
}

type tipRecord struct {
	Invoice    string
	PostID     string
	FromUserID string
	ToUserID   string
	Asset      string
	Amount     string
	Message    string
	State      string
	SettledAt  string
}

type rawNode interface {
	CreateInvoice(context.Context, string, string) (string, error)
	GetInvoiceStatus(context.Context, string) (string, error)
	ValidatePaymentRequest(context.Context, string) error
	SendPayment(context.Context, string, string, string, string) (string, error)
}

func NewServer() *Server {
	return NewServerWithOptions(Options{})
}

func NewServerWithOptions(options Options) *Server {
	if options.InvoiceNode == nil {
		endpoint := strings.TrimSpace(options.InvoiceRPCURL)
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("FNN_INVOICE_RPC_URL"))
		}
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("FNN_RPC_URL"))
		}
		options.InvoiceNode = newRPCNode(endpoint)
	}
	if options.PayerNode == nil {
		endpoint := strings.TrimSpace(options.PayerRPCURL)
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("FNN_PAYER_RPC_URL"))
		}
		if endpoint == "" {
			endpoint = strings.TrimSpace(options.InvoiceRPCURL)
		}
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("FNN_INVOICE_RPC_URL"))
		}
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("FNN_RPC_URL"))
		}
		options.PayerNode = newRPCNode(endpoint)
	}

	return &Server{
		invoiceNode:     options.InvoiceNode,
		payerNode:       options.PayerNode,
		tips:            map[string]*tipRecord{},
		withdrawalsByID: map[string]*fiberclient.WithdrawalStatusItem{},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "fiber-adapter"})
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
	case "tip.status":
		s.handleStatus(w, payload.ID, payload.Params)
	case "tip.settled_feed":
		s.handleSettledFeed(w, payload.ID, payload.Params)
	case "withdrawal.quote":
		s.handleQuoteWithdrawal(w, payload.ID, payload.Params)
	case "withdrawal.request":
		s.handleRequestWithdrawal(w, payload.ID, payload.Params)
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
	if input.PostID == "" || input.FromUserID == "" || input.ToUserID == "" || input.Asset == "" || input.Amount == "" {
		writeRPCError(w, id, -32602, "missing required fields")
		return
	}

	invoice, err := s.invoiceNode.CreateInvoice(context.Background(), input.Asset, input.Amount)
	if err != nil {
		writeRPCError(w, id, -32000, err.Error())
		return
	}

	s.mu.Lock()
	s.tipSeq++
	s.tips[invoice] = &tipRecord{
		Invoice:    invoice,
		PostID:     input.PostID,
		FromUserID: input.FromUserID,
		ToUserID:   input.ToUserID,
		Asset:      input.Asset,
		Amount:     input.Amount,
		Message:    input.Message,
		State:      "UNPAID",
	}
	s.mu.Unlock()

	writeRPCResult(w, id, fiberclient.CreateInvoiceResult{Invoice: invoice})
}

func (s *Server) handleStatus(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input struct {
		Invoice string `json:"invoice"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		writeRPCError(w, id, -32602, "invalid params")
		return
	}
	if strings.TrimSpace(input.Invoice) == "" {
		writeRPCError(w, id, -32602, "missing invoice")
		return
	}

	state, err := s.invoiceNode.GetInvoiceStatus(context.Background(), input.Invoice)
	if err != nil {
		writeRPCError(w, id, -32000, err.Error())
		return
	}
	s.updateTipState(input.Invoice, state)
	writeRPCResult(w, id, fiberclient.InvoiceStatusResult{State: state})
}

func (s *Server) handleSettledFeed(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input fiberclient.SettledFeedInput
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &input); err != nil {
			writeRPCError(w, id, -32602, "invalid params")
			return
		}
	}

	records := s.snapshotTips()
	items := make([]fiberclient.SettledFeedItem, 0, len(records))
	for _, record := range records {
		state, err := s.invoiceNode.GetInvoiceStatus(context.Background(), record.Invoice)
		if err != nil {
			writeRPCError(w, id, -32000, err.Error())
			return
		}
		updated := s.updateTipState(record.Invoice, state)
		if updated == nil || updated.State != "SETTLED" {
			continue
		}
		items = append(items, fiberclient.SettledFeedItem{
			TipIntentID: makeTipIntentID(updated.Invoice),
			PostID:      updated.PostID,
			Invoice:     updated.Invoice,
			Amount:      updated.Amount,
			Asset:       updated.Asset,
			FromUserID:  updated.FromUserID,
			ToUserID:    updated.ToUserID,
			Message:     updated.Message,
			SettledAt:   updated.SettledAt,
		})
	}
	if input.Limit > 0 && len(items) > input.Limit {
		items = items[:input.Limit]
	}

	writeRPCResult(w, id, fiberclient.SettledFeedResult{Items: items})
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

	destinationValid := true
	var validationMessage *string
	if input.Destination.Kind == "PAYMENT_REQUEST" {
		if err := s.payerNode.ValidatePaymentRequest(context.Background(), input.Destination.PaymentRequest); err != nil {
			destinationValid = false
			message := err.Error()
			validationMessage = &message
		}
	}

	writeRPCResult(w, id, fiberclient.QuotePayoutResult{
		Asset:             input.Asset,
		Amount:            input.Amount,
		MinimumAmount:     input.Amount,
		AvailableBalance:  input.Amount,
		LockedBalance:     "0",
		NetworkFee:        "0",
		ReceiveAmount:     input.Amount,
		DestinationValid:  destinationValid,
		ValidationMessage: validationMessage,
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
	if input.Destination.Kind != "PAYMENT_REQUEST" {
		writeRPCError(w, id, -32602, "only PAYMENT_REQUEST is currently supported")
		return
	}

	requestID := nextID("wdreq")
	externalID, err := s.payerNode.SendPayment(context.Background(), input.Destination.PaymentRequest, input.Amount, input.Asset, requestID)
	if err != nil {
		writeRPCError(w, id, -32000, err.Error())
		return
	}

	s.mu.Lock()
	s.withdrawalSeq++
	withdrawal := fiberclient.WithdrawalStatusItem{
		ID:     externalID,
		UserID: input.UserID,
		Asset:  input.Asset,
		Amount: input.Amount,
		State:  "PROCESSING",
	}
	s.withdrawals = append(s.withdrawals, withdrawal)
	s.withdrawalsByID[withdrawal.ID] = &s.withdrawals[len(s.withdrawals)-1]
	s.mu.Unlock()

	writeRPCResult(w, id, fiberclient.RequestPayoutResult{
		ID:    externalID,
		State: "PENDING",
	})
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, id any, raw json.RawMessage) {
	var input struct {
		UserID string `json:"userId"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &input)
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

func (s *Server) snapshotTips() []tipRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]tipRecord, 0, len(s.tips))
	for _, record := range s.tips {
		records = append(records, *record)
	}
	return records
}

func (s *Server) updateTipState(invoice, state string) *tipRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.tips[invoice]
	if !ok {
		return nil
	}
	record.State = state
	if state == "SETTLED" && record.SettledAt == "" {
		record.SettledAt = time.Now().UTC().Format(time.RFC3339)
	}
	return &tipRecord{
		Invoice:    record.Invoice,
		PostID:     record.PostID,
		FromUserID: record.FromUserID,
		ToUserID:   record.ToUserID,
		Asset:      record.Asset,
		Amount:     record.Amount,
		Message:    record.Message,
		State:      record.State,
		SettledAt:  record.SettledAt,
	}
}

type rpcNode struct {
	endpoint   string
	httpClient *http.Client
}

func newRPCNode(endpoint string) rawNode {
	return &rpcNode{
		endpoint:   strings.TrimSpace(endpoint),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (n *rpcNode) CreateInvoice(ctx context.Context, asset, amount string) (string, error) {
	hexAmount, err := toHexQuantity(amount)
	if err != nil {
		return "", err
	}
	result, err := n.call(ctx, "new_invoice", map[string]any{
		"amount":   hexAmount,
		"currency": mapAssetToCurrency(asset),
	})
	if err != nil {
		return "", err
	}
	invoice, _ := result["invoice_address"].(string)
	if strings.TrimSpace(invoice) == "" {
		return "", errors.New("new_invoice response is missing invoice_address")
	}
	return invoice, nil
}

func (n *rpcNode) GetInvoiceStatus(ctx context.Context, invoice string) (string, error) {
	paymentHash, err := n.invoicePaymentHash(ctx, invoice)
	if err != nil {
		return "", err
	}
	result, err := n.call(ctx, "get_invoice", map[string]any{
		"payment_hash": paymentHash,
	})
	if err != nil {
		return "", err
	}
	status, _ := result["status"].(string)
	if strings.TrimSpace(status) == "" {
		return "", errors.New("get_invoice response is missing status")
	}
	return mapInvoiceState(status), nil
}

func (n *rpcNode) ValidatePaymentRequest(ctx context.Context, invoice string) error {
	_, err := n.invoicePaymentHash(ctx, invoice)
	return err
}

func (n *rpcNode) SendPayment(ctx context.Context, invoice, amount, asset, requestID string) (string, error) {
	paymentHash, err := n.invoicePaymentHash(ctx, invoice)
	if err != nil {
		return "", err
	}
	hexAmount, err := toHexQuantity(amount)
	if err != nil {
		return "", err
	}
	result, err := n.call(ctx, "send_payment", map[string]any{
		"payment_hash": paymentHash,
		"amount":       hexAmount,
		"currency":     mapAssetToCurrency(asset),
		"request_id":   requestID,
		"invoice":      invoice,
	})
	if err != nil {
		return "", err
	}
	if evidence := pickTxEvidence(result); evidence != "" {
		return evidence, nil
	}
	return "", errors.New("send_payment response is missing transaction evidence")
}

func (n *rpcNode) invoicePaymentHash(ctx context.Context, invoice string) (string, error) {
	result, err := n.call(ctx, "parse_invoice", map[string]any{
		"invoice": invoice,
	})
	if err != nil {
		return "", err
	}

	invoiceRecord, _ := result["invoice"].(map[string]any)
	dataRecord, _ := invoiceRecord["data"].(map[string]any)
	hash, _ := dataRecord["payment_hash"].(string)
	if strings.TrimSpace(hash) == "" {
		return "", errors.New("parse_invoice response is missing invoice.data.payment_hash")
	}
	return hash, nil
}

func (n *rpcNode) call(ctx context.Context, method string, params any) (map[string]any, error) {
	if n == nil || strings.TrimSpace(n.endpoint) == "" {
		return nil, errors.New("fnn rpc endpoint is not configured")
	}

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  []any{params},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := n.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("fnn rpc returned %d", res.StatusCode)
	}

	var body struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Error != nil {
		return nil, fmt.Errorf("fnn rpc error %d: %s", body.Error.Code, body.Error.Message)
	}
	return body.Result, nil
}

func mapAssetToCurrency(asset string) string {
	switch strings.ToUpper(strings.TrimSpace(asset)) {
	case "CKB":
		return "Fibt"
	default:
		return "Fibt"
	}
}

func mapInvoiceState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "paid", "settled":
		return "SETTLED"
	case "cancelled", "expired", "failed":
		return "FAILED"
	default:
		return "UNPAID"
	}
}

func toHexQuantity(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(trimmed), "0x") {
		return strings.ToLower(trimmed), nil
	}
	if strings.Contains(trimmed, ".") {
		parts := strings.SplitN(trimmed, ".", 2)
		if len(parts) != 2 || strings.Trim(parts[1], "0") != "" {
			return "", errors.New("adapter currently requires integer amounts for raw fnn calls")
		}
		trimmed = parts[0]
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed < 0 {
		return "", errors.New("adapter currently requires integer amounts for raw fnn calls")
	}
	return fmt.Sprintf("0x%x", parsed), nil
}

func pickTxEvidence(result map[string]any) string {
	for _, key := range []string{"tx_hash", "txHash", "payment_hash", "paymentHash", "hash"} {
		value, _ := result[key].(string)
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func makeTipIntentID(invoice string) string {
	sum := sha256.Sum256([]byte(invoice))
	return "tip_" + hex.EncodeToString(sum[:8])
}

func nextID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	}
	return prefix + "_" + hex.EncodeToString(buf)
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
