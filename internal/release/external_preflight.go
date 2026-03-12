package release

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type ExternalDependencyConfig struct {
	FiberRPCURL           string
	FiberHealthcheckURL   string
	CarrierGatewayURL     string
	CarrierHealthcheckURL string
	CarrierGatewayToken   string
}

func ExternalDependencyConfigFromEnv() ExternalDependencyConfig {
	return ExternalDependencyConfig{
		FiberRPCURL:           strings.TrimSpace(os.Getenv("DEPENDENCY_FIBER_RPC_URL")),
		FiberHealthcheckURL:   strings.TrimSpace(os.Getenv("DEPENDENCY_FIBER_HEALTHCHECK_URL")),
		CarrierGatewayURL:     strings.TrimSpace(os.Getenv("DEPENDENCY_CARRIER_GATEWAY_URL")),
		CarrierHealthcheckURL: strings.TrimSpace(os.Getenv("DEPENDENCY_CARRIER_HEALTHCHECK_URL")),
		CarrierGatewayToken:   strings.TrimSpace(os.Getenv("DEPENDENCY_CARRIER_GATEWAY_API_TOKEN")),
	}
}

func RunExternalDependencyPreflight(ctx context.Context, cfg ExternalDependencyConfig) error {
	if strings.TrimSpace(cfg.FiberRPCURL) == "" {
		return errors.New("DEPENDENCY_FIBER_RPC_URL is required")
	}
	if strings.TrimSpace(cfg.CarrierGatewayURL) == "" {
		return errors.New("DEPENDENCY_CARRIER_GATEWAY_URL is required")
	}
	if strings.TrimSpace(cfg.CarrierGatewayToken) == "" {
		return errors.New("DEPENDENCY_CARRIER_GATEWAY_API_TOKEN is required")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	checks := []struct {
		name string
		url  string
	}{
		{
			name: "fiber",
			url:  externalHealthcheckURL(cfg.FiberRPCURL, cfg.FiberHealthcheckURL),
		},
		{
			name: "carrier",
			url:  externalHealthcheckURL(cfg.CarrierGatewayURL, cfg.CarrierHealthcheckURL),
		},
	}

	for _, check := range checks {
		if err := runHealthcheck(ctx, client, check.url); err != nil {
			return fmt.Errorf("%s preflight failed: %w", check.name, err)
		}
	}

	return nil
}

func externalHealthcheckURL(baseURL, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/healthz"
}

func runHealthcheck(ctx context.Context, client *http.Client, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", res.StatusCode, target)
	}
	return nil
}
