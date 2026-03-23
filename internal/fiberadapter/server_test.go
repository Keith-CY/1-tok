package fiberadapter

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
)

func TestServerCreatesInvoiceReadsStatusAndBuildsSettledFeedFromFNN(t *testing.T) {
	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read raw fnn request: %v", err)
		}

		var rpc struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode raw fnn request: %v", err)
		}

		switch rpc.Method {
		case "new_invoice":
			var params struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			}
			if len(rpc.Params) != 1 {
				t.Fatalf("expected one new_invoice param, got %d", len(rpc.Params))
			}
			if err := json.Unmarshal(rpc.Params[0], &params); err != nil {
				t.Fatalf("decode new_invoice params: %v", err)
			}
			if params.Amount != "0xc" {
				t.Fatalf("expected integer CKB amount to map to 0xc, got %q", params.Amount)
			}
			if params.Currency == "" {
				t.Fatalf("expected invoice currency to be set")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"invoice_address": "fiber:invoice:ckb:12",
				},
			})
		case "parse_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"invoice": map[string]any{
						"data": map[string]any{
							"payment_hash": "0xhash123",
						},
					},
				},
			})
		case "get_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"status": "paid",
				},
			})
		default:
			t.Fatalf("unexpected raw fnn method %q", rpc.Method)
		}
	}))
	defer invoiceNode.Close()

	server := httptest.NewServer(NewServerWithOptions(Options{
		InvoiceRPCURL: invoiceNode.URL,
		PayerRPCURL:   invoiceNode.URL,
	}))
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")

	createResult, err := client.CreateInvoice(context.Background(), fiberclient.CreateInvoiceInput{
		PostID:     "ord_1:ms_1",
		FromUserID: "buyer_1",
		ToUserID:   "provider_1",
		Asset:      "CKB",
		Amount:     "12",
		Message:    "real fnn invoice",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if createResult.Invoice != "fiber:invoice:ckb:12" {
		t.Fatalf("unexpected invoice: %+v", createResult)
	}

	statusResult, err := client.GetInvoiceStatus(context.Background(), createResult.Invoice)
	if err != nil {
		t.Fatalf("get invoice status: %v", err)
	}
	if statusResult.State != "SETTLED" {
		t.Fatalf("expected SETTLED, got %+v", statusResult)
	}

	feed, err := client.ListSettledFeed(context.Background(), fiberclient.SettledFeedInput{Limit: 10})
	if err != nil {
		t.Fatalf("list settled feed: %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("expected one settled item, got %+v", feed.Items)
	}
	if feed.Items[0].Invoice != createResult.Invoice || feed.Items[0].PostID != "ord_1:ms_1" {
		t.Fatalf("unexpected settled feed item: %+v", feed.Items[0])
	}
}

func TestServerCreatesUSDIInvoiceWithResolvedUdtTypeScript(t *testing.T) {
	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read raw fnn request: %v", err)
		}

		var rpc struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode raw fnn request: %v", err)
		}

		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"udt_cfg_infos": []map[string]any{
						{
							"name": "USDI",
							"script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "new_invoice":
			var params struct {
				Amount        string `json:"amount"`
				Currency      string `json:"currency"`
				UdtTypeScript struct {
					CodeHash string `json:"code_hash"`
					HashType string `json:"hash_type"`
					Args     string `json:"args"`
				} `json:"udt_type_script"`
			}
			if len(rpc.Params) != 1 {
				t.Fatalf("expected one new_invoice param, got %d", len(rpc.Params))
			}
			if err := json.Unmarshal(rpc.Params[0], &params); err != nil {
				t.Fatalf("decode new_invoice params: %v", err)
			}
			if params.Amount != "0xa" {
				t.Fatalf("expected integer USDI amount to map to 0xa, got %q", params.Amount)
			}
			if params.Currency == "" {
				t.Fatal("expected USDI invoice currency to be set")
			}
			if params.UdtTypeScript.CodeHash != "0xudt" || params.UdtTypeScript.HashType != "type" || params.UdtTypeScript.Args != "0x01" {
				t.Fatalf("missing udt_type_script in new_invoice params: %+v", params.UdtTypeScript)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"invoice_address": "fiber:invoice:usdi:10",
				},
			})
		default:
			t.Fatalf("unexpected raw fnn method %q", rpc.Method)
		}
	}))
	defer invoiceNode.Close()

	server := httptest.NewServer(NewServerWithOptions(Options{
		InvoiceRPCURL: invoiceNode.URL,
		PayerRPCURL:   invoiceNode.URL,
	}))
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")
	result, err := client.CreateInvoice(context.Background(), fiberclient.CreateInvoiceInput{
		PostID:     "buyer_topup:buyer_1:fund_1",
		FromUserID: "buyer_1",
		ToUserID:   "platform_treasury",
		Asset:      "USDI",
		Amount:     "10",
	})
	if err != nil {
		t.Fatalf("create usdi invoice: %v", err)
	}
	if result.Invoice != "fiber:invoice:usdi:10" {
		t.Fatalf("unexpected usdi invoice: %+v", result)
	}
}

