package execution

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMilestoneReadyEventSettlesThroughGateway(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":     "ord_1",
				"status": "running",
			},
			"ledgerEntry": map[string]any{
				"kind":        "platform_exposure",
				"amountCents": 1200,
			},
		})
	}))
	defer upstream.Close()

	server := NewServerWithUpstream(upstream.URL)
	body := map[string]any{
		"orderId":     "ord_1",
		"milestoneId": "ms_1",
		"eventType":   "milestone_ready",
		"summary":     "carrier completed milestone",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	if receivedPath != "/api/v1/orders/ord_1/milestones/ms_1/settle" {
		t.Fatalf("unexpected upstream path %s", receivedPath)
	}

	var response struct {
		ContinueAllowed bool `json:"continueAllowed"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.ContinueAllowed {
		t.Fatalf("expected continueAllowed true after settlement")
	}
}

func TestUsageReportedCanPauseOrderViaGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":     "ord_1",
				"status": "awaiting_budget",
			},
		})
	}))
	defer upstream.Close()

	server := NewServerWithUpstream(upstream.URL)
	body := map[string]any{
		"orderId":     "ord_1",
		"milestoneId": "ms_1",
		"eventType":   "usage_reported",
		"usageKind":   "external_api",
		"amountCents": 150,
		"proofRef":    "evt_1",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var response struct {
		ContinueAllowed   bool `json:"continueAllowed"`
		RecommendedAction struct {
			Type string `json:"type"`
		} `json:"recommendedAction"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ContinueAllowed {
		t.Fatalf("expected continueAllowed false")
	}

	if response.RecommendedAction.Type != "pause" {
		t.Fatalf("expected pause action, got %s", response.RecommendedAction.Type)
	}
}
