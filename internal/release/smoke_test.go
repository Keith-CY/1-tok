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

func TestRunSmokeSendsExecutionEventTokenWhenConfigured(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/rfqs" || strings.HasPrefix(r.URL.Path, "/api/v1/rfqs/")):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": map[string]any{"id": "ord_secure"},
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
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_secure"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"invoice": "inv_secure"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			_ = json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{{"id": "fund_secure"}}})
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
			if got := r.Header.Get("X-One-Tok-Service-Token"); got != "exec-event-token" {
				t.Fatalf("expected execution event token, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true, "continueAllowed": true})
		default:
			t.Fatalf("unexpected execution request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer execution.Close()

	_, err := RunSmoke(context.Background(), Config{
		APIBaseURL:          api.URL,
		SettlementBaseURL:   settlement.URL,
		ExecutionBaseURL:    execution.URL,
		ExecutionEventToken: "exec-event-token",
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}
}

func TestRunSmokeSendsSettlementServiceTokenWhenConfigured(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/rfqs" || strings.HasPrefix(r.URL.Path, "/api/v1/rfqs/")):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": map[string]any{"id": "ord_settlement_secure"},
			})
		default:
			t.Fatalf("unexpected api request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	var invoiceToken string
	var settledFeedToken string
	settlement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/invoices":
			invoiceToken = r.Header.Get("X-One-Tok-Service-Token")
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_settlement_secure"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			settledFeedToken = r.Header.Get("X-One-Tok-Service-Token")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"invoice": "inv_settlement_secure"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			_ = json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{{"id": "fund_settlement_secure"}}})
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

	_, err := RunSmoke(context.Background(), Config{
		APIBaseURL:             api.URL,
		SettlementBaseURL:      settlement.URL,
		SettlementServiceToken: "settlement-service-token",
		ExecutionBaseURL:       execution.URL,
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}
	if invoiceToken != "settlement-service-token" || settledFeedToken != "settlement-service-token" {
		t.Fatalf("expected settlement service token on invoice and settled feed, got invoice=%q settledFeed=%q", invoiceToken, settledFeedToken)
	}
}

func TestRunSmokeUsesIAMSessionsForMarketplaceAndFundingReads(t *testing.T) {
	type identity struct {
		Token string
		OrgID string
	}
	identities := map[string]identity{
		"buyer": {
			Token: "buyer-session-token",
			OrgID: "org_buyer_secure",
		},
		"provider": {
			Token: "provider-session-token",
			OrgID: "org_provider_secure",
		},
	}

	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/signup":
			var payload struct {
				OrganizationKind string `json:"organizationKind"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode iam signup: %v", err)
			}
			identity, ok := identities[payload.OrganizationKind]
			if !ok {
				t.Fatalf("unexpected organization kind %q", payload.OrganizationKind)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"organization": map[string]any{
					"id": identity.OrgID,
				},
				"session": map[string]any{
					"token": identity.Token,
				},
			})
		default:
			t.Fatalf("unexpected iam request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer iam.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["buyer"].Token {
				t.Fatalf("expected buyer auth on rfq create, got %q", got)
			}
			var payload struct {
				BuyerOrgID string `json:"buyerOrgId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode rfq payload: %v", err)
			}
			if payload.BuyerOrgID != "" {
				t.Fatalf("expected buyerOrgId to be omitted under IAM, got %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rfq": map[string]any{"id": "rfq_secure"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs/rfq_secure/bids":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["provider"].Token {
				t.Fatalf("expected provider auth on bid create, got %q", got)
			}
			var payload struct {
				ProviderOrgID string `json:"providerOrgId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode bid payload: %v", err)
			}
			if payload.ProviderOrgID != "" {
				t.Fatalf("expected providerOrgId to be omitted under IAM, got %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"bid": map[string]any{"id": "bid_secure"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rfqs/rfq_secure/award":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["buyer"].Token {
				t.Fatalf("expected buyer auth on award, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rfq":   map[string]any{"id": "rfq_secure", "status": "awarded"},
				"order": map[string]any{"id": "ord_secure"},
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
			var payload struct {
				BuyerOrgID    string `json:"buyerOrgId"`
				ProviderOrgID string `json:"providerOrgId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode invoice payload: %v", err)
			}
			if payload.BuyerOrgID != identities["buyer"].OrgID || payload.ProviderOrgID != identities["provider"].OrgID {
				t.Fatalf("unexpected invoice payload %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_secure"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"invoice": "inv_secure"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["provider"].Token {
				t.Fatalf("expected provider auth on funding records, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{
				{"id": "fund_secure_invoice"},
				{"id": "fund_secure_withdrawal"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/withdrawals":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["provider"].Token {
				t.Fatalf("expected provider auth on withdrawal request, got %q", got)
			}
			var payload struct {
				ProviderOrgID string `json:"providerOrgId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode withdrawal payload: %v", err)
			}
			if payload.ProviderOrgID != "" {
				t.Fatalf("expected providerOrgId to be omitted under IAM withdrawal, got %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wd_secure", "state": "PENDING"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/withdrawals/status":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["provider"].Token {
				t.Fatalf("expected provider auth on withdrawal status, got %q", got)
			}
			if got := r.URL.Query().Get("providerOrgId"); got != "" {
				t.Fatalf("expected providerOrgId query to be omitted under IAM withdrawal status, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"withdrawals": []map[string]any{{"id": "wd_secure", "state": "PROCESSING"}}})
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

	_, err := RunSmoke(context.Background(), Config{
		APIBaseURL:        api.URL,
		SettlementBaseURL: settlement.URL,
		ExecutionBaseURL:  execution.URL,
		IAMBaseURL:        iam.URL,
		IncludeWithdrawal: true,
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}
}

func TestRunSmokeUsesIAMProviderOrgForDirectOrderFallback(t *testing.T) {
	type identity struct {
		Token string
		OrgID string
	}
	identities := map[string]identity{
		"buyer": {
			Token: "buyer-session-token",
			OrgID: "org_buyer_direct",
		},
		"provider": {
			Token: "provider-session-token",
			OrgID: "org_provider_direct",
		},
	}

	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/signup":
			var payload struct {
				OrganizationKind string `json:"organizationKind"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode iam signup: %v", err)
			}
			identity, ok := identities[payload.OrganizationKind]
			if !ok {
				t.Fatalf("unexpected organization kind %q", payload.OrganizationKind)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"organization": map[string]any{
					"id": identity.OrgID,
				},
				"session": map[string]any{
					"token": identity.Token,
				},
			})
		default:
			t.Fatalf("unexpected iam request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer iam.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/rfqs"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["buyer"].Token {
				t.Fatalf("expected buyer auth on direct order create, got %q", got)
			}
			var payload struct {
				BuyerOrgID    string `json:"buyerOrgId"`
				ProviderOrgID string `json:"providerOrgId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode direct order payload: %v", err)
			}
			if payload.BuyerOrgID != "" || payload.ProviderOrgID != identities["provider"].OrgID {
				t.Fatalf("unexpected direct order payload %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"order": map[string]any{"id": "ord_direct_secure"},
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
			var payload struct {
				BuyerOrgID    string `json:"buyerOrgId"`
				ProviderOrgID string `json:"providerOrgId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode invoice payload: %v", err)
			}
			if payload.BuyerOrgID != identities["buyer"].OrgID || payload.ProviderOrgID != identities["provider"].OrgID {
				t.Fatalf("unexpected invoice payload %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"invoice": "inv_direct_secure"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/settled-feed":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"invoice": "inv_direct_secure"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			if got := r.Header.Get("Authorization"); got != "Bearer "+identities["provider"].Token {
				t.Fatalf("expected provider auth on funding records, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{{"id": "fund_direct_secure"}}})
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

	_, err := RunSmoke(context.Background(), Config{
		APIBaseURL:        api.URL,
		SettlementBaseURL: settlement.URL,
		ExecutionBaseURL:  execution.URL,
		IAMBaseURL:        iam.URL,
	})
	if err != nil {
		t.Fatalf("run smoke: %v", err)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("RELEASE_SMOKE_API_BASE_URL", "http://api:8080")
	t.Setenv("RELEASE_SMOKE_IAM_BASE_URL", "http://iam:8081")
	cfg := ConfigFromEnv()
	if cfg.APIBaseURL != "http://api:8080" {
		t.Errorf("APIBaseURL = %s", cfg.APIBaseURL)
	}
	if cfg.IAMBaseURL != "http://iam:8081" {
		t.Errorf("IAMBaseURL = %s", cfg.IAMBaseURL)
	}
}

func TestEnvBoolDefaultFalse(t *testing.T) {
	t.Setenv("TEST_BOOL", "true")
	if !envBoolDefaultFalse("TEST_BOOL") {
		t.Error("expected true")
	}
	t.Setenv("TEST_BOOL", "")
	if envBoolDefaultFalse("TEST_BOOL") {
		t.Error("expected false for empty")
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_ENV", "value")
	if envOrDefault("TEST_ENV", "default") != "value" {
		t.Error("expected value")
	}
	t.Setenv("TEST_ENV", "")
	if envOrDefault("TEST_ENV", "default") != "default" {
		t.Error("expected default")
	}
}

func TestRunSmoke_MissingAPIURL(t *testing.T) {
	_, err := RunSmoke(context.Background(), Config{})
	if err == nil {
		t.Error("expected error for missing API URL")
	}
}

func TestRunSmoke_APIHealthFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := RunSmoke(context.Background(), Config{
		APIBaseURL: srv.URL,
	})
	if err == nil {
		t.Error("expected error for unhealthy API")
	}
}

func TestRunSmoke_SettlementHealthFail(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer healthy.Close()

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()

	_, err := RunSmoke(context.Background(), Config{
		APIBaseURL:        healthy.URL,
		SettlementBaseURL: down.URL,
	})
	if err == nil {
		t.Error("expected error for unhealthy settlement")
	}
}
