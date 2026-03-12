package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestListDisputesReturnsPersistedCases(t *testing.T) {
	app := platform.NewAppWithMemory()
	order, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Operate agent",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1200,
				BudgetCents:    1800,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, _, err := app.SettleMilestone(order.ID, platform.SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle milestone: %v", err)
	}
	if _, _, _, err := app.OpenDispute(order.ID, platform.OpenDisputeInput{
		MilestoneID: "ms_1",
		Reason:      "carrier output was incomplete",
		RefundCents: 800,
	}); err != nil {
		t.Fatalf("open dispute: %v", err)
	}

	server := NewServerWithApp(app)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/disputes", nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Disputes []struct {
			OrderID string `json:"orderId"`
			Reason  string `json:"reason"`
		} `json:"disputes"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Disputes) != 1 || response.Disputes[0].Reason != "carrier output was incomplete" {
		t.Fatalf("unexpected disputes: %+v", response.Disputes)
	}
}

func TestResolveDisputeRequiresOpsMembershipAndReturnsResolvedCase(t *testing.T) {
	app := platform.NewAppWithMemory()
	order, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Operate agent",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1200,
				BudgetCents:    1800,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, _, err := app.SettleMilestone(order.ID, platform.SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle milestone: %v", err)
	}
	if _, _, _, err := app.OpenDispute(order.ID, platform.OpenDisputeInput{
		MilestoneID: "ms_1",
		Reason:      "carrier output was incomplete",
		RefundCents: 800,
	}); err != nil {
		t.Fatalf("open dispute: %v", err)
	}
	disputes, err := app.ListDisputes()
	if err != nil {
		t.Fatalf("list disputes: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_ops_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "ops_1",
						OrganizationKind: "ops",
						Role:             "ops_reviewer",
					},
				},
			},
		},
	})

	body, _ := json.Marshal(map[string]any{
		"resolution": "Approved reimbursement after provider remediation review.",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/disputes/"+disputes[0].ID+"/resolve", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer ops-session-token")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Dispute struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			ResolvedBy string `json:"resolvedBy"`
			Resolution string `json:"resolution"`
		} `json:"dispute"`
		Order struct {
			Milestones []struct {
				DisputeStatus string `json:"disputeStatus"`
			} `json:"milestones"`
		} `json:"order"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Dispute.ID != disputes[0].ID || response.Dispute.Status != "resolved" {
		t.Fatalf("unexpected dispute response: %+v", response.Dispute)
	}
	if response.Dispute.ResolvedBy != "usr_ops_1" || response.Dispute.Resolution == "" {
		t.Fatalf("expected resolver metadata, got %+v", response.Dispute)
	}
	if response.Order.Milestones[0].DisputeStatus != "resolved" {
		t.Fatalf("expected resolved milestone dispute status, got %+v", response.Order.Milestones)
	}
}

