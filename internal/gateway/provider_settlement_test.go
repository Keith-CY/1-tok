package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

type gatewayStubSettlementProvisioner struct {
	result platform.EnsureProviderLiquidityResult
	err    error
}

func (s *gatewayStubSettlementProvisioner) EnsureProviderLiquidity(input platform.EnsureProviderLiquidityInput) (platform.EnsureProviderLiquidityResult, error) {
	if s.err != nil {
		return platform.EnsureProviderLiquidityResult{}, s.err
	}
	return s.result, nil
}

func TestProviderSettlementBindingLifecycleEndpoints(t *testing.T) {
	server := NewServer()

	payload := map[string]any{
		"providerOrgId":         "org_provider",
		"asset":                 "USDI",
		"peerId":                "peer_provider",
		"p2pAddress":            "/dns4/provider/tcp/8228/p2p/peer_provider",
		"paymentRequestBaseUrl": "https://carrier.example.com/payment-requests",
		"ownershipProof":        "proof_1",
		"udtTypeScript": map[string]any{
			"codeHash": "0xudt",
			"hashType": "type",
			"args":     "0x01",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/provider-settlement-bindings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("register settlement binding: %d %s", res.Code, res.Body.String())
	}

	var created struct {
		Binding platform.ProviderSettlementBinding `json:"binding"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Binding.ID == "" {
		t.Fatal("expected binding id")
	}
	if created.Binding.OwnershipProof != "" {
		t.Fatalf("expected ownership proof to be sanitized, got %q", created.Binding.OwnershipProof)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/provider-settlement-bindings/"+created.Binding.ID+"/verify", nil)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("verify settlement binding: %d %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/provider-settlement-bindings/org_provider", nil)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get settlement binding: %d %s", res.Code, res.Body.String())
	}

	var fetched struct {
		Binding platform.ProviderSettlementBinding `json:"binding"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Binding.Status != "active" {
		t.Fatalf("binding status = %s, want active", fetched.Binding.Status)
	}
	if fetched.Binding.OwnershipProof != "" {
		t.Fatalf("expected sanitized ownership proof, got %q", fetched.Binding.OwnershipProof)
	}
}

func TestProviderSettlementBindingGetNotFound(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provider-settlement-bindings/nonexistent", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestGetProviderSettlementPoolEndpoint(t *testing.T) {
	app := platform.NewAppWithMemory()
	server := NewServerWithApp(app)

	binding, err := app.RegisterProviderSettlementBinding(platform.ProviderSettlementBinding{
		ProviderOrgID:         "org_provider",
		Asset:                 "USDI",
		PeerID:                "peer_provider",
		P2PAddress:            "/dns4/provider/tcp/8228/p2p/peer_provider",
		PaymentRequestBaseURL: "https://carrier.example.com/payment-requests",
		OwnershipProof:        "proof_1",
		UDTTypeScript: platform.UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
	})
	if err != nil {
		t.Fatalf("register binding: %v", err)
	}
	if _, err := app.VerifyProviderSettlementBinding(binding.ID); err != nil {
		t.Fatalf("verify binding: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/provider-settlement-bindings/org_provider/pool", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get settlement pool: %d %s", res.Code, res.Body.String())
	}

	var response struct {
		Pool platform.ProviderLiquidityPool `json:"pool"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode pool response: %v", err)
	}
	if response.Pool.Status != platform.ProviderLiquidityPoolStatusHealthy {
		t.Fatalf("pool status = %s, want healthy", response.Pool.Status)
	}
	if response.Pool.ProviderOrgID != "org_provider" {
		t.Fatalf("provider org = %s", response.Pool.ProviderOrgID)
	}
}

func TestGetOrderProviderSettlementReservationEndpoint(t *testing.T) {
	app := platform.NewAppWithMemory()
	registerActiveCarrierBindingForGatewayTest(t, app, "org_provider")
	bindingID := registerActiveSettlementBindingForGatewayTest(t, app, "org_provider")
	_ = bindingID
	app.SetProviderSettlementProvisioner(&gatewayStubSettlementProvisioner{
		result: platform.EnsureProviderLiquidityResult{
			ChannelID:           "ch_1",
			ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 7_000,
		},
	})

	rfq, bid := createGatewayCarrierBackedRFQAndBid(t, app, "org_buyer", "org_provider", 5_000)
	_, order, err := app.AwardRFQ(rfq.ID, platform.AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: core.FundingModePrepaid,
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}

	server := NewServerWithApp(app)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+order.ID+"/provider-settlement-reservation", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get order reservation: %d %s", res.Code, res.Body.String())
	}

	var response struct {
		Reservation platform.ProviderLiquidityReservation `json:"reservation"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode reservation response: %v", err)
	}
	if response.Reservation.OrderID != order.ID {
		t.Fatalf("reservation order id = %s, want %s", response.Reservation.OrderID, order.ID)
	}
	if response.Reservation.ChannelID != "ch_1" {
		t.Fatalf("reservation channel id = %s, want ch_1", response.Reservation.ChannelID)
	}
}

func TestProviderSettlementDisconnectAndRecoverEndpoints(t *testing.T) {
	app := platform.NewAppWithMemory()
	registerActiveCarrierBindingForGatewayTest(t, app, "org_provider")
	registerActiveSettlementBindingForGatewayTest(t, app, "org_provider")
	app.SetProviderSettlementProvisioner(&gatewayStubSettlementProvisioner{
		result: platform.EnsureProviderLiquidityResult{
			ChannelID:           "ch_1",
			ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 7_000,
		},
	})

	rfq, bid := createGatewayCarrierBackedRFQAndBid(t, app, "org_buyer", "org_provider", 5_000)
	_, order, err := app.AwardRFQ(rfq.ID, platform.AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: core.FundingModePrepaid,
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}

	server, err := NewServerWithOptionsE(Options{
		App:             app,
		ExecutionTokens: serviceauth.NewTokenSet("exec-token"),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"reason": "provider closed channel"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/provider-settlement-bindings/org_provider/disconnect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(serviceauth.HeaderName, "exec-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("disconnect settlement binding: %d %s", res.Code, res.Body.String())
	}

	var disconnectResponse struct {
		Pool platform.ProviderLiquidityPool `json:"pool"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &disconnectResponse); err != nil {
		t.Fatalf("decode disconnect response: %v", err)
	}
	if disconnectResponse.Pool.Status != platform.ProviderLiquidityPoolStatusDisconnected {
		t.Fatalf("pool status after disconnect = %s, want disconnected", disconnectResponse.Pool.Status)
	}

	recoverProvisioner := &gatewayStubSettlementProvisioner{
		result: platform.EnsureProviderLiquidityResult{
			ChannelID:           "ch_1",
			ReuseSource:         platform.ProviderLiquidityReuseReused,
			ReadyChannelCount:   1,
			TotalSpendableCents: 7_000,
		},
	}
	app.SetProviderSettlementProvisioner(recoverProvisioner)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/provider-settlement-bindings/org_provider/recover", nil)
	req.Header.Set(serviceauth.HeaderName, "exec-token")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("recover settlement binding: %d %s", res.Code, res.Body.String())
	}

	var recoverResponse struct {
		Pool platform.ProviderLiquidityPool `json:"pool"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &recoverResponse); err != nil {
		t.Fatalf("decode recover response: %v", err)
	}
	if recoverResponse.Pool.Status != platform.ProviderLiquidityPoolStatusHealthy {
		t.Fatalf("pool status after recover = %s, want healthy", recoverResponse.Pool.Status)
	}

	updated, err := app.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("get recovered order: %v", err)
	}
	if updated.Status != core.OrderStatusRunning {
		t.Fatalf("order status after recover = %s, want running", updated.Status)
	}
}

func registerActiveCarrierBindingForGatewayTest(t *testing.T, app *platform.App, providerOrgID string) {
	t.Helper()

	binding, err := app.RegisterCarrierBinding(platform.ProviderCarrierBinding{
		ProviderOrgID:  providerOrgID,
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
	})
	if err != nil {
		t.Fatalf("register carrier binding: %v", err)
	}
	if _, err := app.VerifyCarrierBinding(binding.ID); err != nil {
		t.Fatalf("verify carrier binding: %v", err)
	}
}

func registerActiveSettlementBindingForGatewayTest(t *testing.T, app *platform.App, providerOrgID string) string {
	t.Helper()

	binding, err := app.RegisterProviderSettlementBinding(platform.ProviderSettlementBinding{
		ProviderOrgID:         providerOrgID,
		Asset:                 "USDI",
		PeerID:                "peer_provider",
		P2PAddress:            "/dns4/provider/tcp/8228/p2p/peer_provider",
		PaymentRequestBaseURL: "https://carrier.example.com/payment-requests",
		OwnershipProof:        "proof_1",
		UDTTypeScript: platform.UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
	})
	if err != nil {
		t.Fatalf("register settlement binding: %v", err)
	}
	if _, err := app.VerifyProviderSettlementBinding(binding.ID); err != nil {
		t.Fatalf("verify settlement binding: %v", err)
	}
	return binding.ID
}

func createGatewayCarrierBackedRFQAndBid(t *testing.T, app *platform.App, buyerOrgID, providerOrgID string, budgetCents int64) (platform.RFQ, platform.Bid) {
	t.Helper()

	rfq, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         buyerOrgID,
		Title:              "Carrier settlement order",
		Category:           "ops",
		Scope:              "run provider settlement flow",
		BudgetCents:        budgetCents,
		ResponseDeadlineAt: time.Date(2099, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	bid, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: providerOrgID,
		Message:       "bid",
		QuoteCents:    budgetCents,
		Milestones: []platform.BidMilestoneInput{{
			ID:             "ms_1",
			Title:          "Execution",
			BasePriceCents: budgetCents,
			BudgetCents:    budgetCents,
		}},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	return rfq, bid
}