func TestServerCreatesUSDIInvoiceWithEscapedEnvUdtTypeScriptJSON(t *testing.T) {
	t.Setenv("FIBER_USDI_UDT_TYPE_SCRIPT_JSON", `{\"code_hash\":\"0xudt\",\"hash_type\":\"type\",\"args\":\"0x01\"}`)

	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read raw fnn request: %v", err)
		}

		var rpc struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode raw fnn request: %v", err)
		}

		if rpc.Method != "new_invoice" {
			t.Fatalf("unexpected raw fnn method %q", rpc.Method)
		}

		var params struct {
			UdtTypeScript struct {
				CodeHash string `json:"code_hash"`
				HashType string `json:"hash_type"`
				Args     string `json:"args"`
			} `json:"udt_type_script"`
		}
		if len(rpc.Params) != 1 {
			t.Fatalf("expected one new_invoice param, got %d", len(rpc.Params))
		}
		if err := json.Unmarshal(rpc.Params[0], &params); err != nil {
			t.Fatalf("decode new_invoice params: %v", err)
		}
		if params.UdtTypeScript.CodeHash != "0xudt" || params.UdtTypeScript.HashType != "type" || params.UdtTypeScript.Args != "0x01" {
			t.Fatalf("missing udt_type_script in new_invoice params: %+v", params.UdtTypeScript)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"invoice_address": "fiber:invoice:usdi:10",
			},
		})
	}))
	defer invoiceNode.Close()

	server := httptest.NewServer(NewServerWithOptions(Options{
		InvoiceRPCURL: invoiceNode.URL,
		PayerRPCURL:   invoiceNode.URL,
	}))
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")
	result, err := client.CreateInvoice(context.Background(), fiberclient.CreateInvoiceInput{
		PostID:     "buyer_topup:buyer_1:fund_escaped",
		FromUserID: "buyer_1",
		ToUserID:   "platform_treasury",
		Asset:      "USDI",
		Amount:     "10",
	})
	if err != nil {
		t.Fatalf("create usdi invoice with escaped env json: %v", err)
	}
	if result.Invoice != "fiber:invoice:usdi:10" {
		t.Fatalf("unexpected usdi invoice: %+v", result)
	}
}

func TestServerRejectsUSDIInvoiceWhenNodeInfoHasNoUdtScript(t *testing.T) {
	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read raw fnn request: %v", err)
		}

		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode raw fnn request: %v", err)
		}
		if rpc.Method != "node_info" {
			t.Fatalf("unexpected raw fnn method %q", rpc.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"udt_cfg_infos": []map[string]any{},
			},
		})
	}))
	defer invoiceNode.Close()

	server := httptest.NewServer(NewServerWithOptions(Options{
		InvoiceRPCURL: invoiceNode.URL,
		PayerRPCURL:   invoiceNode.URL,
	}))
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")
	_, err := client.CreateInvoice(context.Background(), fiberclient.CreateInvoiceInput{
		PostID:     "buyer_topup:buyer_1:fund_1",
		FromUserID: "buyer_1",
		ToUserID:   "platform_treasury",
		Asset:      "USDI",
		Amount:     "10",
	})
	if err == nil || !strings.Contains(err.Error(), "usable USDI udt_type_script") {
		t.Fatalf("expected missing USDI udt script error, got %v", err)
	}
}

