package proxy

import (
	"net/http"
	"testing"
)

func TestNewSingleHostE_ValidURL(t *testing.T) {
	h, err := NewSingleHostE("http://localhost:8080", func(r *http.Request) {})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewSingleHostE_InvalidURL(t *testing.T) {
	_, err := NewSingleHostE("://bad", func(r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewSingleHostE_MissingScheme(t *testing.T) {
	_, err := NewSingleHostE("localhost:8080", func(r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for URL without scheme")
	}
}

func TestNewSingleHostE_EmptyHost(t *testing.T) {
	_, err := NewSingleHostE("http://", func(r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for URL without host")
	}
}

func TestNewSingleHost_PanicsOnBadURL(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid URL")
		}
	}()
	NewSingleHost("://bad", func(r *http.Request) {})
}
