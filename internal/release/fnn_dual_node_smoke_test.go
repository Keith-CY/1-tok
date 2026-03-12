package release

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunFNNDualNodeSmokeBootstrapsChannelAndPaysThroughAdapter(t *testing.T) {
	var invoiceMethods []string
	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read invoice node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode invoice node request: %v", err)
		}
		invoiceMethods = append(invoiceMethods, rpc.Method)
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id":                                "0x021111111111111111111111111111111111111111111111111111111111111111",
					"auto_accept_channel_ckb_funding_amount": "0x2540be400",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "accept_channel":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"state":   map[string]any{"state_name": "CHANNEL_READY"},
							"enabled": true,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected invoice node method %q", rpc.Method)
		}
	}))
	defer invoiceNode.Close()

	var payerMethods []string
	payerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read payer node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode payer node request: %v", err)
		}
		payerMethods = append(payerMethods, rpc.Method)
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id": "0x032222222222222222222222222222222222222222222222222222222222222222",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "open_channel":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"temporary_channel_id": "tmp_channel_1",
				},
			})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"state":   "CHANNEL_READY",
							"enabled": true,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected payer node method %q", rpc.Method)
		}
	}))
	defer payerNode.Close()

	adapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/":
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read adapter body: %v", err)
			}
			var rpc struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal(payload, &rpc); err != nil {
				t.Fatalf("decode adapter payload: %v", err)
			}
			switch rpc.Method {
			case "tip.create":
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "req1", "result": map[string]any{"invoice": "fiber:invoice:dual"}})
			case "tip.status":
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "req2", "result": map[string]any{"state": "UNPAID"}})
			case "withdrawal.quote":
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "req3", "result": map[string]any{"destinationValid": true}})
			case "withdrawal.request":
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "req4", "result": map[string]any{"id": "wd_dual_1", "state": "PENDING"}})
			case "dashboard.summary":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req5",
					"result": map[string]any{
						"admin": map[string]any{
							"withdrawals": []map[string]any{
								{"id": "wd_dual_1", "userId": "provider_smoke", "state": "PROCESSING"},
							},
						},
					},
				})
			default:
				t.Fatalf("unexpected adapter method %q", rpc.Method)
			}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer adapter.Close()

	summary, err := RunFNNDualNodeSmoke(context.Background(), FNNDualNodeSmokeConfig{
		InvoiceRPCURL:  invoiceNode.URL,
		PayerRPCURL:    payerNode.URL,
		InvoiceP2PHost: "fnn",
		PayerP2PHost:   "fnn2",
		P2PPort:        8228,
		FundingAmount:  "10000000000",
		PollInterval:   5 * time.Millisecond,
		WaitTimeout:    time.Second,
		Adapter: FNNAdapterSmokeConfig{
			BaseURL:        adapter.URL,
			AppID:          "app_local",
			HMACSecret:     "secret_local",
			IncludePayment: true,
		},
	})
	if err != nil {
		t.Fatalf("run dual fnn smoke: %v", err)
	}
	if summary.ChannelTemporaryID != "tmp_channel_1" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Adapter.WithdrawalID != "wd_dual_1" {
		t.Fatalf("expected adapter withdrawal id, got %+v", summary)
	}
	if got := strings.Join(invoiceMethods, ","); got != "node_info,connect_peer,accept_channel,list_channels" {
		t.Fatalf("unexpected invoice node sequence %q", got)
	}
	if got := strings.Join(payerMethods, ","); got != "node_info,connect_peer,open_channel,list_channels" {
		t.Fatalf("unexpected payer node sequence %q", got)
	}
	if summary.InvoicePeerID == "" || summary.PayerPeerID == "" {
		t.Fatalf("expected derived peer ids, got %+v", summary)
	}
}

func TestRunFNNDualNodeSmokeRetriesOpenChannelUntilPeerInitIsReady(t *testing.T) {
	var openChannelAttempts int

	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read invoice node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode invoice node request: %v", err)
		}
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id":                                "0x021111111111111111111111111111111111111111111111111111111111111111",
					"auto_accept_channel_ckb_funding_amount": "0x2540be400",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "accept_channel":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"state":   "CHANNEL_READY",
							"enabled": true,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected invoice node method %q", rpc.Method)
		}
	}))
	defer invoiceNode.Close()

	payerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read payer node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode payer node request: %v", err)
		}
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id": "0x032222222222222222222222222222222222222222222222222222222222222222",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "open_channel":
			openChannelAttempts++
			if openChannelAttempts == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      1,
					"error": map[string]any{
						"code":    -32602,
						"message": "Invalid parameter: Peer PeerId(QmTest)'s feature not found, waiting for peer to send Init message",
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"temporary_channel_id": "tmp_retry_1",
				},
			})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"state":   "CHANNEL_READY",
							"enabled": true,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected payer node method %q", rpc.Method)
		}
	}))
	defer payerNode.Close()

	summary, err := RunFNNDualNodeSmoke(context.Background(), FNNDualNodeSmokeConfig{
		InvoiceRPCURL:  invoiceNode.URL,
		PayerRPCURL:    payerNode.URL,
		InvoiceP2PHost: "fnn",
		PayerP2PHost:   "fnn2",
		P2PPort:        8228,
		FundingAmount:  "10000000000",
		PollInterval:   5 * time.Millisecond,
		WaitTimeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("run dual fnn smoke: %v", err)
	}
	if summary.ChannelTemporaryID != "tmp_retry_1" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if openChannelAttempts < 2 {
		t.Fatalf("expected open_channel retry, got %d attempts", openChannelAttempts)
	}
}

func TestRunFNNDualNodeSmokeRetriesAcceptChannelUntilTempIDPropagates(t *testing.T) {
	var acceptAttempts int

	invoiceNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read invoice node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode invoice node request: %v", err)
		}
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id":                                "0x021111111111111111111111111111111111111111111111111111111111111111",
					"auto_accept_channel_ckb_funding_amount": "0x2540be400",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "accept_channel":
			acceptAttempts++
			if acceptAttempts == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      1,
					"error": map[string]any{
						"code":    -32602,
						"message": "Invalid parameter: No channel with temp id Hash256(0xtemp) found",
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"state":   "CHANNEL_READY",
							"enabled": true,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected invoice node method %q", rpc.Method)
		}
	}))
	defer invoiceNode.Close()

	payerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read payer node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode payer node request: %v", err)
		}
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id": "0x032222222222222222222222222222222222222222222222222222222222222222",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "open_channel":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"temporary_channel_id": "tmp_accept_retry_1",
				},
			})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"state":   "CHANNEL_READY",
							"enabled": true,
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected payer node method %q", rpc.Method)
		}
	}))
	defer payerNode.Close()

	summary, err := RunFNNDualNodeSmoke(context.Background(), FNNDualNodeSmokeConfig{
		InvoiceRPCURL:  invoiceNode.URL,
		PayerRPCURL:    payerNode.URL,
		InvoiceP2PHost: "fnn",
		PayerP2PHost:   "fnn2",
		P2PPort:        8228,
		FundingAmount:  "10000000000",
		PollInterval:   5 * time.Millisecond,
		WaitTimeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("run dual fnn smoke: %v", err)
	}
	if summary.ChannelTemporaryID != "tmp_accept_retry_1" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if acceptAttempts < 2 {
		t.Fatalf("expected accept_channel retry, got %d attempts", acceptAttempts)
	}
}
