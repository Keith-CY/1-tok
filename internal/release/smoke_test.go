package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRunSmokeDefaultsToMinimalCrossServiceFlow(t *testing.T) {
	var settledPath string

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/rfqs" || strings.HasPrefix(r.URL.Path, "/api/v1/rfqs/")):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": map[string]any{
					"id": "ord_minimal",
				},
			})
		default:
			t.Fatalf("unexpected api request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	settlement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/invoices":
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_minimal"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"invoice": "inv_minimal"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			query, err := url.QueryUnescape(r.URL.RawQuery)
			if err != nil {
				t.Fatalf("decode query: %v", err)
			}
			if !strings.Contains(query, "kind=invoice") || !strings.Contains(query, "orderId=ord_minimal") {
				t.Fatalf("unexpected funding query %q", query)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"records": []map[string]any{
					{"id": "fund_invoice_1", "kind": "invoice", "state": "SETTLED"},
				},
			})
		default:
			t.Fatalf("unexpected settlement request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer settlement.Close()

	execution := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events":
			settledPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accepted":        true,
				"continueAllowed": true,
			})
		default:
			t.Fatalf("unexpected execution request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer execution.Close()

	summary, err := RunSmoke(context.Background(), Config{
		APIBaseURL:        api.URL,
		SettlementBaseURL: settlement.URL,
		ExecutionBaseURL:  execution.URL,
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}

	if summary.OrderID != "ord_minimal" || summary.Invoice != "inv_minimal" {
		t.Fatalf("unexpected minimal summary: %+v", summary)
	}
	if summary.FundingRecordCount != 1 {
		t.Fatalf("expected 1 funding record, got %+v", summary)
	}
	if summary.WithdrawalID != "" || summary.CodeAgentPolicy != "" {
		t.Fatalf("expected optional fields to be empty, got %+v", summary)
	}
	if settledPath != "/v1/carrier/events" {
		t.Fatalf("expected execution event path, got %q", settledPath)
	}
}

func TestRunSmokeExercisesMarketplaceFlow(t *testing.T) {
	var createdOrderID string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/rfqs" || strings.HasPrefix(r.URL.Path, "/api/v1/rfqs/")):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			var body struct {
				BuyerOrgID    string `json:"buyerOrgId"`
				ProviderOrgID string `json:"providerOrgId"`
				FundingMode   string `json:"fundingMode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create order: %v", err)
			}
			if body.BuyerOrgID != "buyer_1" || body.ProviderOrgID != "provider_1" || body.FundingMode != "credit" {
				t.Fatalf("unexpected create order body: %+v", body)
			}
			createdOrderID = "ord_1"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": map[string]any{
					"id":     createdOrderID,
					"status": "running",
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/settle"):
			if !strings.Contains(r.URL.Path, "/api/v1/orders/ord_1/milestones/ms_1/settle") {
				t.Fatalf("unexpected settle path %s", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": map[string]any{
					"id":     createdOrderID,
					"status": "completed",
				},
				"ledgerEntry": map[string]any{
					"kind": "platform_exposure",
				},
			})
		default:
			t.Fatalf("unexpected api request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	settlement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/invoices":
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_123"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"invoice":   "inv_123",
						"settledAt": "2026-03-12T00:00:00Z",
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/withdrawals":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wd_123", "state": "PENDING"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/withdrawals/status":
			if r.URL.Query().Get("providerOrgId") != "provider_1" {
				t.Fatalf("unexpected providerOrgId %q", r.URL.Query().Get("providerOrgId"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"withdrawals": []map[string]any{
					{"id": "wd_123", "state": "PROCESSING"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"records": []map[string]any{
					{"id": "fund_1", "kind": "invoice", "state": "SETTLED"},
					{"id": "fund_2", "kind": "withdrawal", "state": "PROCESSING"},
				},
			})
		default:
			t.Fatalf("unexpected settlement request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer settlement.Close()

	execution := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accepted":          true,
				"continueAllowed":   true,
				"recommendedAction": map[string]any{"type": "settle"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/carrier/codeagent/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"health": map[string]any{"healthy": true},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/codeagent/run":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"run": map[string]any{
					"result": map[string]any{
						"ok":              true,
						"policy_decision": "allow",
					},
				},
			})
		default:
			t.Fatalf("unexpected execution request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer execution.Close()

	summary, err := RunSmoke(context.Background(), Config{
		APIBaseURL:          api.URL,
		SettlementBaseURL:   settlement.URL,
		ExecutionBaseURL:    execution.URL,
		IncludeWithdrawal:   true,
		IncludeCarrierProbe: true,
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}

	if summary.OrderID != "ord_1" {
		t.Fatalf("expected order ord_1, got %+v", summary)
	}
	if summary.Invoice != "inv_123" || summary.WithdrawalID != "wd_123" {
		t.Fatalf("unexpected funding summary: %+v", summary)
	}
	if summary.FundingRecordCount != 2 {
		t.Fatalf("expected 2 funding records, got %+v", summary)
	}
	if summary.CodeAgentPolicy != "allow" {
		t.Fatalf("expected allow policy, got %+v", summary)
	}
}

func TestRunSmokePrefersRFQAwardFlowWhenMarketplaceRoutesExist(t *testing.T) {
	var requestLog []string

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rfq": map[string]any{
					"id": "rfq_smoke",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs/rfq_smoke/bids":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"bid": map[string]any{
					"id": "bid_smoke",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs/rfq_smoke/award":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rfq": map[string]any{
					"id":     "rfq_smoke",
					"status": "awarded",
				},
				"order": map[string]any{
					"id": "ord_from_rfq",
				},
			})
		default:
			t.Fatalf("unexpected api request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	settlement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/invoices":
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_rfq"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"invoice": "inv_rfq"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"records": []map[string]any{
					{"id": "fund_invoice_rfq", "kind": "invoice", "state": "SETTLED"},
				},
			})
		default:
			t.Fatalf("unexpected settlement request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer settlement.Close()

	execution := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events":
			_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true, "continueAllowed": true})
		default:
			t.Fatalf("unexpected execution request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer execution.Close()

	summary, err := RunSmoke(context.Background(), Config{
		APIBaseURL:        api.URL,
		SettlementBaseURL: settlement.URL,
		ExecutionBaseURL:  execution.URL,
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}

	if summary.OrderID != "ord_from_rfq" || summary.Invoice != "inv_rfq" {
		t.Fatalf("unexpected rfq summary: %+v", summary)
	}

	expectedPrefix := []string{
		"GET /healthz",
		"POST /api/v1/rfqs",
		"POST /api/v1/rfqs/rfq_smoke/bids",
		"POST /api/v1/rfqs/rfq_smoke/award",
	}
	for index, expected := range expectedPrefix {
		if requestLog[index] != expected {
			t.Fatalf("expected request %d to be %q, got %q", index, expected, requestLog[index])
		}
	}
}
