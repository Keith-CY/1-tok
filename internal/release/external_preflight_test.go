package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunExternalDependencyPreflightUsesDefaultHealthzRoutes(t *testing.T) {
	fiberHits := 0
	fiber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("expected default fiber healthz path, got %s", r.URL.Path)
		}
		fiberHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer fiber.Close()

	carrierHits := 0
	carrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("expected default carrier healthz path, got %s", r.URL.Path)
		}
		carrierHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer carrier.Close()

	err := RunExternalDependencyPreflight(context.Background(), ExternalDependencyConfig{
		FiberRPCURL:         fiber.URL,
		CarrierGatewayURL:   carrier.URL,
		CarrierGatewayToken: "carrier-token",
	})
	if err != nil {
		t.Fatalf("run external dependency preflight: %v", err)
	}
	if fiberHits != 1 || carrierHits != 1 {
		t.Fatalf("expected one healthcheck each, got fiber=%d carrier=%d", fiberHits, carrierHits)
	}
}

func TestRunExternalDependencyPreflightHonorsHealthcheckOverrides(t *testing.T) {
	fiber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ready" {
			t.Fatalf("expected overridden fiber path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer fiber.Close()

	carrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/live" {
			t.Fatalf("expected overridden carrier path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer carrier.Close()

	err := RunExternalDependencyPreflight(context.Background(), ExternalDependencyConfig{
		FiberRPCURL:           fiber.URL,
		FiberHealthcheckURL:   fiber.URL + "/ready",
		CarrierGatewayURL:     carrier.URL,
		CarrierHealthcheckURL: carrier.URL + "/live",
		CarrierGatewayToken:   "carrier-token",
	})
	if err != nil {
		t.Fatalf("run external dependency preflight with overrides: %v", err)
	}
}
