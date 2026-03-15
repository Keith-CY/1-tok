//go:build integration

package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
	iamserver "github.com/chenyu/1-tok/internal/services/iam"
	"github.com/chenyu/1-tok/internal/identity"
)

func TestFullBusinessFlow(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")

	// Bootstrap platform app
	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Create IAM server
	iam := iamserver.NewServerWithOptions(iamserver.Options{
		Store: identity.NewMemoryStore(),
	})

	// Create gateway
	gw, err := gateway.NewServerWithOptionsE(gateway.Options{App: app})
	if err != nil {
		t.Fatal(err)
	}

	// === 1. Signup buyer ===
	signupPayload := `{"email":"buyer@flow.test","password":"correct horse battery staple 123","name":"Flow Buyer","organizationName":"Flow Buyers Inc","organizationKind":"buyer"}`
	buyerToken := signup(t, iam, signupPayload)

	// === 2. Signup provider ===
	providerSignup := `{"email":"provider@flow.test","password":"correct horse battery staple 123","name":"Flow Provider","organizationName":"Flow Providers Inc","organizationKind":"provider"}`
	_ = signup(t, iam, providerSignup)

	// === 3. List providers ===
	resp := gwRequest(t, gw, "GET", "/api/v1/providers", nil)
	assertStatus(t, resp, http.StatusOK)
	var provResp map[string]any
	json.Unmarshal(resp.Body.Bytes(), &provResp)
	t.Logf("Providers: %d", len(provResp["providers"].([]any)))

	// === 4. List listings ===
	resp = gwRequest(t, gw, "GET", "/api/v1/listings", nil)
	assertStatus(t, resp, http.StatusOK)

	// === 5. Create RFQ ===
	rfqPayload := map[string]any{
		"buyerOrgId": "org_1", "title": "Integration test RFQ",
		"category": "ai", "scope": "End-to-end flow test",
		"budgetCents": 50000,
		"responseDeadlineAt": time.Now().Add(72 * time.Hour).Format(time.RFC3339),
	}
	resp = gwRequest(t, gw, "POST", "/api/v1/rfqs", rfqPayload)
	assertStatus(t, resp, http.StatusCreated)
	var rfqResp map[string]any
	json.Unmarshal(resp.Body.Bytes(), &rfqResp)
	rfqID := rfqResp["rfq"].(map[string]any)["id"].(string)
	t.Logf("Created RFQ: %s", rfqID)

	// Verify default milestones generated
	rfqData := rfqResp["rfq"].(map[string]any)
	if milestones, ok := rfqData["defaultMilestones"].([]any); ok {
		if len(milestones) != 3 {
			t.Errorf("expected 3 default milestones, got %d", len(milestones))
		}
	}

	// === 6. Create bid ===
	bidPayload := map[string]any{
		"providerOrgId": "org_2", "message": "Integration bid",
		"quoteCents": 45000,
		"milestones": []map[string]any{
			{"id": "ms_1", "title": "Setup", "basePriceCents": 9000, "budgetCents": 9000},
			{"id": "ms_2", "title": "Execute", "basePriceCents": 27000, "budgetCents": 27000},
			{"id": "ms_3", "title": "Deliver", "basePriceCents": 9000, "budgetCents": 9000},
		},
	}
	resp = gwRequest(t, gw, "POST", fmt.Sprintf("/api/v1/rfqs/%s/bids", rfqID), bidPayload)
	assertStatus(t, resp, http.StatusCreated)
	var bidResp map[string]any
	json.Unmarshal(resp.Body.Bytes(), &bidResp)
	bidID := bidResp["bid"].(map[string]any)["id"].(string)
	t.Logf("Created bid: %s", bidID)

	// === 7. List bids ===
	resp = gwRequest(t, gw, "GET", fmt.Sprintf("/api/v1/rfqs/%s/bids", rfqID), nil)
	assertStatus(t, resp, http.StatusOK)

	// === 8. Award RFQ ===
	awardPayload := map[string]any{
		"bidId": bidID, "fundingMode": "credit",
	}
	resp = gwRequest(t, gw, "POST", fmt.Sprintf("/api/v1/rfqs/%s/award", rfqID), awardPayload)
	assertStatus(t, resp, http.StatusOK)
	var awardResp map[string]any
	json.Unmarshal(resp.Body.Bytes(), &awardResp)
	orderID := awardResp["order"].(map[string]any)["id"].(string)
	t.Logf("Awarded → Order: %s", orderID)

	// === 9. Get order ===
	resp = gwRequest(t, gw, "GET", fmt.Sprintf("/api/v1/orders/%s", orderID), nil)
	assertStatus(t, resp, http.StatusOK)

	// === 10. Record usage ===
	usagePayload := map[string]any{
		"kind": "token", "amountCents": 500,
	}
	resp = gwRequest(t, gw, "POST", fmt.Sprintf("/api/v1/orders/%s/milestones/ms_1/usage", orderID), usagePayload)
	assertStatus(t, resp, http.StatusOK)

	// === 11. Settle milestone ===
	settlePayload := map[string]any{"summary": "Setup complete"}
	resp = gwRequest(t, gw, "POST", fmt.Sprintf("/api/v1/orders/%s/milestones/ms_1/settle", orderID), settlePayload)
	assertStatus(t, resp, http.StatusOK)

	// === 12. Open dispute on settled milestone ===
	disputePayload := map[string]any{
		"milestoneId": "ms_1", "reason": "Quality below spec", "refundCents": 2000,
	}
	resp = gwRequest(t, gw, "POST", fmt.Sprintf("/api/v1/orders/%s/disputes", orderID), disputePayload)
	assertStatus(t, resp, http.StatusCreated)

	// === 13. List disputes ===
	resp = gwRequest(t, gw, "GET", "/api/v1/disputes", nil)
	assertStatus(t, resp, http.StatusOK)
	var dispResp map[string]any
	json.Unmarshal(resp.Body.Bytes(), &dispResp)
	disputes := dispResp["disputes"].([]any)
	var disputeID string
	for _, d := range disputes {
		dm := d.(map[string]any)
		status, _ := dm["status"].(string)
		dOrderID, _ := dm["orderId"].(string)
		if dOrderID == "" {
			dOrderID, _ = dm["orderID"].(string)
		}
		if status == "open" && dOrderID == orderID {
			disputeID, _ = dm["id"].(string)
			break
		}
	}
	if disputeID == "" {
		t.Log("No open dispute found for this order — skipping resolve")
	} else {
		// === 14. Resolve dispute ===
		resolvePayload := map[string]any{
			"resolution": "Partial refund approved", "resolvedBy": "ops_admin",
		}
		resp = gwRequest(t, gw, "POST", fmt.Sprintf("/api/v1/disputes/%s/resolve", disputeID), resolvePayload)
		assertStatus(t, resp, http.StatusOK)
	}

	// === 15. Create message ===
	msgPayload := map[string]any{
		"orderId": orderID, "author": "buyer", "body": "Thanks for the delivery",
	}
	resp = gwRequest(t, gw, "POST", "/api/v1/messages", msgPayload)
	assertStatus(t, resp, http.StatusCreated)

	// === 16. Credit decision ===
	creditPayload := map[string]any{
		"buyerOrgId": "org_1", "requestedCents": 100000,
	}
	resp = gwRequest(t, gw, "POST", "/api/v1/credits/decision", creditPayload)
	assertStatus(t, resp, http.StatusOK)

	// === 17. Health check ===
	resp = gwRequest(t, gw, "GET", "/healthz", nil)
	assertStatus(t, resp, http.StatusOK)

	t.Log("Full business flow completed successfully")
	_ = buyerToken
}

func signup(t *testing.T, iam http.Handler, payload string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	iam.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Session struct{ Token string } `json:"session"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.Session.Token
}

func gwRequest(t *testing.T, gw http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	} else {
		body = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, rec.Code, rec.Body.String())
	}
}
