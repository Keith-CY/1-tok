package carrier

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidIntegrationCallbackSignature = errors.New("invalid integration callback signature")

type IntegrationCallbackEnvelope struct {
	EventID            string `json:"eventId"`
	Sequence           int64  `json:"sequence"`
	EventType          string `json:"eventType"`
	BindingID          string `json:"bindingId"`
	CarrierExecutionID string `json:"carrierExecutionId"`
	AttemptID          string `json:"attemptId,omitempty"`
	CreatedAt          string `json:"createdAt,omitempty"`
	Payload            any    `json:"payload,omitempty"`
}

func SignIntegrationCallbackBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func VerifyIntegrationCallbackBody(secret string, body []byte, signature string) error {
	if strings.TrimSpace(secret) == "" {
		if strings.TrimSpace(signature) == "" {
			return nil
		}
		return ErrMissingCallbackSecret
	}
	if strings.TrimSpace(signature) == "" {
		return ErrMissingCallbackSignature
	}
	expected := SignIntegrationCallbackBody(secret, body)
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature))) {
		return ErrInvalidIntegrationCallbackSignature
	}
	return nil
}

func (e IntegrationCallbackEnvelope) ToCallbackEvent() CallbackEvent {
	payloadMap := map[string]any{}
	switch typed := e.Payload.(type) {
	case map[string]any:
		for key, value := range typed {
			payloadMap[key] = value
		}
	}
	if strings.TrimSpace(e.EventID) != "" {
		payloadMap["eventId"] = strings.TrimSpace(e.EventID)
	}
	if e.Sequence > 0 {
		payloadMap["sequence"] = float64(e.Sequence)
	}
	if strings.TrimSpace(e.AttemptID) != "" {
		payloadMap["attemptId"] = strings.TrimSpace(e.AttemptID)
	}
	if strings.TrimSpace(e.CarrierExecutionID) != "" {
		payloadMap["carrierExecutionId"] = strings.TrimSpace(e.CarrierExecutionID)
	}

	return CallbackEvent{
		Type:      NormalizeEventName(e.EventType),
		JobID:     firstNonEmptyPayloadValue(payloadMap, "jobId", "externalExecutionId", "oneTokJobId", "orderJobId", "executionJobId", "carrierExecutionId"),
		BindingID: strings.TrimSpace(e.BindingID),
		Timestamp: strings.TrimSpace(e.CreatedAt),
		Payload:   payloadMap,
	}
}

func firstNonEmptyPayloadValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
