package execution

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"execution"}`))
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events" {
		var payload struct {
			OrderID     string               `json:"orderId"`
			MilestoneID string               `json:"milestoneId"`
			EventType   string               `json:"eventType"`
			UsageKind   core.UsageChargeKind `json:"usageKind"`
			AmountCents int64                `json:"amountCents"`
			ProofRef    string               `json:"proofRef"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		response := map[string]any{
			"accepted":        true,
			"continueAllowed": payload.EventType != "budget_low",
			"recommendedAction": map[string]any{
				"type":      actionForEvent(payload.EventType),
				"timestamp": time.Now().UTC(),
			},
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
	default:
		return "continue"
	}
}
