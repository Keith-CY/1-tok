package fiber

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCreateInvoiceSignsJSONRPCRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/rpc" {
			t.Fatalf("expected /rpc, got %s", r.URL.Path)
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if got := r.Header.Get("x-app-id"); got != "app_1" {
			t.Fatalf("expected x-app-id app_1, got %q", got)
		}
		ts := r.Header.Get("x-ts")
		nonce := r.Header.Get("x-nonce")
		if ts == "" || nonce == "" {
			t.Fatalf("expected auth timestamp and nonce headers")
		}
		if want := signPayloadForTest("secret_1", payload, ts, nonce); r.Header.Get("x-signature") != want {
			t.Fatalf("unexpected signature: got=%q want=%q", r.Header.Get("x-signature"), want)
		}

		var rpc struct {
			Method string `json:"method"`
			Params struct {
				PostID     string `json:"postId"`
				FromUserID string `json:"fromUserId"`
				ToUserID   string `json:"toUserId"`
				Asset      string `json:"asset"`
				Amount     string `json:"amount"`
				Message    string `json:"message"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode rpc payload: %v", err)
		}
		if rpc.Method != "tip.create" {
			t.Fatalf("expected method tip.create, got %q", rpc.Method)
		}
		if rpc.Params.PostID != "ord_1:ms_1" || rpc.Params.FromUserID != "buyer_1" || rpc.Params.ToUserID != "provider_1" {
			t.Fatalf("unexpected params: %+v", rpc.Params)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "req_1",
			"result": map[string]any{
				"invoice": "inv_123",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/rpc", "app_1", "secret_1")
	result, err := client.CreateInvoice(context.Background(), CreateInvoiceInput{
		PostID:     "ord_1:ms_1",
		FromUserID: "buyer_1",
		ToUserID:   "provider_1",
		Asset:      "CKB",
		Amount:     "12.5",
		Message:    "prefund milestone",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if result.Invoice != "inv_123" {
		t.Fatalf("expected invoice inv_123, got %q", result.Invoice)
	}
}

func TestClientGetsInvoiceStatusFromJSONRPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var rpc struct {
			Method string `json:"method"`
			Params struct {
				Invoice string `json:"invoice"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode rpc payload: %v", err)
		}
		if rpc.Method != "tip.status" {
			t.Fatalf("expected method tip.status, got %q", rpc.Method)
		}
		if rpc.Params.Invoice != "inv_123" {
			t.Fatalf("expected invoice inv_123, got %q", rpc.Params.Invoice)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "req_2",
			"result": map[string]any{
				"state": "SETTLED",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/rpc", "app_1", "secret_1")
	result, err := client.GetInvoiceStatus(context.Background(), "inv_123")
	if err != nil {
		t.Fatalf("get invoice status: %v", err)
	}
	if result.State != "SETTLED" {
		t.Fatalf("expected state SETTLED, got %q", result.State)
	}
}

func TestClientQuotesPayoutFromJSONRPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var rpc struct {
			Method string `json:"method"`
			Params struct {
				UserID      string                `json:"userId"`
				Asset       string                `json:"asset"`
				Amount      string                `json:"amount"`
				Destination WithdrawalDestination `json:"destination"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode rpc payload: %v", err)
		}
		if rpc.Method != "withdrawal.quote" {
			t.Fatalf("expected method withdrawal.quote, got %q", rpc.Method)
		}
		if rpc.Params.UserID != "provider_1" || rpc.Params.Destination.Kind != "CKB_ADDRESS" {
			t.Fatalf("unexpected params: %+v", rpc.Params)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "req_3",
			"result": map[string]any{
				"asset":             "CKB",
				"amount":            "61",
				"minimumAmount":     "61",
				"availableBalance":  "124",
				"lockedBalance":     "0",
				"networkFee":        "0.00001",
				"receiveAmount":     "60.99999",
				"destinationValid":  true,
				"validationMessage": nil,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/rpc", "app_1", "secret_1")
	result, err := client.QuotePayout(context.Background(), QuotePayoutInput{
		UserID: "provider_1",
		Asset:  "CKB",
		Amount: "61",
		Destination: WithdrawalDestination{
			Kind:    "CKB_ADDRESS",
			Address: "ckt1qyqfth8m4fevfzh5hhd088s78qcdjjp8cehs7z8jhw",
		},
	})
	if err != nil {
		t.Fatalf("quote payout: %v", err)
	}
	if !result.DestinationValid || result.ReceiveAmount != "60.99999" {
		t.Fatalf("unexpected quote result: %+v", result)
	}
}

func TestClientRequestsPayoutFromJSONRPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var rpc struct {
			Method string `json:"method"`
			Params struct {
				UserID      string                `json:"userId"`
				Asset       string                `json:"asset"`
				Amount      string                `json:"amount"`
				Destination WithdrawalDestination `json:"destination"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode rpc payload: %v", err)
		}
		if rpc.Method != "withdrawal.request" {
			t.Fatalf("expected method withdrawal.request, got %q", rpc.Method)
		}
		if rpc.Params.UserID != "provider_1" || rpc.Params.Destination.Kind != "PAYMENT_REQUEST" {
			t.Fatalf("unexpected params: %+v", rpc.Params)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "req_4",
			"result": map[string]any{
				"id":    "wd_123",
				"state": "PENDING",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/rpc", "app_1", "secret_1")
	result, err := client.RequestPayout(context.Background(), RequestPayoutInput{
		UserID: "provider_1",
		Asset:  "USDI",
		Amount: "10",
		Destination: WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: "fiber:invoice:example",
		},
	})
	if err != nil {
		t.Fatalf("request payout: %v", err)
	}
	if result.ID != "wd_123" || result.State != "PENDING" {
		t.Fatalf("unexpected request result: %+v", result)
	}
}

func TestClientListsSettledFeedFromJSONRPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var rpc struct {
			Method string `json:"method"`
			Params struct {
				Limit int `json:"limit"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode rpc payload: %v", err)
		}
		if rpc.Method != "tip.settled_feed" {
			t.Fatalf("expected method tip.settled_feed, got %q", rpc.Method)
		}
		if rpc.Params.Limit != 20 {
			t.Fatalf("expected limit 20, got %+v", rpc.Params)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "req_5",
			"result": map[string]any{
				"items": []map[string]any{
					{
						"tipIntentId": "tip_1",
						"postId":      "ord_1:ms_1",
						"invoice":     "inv_123",
						"amount":      "12.5",
						"asset":       "CKB",
						"fromUserId":  "buyer_1",
						"toUserId":    "provider_1",
						"settledAt":   "2026-03-12T00:00:00Z",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/rpc", "app_1", "secret_1")
	result, err := client.ListSettledFeed(context.Background(), SettledFeedInput{Limit: 20})
	if err != nil {
		t.Fatalf("list settled feed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Invoice != "inv_123" {
		t.Fatalf("unexpected settled feed result: %+v", result)
	}
}

func TestClientListsWithdrawalStatusesFromDashboardSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var rpc struct {
			Method string `json:"method"`
			Params struct {
				UserID       string `json:"userId"`
				IncludeAdmin bool   `json:"includeAdmin"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode rpc payload: %v", err)
		}
		if rpc.Method != "dashboard.summary" {
			t.Fatalf("expected method dashboard.summary, got %q", rpc.Method)
		}
		if rpc.Params.UserID != "provider_1" || !rpc.Params.IncludeAdmin {
			t.Fatalf("unexpected params: %+v", rpc.Params)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "req_6",
			"result": map[string]any{
				"balance": "0",
				"balances": map[string]any{
					"available": "0",
					"pending":   "0",
					"locked":    "0",
					"asset":     "CKB",
				},
				"stats": map[string]any{
					"pendingCount":   0,
					"completedCount": 0,
					"failedCount":    0,
				},
				"tips":        []any{},
				"generatedAt": "2026-03-12T00:00:00Z",
				"admin": map[string]any{
					"withdrawals": []map[string]any{
						{
							"id":         "wd_123",
							"userId":     "provider_1",
							"asset":      "USDI",
							"amount":     "10",
							"state":      "PROCESSING",
							"retryCount": 0,
							"createdAt":  "2026-03-12T00:00:00Z",
							"updatedAt":  "2026-03-12T00:01:00Z",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/rpc", "app_1", "secret_1")
	result, err := client.ListWithdrawalStatuses(context.Background(), "provider_1")
	if err != nil {
		t.Fatalf("list withdrawal statuses: %v", err)
	}
	if len(result.Withdrawals) != 1 || result.Withdrawals[0].State != "PROCESSING" {
		t.Fatalf("unexpected withdrawal status result: %+v", result)
	}
}

func signPayloadForTest(secret string, payload []byte, ts, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
