package notifications

import (
	"testing"
)

func TestRegistry_RegisterAndList(t *testing.T) {
	r := NewRegistry(nil)
	r.Register("org_buyer", "https://example.com/hook")
	r.Register("org_provider", "https://example.com/hook2")

	list := r.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry(nil)
	r.Register("org_buyer", "https://example.com/hook")
	r.Unregister("org_buyer")

	list := r.List()
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry(nil)
	r.Register("org_buyer", "https://example.com/hook")

	reg, ok := r.Get("org_buyer")
	if !ok {
		t.Fatal("expected found")
	}
	if reg.URL != "https://example.com/hook" {
		t.Errorf("url = %s", reg.URL)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry(nil)
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestRegistry_WithWebhookService(t *testing.T) {
	svc := NewWebhookService()
	r := NewRegistry(svc)
	r.Register("org_buyer", "https://example.com/hook")

	// Verify it was also registered on the service
	r.Unregister("org_buyer")
	list := r.List()
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}
