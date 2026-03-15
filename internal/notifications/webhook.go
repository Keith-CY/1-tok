package notifications

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// WebhookPayload is the JSON body sent to webhook endpoints.
type WebhookPayload struct {
	Event     EventType      `json:"event"`
	Target    string         `json:"target"`
	Payload   map[string]any `json:"payload"`
	Timestamp string         `json:"timestamp"`
}

// WebhookService delivers notifications via HTTP webhooks.
type WebhookService struct {
	mu       sync.RWMutex
	urls     map[string]string // target → webhook URL
	secret   string           // HMAC signing secret
	client   HTTPClient
	fallback Service // optional fallback (e.g., InMemoryService)
}

// HTTPClient is the interface for making HTTP requests (for testing).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebhookOption configures the webhook service.
type WebhookOption func(*WebhookService)

// WithSecret sets the HMAC signing secret.
func WithSecret(secret string) WebhookOption {
	return func(s *WebhookService) { s.secret = secret }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client HTTPClient) WebhookOption {
	return func(s *WebhookService) { s.client = client }
}

// WithFallback sets a fallback service for logging/storage.
func WithFallback(fallback Service) WebhookOption {
	return func(s *WebhookService) { s.fallback = fallback }
}

// NewWebhookService creates a new webhook notification service.
func NewWebhookService(opts ...WebhookOption) *WebhookService {
	s := &WebhookService{
		urls:   make(map[string]string),
		client: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register associates a webhook URL with a target.
func (s *WebhookService) Register(target, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls[target] = url
}

// Unregister removes a webhook URL for a target.
func (s *WebhookService) Unregister(target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.urls, target)
}

// Send delivers a notification via webhook.
func (s *WebhookService) Send(event EventType, target string, payload map[string]any) error {
	// Also store in fallback if configured
	if s.fallback != nil {
		_ = s.fallback.Send(event, target, payload)
	}

	s.mu.RLock()
	url, ok := s.urls[target]
	s.mu.RUnlock()

	if !ok {
		return nil // No webhook registered — silent skip
	}

	wp := WebhookPayload{
		Event:     event,
		Target:    target,
		Payload:   payload,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(wp)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-1Tok-Event", string(event))

	if s.secret != "" {
		sig := signPayload(s.secret, body)
		req.Header.Set("X-1Tok-Signature", sig)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook delivery failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// List delegates to the fallback service if available.
func (s *WebhookService) List(target string) ([]Notification, error) {
	if s.fallback != nil {
		return s.fallback.List(target)
	}
	return nil, nil
}

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