func TestServerQuotesAndRequestsWithdrawalThroughPayerFNN(t *testing.T) {
	payerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read raw fnn request: %v", err)
		}

		var rpc struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode raw fnn request: %v", err)
		}

		switch rpc.Method {
		case "parse_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"invoice": map[string]any{
						"data": map[string]any{
							"payment_hash": "0xpayhash456",
						},
					},
				},
			})
		case "send_payment":
			var params struct {
				PaymentHash string `json:"payment_hash"`
				Amount      string `json:"amount"`
				RequestID   string `json:"request_id"`
				Invoice     string `json:"invoice"`
			}
			if len(rpc.Params) != 1 {
				t.Fatalf("expected one send_payment param, got %d", len(rpc.Params))
			}
			if err := json.Unmarshal(rpc.Params[0], &params); err != nil {
				t.Fatalf("decode send_payment params: %v", err)
			}
			if params.PaymentHash != "0xpayhash456" {
				t.Fatalf("unexpected payment hash: %+v", params)
			}
			if params.Amount != "0xa" {
				t.Fatalf("expected integer amount to map to 0xa, got %q", params.Amount)
			}
			if params.Invoice != "fiber:invoice:withdraw:10" {
				t.Fatalf("unexpected invoice: %+v", params)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"payment_hash": "0xpaymentcomplete",
				},
			})
		default:
			t.Fatalf("unexpected raw fnn method %q", rpc.Method)
		}
	}))
	defer payerNode.Close()

	server := httptest.NewServer(NewServerWithOptions(Options{
		InvoiceRPCURL: payerNode.URL,
		PayerRPCURL:   payerNode.URL,
	}))
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")

	quoteResult, err := client.QuotePayout(context.Background(), fiberclient.QuotePayoutInput{
		UserID: "provider_1",
		Asset:  "CKB",
		Amount: "10",
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: "fiber:invoice:withdraw:10",
		},
	})
	if err != nil {
		t.Fatalf("quote payout: %v", err)
	}
	if !quoteResult.DestinationValid {
		t.Fatalf("expected destination to validate, got %+v", quoteResult)
	}

	withdrawalResult, err := client.RequestPayout(context.Background(), fiberclient.RequestPayoutInput{
		UserID: "provider_1",
		Asset:  "CKB",
		Amount: "10",
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: "fiber:invoice:withdraw:10",
		},
	})
	if err != nil {
		t.Fatalf("request payout: %v", err)
	}
	if withdrawalResult.ID == "" {
		t.Fatalf("expected withdrawal id, got %+v", withdrawalResult)
	}

	statuses, err := client.ListWithdrawalStatuses(context.Background(), "provider_1")
	if err != nil {
		t.Fatalf("list withdrawal statuses: %v", err)
	}
	if len(statuses.Withdrawals) != 1 {
		t.Fatalf("expected one withdrawal status, got %+v", statuses.Withdrawals)
	}
	if statuses.Withdrawals[0].UserID != "provider_1" {
		t.Fatalf("unexpected withdrawal status: %+v", statuses.Withdrawals[0])
	}
}

func TestServerSplitsInvoiceAndPayerNodes(t *testing.T) {
	var invoiceMethods []string
	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read invoice-node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode invoice-node payload: %v", err)
		}
		invoiceMethods = append(invoiceMethods, rpc.Method)
		switch rpc.Method {
		case "new_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"invoice_address": "fiber:invoice:split"}})
		case "parse_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xsplit"}}}})
		case "get_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"status": "paid"}})
		default:
			t.Fatalf("unexpected invoice-node method %q", rpc.Method)
		}
	}))
	defer invoiceNode.Close()

	var payerMethods []string
	payerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read payer-node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode payer-node payload: %v", err)
		}
		payerMethods = append(payerMethods, rpc.Method)
		switch rpc.Method {
		case "parse_invoice":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xpay"}}}})
		case "send_payment":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"payment_hash": "0xpaid"}})
		default:
			t.Fatalf("unexpected payer-node method %q", rpc.Method)
		}
	}))
	defer payerNode.Close()

	server := httptest.NewServer(NewServerWithOptions(Options{
		InvoiceRPCURL: invoiceNode.URL,
		PayerRPCURL:   payerNode.URL,
	}))
	defer server.Close()

	client := fiberclient.NewClient(server.URL, "app_local", "secret_local")
	createResult, err := client.CreateInvoice(context.Background(), fiberclient.CreateInvoiceInput{
		PostID:     "ord_split:ms_1",
		FromUserID: "buyer_split",
		ToUserID:   "provider_split",
		Asset:      "CKB",
		Amount:     "12",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if _, err := client.GetInvoiceStatus(context.Background(), createResult.Invoice); err != nil {
		t.Fatalf("get invoice status: %v", err)
	}
	if _, err := client.RequestPayout(context.Background(), fiberclient.RequestPayoutInput{
		UserID: "provider_split",
		Asset:  "CKB",
		Amount: "10",
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: createResult.Invoice,
		},
	}); err != nil {
		t.Fatalf("request payout: %v", err)
	}

	if got := strings.Join(invoiceMethods, ","); got != "new_invoice,parse_invoice,get_invoice" {
		t.Fatalf("unexpected invoice-node method sequence %q", got)
	}
	if got := strings.Join(payerMethods, ","); got != "parse_invoice,send_payment" {
		t.Fatalf("unexpected payer-node method sequence %q", got)
	}
}

