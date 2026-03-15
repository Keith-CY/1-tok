package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestWebhookService_Send(t *testing.T) {
	var received WebhookPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	svc := NewWebhookService()
	svc.Register("org_buyer", server.URL)

	err := svc.Send(EventOrderCreated, "org_buyer", map[string]any{"orderId": "ord_1"})
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if received.Event != EventOrderCreated {
		t.Errorf("event = %s", received.Event)
	}
	if received.Target != "org_buyer" {
		t.Errorf("target = %s", received.Target)
	}
}

func TestWebhookService_NoURL(t *testing.T) {
	svc := NewWebhookService()
	// No URL registered — should silently skip
	err := svc.Send(EventOrderCreated, "org_unknown", nil)
	if err != nil {
		t.Fatalf("expected nil for unregistered target, got %v", err)
	}
}

func TestWebhookService_WithSignature(t *testing.T) {
	var sigHeader string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sigHeader = r.Header.Get("X-1Tok-Signature")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	svc := NewWebhookService(WithSecret("test-secret"))
	svc.Register("org_buyer", server.URL)

	svc.Send(EventOrderCreated, "org_buyer", map[string]any{"orderId": "ord_1"})

	mu.Lock()
	defer mu.Unlock()
	if sigHeader == "" {
		t.Error("expected signature header")
	}
}

func TestWebhookService_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	svc := NewWebhookService()
	svc.Register("org_buyer", server.URL)

	err := svc.Send(EventOrderCreated, "org_buyer", nil)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestWebhookService_WithFallback(t *testing.T) {
	fallback := NewInMemoryService()
	svc := NewWebhookService(WithFallback(fallback))

	svc.Send(EventOrderCreated, "org_buyer", map[string]any{"orderId": "ord_1"})

	if fallback.Count() != 1 {
		t.Errorf("fallback count = %d", fallback.Count())
	}
}

func TestWebhookService_List_WithFallback(t *testing.T) {
	fallback := NewInMemoryService()
	svc := NewWebhookService(WithFallback(fallback))

	svc.Send(EventOrderCreated, "org_buyer", nil)
	list, err := svc.List("org_buyer")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list count = %d", len(list))
	}
}

func TestWebhookService_List_NoFallback(t *testing.T) {
	svc := NewWebhookService()
	list, err := svc.List("org_buyer")
	if err != nil {
		t.Fatal(err)
	}
	if list != nil {
		t.Errorf("expected nil, got %v", list)
	}
}

func TestWebhookService_Unregister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	svc := NewWebhookService()
	svc.Register("org_buyer", server.URL)
	svc.Unregister("org_buyer")

	// Should silently skip after unregister
	err := svc.Send(EventOrderCreated, "org_buyer", nil)
	if err != nil {
		t.Fatalf("expected nil after unregister, got %v", err)
	}
}

func TestWebhookService_EventHeader(t *testing.T) {
	var eventHeader string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		eventHeader = r.Header.Get("X-1Tok-Event")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	svc := NewWebhookService()
	svc.Register("org_buyer", server.URL)
	svc.Send(EventMilestoneSettled, "org_buyer", nil)

	mu.Lock()
	defer mu.Unlock()
	if eventHeader != string(EventMilestoneSettled) {
		t.Errorf("event header = %s", eventHeader)
	}
}

type mockHTTPClient struct {
	statusCode int
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(&bytes.Buffer{}),
	}, nil
}

func TestWebhookService_CustomHTTPClient(t *testing.T) {
	svc := NewWebhookService(WithHTTPClient(&mockHTTPClient{statusCode: 200}))
	svc.Register("org_buyer", "http://example.com/hook")

	err := svc.Send(EventOrderCreated, "org_buyer", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWebhookService_InvalidURL(t *testing.T) {
	svc := NewWebhookService()
	svc.Register("org_buyer", "://invalid")

	err := svc.Send(EventOrderCreated, "org_buyer", nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
