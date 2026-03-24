package demoenv

import (
	"os"
	"strconv"
	"strings"
)

type ActorConfig struct {
	Email            string `json:"email"`
	Password         string `json:"-"`
	Name             string `json:"name"`
	OrganizationName string `json:"organizationName"`
	OrganizationKind string `json:"organizationKind"`
	OrganizationID   string `json:"organizationId"`
}

type Config struct {
	APIBaseURL                string      `json:"apiBaseUrl"`
	IAMBaseURL                string      `json:"iamBaseUrl"`
	SettlementBaseURL         string      `json:"settlementBaseUrl"`
	ExecutionBaseURL          string      `json:"executionBaseUrl"`
	CarrierBaseURL            string      `json:"carrierBaseUrl"`
	FiberAdapterBaseURL       string      `json:"fiberAdapterBaseUrl"`
	SettlementServiceToken    string      `json:"-"`
	Buyer                     ActorConfig `json:"buyer"`
	Provider                  ActorConfig `json:"provider"`
	Ops                       ActorConfig `json:"ops"`
	MinBuyerBalanceCents      int64       `json:"minBuyerBalanceCents"`
	MinProviderLiquidityCents int64       `json:"minProviderLiquidityCents"`
	BuyerTopUpAmount          string      `json:"buyerTopUpAmount"`
	ResourcePrefix            string      `json:"resourcePrefix"`
}

func ConfigFromEnv() Config {
	return Config{
		APIBaseURL:             firstNonEmptyEnv("DEMO_API_BASE_URL", "RELEASE_USDI_E2E_API_BASE_URL", "RELEASE_SMOKE_API_BASE_URL"),
		IAMBaseURL:             firstNonEmptyEnv("DEMO_IAM_BASE_URL", "RELEASE_USDI_E2E_IAM_BASE_URL", "RELEASE_SMOKE_IAM_BASE_URL"),
		SettlementBaseURL:      firstNonEmptyEnv("DEMO_SETTLEMENT_BASE_URL", "RELEASE_USDI_E2E_SETTLEMENT_BASE_URL", "RELEASE_SMOKE_SETTLEMENT_BASE_URL"),
		ExecutionBaseURL:       firstNonEmptyEnv("DEMO_EXECUTION_BASE_URL", "RELEASE_USDI_E2E_EXECUTION_BASE_URL", "RELEASE_SMOKE_EXECUTION_BASE_URL"),
		CarrierBaseURL:         firstNonEmptyEnv("DEMO_CARRIER_BASE_URL", "RELEASE_USDI_E2E_CARRIER_BASE_URL", "CARRIER_GATEWAY_URL"),
		FiberAdapterBaseURL:    firstNonEmptyEnv("DEMO_FIBER_ADAPTER_BASE_URL", "RELEASE_USDI_E2E_FIBER_ADAPTER_BASE_URL", "FIBER_RPC_URL"),
		SettlementServiceToken: firstNonEmptyEnv("DEMO_SETTLEMENT_SERVICE_TOKEN", "RELEASE_USDI_E2E_SETTLEMENT_SERVICE_TOKEN", "RELEASE_SMOKE_SETTLEMENT_SERVICE_TOKEN", "SETTLEMENT_SERVICE_TOKEN"),
		Buyer: ActorConfig{
			Email:            envOrDefault("DEMO_BUYER_EMAIL", "demo-buyer@example.com"),
			Password:         envOrDefault("DEMO_BUYER_PASSWORD", "correct horse battery staple 123"),
			Name:             envOrDefault("DEMO_BUYER_NAME", "Demo Buyer"),
			OrganizationName: envOrDefault("DEMO_BUYER_ORG_NAME", "Demo Buyer Org"),
			OrganizationKind: envOrDefault("DEMO_BUYER_ORG_KIND", "buyer"),
			OrganizationID:   strings.TrimSpace(os.Getenv("DEMO_BUYER_ORG_ID")),
		},
		Provider: ActorConfig{
			Email:            envOrDefault("DEMO_PROVIDER_EMAIL", "demo-provider@example.com"),
			Password:         envOrDefault("DEMO_PROVIDER_PASSWORD", "correct horse battery staple 123"),
			Name:             envOrDefault("DEMO_PROVIDER_NAME", "Demo Provider"),
			OrganizationName: envOrDefault("DEMO_PROVIDER_ORG_NAME", "Demo Provider Org"),
			OrganizationKind: envOrDefault("DEMO_PROVIDER_ORG_KIND", "provider"),
			OrganizationID:   strings.TrimSpace(os.Getenv("DEMO_PROVIDER_ORG_ID")),
		},
		Ops: ActorConfig{
			Email:            envOrDefault("DEMO_OPS_EMAIL", "demo-ops@example.com"),
			Password:         envOrDefault("DEMO_OPS_PASSWORD", "correct horse battery staple 123"),
			Name:             envOrDefault("DEMO_OPS_NAME", "Demo Ops"),
			OrganizationName: envOrDefault("DEMO_OPS_ORG_NAME", "Demo Ops Org"),
			OrganizationKind: envOrDefault("DEMO_OPS_ORG_KIND", "ops"),
			OrganizationID:   strings.TrimSpace(os.Getenv("DEMO_OPS_ORG_ID")),
		},
		MinBuyerBalanceCents:      envInt64OrDefault("DEMO_MIN_BUYER_BALANCE_CENTS", 5_000),
		MinProviderLiquidityCents: envInt64OrDefault("DEMO_MIN_PROVIDER_LIQUIDITY_CENTS", 88_000),
		BuyerTopUpAmount:          envOrDefault("DEMO_BUYER_TOPUP_AMOUNT", "50.00"),
		ResourcePrefix:            envOrDefault("DEMO_RESOURCE_PREFIX", "demo-live"),
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envInt64OrDefault(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