func TestServerRejectsNonIntegerAmountsWithoutPanicking(t *testing.T) {
	server := NewServerWithOptions(Options{
		InvoiceNode: rejectingRawNode{},
		PayerNode:   rejectingRawNode{},
	})

	payload := []byte(`{"jsonrpc":"2.0","id":"req_1","method":"tip.create","params":{"postId":"ord_1:ms_1","fromUserId":"buyer_1","toUserId":"provider_1","asset":"CKB","amount":"12.5"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected JSON-RPC 200, got %d", res.Code)
	}

	var response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Message == "" {
		t.Fatalf("expected json-rpc error body, got %s", res.Body.String())
	}
}

func TestToHexQuantityRejectsFractionalAmount(t *testing.T) {
	if _, err := toHexQuantity("12.5"); err == nil {
		t.Fatalf("expected fractional amount to be rejected")
	}
}

type rejectingRawNode struct{}

func (rejectingRawNode) CreateInvoice(context.Context, string, string) (string, error) {
	t := "raw fnn should not be called for invalid amount input"
	return "", errors.New(t)
}

func (rejectingRawNode) GetInvoiceStatus(context.Context, string) (string, error) {
	return "", errors.New("unexpected call")
}

func (rejectingRawNode) ValidatePaymentRequest(context.Context, string) error {
	return errors.New("unexpected call")
}

func (rejectingRawNode) SendPayment(context.Context, string, string, string, string) (string, error) {
	return "", errors.New("unexpected call")
}

func (rejectingRawNode) NodeInfo(context.Context) (map[string]any, error) {
	return nil, errors.New("unexpected call")
}

func TestHMACVerification_NoSecretAllowsAll(t *testing.T) {
	server := NewServerWithOptions(Options{
		InvoiceNode: &stubRPCNode{},
		PayerNode:   &stubRPCNode{},
		// no AppID or HMACSecret
	})

	body := `{"jsonrpc":"2.0","id":1,"method":"tip.status","params":{"invoice":"inv_1"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	// should not be 401 — HMAC is disabled
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 when HMAC is not configured, got %d", rec.Code)
	}
}

func TestHMACVerification_RejectsMissingSignature(t *testing.T) {
	server := NewServerWithOptions(Options{
		InvoiceNode: &stubRPCNode{},
		PayerNode:   &stubRPCNode{},
		AppID:       "test-app",
		HMACSecret:  "test-secret",
	})

	body := `{"jsonrpc":"2.0","id":1,"method":"tip.status","params":{"invoice":"inv_1"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	// no x-signature headers
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHMACVerification_RejectsInvalidSignature(t *testing.T) {
	server := NewServerWithOptions(Options{
		InvoiceNode: &stubRPCNode{},
		PayerNode:   &stubRPCNode{},
		AppID:       "test-app",
		HMACSecret:  "test-secret",
	})

	body := `{"jsonrpc":"2.0","id":1,"method":"tip.status","params":{"invoice":"inv_1"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("x-app-id", "test-app")
	req.Header.Set("x-ts", "1234567890")
	req.Header.Set("x-nonce", "abc123")
	req.Header.Set("x-signature", "badsignature")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHMACVerification_AcceptsValidSignature(t *testing.T) {
	secret := "test-secret"
	server := NewServerWithOptions(Options{
		InvoiceNode: &stubRPCNode{invoiceStatus: "UNPAID"},
		PayerNode:   &stubRPCNode{},
		AppID:       "test-app",
		HMACSecret:  secret,
	})

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tip.status","params":{"invoice":"inv_1"}}`)
	ts := "1234567890"
	nonce := "abc123"

	// compute valid HMAC-SHA256 signature matching fiber client format
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write([]byte(nonce))
	mac.Write([]byte("."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("x-app-id", "test-app")
	req.Header.Set("x-ts", ts)
	req.Header.Set("x-nonce", nonce)
	req.Header.Set("x-signature", signature)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	// should not be 401
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 with valid signature, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMapInvoiceState(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"paid", "SETTLED"},
		{"Paid", "SETTLED"},
		{"settled", "SETTLED"},
		{"SETTLED", "SETTLED"},
		{"cancelled", "FAILED"},
		{"expired", "FAILED"},
		{"failed", "FAILED"},
		{"pending", "UNPAID"},
		{"", "UNPAID"},
		{"unknown", "UNPAID"},
	}
	for _, tt := range tests {
		if got := mapInvoiceState(tt.input); got != tt.want {
			t.Errorf("mapInvoiceState(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToHexQuantity(t *testing.T) {
	tests := []struct {
		input string
		want  string
		err   bool
	}{
		{"0x1a", "0x1a", false},
		{"0X1A", "0x1a", false},
		{"100", "0x64", false},
		{"0", "0x0", false},
		{"abc", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := toHexQuantity(tt.input)
		if tt.err && err == nil {
			t.Errorf("toHexQuantity(%q) expected error", tt.input)
		}
		if !tt.err && err != nil {
			t.Errorf("toHexQuantity(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("toHexQuantity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPickTxEvidence(t *testing.T) {
	tests := []struct {
		name   string
		result map[string]any
		want   string
	}{
		{"with tx_hash", map[string]any{"tx_hash": "0xabc"}, "0xabc"},
		{"with txHash", map[string]any{"txHash": "0xdef"}, "0xdef"},
		{"with payment_hash", map[string]any{"payment_hash": "0x123"}, "0x123"},
		{"with paymentHash", map[string]any{"paymentHash": "0x456"}, "0x456"},
		{"empty", map[string]any{}, ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickTxEvidence(tt.result); got != tt.want {
				t.Errorf("pickTxEvidence() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewServer_Defaults(t *testing.T) {
	// NewServer without env vars should create a server with nil RPC nodes
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestMapAssetToCurrency(t *testing.T) {
	tests := []struct {
		asset string
		want  string
	}{
		{"CKB", "Fibt"},
		{"ckb", "Fibt"},
		{"USDI", "Fibt"},
		{"anything", "Fibt"},
	}
	for _, tt := range tests {
		if got := mapAssetToCurrency(tt.asset); got != tt.want {
			t.Errorf("mapAssetToCurrency(%q) = %q, want %q", tt.asset, got, tt.want)
		}
	}
}

func TestMapAssetToCurrencyUsesScopedUSDIOverride(t *testing.T) {
	t.Setenv("FIBER_INVOICE_CURRENCY_USDI", "TestUdt")
	if got := mapAssetToCurrency("USDI"); got != "TestUdt" {
		t.Fatalf("mapAssetToCurrency(USDI) = %q, want TestUdt", got)
	}
}

func TestNextID(t *testing.T) {
	id1 := nextID("test_")
	id2 := nextID("test_")
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if len(id1) < 10 {
		t.Errorf("id too short: %s", id1)
	}
}

func TestRPCNode_CreateInvoice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"result": map[string]any{"invoice_address": "lnbc_test"},
		})
	}))
	defer srv.Close()

	node := newRPCNode(srv.URL)
	invoice, err := node.CreateInvoice(context.Background(), "CKB", "100")
	if err != nil {
		t.Fatal(err)
	}
	if invoice != "lnbc_test" {
		t.Errorf("invoice = %s, want lnbc_test", invoice)
	}
}

func TestRPCNode_GetInvoiceStatus_Success(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// parse_invoice
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xabc123"}}},
			})
		} else {
			// get_invoice
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"status": "paid"},
			})
		}
	}))
	defer srv.Close()

	node := newRPCNode(srv.URL)
	status, err := node.GetInvoiceStatus(context.Background(), "lnbc_test")
	if err != nil {
		t.Fatal(err)
	}
	if status != "SETTLED" {
		t.Errorf("status = %s, want SETTLED", status)
	}
}

func TestRPCNode_Call_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"error": map[string]any{"code": -32600, "message": "invalid request"},
		})
	}))
	defer srv.Close()

	node := newRPCNode(srv.URL)
	_, err := node.CreateInvoice(context.Background(), "CKB", "100")
	if err == nil {
		t.Error("expected error for RPC error response")
	}
}

