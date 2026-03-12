package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/platform"
)

type stubIAMClient struct {
	token string
	actor iamclient.Actor
}

func (s *stubIAMClient) GetActor(_ context.Context, bearerToken string) (iamclient.Actor, error) {
	s.token = bearerToken
	return s.actor, nil
}

func TestCreateOrderReturnsCreditFundingAndMilestones(t *testing.T) {
	server := NewServer()

	payload := map[string]any{
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"title":         "Operate agent",
		"fundingMode":   "credit",
		"creditLineId":  "credit_1",
		"milestones": []map[string]any{
			{
				"id":             "ms_1",
				"title":          "Plan",
				"basePriceCents": 1200,
				"budgetCents":    1800,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.Code)
	}

	var response struct {
		Order struct {
			FundingMode string `json:"fundingMode"`
			CreditLine  string `json:"creditLineId"`
			Status      string `json:"status"`
		} `json:"order"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Order.FundingMode != "credit" {
		t.Fatalf("expected credit funding mode, got %s", response.Order.FundingMode)
	}

	if response.Order.CreditLine != "credit_1" {
		t.Fatalf("expected credit line id credit_1, got %s", response.Order.CreditLine)
	}

	if response.Order.Status != "running" {
		t.Fatalf("expected running order, got %s", response.Order.Status)
	}
}

func TestCreateOrderDerivesBuyerOrgFromAuthenticatedMembership(t *testing.T) {
	server := NewServerWithOptions(Options{
		App: platform.NewAppWithMemory(),
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_auth_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	payload := map[string]any{
		"providerOrgId": "provider_1",
		"title":         "Operate agent",
		"fundingMode":   "credit",
		"creditLineId":  "credit_1",
		"milestones": []map[string]any{
			{
				"id":             "ms_1",
				"title":          "Plan",
				"basePriceCents": 1200,
				"budgetCents":    1800,
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Order struct {
			BuyerOrgID string `json:"buyerOrgId"`
		} `json:"order"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Order.BuyerOrgID != "buyer_auth_1" {
		t.Fatalf("expected authenticated buyer org, got %+v", response)
	}
}

func TestCreateRFQDerivesBuyerOrgFromAuthenticatedMembership(t *testing.T) {
	server := NewServerWithOptions(Options{
		App: platform.NewAppWithMemory(),
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_auth_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	payload := map[string]any{
		"title":              "Need carrier-backed triage",
		"category":           "agent-ops",
		"scope":              "Investigate failures and propose a fix plan.",
		"budgetCents":        8000,
		"responseDeadlineAt": "2026-03-15T12:00:00Z",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		RFQ struct {
			BuyerOrgID string `json:"buyerOrgId"`
			Status     string `json:"status"`
		} `json:"rfq"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.RFQ.BuyerOrgID != "buyer_auth_1" {
		t.Fatalf("expected authenticated buyer org, got %+v", response)
	}

	if response.RFQ.Status != "open" {
		t.Fatalf("expected open rfq, got %s", response.RFQ.Status)
	}
}

func TestCreateBidDerivesProviderOrgFromAuthenticatedMembership(t *testing.T) {
	app := platform.NewAppWithMemory()
	rfq, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures and propose a fix plan.",
		BudgetCents:        8_000,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_provider_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "provider_auth_1",
						OrganizationKind: "provider",
						Role:             "sales",
					},
				},
			},
		},
	})

	payload := map[string]any{
		"message":    "Carrier-ready response",
		"quoteCents": 7200,
		"milestones": []map[string]any{
			{
				"id":             "ms_1",
				"title":          "Triage",
				"basePriceCents": 3000,
				"budgetCents":    3600,
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/"+rfq.ID+"/bids", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer provider-session-token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Bid struct {
			ProviderOrgID string `json:"providerOrgId"`
			Status        string `json:"status"`
		} `json:"bid"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Bid.ProviderOrgID != "provider_auth_1" {
		t.Fatalf("expected authenticated provider org, got %+v", response)
	}

	if response.Bid.Status != "open" {
		t.Fatalf("expected open bid, got %s", response.Bid.Status)
	}
}

func TestAwardRFQCreatesOrderFromWinningBid(t *testing.T) {
	app := platform.NewAppWithMemory()
	rfq, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_auth_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures and propose a fix plan.",
		BudgetCents:        8_000,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	bid, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "Carrier-ready response",
		QuoteCents:    7_200,
		Milestones: []platform.BidMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 3000,
				BudgetCents:    3600,
			},
		},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_auth_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	payload := map[string]any{
		"bidId":        bid.ID,
		"fundingMode":  "credit",
		"creditLineId": "credit_1",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rfqs/"+rfq.ID+"/award", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		RFQ struct {
			Status       string `json:"status"`
			AwardedBidID string `json:"awardedBidId"`
		} `json:"rfq"`
		Order struct {
			ProviderOrgID string `json:"providerOrgId"`
			FundingMode   string `json:"fundingMode"`
		} `json:"order"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.RFQ.Status != "awarded" || response.RFQ.AwardedBidID != bid.ID {
		t.Fatalf("unexpected rfq response: %+v", response.RFQ)
	}

	if response.Order.ProviderOrgID != "provider_1" || response.Order.FundingMode != "credit" {
		t.Fatalf("unexpected order response: %+v", response.Order)
	}
}

func TestCarrierMilestoneSettlementReturnsLedgerEntry(t *testing.T) {
	server := NewServer()

	create := map[string]any{
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"title":         "Operate agent",
		"fundingMode":   "credit",
		"creditLineId":  "credit_1",
		"milestones": []map[string]any{
			{
				"id":             "ms_1",
				"title":          "Plan",
				"basePriceCents": 1200,
				"budgetCents":    1800,
			},
		},
	}

	body, _ := json.Marshal(create)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)

	var created struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	payload := map[string]any{
		"milestoneId": "ms_1",
		"summary":     "carrier finished work",
		"source":      "carrier",
	}

	settleBody, _ := json.Marshal(payload)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+created.Order.ID+"/milestones/ms_1/settle",
		bytes.NewReader(settleBody),
	)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var response struct {
		LedgerEntry struct {
			Kind        string `json:"kind"`
			AmountCents int64  `json:"amountCents"`
		} `json:"ledgerEntry"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode settlement response: %v", err)
	}

	if response.LedgerEntry.Kind != "platform_exposure" {
		t.Fatalf("expected platform exposure entry, got %s", response.LedgerEntry.Kind)
	}

	if response.LedgerEntry.AmountCents != 1200 {
		t.Fatalf("expected 1200 cents, got %d", response.LedgerEntry.AmountCents)
	}
}
