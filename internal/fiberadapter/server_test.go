package fiberadapter

import (
	"context"
	"bytes"
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