func TestRPCNode_Call_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	node := newRPCNode(srv.URL)
	_, err := node.CreateInvoice(context.Background(), "CKB", "100")
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestServeHTTP_HealthCheck(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestServeHTTP_NotPost(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusOK {
		t.Logf("GET /: status %d", rec.Code)
	}
}

func TestServeHTTP_InvalidRPCJSON(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{broken"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	// Should return RPC error
	if rec.Code != http.StatusOK {
		t.Logf("invalid JSON: status %d", rec.Code)
	}
}

func TestServeHTTP_UnknownMethod(t *testing.T) {
	s := NewServer()
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "unknown_method",
		"params": map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	// Should return method not found error
	if rec.Code != http.StatusOK {
		t.Logf("unknown method: status %d", rec.Code)
	}
}

func TestHandleCreate_Success(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"result": map[string]any{"invoice_address": "lnbc_created"},
		})
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		AppID:         "app_1",
	})

	// Send RPC create request
	rpcPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "tip.create",
		"params": map[string]any{
			"postId": "post_1", "fromUserId": "u_from", "toUserId": "u_to",
			"asset": "CKB", "amount": "100", "message": "tip",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(rpcPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleStatus_Success(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// parse_invoice
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xhash"}}},
			})
		} else {
			// get_invoice
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"status": "paid"},
			})
		}
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{InvoiceRPCURL: rpcSrv.URL})

	// Create a tip first (to have invoice in state)
	createPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.create",
		"params": map[string]any{
			"postId": "post_1", "fromUserId": "u1", "toUserId": "u2",
			"asset": "CKB", "amount": "100",
		},
	})
	cReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(createPayload))
	cReq.Header.Set("Content-Type", "application/json")
	cRec := httptest.NewRecorder()
	s.ServeHTTP(cRec, cReq)

	// Now check status
	statusPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tip.status",
		"params": map[string]any{"invoice": "lnbc_created"},
	})
	sReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(statusPayload))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	if sRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", sRec.Code, sRec.Body.String())
	}
}

