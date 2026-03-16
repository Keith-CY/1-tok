package carrier

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidCallbackSignature = errors.New("invalid callback signature")
	ErrExpiredCallback          = errors.New("callback timestamp expired")
	ErrMissingCallbackSecret    = errors.New("callback secret not configured")
	ErrMissingCallbackSignature = errors.New("callback signature missing")
)

// CallbackMaxAge is how old a callback timestamp can be.
const CallbackMaxAge = 5 * time.Minute

// CallbackEvent represents a signed lifecycle event from a Carrier.
type CallbackEvent struct {
	Type      string         `json:"type"` // job.started, job.completed, job.failed, usage.reported, heartbeat
	JobID     string         `json:"jobId"`
	BindingID string         `json:"bindingId"`
	Timestamp string         `json:"timestamp"`
	Signature string         `json:"signature"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// SignCallback creates an HMAC-SHA256 signature for a callback event.
func SignCallback(secret string, event CallbackEvent) string {
	data := fmt.Sprintf("%s|%s|%s|%s", event.Type, event.JobID, event.BindingID, event.Timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCallback checks the callback signature and timestamp.
func VerifyCallback(secret string, event CallbackEvent) error {
	if secret == "" {
		if strings.TrimSpace(event.Signature) == "" {
			return nil
		}
		return ErrMissingCallbackSecret
	}

	if strings.TrimSpace(event.Signature) == "" {
		return ErrMissingCallbackSignature
	}

	expected := SignCallback(secret, event)
	if !hmac.Equal([]byte(expected), []byte(event.Signature)) {
		return ErrInvalidCallbackSignature
	}

	ts, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		return fmt.Errorf("invalid callback timestamp: %w", err)
	}
	if time.Since(ts) > CallbackMaxAge {
		return ErrExpiredCallback
	}

	return nil
}
