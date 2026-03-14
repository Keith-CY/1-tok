package release

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunFNNAdapterSmokeExercisesAdapterFlow(t *testing.T) {
	adapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/":
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var rpc struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal(payload, &rpc); err != nil {
				t.Fatalf("decode rpc payload: %v", err)
			}
			switch rpc.Method {
			case "tip.create":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_1",
					"result":  map[string]any{"invoice": "fiber:invoice:smoke"},
				})
			case "tip.status":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_2",
					"result":  map[string]any{"state": "UNPAID"},
				})
			case "withdrawal.quote":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_3",
					"result":  map[string]any{"destinationValid": true},
				})
			default:
				t.Fatalf("unexpected rpc method %q", rpc.Method)
			}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer adapter.Close()

	summary, err := RunFNNAdapterSmoke(context.Background(), FNNAdapterSmokeConfig{
		BaseURL:    adapter.URL,
		AppID:      "app_local",
		HMACSecret: "secret_local",
	})
	if err != nil {
		t.Fatalf("run fnn adapter smoke: %v", err)
	}
	if summary.Invoice != "fiber:invoice:smoke" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Status != "UNPAID" {
		t.Fatalf("expected UNPAID status, got %+v", summary)
	}
	if !summary.QuoteValid {
		t.Fatalf("expected quote validation to pass, got %+v", summary)
	}
}

func TestRunFNNAdapterSmokeCanRequestPaymentWhenEnabled(t *testing.T) {
	var methods []string
	var quoteAmount string
	var requestAmount string
	adapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/":
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var rpc struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal(payload, &rpc); err != nil {
				t.Fatalf("decode rpc payload: %v", err)
			}
			methods = append(methods, rpc.Method)
			switch rpc.Method {
			case "tip.create":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_1",
					"result":  map[string]any{"invoice": "fiber:invoice:payme"},
				})
			case "tip.status":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_2",
					"result":  map[string]any{"state": "UNPAID"},
				})
			case "withdrawal.quote":
				var params struct {
					Amount string `json:"amount"`
				}
				if err := json.Unmarshal(payload, &struct {
					Params *struct {
						Amount string `json:"amount"`
					} `json:"params"`
				}{Params: &params}); err != nil {
					t.Fatalf("decode withdrawal.quote params: %v", err)
				}
				quoteAmount = params.Amount
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_3",
					"result":  map[string]any{"destinationValid": true},
				})
			case "withdrawal.request":
				var params struct {
					Amount string `json:"amount"`
				}
				if err := json.Unmarshal(payload, &struct {
					Params *struct {
						Amount string `json:"amount"`
					} `json:"params"`
				}{Params: &params}); err != nil {
					t.Fatalf("decode withdrawal.request params: %v", err)
				}
				requestAmount = params.Amount
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_4",
					"result":  map[string]any{"id": "wd_live_1", "state": "PENDING"},
				})
			case "dashboard.summary":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      "req_5",
					"result": map[string]any{
						"admin": map[string]any{
							"withdrawals": []map[string]any{
								{"id": "wd_live_1", "userId": "provider_smoke", "state": "PROCESSING"},
							},
						},
					},
				})
			default:
				t.Fatalf("unexpected rpc method %q", rpc.Method)
			}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer adapter.Close()

	summary, err := RunFNNAdapterSmoke(context.Background(), FNNAdapterSmokeConfig{
		BaseURL:        adapter.URL,
		AppID:          "app_local",
		HMACSecret:     "secret_local",
		IncludePayment: true,
	})
	if err != nil {
		t.Fatalf("run fnn adapter smoke: %v", err)
	}
	if summary.WithdrawalID != "wd_live_1" {
		t.Fatalf("expected withdrawal id in summary, got %+v", summary)
	}
	if got := strings.Join(methods, ","); got != "tip.create,tip.status,withdrawal.quote,withdrawal.request,dashboard.summary" {
		t.Fatalf("unexpected rpc sequence %q", got)
	}
	if quoteAmount != "12" {
		t.Fatalf("expected quote amount to match invoice amount, got %q", quoteAmount)
	}
	if requestAmount != "12" {
		t.Fatalf("expected request amount to match invoice amount, got %q", requestAmount)
	}
}

func TestFNNAdapterSmokeConfigFromEnv(t *testing.T) {
	t.Setenv("RELEASE_FNN_ADAPTER_BASE_URL", "http://adapter:8091")
	t.Setenv("RELEASE_FNN_ADAPTER_APP_ID", "app_1")
	t.Setenv("RELEASE_FNN_ADAPTER_HMAC_SECRET", "secret")
	cfg := FNNAdapterSmokeConfigFromEnv()
	if cfg.BaseURL != "http://adapter:8091" {
		t.Errorf("BaseURL = %s", cfg.BaseURL)
	}
}

func TestEnvBoolDefaultFalse_True(t *testing.T) {
	t.Setenv("TEST_BOOL_DF", "true")
	if !envBoolDefaultFalse("TEST_BOOL_DF") {
		t.Error("expected true")
	}
}

func TestEnvBoolDefaultFalse_False(t *testing.T) {
	t.Setenv("TEST_BOOL_DF", "false")
	if envBoolDefaultFalse("TEST_BOOL_DF") {
		t.Error("expected false")
	}
}

func TestEnvBoolDefaultFalse_One(t *testing.T) {
	t.Setenv("TEST_BOOL_DF", "1")
	if !envBoolDefaultFalse("TEST_BOOL_DF") {
		t.Error("expected true for '1'")
	}
}

func TestEnvBoolDefaultFalse_Invalid(t *testing.T) {
	t.Setenv("TEST_BOOL_DF", "maybe")
	if envBoolDefaultFalse("TEST_BOOL_DF") {
		t.Error("expected false for invalid")
	}
}