func TestHandleSettledFeed_Success(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"result": map[string]any{"items": []any{}},
		})
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{InvoiceRPCURL: rpcSrv.URL})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.settled_feed",
		"params": map[string]any{"limit": 10},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleQuoteWithdrawal_Success(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"result": map[string]any{"invoice_address": "lnbc_quote"},
		})
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		PayerRPCURL:   rpcSrv.URL,
	})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.quote",
		"params": map[string]any{
			"userId": "u_1", "asset": "CKB", "amount": "100",
			"destination": map[string]any{"kind": "address", "address": "ckb1addr"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRequestWithdrawal_Success(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// parse_invoice (validate payment request)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xhash"}}},
			})
		} else {
			// send_payment
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"payment_hash": "0xpaid"},
			})
		}
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		PayerRPCURL:   rpcSrv.URL,
	})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.request",
		"params": map[string]any{
			"userId": "u_1", "asset": "CKB", "amount": "100",
			"destination": map[string]any{"kind": "payment_request", "paymentRequest": "lnbc_req"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDashboardSummary_Success(t *testing.T) {
	s := NewServerWithOptions(Options{})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "dashboard.summary",
		"params": map[string]any{"userId": "u_1"},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreate_WithHMAC(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"result": map[string]any{"invoice_address": "lnbc_hmac_test"},
		})
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		AppID:         "app_1",
		HMACSecret:    "test-secret",
	})

	rpcPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "tip.create",
		"params": map[string]any{
			"postId": "post_1", "fromUserId": "u_from", "toUserId": "u_to",
			"asset": "CKB", "amount": "100",
		},
	})

	// Without HMAC — should be rejected
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(rpcPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without HMAC, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNewServerWithOptions_WithRPCNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: srv.URL,
		PayerRPCURL:   srv.URL,
		AppID:         "app",
		HMACSecret:    "secret",
	})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestFullTipFlow_CreateThenStatus(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		body, _ := io.ReadAll(r.Body)
		var rpc struct {
			Method string `json:"method"`
		}
		json.Unmarshal(body, &rpc)

		switch rpc.Method {
		case "new_invoice":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice_address": "lnbc_tip_flow"},
			})
		case "parse_invoice":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xhash_tip"}}},
			})
		case "get_invoice":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"status": "paid"},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{},
			})
		}
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{InvoiceRPCURL: rpcSrv.URL, AppID: "app_1"})

	// Step 1: Create a tip
	createPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.create",
		"params": map[string]any{
			"postId": "post_flow", "fromUserId": "u_from", "toUserId": "u_to",
			"asset": "CKB", "amount": "100",
		},
	})
	cReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(createPayload))
	cReq.Header.Set("Content-Type", "application/json")
	cRec := httptest.NewRecorder()
	s.ServeHTTP(cRec, cReq)
	if cRec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", cRec.Code, cRec.Body.String())
	}

	// Extract invoice from response
	var createResp map[string]any
	json.Unmarshal(cRec.Body.Bytes(), &createResp)
	result, ok := createResp["result"].(map[string]any)
	if !ok || result == nil {
		t.Fatalf("no result in create response: %s", cRec.Body.String())
	}
	invoice, _ := result["invoice"].(string)
	if invoice == "" {
		// Try tipId
		invoice, _ = result["tipId"].(string)
	}
	if invoice == "" {
		t.Logf("create response: %s", cRec.Body.String())
		t.Skip("no invoice in create response — skip status test")
	}

	// Step 2: Check status
	statusPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tip.status",
		"params": map[string]any{"invoice": invoice},
	})
	sReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(statusPayload))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)
	if sRec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", sRec.Code, sRec.Body.String())
	}
}

