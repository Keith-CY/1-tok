package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestExternalDependencyConfigFromEnv(t *testing.T) {
	t.Setenv("DEPENDENCY_FIBER_RPC_URL", "http://fiber:8091")
	t.Setenv("DEPENDENCY_CARRIER_GATEWAY_URL", "http://carrier:8090")
	cfg := ExternalDependencyConfigFromEnv()
	if cfg.FiberRPCURL != "http://fiber:8091" {
		t.Errorf("FiberRPCURL = %s", cfg.FiberRPCURL)
	}
	if cfg.CarrierGatewayURL != "http://carrier:8090" {
		t.Errorf("CarrierGatewayURL = %s", cfg.CarrierGatewayURL)
	}
}

func TestRunHealthcheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	if err := runHealthcheck(context.Background(), client, srv.URL+"/healthz"); err != nil {
		t.Fatal(err)
	}
}

func TestRunHealthcheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	if err := runHealthcheck(context.Background(), client, srv.URL+"/healthz"); err == nil {
		t.Error("expected error for 503")
	}
}

func TestExternalHealthcheckURL(t *testing.T) {
	tests := []struct {
		base     string
		explicit string
		want     string
	}{
		{"http://fiber:8091", "", "http://fiber:8091/healthz"},
		{"http://fiber:8091", "http://custom/check", "http://custom/check"},
	}
	for _, tt := range tests {
		if got := externalHealthcheckURL(tt.base, tt.explicit); got != tt.want {
			t.Errorf("externalHealthcheckURL(%q, %q) = %q, want %q", tt.base, tt.explicit, got, tt.want)
		}
	}
}
