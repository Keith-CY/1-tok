package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

type Server struct {
	client   *http.Client
	upstream string
}

type carrierEventPayload struct {
	OrderID     string               `json:"orderId"`
	MilestoneID string               `json:"milestoneId"`
	EventType   string               `json:"eventType"`
	UsageKind   core.UsageChargeKind `json:"usageKind"`
	AmountCents int64                `json:"amountCents"`
	ProofRef    string               `json:"proofRef"`
	Summary     string               `json:"summary"`
}

func NewServer() *Server {
	return NewServerWithUpstream(upstream())
}

func NewServerWithUpstream(upstream string) *Server {
	return &Server{
		client:   &http.Client{Timeout: 5 * time.Second},
		upstream: upstream,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"execution"}`))
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events" {
		var payload carrierEventPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		response, err := s.handleCarrierEvent(payload)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	http.NotFound(w, r)
}

func actionForEvent(eventType string) string {
	switch eventType {
	case "budget_low":
		return "pause"
	case "milestone_ready":
		return "settle"
	case "usage_reported":
		return "continue"
	default:
		return "continue"
	}
}

func (s *Server) handleCarrierEvent(payload carrierEventPayload) (map[string]any, error) {
	result := map[string]any{
		"accepted": true,
		"recommendedAction": map[string]any{
			"type":      actionForEvent(payload.EventType),
			"timestamp": time.Now().UTC(),
		},
	}

	switch payload.EventType {
	case "milestone_ready":
		var gatewayResponse map[string]any
		if err := s.postJSON(
			fmt.Sprintf("/api/v1/orders/%s/milestones/%s/settle", payload.OrderID, payload.MilestoneID),
			map[string]any{
				"milestoneId": payload.MilestoneID,
				"summary":     payload.Summary,
				"source":      "carrier",
			},
			&gatewayResponse,
		); err != nil {
			return nil, err
		}

		result["continueAllowed"] = true
		result["result"] = gatewayResponse
		return result, nil
	case "usage_reported":
		var gatewayResponse struct {
			Order struct {
				Status string `json:"status"`
			} `json:"order"`
		}
		if err := s.postJSON(
			fmt.Sprintf("/api/v1/orders/%s/milestones/%s/usage", payload.OrderID, payload.MilestoneID),
			map[string]any{
				"kind":        payload.UsageKind,
				"amountCents": payload.AmountCents,
				"proofRef":    payload.ProofRef,
			},
			&gatewayResponse,
		); err != nil {
			return nil, err
		}

		continueAllowed := gatewayResponse.Order.Status != string(core.OrderStatusAwaitingBudget)
		result["continueAllowed"] = continueAllowed
		if !continueAllowed {
			result["recommendedAction"] = map[string]any{
				"type":      "pause",
				"timestamp": time.Now().UTC(),
			}
		}
		result["result"] = gatewayResponse
		return result, nil
	case "budget_low":
		result["continueAllowed"] = false
		return result, nil
	default:
		result["continueAllowed"] = true
		return result, nil
	}
}

func (s *Server) postJSON(path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.upstream+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		responseBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("gateway returned %d: %s", res.StatusCode, string(responseBody))
	}

	if target == nil {
		return nil
	}

	return json.NewDecoder(res.Body).Decode(target)
}

func upstream() string {
	if value := os.Getenv("API_GATEWAY_UPSTREAM"); value != "" {
		return value
	}
	return "http://127.0.0.1:8080"
}