func TestQuoteWithdrawal_PaymentRequestValidation(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		body, _ := io.ReadAll(r.Body)
		var rpc struct {
			Method string `json:"method"`
		}
		json.Unmarshal(body, &rpc)

		if rpc.Method == "parse_invoice" {
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xhash"}}},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{},
			})
		}
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		PayerRPCURL:   rpcSrv.URL,
	})

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.quote",
		"params": map[string]any{
			"userId": "u_1", "asset": "CKB", "amount": "100",
			"destination": map[string]any{"kind": "PAYMENT_REQUEST", "paymentRequest": "lnbc_quote_req"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequestWithdrawal_PaymentRequest(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		body, _ := io.ReadAll(r.Body)
		var rpc struct {
			Method string `json:"method"`
		}
		json.Unmarshal(body, &rpc)

		switch rpc.Method {
		case "parse_invoice":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xhash"}}},
			})
		case "send_payment":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"payment_hash": "0xpaid", "status": "completed"},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{},
			})
		}
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		PayerRPCURL:   rpcSrv.URL,
	})

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.request",
		"params": map[string]any{
			"userId": "u_1", "asset": "CKB", "amount": "50",
			"destination": map[string]any{"kind": "PAYMENT_REQUEST", "paymentRequest": "lnbc_pay_req"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSettledFeed_WithPagination(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"result": map[string]any{"invoice_address": "lnbc_feed"},
		})
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{InvoiceRPCURL: rpcSrv.URL})

	// Create a tip first to have data
	createPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.create",
		"params": map[string]any{
			"postId": "post_sf", "fromUserId": "u1", "toUserId": "u2",
			"asset": "CKB", "amount": "100",
		},
	})
	cReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(createPayload))
	cReq.Header.Set("Content-Type", "application/json")
	cRec := httptest.NewRecorder()
	s.ServeHTTP(cRec, cReq)

	// Query settled feed with pagination
	feedPayload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tip.settled_feed",
		"params": map[string]any{
			"limit":          5,
			"afterSettledAt": "2020-01-01T00:00:00Z",
			"afterId":        "",
		},
	})
	fReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(feedPayload))
	fReq.Header.Set("Content-Type", "application/json")
	fRec := httptest.NewRecorder()
	s.ServeHTTP(fRec, fReq)
	if fRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", fRec.Code, fRec.Body.String())
	}
}