func TestListDisputesRejectsNonOpsMembershipWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	order, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Operate agent",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1200,
				BudgetCents:    1800,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, _, err := app.SettleMilestone(order.ID, platform.SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle milestone: %v", err)
	}
	if _, _, _, err := app.OpenDispute(order.ID, platform.OpenDisputeInput{
		MilestoneID: "ms_1",
		Reason:      "carrier output was incomplete",
		RefundCents: 800,
	}); err != nil {
		t.Fatalf("open dispute: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disputes", nil)
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestCreditDecisionRejectsNonOpsMembershipWhenIAMConfigured(t *testing.T) {
	server := NewServerWithOptions(Options{
		App: platform.NewAppWithMemory(),
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	body, _ := json.Marshal(map[string]any{
		"completedOrders":    12,
		"successfulPayments": 11,
		"failedPayments":     1,
		"disputedOrders":     1,
		"lifetimeSpendCents": 480000,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credits/decision", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestListOrdersScopesBuyerMembershipWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	if _, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Buyer one order",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{ID: "ms_1", Title: "Plan", BasePriceCents: 1200, BudgetCents: 1800},
		},
	}); err != nil {
		t.Fatalf("create order 1: %v", err)
	}
	if _, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_2",
		ProviderOrgID: "provider_2",
		Title:         "Buyer two order",
		FundingMode:   "credit",
		CreditLineID:  "credit_2",
		Milestones: []platform.CreateMilestoneInput{
			{ID: "ms_1", Title: "Plan", BasePriceCents: 900, BudgetCents: 1400},
		},
	}); err != nil {
		t.Fatalf("create order 2: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Orders []struct {
			BuyerOrgID string `json:"buyerOrgId"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Orders) != 1 || response.Orders[0].BuyerOrgID != "buyer_1" {
		t.Fatalf("expected only buyer_1 orders, got %+v", response.Orders)
	}
}

func TestListOrdersScopesProviderMembershipWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	if _, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Provider one order",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{ID: "ms_1", Title: "Plan", BasePriceCents: 1200, BudgetCents: 1800},
		},
	}); err != nil {
		t.Fatalf("create order 1: %v", err)
	}
	if _, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_2",
		ProviderOrgID: "provider_2",
		Title:         "Provider two order",
		FundingMode:   "credit",
		CreditLineID:  "credit_2",
		Milestones: []platform.CreateMilestoneInput{
			{ID: "ms_1", Title: "Plan", BasePriceCents: 900, BudgetCents: 1400},
		},
	}); err != nil {
		t.Fatalf("create order 2: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_provider_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "provider_1",
						OrganizationKind: "provider",
						Role:             "sales",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer provider-session-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Orders []struct {
			ProviderOrgID string `json:"providerOrgId"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Orders) != 1 || response.Orders[0].ProviderOrgID != "provider_1" {
		t.Fatalf("expected only provider_1 orders, got %+v", response.Orders)
	}
}

func TestListRFQsScopesBuyerMembershipWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	if _, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Buyer one rfq",
		Category:           "agent-ops",
		Scope:              "Scope one",
		BudgetCents:        4200,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create rfq 1: %v", err)
	}
	if _, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_2",
		Title:              "Buyer two rfq",
		Category:           "agent-ops",
		Scope:              "Scope two",
		BudgetCents:        5200,
		ResponseDeadlineAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create rfq 2: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_1",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs", nil)
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		RFQs []struct {
			BuyerOrgID string `json:"buyerOrgId"`
		} `json:"rfqs"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.RFQs) != 1 || response.RFQs[0].BuyerOrgID != "buyer_1" {
		t.Fatalf("expected only buyer_1 rfqs, got %+v", response.RFQs)
	}
}

func TestListRFQsShowsOpenAndAwardedProviderRFQsWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()

	openRFQ, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Open rfq",
		Category:           "agent-ops",
		Scope:              "Open scope",
		BudgetCents:        4200,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create open rfq: %v", err)
	}

	awardedProviderOneRFQ, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_2",
		Title:              "Awarded to provider one",
		Category:           "agent-ops",
		Scope:              "Award scope one",
		BudgetCents:        5200,
		ResponseDeadlineAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create awarded rfq 1: %v", err)
	}
	providerOneBid, err := app.CreateBid(awardedProviderOneRFQ.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "Provider one bid",
		QuoteCents:    4800,
		Milestones: []platform.BidMilestoneInput{
			{ID: "ms_1", Title: "Execute", BasePriceCents: 4800, BudgetCents: 5200},
		},
	})
	if err != nil {
		t.Fatalf("create provider one bid: %v", err)
	}
	if _, _, err := app.AwardRFQ(awardedProviderOneRFQ.ID, platform.AwardRFQInput{
		BidID:        providerOneBid.ID,
		FundingMode:  "credit",
		CreditLineID: "credit_1",
	}); err != nil {
		t.Fatalf("award provider one rfq: %v", err)
	}

	awardedProviderTwoRFQ, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_3",
		Title:              "Awarded to provider two",
		Category:           "agent-ops",
		Scope:              "Award scope two",
		BudgetCents:        6200,
		ResponseDeadlineAt: time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create awarded rfq 2: %v", err)
	}
	providerTwoBid, err := app.CreateBid(awardedProviderTwoRFQ.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_2",
		Message:       "Provider two bid",
		QuoteCents:    5800,
		Milestones: []platform.BidMilestoneInput{
			{ID: "ms_1", Title: "Execute", BasePriceCents: 5800, BudgetCents: 6200},
		},
	})
	if err != nil {
		t.Fatalf("create provider two bid: %v", err)
	}
	if _, _, err := app.AwardRFQ(awardedProviderTwoRFQ.ID, platform.AwardRFQInput{
		BidID:        providerTwoBid.ID,
		FundingMode:  "credit",
		CreditLineID: "credit_2",
	}); err != nil {
		t.Fatalf("award provider two rfq: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_provider_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "provider_1",
						OrganizationKind: "provider",
						Role:             "sales",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs", nil)
	req.Header.Set("Authorization", "Bearer provider-session-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		RFQs []struct {
			ID string `json:"id"`
		} `json:"rfqs"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.RFQs) != 2 {
		t.Fatalf("expected 2 rfqs for provider scope, got %+v", response.RFQs)
	}
	ids := []string{response.RFQs[0].ID, response.RFQs[1].ID}
	if !slices.Contains(ids, openRFQ.ID) || !slices.Contains(ids, awardedProviderOneRFQ.ID) || slices.Contains(ids, awardedProviderTwoRFQ.ID) {
		t.Fatalf("unexpected provider rfq scope: %+v", ids)
	}
}

func TestListRFQBidsScopesProviderMembershipWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	rfq, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Shared rfq",
		Category:           "agent-ops",
		Scope:              "Shared scope",
		BudgetCents:        4200,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}
	if _, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "Provider one bid",
		QuoteCents:    3900,
		Milestones: []platform.BidMilestoneInput{
			{ID: "ms_1", Title: "Execute", BasePriceCents: 3900, BudgetCents: 4200},
		},
	}); err != nil {
		t.Fatalf("create provider one bid: %v", err)
	}
	if _, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_2",
		Message:       "Provider two bid",
		QuoteCents:    4000,
		Milestones: []platform.BidMilestoneInput{
			{ID: "ms_1", Title: "Execute", BasePriceCents: 4000, BudgetCents: 4200},
		},
	}); err != nil {
		t.Fatalf("create provider two bid: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_provider_1",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "provider_1",
						OrganizationKind: "provider",
						Role:             "sales",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rfqs/"+rfq.ID+"/bids", nil)
	req.Header.Set("Authorization", "Bearer provider-session-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Bids []struct {
			ProviderOrgID string `json:"providerOrgId"`
		} `json:"bids"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Bids) != 1 || response.Bids[0].ProviderOrgID != "provider_1" {
		t.Fatalf("expected only provider_1 bids, got %+v", response.Bids)
	}
}

func TestGetOrderRejectsForeignBuyerWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	order, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Protected order",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{ID: "ms_1", Title: "Plan", BasePriceCents: 1200, BudgetCents: 1800},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_2",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_2",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestCreateDisputeRejectsForeignBuyerWhenIAMConfigured(t *testing.T) {
	app := platform.NewAppWithMemory()
	order, err := app.CreateOrder(platform.CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Protected order",
		FundingMode:   "credit",
		CreditLineID:  "credit_1",
		Milestones: []platform.CreateMilestoneInput{
			{ID: "ms_1", Title: "Plan", BasePriceCents: 1200, BudgetCents: 1800},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, _, err := app.SettleMilestone(order.ID, platform.SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle milestone: %v", err)
	}

	server := NewServerWithOptions(Options{
		App: app,
		IAM: &stubIAMClient{
			actor: iamclient.Actor{
				UserID: "usr_buyer_2",
				Memberships: []iamclient.ActorMembership{
					{
						OrganizationID:   "buyer_2",
						OrganizationKind: "buyer",
						Role:             "procurement",
					},
				},
			},
		},
	})

	body, _ := json.Marshal(map[string]any{
		"milestoneId": "ms_1",
		"reason":      "Not my order but trying anyway",
		"refundCents": 500,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+order.ID+"/disputes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer buyer-session-token")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
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
