package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chenyu/1-tok/internal/platform"
)

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