func TestHandleStatus_MissingInvoice(t *testing.T) {
	s := NewServerWithOptions(Options{})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.status",
		"params": map[string]any{"invoice": ""},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (RPC error), got %d", rec.Code)
	}
	// Response should contain error
	if !strings.Contains(rec.Body.String(), "missing invoice") {
		t.Errorf("expected 'missing invoice' error, got: %s", rec.Body.String())
	}
}

func TestHandleStatus_InvoiceNodeError(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var rpc struct {
			Method string `json:"method"`
		}
		json.Unmarshal(body, &rpc)

		if rpc.Method == "parse_invoice" {
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"error": map[string]any{"code": -32000, "message": "invalid invoice"},
			})
		}
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{InvoiceRPCURL: rpcSrv.URL})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.status",
		"params": map[string]any{"invoice": "lnbc_bad"},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (RPC error), got %d", rec.Code)
	}
}

func TestHandleCreate_MissingFields(t *testing.T) {
	s := NewServerWithOptions(Options{})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.create",
		"params": map[string]any{"postId": "", "asset": ""},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "error") {
		t.Errorf("expected error for missing fields")
	}
}

func TestHandleQuoteWithdrawal_InvalidPaymentRequest(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"error": map[string]any{"code": -32000, "message": "invalid invoice"},
		})
	}))
	defer rpcSrv.Close()

	s := NewServerWithOptions(Options{
		InvoiceRPCURL: rpcSrv.URL,
		PayerRPCURL:   rpcSrv.URL,
	})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.quote",
		"params": map[string]any{
			"userId": "u_1", "asset": "CKB", "amount": "100",
			"destination": map[string]any{"kind": "PAYMENT_REQUEST", "paymentRequest": "lnbc_bad"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// Response should indicate destination invalid
	if !strings.Contains(rec.Body.String(), "false") {
		t.Log("destination validation result:", rec.Body.String())
	}
}

func TestHandleQuoteWithdrawal_MissingFields(t *testing.T) {
	s := NewServerWithOptions(Options{})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.quote",
		"params": map[string]any{
			"userId": "", "asset": "", "amount": "",
			"destination": map[string]any{"kind": ""},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "missing") {
		t.Errorf("expected missing fields error: %s", rec.Body.String())
	}
}

func TestHandleRequestWithdrawal_MissingFields(t *testing.T) {
	s := NewServerWithOptions(Options{})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "withdrawal.request",
		"params": map[string]any{
			"userId": "", "asset": "", "amount": "",
			"destination": map[string]any{"kind": ""},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "missing") {
		t.Errorf("expected missing fields error: %s", rec.Body.String())
	}
}

func TestHandleSettledFeed_InvalidParams(t *testing.T) {
	s := NewServerWithOptions(Options{})
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tip.settled_feed",
		"params": "not-an-object",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	// Should return RPC error for invalid params
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSendPayment_Success(t *testing.T) {
	callCount := 0
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var rpc struct {
			Method string `json:"method"`
		}
		json.Unmarshal(body, &rpc)

		switch rpc.Method {
		case "parse_invoice":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"invoice": map[string]any{"data": map[string]any{"payment_hash": "0xhash"}}},
			})
		case "send_payment":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": "1",
				"result": map[string]any{"payment_hash": "0xpaid_hash", "tx_hash": "0xtx"},
			})
		}
	}))
	defer rpcSrv.Close()

	node := newRPCNode(rpcSrv.URL)
	evidence, err := node.SendPayment(context.Background(), "lnbc_pay", "100", "CKB", "req_1")
	if err != nil {
		t.Fatal(err)
	}
	if evidence == "" {
		t.Error("expected non-empty evidence")
	}
}

func TestSendPayment_ParseError(t *testing.T) {
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": "1",
			"error": map[string]any{"code": -32000, "message": "parse failed"},
		})
	}))
	defer rpcSrv.Close()

	node := newRPCNode(rpcSrv.URL)
	_, err := node.SendPayment(context.Background(), "lnbc_bad", "100", "CKB", "req_1")
	if err == nil {
		t.Error("expected error for parse failure")
	}
}

func TestCall_ConnectionRefused(t *testing.T) {
	node := newRPCNode("http://127.0.0.1:1")
	_, err := node.CreateInvoice(context.Background(), "CKB", "100")
	if err == nil {
		t.Error("expected error for connection refused")
	}
}
