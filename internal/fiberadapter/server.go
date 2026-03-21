package fiberadapter

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/chenyu/1-tok/internal/httputil"
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
	AppID         string
	HMACSecret    string
}

type Server struct {
	invoiceNode rawNode
	payerNode   rawNode
	appID       string
	hmacSecret  string

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
	NodeInfo(context.Context) (map[string]any, error)
}

type rpcUdtTypeScript struct {
	CodeHash string `json:"code_hash"`
	HashType string `json:"hash_type"`
	Args     string `json:"args"`
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		AppID:      strings.TrimSpace(os.Getenv("FIBER_APP_ID")),
		HMACSecret: strings.TrimSpace(os.Getenv("FIBER_HMAC_SECRET")),
	})
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
		appID:           strings.TrimSpace(options.AppID),
		hmacSecret:      strings.TrimSpace(options.HMACSecret),
		tips:            map[string]*tipRecord{},
		withdrawalsByID: map[string]*fiberclient.WithdrawalStatusItem{},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "fiber-adapter"})
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Read body into memory so we can verify the HMAC signature.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
	if err != nil {
		writeRPCError(w, nil, -32700, "failed to read request body")
		return
	}

	if err := s.verifyHMAC(r, body); err != nil {
		httputil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
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
		State:  "COMPLETED",
	}
	s.withdrawals = append(s.withdrawals, withdrawal)
	s.withdrawalsByID[withdrawal.ID] = &s.withdrawals[len(s.withdrawals)-1]
	s.mu.Unlock()

	writeRPCResult(w, id, fiberclient.RequestPayoutResult{
		ID:    externalID,
		State: "COMPLETED",
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
	params := map[string]any{
		"amount":   hexAmount,
		"currency": mapAssetToCurrency(asset),
	}
	if strings.EqualFold(strings.TrimSpace(asset), "USDI") {
		udtTypeScript, err := resolveUSDIUdtTypeScript(ctx, n)
		if err != nil {
			return "", err
		}
		params["udt_type_script"] = map[string]any{
			"code_hash": udtTypeScript.CodeHash,
			"hash_type": udtTypeScript.HashType,
			"args":      udtTypeScript.Args,
		}
	}
	result, err := n.call(ctx, "new_invoice", params)
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

func (n *rpcNode) NodeInfo(ctx context.Context) (map[string]any, error) {
	return n.callNoParams(ctx, "node_info")
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

func (n *rpcNode) callNoParams(ctx context.Context, method string) (map[string]any, error) {
	if n == nil || strings.TrimSpace(n.endpoint) == "" {
		return nil, errors.New("fnn rpc endpoint is not configured")
	}

	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  []any{},
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
	if strings.EqualFold(strings.TrimSpace(asset), "CKB") {
		if scoped := strings.TrimSpace(os.Getenv("FIBER_INVOICE_CURRENCY_CKB")); scoped != "" {
			return scoped
		}
		if global := strings.TrimSpace(os.Getenv("FIBER_INVOICE_CURRENCY")); global != "" {
			return global
		}
		return "Fibt"
	}
	if scoped := strings.TrimSpace(os.Getenv("FIBER_INVOICE_CURRENCY_USDI")); scoped != "" {
		return scoped
	}
	return mapAssetToCurrency("CKB")
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

func resolveUSDIUdtTypeScript(ctx context.Context, node rawNode) (rpcUdtTypeScript, error) {
	if envJSON := strings.TrimSpace(os.Getenv("FIBER_USDI_UDT_TYPE_SCRIPT_JSON")); envJSON != "" {
		var script rpcUdtTypeScript
		if err := json.Unmarshal([]byte(envJSON), &script); err != nil {
			return rpcUdtTypeScript{}, errors.New("FIBER_USDI_UDT_TYPE_SCRIPT_JSON must be valid JSON")
		}
		if !script.valid() {
			return rpcUdtTypeScript{}, errors.New("FIBER_USDI_UDT_TYPE_SCRIPT_JSON must include code_hash/hash_type/args")
		}
		return script, nil
	}

	nodeInfo, err := node.NodeInfo(ctx)
	if err != nil {
		return rpcUdtTypeScript{}, err
	}
	script := pickUSDIUdtTypeScript(nodeInfo)
	if !script.valid() {
		return rpcUdtTypeScript{}, errors.New("node_info does not expose a usable USDI udt_type_script")
	}
	return script, nil
}

func pickUSDIUdtTypeScript(nodeInfo map[string]any) rpcUdtTypeScript {
	result, _ := nodeInfo["result"].(map[string]any)
	if len(result) == 0 {
		result = nodeInfo
	}
	infos, _ := result["udt_cfg_infos"].([]any)
	if len(infos) == 0 {
		return rpcUdtTypeScript{}
	}

	preferredName := normalizeOptionalAssetName(os.Getenv("FIBER_USDI_UDT_NAME"))
	var fallback rpcUdtTypeScript
	for _, item := range infos {
		record, _ := item.(map[string]any)
		if len(record) == 0 {
			continue
		}
		script := parseRPCUdtTypeScript(record["script"])
		if !script.valid() {
			continue
		}
		if !fallback.valid() {
			fallback = script
		}
		name := normalizeOptionalAssetName(record["name"])
		if preferredName != "" {
			if name == preferredName {
				return script
			}
			continue
		}
		if name == "usdi" || name == "rusd" {
			return script
		}
	}
	return fallback
}

func parseRPCUdtTypeScript(value any) rpcUdtTypeScript {
	record, _ := value.(map[string]any)
	if len(record) == 0 {
		return rpcUdtTypeScript{}
	}
	return rpcUdtTypeScript{
		CodeHash: strings.TrimSpace(stringValue(record["code_hash"])),
		HashType: strings.TrimSpace(stringValue(record["hash_type"])),
		Args:     strings.TrimSpace(stringValue(record["args"])),
	}
}

func (s rpcUdtTypeScript) valid() bool {
	return s.CodeHash != "" && s.HashType != "" && s.Args != ""
}

func normalizeOptionalAssetName(value any) string {
	return strings.ToLower(strings.TrimSpace(stringValue(value)))
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

// verifyHMAC checks the x-signature header against the request body using the
// shared HMAC secret. When no secret is configured, verification is skipped
// (backward compatible with existing deployments that haven't set FIBER_HMAC_SECRET).
//
// The signature format matches the Fiber client: HMAC-SHA256(secret, ts + "." + nonce + "." + body).
func (s *Server) verifyHMAC(r *http.Request, body []byte) error {
	if s.hmacSecret == "" {
		return nil // auth not configured — allow all (backward compat)
	}

	appID := strings.TrimSpace(r.Header.Get("x-app-id"))
	ts := strings.TrimSpace(r.Header.Get("x-ts"))
	nonce := strings.TrimSpace(r.Header.Get("x-nonce"))
	signature := strings.TrimSpace(r.Header.Get("x-signature"))

	if signature == "" || ts == "" || nonce == "" {
		return errors.New("missing signature headers")
	}
	if s.appID != "" && appID != s.appID {
		return errors.New("app id mismatch")
	}

	mac := hmac.New(sha256.New, []byte(s.hmacSecret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid signature")
	}
	return nil
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeRPCError(w http.ResponseWriter, id any, code int, message string) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
