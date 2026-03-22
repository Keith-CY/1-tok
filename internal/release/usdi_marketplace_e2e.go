package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/carrier"
	"github.com/chenyu/1-tok/internal/core"
	fiberclient "github.com/chenyu/1-tok/internal/integrations/fiber"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/serviceauth"
	"github.com/chenyu/1-tok/internal/usageproof"
)

const marketplaceTreasuryUserID = "platform_treasury"
const usdiMarketplaceE2EHTTPTimeout = 2 * time.Minute

type USDIMarketplaceE2EConfig struct {
	APIBaseURL                          string
	IAMBaseURL                          string
	SettlementBaseURL                   string
	SettlementServiceToken              string
	ExecutionBaseURL                    string
	ExecutionEventToken                 string
	GatewayServiceToken                 string
	FiberAdapterBaseURL                 string
	FiberAdapterAppID                   string
	FiberAdapterHMACSecret              string
	CarrierBaseURL                      string
	CarrierGatewayToken                 string
	CarrierIntegrationToken             string
	CarrierHostID                       string
	CarrierAgentID                      string
	CarrierBackend                      string
	CarrierWorkspaceRoot                string
	CarrierRemoteHostName               string
	CarrierRemoteHostHost               string
	CarrierRemoteHostPort               int
	CarrierRemoteHostUser               string
	CarrierRemoteKeyPath                string
	CarrierAuthConfigured               bool
	CarrierCallbackSecret               string
	CarrierCallbackKeyID                string
	IncludeCarrierProbe                 bool
	FaucetTxHash                        string
	ExplorerProofURLs                   []string
	ProviderSettlementRPCURL            string
	ProviderSettlementP2PHost           string
	ProviderSettlementP2PPort           int
	ProviderSettlementUDTTypeScriptJSON string
}

type USDIMarketplaceE2ESummary struct {
	Asset                       string   `json:"asset"`
	FaucetTxHash                string   `json:"faucetTxHash,omitempty"`
	ExplorerProofURLs           []string `json:"explorerProofUrls,omitempty"`
	IntegrationIssues           []string `json:"integrationIssues,omitempty"`
	BuyerUserEmail              string   `json:"buyerUserEmail,omitempty"`
	ProviderUserEmail           string   `json:"providerUserEmail,omitempty"`
	BuyerOrgID                  string   `json:"buyerOrgId"`
	ProviderOrgID               string   `json:"providerOrgId"`
	BuyerTopUpInvoice           string   `json:"buyerTopUpInvoice"`
	BuyerTopUpFundingRecordID   string   `json:"buyerTopUpFundingRecordId"`
	BuyerTopUpPaymentID         string   `json:"buyerTopUpPaymentId,omitempty"`
	RFQID                       string   `json:"rfqId"`
	BidID                       string   `json:"bidId"`
	OrderID                     string   `json:"orderId"`
	CarrierProviderBindingID    string   `json:"carrierProviderBindingId"`
	ProviderSettlementBindingID string   `json:"providerSettlementBindingId"`
	CarrierBindingID            string   `json:"carrierBindingId"`
	CarrierExecutionID          string   `json:"carrierExecutionId"`
	UsageChargeCount            int      `json:"usageChargeCount"`
	ProviderPayoutRecordIDs     []string `json:"providerPayoutRecordIds"`
	FinalOrderStatus            string   `json:"finalOrderStatus"`
	CodeAgentPolicy             string   `json:"codeAgentPolicy,omitempty"`
}

type usageReportedStep struct {
	EventID     string
	Sequence    int64
	Kind        core.UsageChargeKind
	AmountCents int64
	ProofRef    string
}

func USDIMarketplaceE2EConfigFromEnv() USDIMarketplaceE2EConfig {
	return USDIMarketplaceE2EConfig{
		APIBaseURL:                          envOrDefault("RELEASE_USDI_E2E_API_BASE_URL", envOrDefault("RELEASE_SMOKE_API_BASE_URL", "http://127.0.0.1:8080")),
		IAMBaseURL:                          envOrDefault("RELEASE_USDI_E2E_IAM_BASE_URL", envOrDefault("RELEASE_SMOKE_IAM_BASE_URL", "")),
		SettlementBaseURL:                   envOrDefault("RELEASE_USDI_E2E_SETTLEMENT_BASE_URL", envOrDefault("RELEASE_SMOKE_SETTLEMENT_BASE_URL", "http://127.0.0.1:8083")),
		SettlementServiceToken:              envOrDefault("RELEASE_USDI_E2E_SETTLEMENT_SERVICE_TOKEN", envOrDefault("RELEASE_SMOKE_SETTLEMENT_SERVICE_TOKEN", "")),
		ExecutionBaseURL:                    envOrDefault("RELEASE_USDI_E2E_EXECUTION_BASE_URL", envOrDefault("RELEASE_SMOKE_EXECUTION_BASE_URL", "http://127.0.0.1:8085")),
		ExecutionEventToken:                 envOrDefault("RELEASE_USDI_E2E_EXECUTION_EVENT_TOKEN", envOrDefault("RELEASE_SMOKE_EXECUTION_EVENT_TOKEN", "")),
		GatewayServiceToken:                 envOrDefault("RELEASE_USDI_E2E_GATEWAY_SERVICE_TOKEN", envOrDefault("EXECUTION_GATEWAY_TOKEN", "")),
		FiberAdapterBaseURL:                 envOrDefault("RELEASE_USDI_E2E_FIBER_ADAPTER_BASE_URL", envOrDefault("FIBER_RPC_URL", "")),
		FiberAdapterAppID:                   envOrDefault("RELEASE_USDI_E2E_FIBER_ADAPTER_APP_ID", envOrDefault("FIBER_APP_ID", "")),
		FiberAdapterHMACSecret:              envOrDefault("RELEASE_USDI_E2E_FIBER_ADAPTER_HMAC_SECRET", envOrDefault("FIBER_HMAC_SECRET", "")),
		CarrierBaseURL:                      envOrDefault("RELEASE_USDI_E2E_CARRIER_BASE_URL", envOrDefault("CARRIER_GATEWAY_URL", "http://127.0.0.1:8787")),
		CarrierGatewayToken:                 envOrDefault("RELEASE_USDI_E2E_CARRIER_GATEWAY_TOKEN", envOrDefault("CARRIER_GATEWAY_API_TOKEN", "")),
		CarrierIntegrationToken:             envOrDefault("RELEASE_USDI_E2E_CARRIER_INTEGRATION_TOKEN", envOrDefault("CARRIER_GATEWAY_API_TOKEN", "")),
		CarrierHostID:                       envOrDefault("RELEASE_USDI_E2E_CARRIER_HOST_ID", "host_1"),
		CarrierAgentID:                      envOrDefault("RELEASE_USDI_E2E_CARRIER_AGENT_ID", "main"),
		CarrierBackend:                      envOrDefault("RELEASE_USDI_E2E_CARRIER_BACKEND", "codex"),
		CarrierWorkspaceRoot:                envOrDefault("RELEASE_USDI_E2E_CARRIER_WORKSPACE_ROOT", "/workspace"),
		CarrierRemoteHostName:               envOrDefault("RELEASE_USDI_E2E_CARRIER_REMOTE_HOST_NAME", ""),
		CarrierRemoteHostHost:               envOrDefault("RELEASE_USDI_E2E_CARRIER_REMOTE_HOST_HOST", ""),
		CarrierRemoteHostPort:               envIntOrDefault("RELEASE_USDI_E2E_CARRIER_REMOTE_HOST_PORT", 22),
		CarrierRemoteHostUser:               envOrDefault("RELEASE_USDI_E2E_CARRIER_REMOTE_HOST_USER", "carrier"),
		CarrierRemoteKeyPath:                envOrDefault("RELEASE_USDI_E2E_CARRIER_REMOTE_KEY_PATH", "/keys/id_ed25519"),
		CarrierAuthConfigured:               strings.TrimSpace(envOrDefault("OPENAI_API_KEY", "")) != "" || strings.TrimSpace(envOrDefault("OPENAI_CODEX_TOKEN", "")) != "",
		CarrierCallbackSecret:               envOrDefault("RELEASE_USDI_E2E_CARRIER_CALLBACK_SECRET", "usdi-e2e-callback-secret"),
		CarrierCallbackKeyID:                envOrDefault("RELEASE_USDI_E2E_CARRIER_CALLBACK_KEY_ID", "usdi-e2e-key"),
		IncludeCarrierProbe:                 envBool("RELEASE_USDI_E2E_INCLUDE_CARRIER_PROBE"),
		FaucetTxHash:                        strings.TrimSpace(envOrDefault("RELEASE_USDI_E2E_FAUCET_TX_HASH", "")),
		ExplorerProofURLs:                   splitCSV(envOrDefault("RELEASE_USDI_E2E_EXPLORER_PROOF_URLS", "")),
		ProviderSettlementRPCURL:            envOrDefault("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_RPC_URL", "http://provider-fnn:8227"),
		ProviderSettlementP2PHost:           envOrDefault("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_P2P_HOST", "provider-fnn"),
		ProviderSettlementP2PPort:           envIntOrDefault("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_P2P_PORT", 8228),
		ProviderSettlementUDTTypeScriptJSON: envOrDefault("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_UDT_TYPE_SCRIPT_JSON", envOrDefault("FIBER_USDI_UDT_TYPE_SCRIPT_JSON", "")),
	}
}

func RunUSDIMarketplaceE2E(ctx context.Context, cfg USDIMarketplaceE2EConfig) (USDIMarketplaceE2ESummary, error) {
	client := &smokeClient{httpClient: &http.Client{Timeout: usdiMarketplaceE2EHTTPTimeout}}
	integrationIssues := []string{"https://github.com/Keith-CY/carrier/issues/1595"}
	if err := client.health(ctx, cfg.APIBaseURL); err != nil {
		return USDIMarketplaceE2ESummary{}, fmt.Errorf("api health: %w", err)
	}
	if err := client.health(ctx, cfg.SettlementBaseURL); err != nil {
		return USDIMarketplaceE2ESummary{}, fmt.Errorf("settlement health: %w", err)
	}
	if err := client.health(ctx, cfg.ExecutionBaseURL); err != nil {
		return USDIMarketplaceE2ESummary{}, fmt.Errorf("execution health: %w", err)
	}
	if err := client.bootstrapCarrierTarget(ctx, &cfg); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}

	buyer := actorIdentity{OrgID: "buyer_1"}
	provider := actorIdentity{OrgID: "provider_1"}
	if strings.TrimSpace(cfg.IAMBaseURL) != "" {
		suffix := nanoSuffix()
		var err error
		buyer, err = client.createIAMUser(ctx, cfg.IAMBaseURL, "buyer", suffix)
		if err != nil {
			return USDIMarketplaceE2ESummary{}, fmt.Errorf("iam buyer signup: %w", err)
		}
		provider, err = client.createIAMUser(ctx, cfg.IAMBaseURL, "provider", suffix)
		if err != nil {
			return USDIMarketplaceE2ESummary{}, fmt.Errorf("iam provider signup: %w", err)
		}
	}

	topUpInvoice, topUpRecordID, err := client.createBuyerTopUp(ctx, cfg.SettlementBaseURL, buyer, cfg.SettlementServiceToken, "25.00")
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	topUpPaymentID, err := client.payInvoiceViaFiber(ctx, cfg, buyer.OrgID, topUpInvoice, "25.00", "USDI")
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	if err := client.syncSettledFeed(ctx, cfg.SettlementBaseURL, cfg.SettlementServiceToken); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	if _, err := client.waitFundingRecordState(ctx, cfg.SettlementBaseURL, buyer.Token, map[string]string{
		"kind":       "buyer_topup",
		"buyerOrgId": buyer.OrgID,
	}, 30*time.Second, "SETTLED"); err != nil {
		return USDIMarketplaceE2ESummary{}, fmt.Errorf("buyer topup not settled: %w", err)
	}

	providerCarrierBindingID, err := client.registerProviderCarrierBinding(ctx, cfg.APIBaseURL, provider.OrgID, cfg)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	if err := client.verifyProviderCarrierBinding(ctx, cfg.APIBaseURL, providerCarrierBindingID); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	providerSettlementBindingID, err := client.registerProviderSettlementBinding(ctx, cfg.APIBaseURL, provider.OrgID, cfg)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}

	rfqID, err := client.createRFQ(ctx, cfg.APIBaseURL, buyer)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	bidID, err := client.createBid(ctx, cfg.APIBaseURL, provider, rfqID)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	orderID, err := client.awardRFQPrepaid(ctx, cfg.APIBaseURL, buyer.Token, rfqID, bidID)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	carrierBindingID, err := client.bindOrderCarrier(ctx, cfg.APIBaseURL, cfg.GatewayServiceToken, orderID, "ms_1", providerCarrierBindingID)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	carrierExecutionID, err := client.createCarrierJob(ctx, cfg.APIBaseURL, cfg.GatewayServiceToken, orderID, "ms_1", carrierBindingID, "run usdi marketplace e2e")
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}

	policy := ""
	if cfg.IncludeCarrierProbe {
		if cfg.CarrierAuthConfigured {
			policy, err = client.verifyCarrierWithConfig(ctx, cfg.ExecutionBaseURL, cfg)
			if err != nil {
				return USDIMarketplaceE2ESummary{}, err
			}
		} else {
			integrationIssues = append(integrationIssues, "carrier codeagent run probe skipped: missing OPENAI_API_KEY or OPENAI_CODEX_TOKEN")
		}
	}

	if err := client.sendCarrierIntegrationCallback(ctx, cfg.APIBaseURL, cfg, carrier.IntegrationCallbackEnvelope{
		EventID:            "evt-start",
		Sequence:           1,
		EventType:          "execution.started",
		BindingID:          carrierBindingID,
		CarrierExecutionID: carrierExecutionID,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]any{
			"jobId": carrierExecutionID,
		},
	}); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}

	usageSteps := []usageReportedStep{
		{EventID: "evt-usage-1", Sequence: 2, Kind: core.UsageChargeKindStep, AmountCents: 100, ProofRef: "fiber:proof:usage-1"},
		{EventID: "evt-usage-2", Sequence: 3, Kind: core.UsageChargeKindStep, AmountCents: 200, ProofRef: "fiber:proof:usage-2"},
	}
	providerPayoutRecordIDs := make([]string, 0, len(usageSteps)+1)
	for _, step := range usageSteps {
		if err := client.sendCarrierIntegrationCallback(ctx, cfg.APIBaseURL, cfg, buildUsageReportedEnvelope(
			carrierBindingID,
			carrierExecutionID,
			"ms_1",
			cfg.CarrierCallbackSecret,
			step,
			time.Now().UTC(),
		)); err != nil {
			return USDIMarketplaceE2ESummary{}, err
		}
		paymentRequest, err := client.createProviderInvoiceViaProviderSettlementNode(ctx, cfg, amountFromCents(step.AmountCents))
		if err != nil {
			return USDIMarketplaceE2ESummary{}, err
		}
		recordID, err := client.requestProviderPayout(ctx, cfg.SettlementBaseURL, cfg.SettlementServiceToken, orderID, buyer.OrgID, provider.OrgID, amountFromCents(step.AmountCents), paymentRequest)
		if err != nil {
			return USDIMarketplaceE2ESummary{}, err
		}
		providerPayoutRecordIDs = append(providerPayoutRecordIDs, recordID)
	}

	if err := client.sendCarrierIntegrationCallback(ctx, cfg.APIBaseURL, cfg, carrier.IntegrationCallbackEnvelope{
		EventID:            "evt-artifact",
		Sequence:           4,
		EventType:          "artifact.ready",
		BindingID:          carrierBindingID,
		CarrierExecutionID: carrierExecutionID,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]any{
			"jobId":   carrierExecutionID,
			"summary": "delivery evidence bundle ready",
			"artifacts": []map[string]any{
				{
					"name":      "delivery-summary",
					"type":      "report",
					"url":       "https://example.test/artifacts/delivery-summary.json",
					"sizeBytes": 128,
				},
			},
			"usageReport": map[string]any{
				"tokenCount":     120,
				"stepCount":      len(usageSteps),
				"apiCallCount":   2,
				"totalCostCents": 300,
			},
		},
	}); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	if err := client.sendCarrierIntegrationCallback(ctx, cfg.APIBaseURL, cfg, carrier.IntegrationCallbackEnvelope{
		EventID:            "evt-ready",
		Sequence:           5,
		EventType:          "milestone.ready",
		BindingID:          carrierBindingID,
		CarrierExecutionID: carrierExecutionID,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]any{
			"jobId":   carrierExecutionID,
			"output":  "milestone completed successfully",
			"summary": "milestone completed successfully",
			"artifacts": []map[string]any{
				{
					"name":      "delivery-archive",
					"type":      "archive",
					"url":       "https://example.test/artifacts/delivery-archive.zip",
					"sizeBytes": 256,
				},
			},
		},
	}); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}

	finalPaymentRequest, err := client.createProviderInvoiceViaProviderSettlementNode(ctx, cfg, "9.00")
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	finalPayoutRecordID, err := client.requestProviderPayout(ctx, cfg.SettlementBaseURL, cfg.SettlementServiceToken, orderID, buyer.OrgID, provider.OrgID, "9.00", finalPaymentRequest)
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	providerPayoutRecordIDs = append(providerPayoutRecordIDs, finalPayoutRecordID)

	if _, err := client.waitFundingRecordState(ctx, cfg.SettlementBaseURL, provider.Token, map[string]string{
		"kind":    "provider_payout",
		"orderId": orderID,
	}, 30*time.Second, "COMPLETED", "SETTLED", "PROCESSING", "PENDING"); err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}
	finalOrderStatus, err := client.waitOrderStatus(ctx, cfg.APIBaseURL, buyer.Token, orderID, 30*time.Second, "completed")
	if err != nil {
		return USDIMarketplaceE2ESummary{}, err
	}

	return USDIMarketplaceE2ESummary{
		Asset:                       "USDI",
		FaucetTxHash:                cfg.FaucetTxHash,
		ExplorerProofURLs:           cfg.ExplorerProofURLs,
		IntegrationIssues:           integrationIssues,
		BuyerUserEmail:              buyer.Email,
		ProviderUserEmail:           provider.Email,
		BuyerOrgID:                  buyer.OrgID,
		ProviderOrgID:               provider.OrgID,
		BuyerTopUpInvoice:           topUpInvoice,
		BuyerTopUpFundingRecordID:   topUpRecordID,
		BuyerTopUpPaymentID:         topUpPaymentID,
		RFQID:                       rfqID,
		BidID:                       bidID,
		OrderID:                     orderID,
		CarrierProviderBindingID:    providerCarrierBindingID,
		ProviderSettlementBindingID: providerSettlementBindingID,
		CarrierBindingID:            carrierBindingID,
		CarrierExecutionID:          carrierExecutionID,
		UsageChargeCount:            len(usageSteps),
		ProviderPayoutRecordIDs:     providerPayoutRecordIDs,
		FinalOrderStatus:            finalOrderStatus,
		CodeAgentPolicy:             policy,
	}, nil
}

func (c *smokeClient) awardRFQPrepaid(ctx context.Context, baseURL, token, rfqID, bidID string) (string, error) {
	var response struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	err := c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/rfqs/"+rfqID+"/award", authHeaders(token), map[string]any{
		"bidId":       bidID,
		"fundingMode": "prepaid",
	}, &response)
	if err != nil {
		return "", fmt.Errorf("award rfq: %w", err)
	}
	if response.Order.ID == "" {
		return "", errors.New("award rfq: missing order id")
	}
	return response.Order.ID, nil
}

func (c *smokeClient) createBuyerTopUp(ctx context.Context, baseURL string, buyer actorIdentity, serviceToken string, amount string) (string, string, error) {
	var response struct {
		Invoice  string `json:"invoice"`
		RecordID string `json:"recordId"`
	}
	headers := authHeaders(buyer.Token)
	if strings.TrimSpace(buyer.Token) == "" && strings.TrimSpace(serviceToken) != "" {
		headers = map[string]string{serviceauth.HeaderName: strings.TrimSpace(serviceToken)}
	}
	payload := map[string]any{
		"asset":  "USDI",
		"amount": amount,
	}
	if strings.TrimSpace(buyer.Token) == "" {
		payload["buyerOrgId"] = buyer.OrgID
	}
	err := c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/v1/topups", headers, payload, &response)
	if err != nil {
		return "", "", fmt.Errorf("create buyer topup: %w", err)
	}
	if response.Invoice == "" || response.RecordID == "" {
		return "", "", errors.New("create buyer topup: missing invoice or record id")
	}
	return response.Invoice, response.RecordID, nil
}

func (c *smokeClient) payInvoiceViaFiber(ctx context.Context, cfg USDIMarketplaceE2EConfig, userID, invoice, amount, asset string) (string, error) {
	if strings.TrimSpace(cfg.FiberAdapterBaseURL) == "" || strings.TrimSpace(cfg.FiberAdapterAppID) == "" || strings.TrimSpace(cfg.FiberAdapterHMACSecret) == "" {
		return "", errors.New("fiber adapter config is required for usdi marketplace e2e")
	}
	client := fiberclient.NewClient(cfg.FiberAdapterBaseURL, cfg.FiberAdapterAppID, cfg.FiberAdapterHMACSecret)
	result, err := client.RequestPayout(ctx, fiberclient.RequestPayoutInput{
		UserID: userID,
		Asset:  asset,
		Amount: amount,
		Destination: fiberclient.WithdrawalDestination{
			Kind:           "PAYMENT_REQUEST",
			PaymentRequest: invoice,
		},
	})
	if err != nil {
		return "", fmt.Errorf("pay invoice via fiber adapter: %w", err)
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", errors.New("pay invoice via fiber adapter: missing payout id")
	}
	return result.ID, nil
}

func (c *smokeClient) createProviderInvoiceViaProviderSettlementNode(ctx context.Context, cfg USDIMarketplaceE2EConfig, amount string) (string, error) {
	if strings.TrimSpace(cfg.ProviderSettlementRPCURL) == "" {
		return "", errors.New("provider settlement rpc url is required for provider invoice creation")
	}
	if strings.TrimSpace(cfg.ProviderSettlementUDTTypeScriptJSON) == "" {
		return "", errors.New("provider settlement udt type script json is required for provider invoice creation")
	}

	var udtTypeScript platform.UDTTypeScript
	if err := json.Unmarshal([]byte(cfg.ProviderSettlementUDTTypeScriptJSON), &udtTypeScript); err != nil {
		return "", fmt.Errorf("provider settlement udt type script: %w", err)
	}

	node := newReleaseRawFNNClient(cfg.ProviderSettlementRPCURL)
	invoice, err := node.CreateInvoice(ctx, "USDI", amount, map[string]string{
		"code_hash": udtTypeScript.CodeHash,
		"hash_type": udtTypeScript.HashType,
		"args":      udtTypeScript.Args,
	})
	if err != nil {
		return "", fmt.Errorf("create provider invoice via provider settlement node: %w", err)
	}
	if strings.TrimSpace(invoice) == "" {
		return "", errors.New("create provider invoice via provider settlement node: missing invoice")
	}
	return invoice, nil
}

func (c *smokeClient) requestProviderPayout(ctx context.Context, baseURL, serviceToken, orderID, buyerOrgID, providerOrgID, amount, paymentRequest string) (string, error) {
	var response struct {
		RecordID string `json:"recordId"`
	}
	err := c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/v1/provider-payouts", map[string]string{
		serviceauth.HeaderName: strings.TrimSpace(serviceToken),
	}, map[string]any{
		"orderId":        orderID,
		"milestoneId":    "ms_1",
		"buyerOrgId":     buyerOrgID,
		"providerOrgId":  providerOrgID,
		"asset":          "USDI",
		"amount":         amount,
		"paymentRequest": paymentRequest,
	}, &response)
	if err != nil {
		return "", fmt.Errorf("request provider payout: %w", err)
	}
	if response.RecordID == "" {
		return "", errors.New("request provider payout: missing record id")
	}
	return response.RecordID, nil
}

func (c *smokeClient) registerProviderCarrierBinding(ctx context.Context, baseURL, providerOrgID string, cfg USDIMarketplaceE2EConfig) (string, error) {
	var response struct {
		Binding struct {
			ID string `json:"id"`
		} `json:"binding"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/carrier-bindings", map[string]any{
		"providerOrgId":    providerOrgID,
		"carrierBaseUrl":   cfg.CarrierBaseURL,
		"integrationToken": cfg.CarrierIntegrationToken,
		"hostId":           cfg.CarrierHostID,
		"agentId":          cfg.CarrierAgentID,
		"backend":          cfg.CarrierBackend,
		"workspaceRoot":    cfg.CarrierWorkspaceRoot,
		"callbackSecret":   cfg.CarrierCallbackSecret,
		"callbackKeyId":    cfg.CarrierCallbackKeyID,
	}, &response)
	if err != nil {
		return "", fmt.Errorf("register provider carrier binding: %w", err)
	}
	if response.Binding.ID == "" {
		return "", errors.New("register provider carrier binding: missing binding id")
	}
	return response.Binding.ID, nil
}

func (c *smokeClient) bootstrapCarrierTarget(ctx context.Context, cfg *USDIMarketplaceE2EConfig) error {
	if cfg == nil {
		return errors.New("carrier bootstrap config is required")
	}
	if strings.TrimSpace(cfg.CarrierBaseURL) == "" || strings.TrimSpace(cfg.CarrierGatewayToken) == "" || strings.TrimSpace(cfg.CarrierRemoteHostHost) == "" {
		return nil
	}
	hostID, err := c.createCarrierRemoteHost(ctx, cfg.CarrierBaseURL, cfg.CarrierGatewayToken, carrierRemoteHostRequest{
		Name:        firstNonEmptyString(cfg.CarrierRemoteHostName, "usdi-remote"),
		Host:        strings.TrimSpace(cfg.CarrierRemoteHostHost),
		Port:        cfg.CarrierRemoteHostPort,
		User:        firstNonEmptyString(cfg.CarrierRemoteHostUser, "carrier"),
		AuthMode:    "private_key",
		KeyPath:     firstNonEmptyString(cfg.CarrierRemoteKeyPath, "/keys/id_ed25519"),
		RuntimeMode: "on_demand",
	})
	if err != nil {
		return fmt.Errorf("bootstrap carrier remote host: %w", err)
	}
	cfg.CarrierHostID = hostID
	if strings.TrimSpace(cfg.CarrierAgentID) == "" {
		cfg.CarrierAgentID = "main"
	}
	if err := c.installCarrierCodeAgent(ctx, cfg.CarrierBaseURL, cfg.CarrierGatewayToken, cfg.CarrierHostID, cfg.CarrierAgentID, cfg.CarrierBackend, cfg.CarrierWorkspaceRoot); err != nil {
		return fmt.Errorf("bootstrap carrier codeagent: %w", err)
	}
	return nil
}

type carrierRemoteHostRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	AuthMode    string `json:"authMode"`
	KeyPath     string `json:"keyPath"`
	RuntimeMode string `json:"runtimeMode"`
}

func (c *smokeClient) createCarrierRemoteHost(ctx context.Context, baseURL, token string, payload carrierRemoteHostRequest) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/remote/hosts", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		responseBody, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("carrier remote host create status %d: %s", res.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var response struct {
		Host struct {
			ID string `json:"id"`
		} `json:"host"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Host.ID) == "" {
		return "", errors.New("carrier remote host create: missing host id")
	}
	return response.Host.ID, nil
}

func (c *smokeClient) installCarrierCodeAgent(ctx context.Context, baseURL, token, hostID, agentID, backend, workspaceRoot string) error {
	body, err := json.Marshal(map[string]any{
		"backend":       firstNonEmptyString(backend, "codex"),
		"workspaceRoot": firstNonEmptyString(workspaceRoot, "/workspace"),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/remote/hosts/"+hostID+"/instances/"+agentID+"/codeagent/install", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		responseBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("carrier codeagent install status %d: %s", res.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func (c *smokeClient) verifyProviderCarrierBinding(ctx context.Context, baseURL, bindingID string) error {
	return c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/carrier-bindings/"+bindingID+"/verify", map[string]any{}, nil)
}

func (c *smokeClient) registerProviderSettlementBinding(ctx context.Context, baseURL, providerOrgID string, cfg USDIMarketplaceE2EConfig) (string, error) {
	if strings.TrimSpace(cfg.ProviderSettlementRPCURL) == "" {
		return "", errors.New("provider settlement rpc url is required")
	}
	if strings.TrimSpace(cfg.ProviderSettlementP2PHost) == "" {
		return "", errors.New("provider settlement p2p host is required")
	}
	if strings.TrimSpace(cfg.ProviderSettlementUDTTypeScriptJSON) == "" {
		return "", errors.New("provider settlement udt type script json is required")
	}

	node := newReleaseRawFNNClient(cfg.ProviderSettlementRPCURL)
	nodeInfo, err := node.NodeInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("provider settlement node info: %w", err)
	}
	peerID, err := derivePeerIDFromNodeID(nodeInfo.NodeID)
	if err != nil {
		return "", fmt.Errorf("provider settlement peer id: %w", err)
	}

	var udtTypeScript platform.UDTTypeScript
	if err := json.Unmarshal([]byte(cfg.ProviderSettlementUDTTypeScriptJSON), &udtTypeScript); err != nil {
		return "", fmt.Errorf("provider settlement udt type script: %w", err)
	}

	var response struct {
		Binding struct {
			ID string `json:"id"`
		} `json:"binding"`
	}
	err = c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/provider-settlement-bindings", map[string]any{
		"providerOrgId": providerOrgID,
		"asset":         "USDI",
		"peerId":        peerID,
		"p2pAddress":    multiaddrForP2P(cfg.ProviderSettlementP2PHost, cfg.ProviderSettlementP2PPort, peerID),
		"nodeRpcUrl":    cfg.ProviderSettlementRPCURL,
		"udtTypeScript": map[string]any{
			"codeHash": udtTypeScript.CodeHash,
			"hashType": udtTypeScript.HashType,
			"args":     udtTypeScript.Args,
		},
		"ownershipProof": "release-usdi-e2e",
	}, &response)
	if err != nil {
		return "", fmt.Errorf("register provider settlement binding: %w", err)
	}
	if response.Binding.ID == "" {
		return "", errors.New("register provider settlement binding: missing binding id")
	}
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/provider-settlement-bindings/"+response.Binding.ID+"/verify", map[string]any{}, nil); err != nil {
		return "", fmt.Errorf("verify provider settlement binding: %w", err)
	}
	return response.Binding.ID, nil
}

func (c *smokeClient) bindOrderCarrier(ctx context.Context, baseURL, gatewayToken, orderID, milestoneID, carrierID string) (string, error) {
	var response struct {
		Binding struct {
			ID string `json:"id"`
		} `json:"binding"`
	}
	err := c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/orders/"+orderID+"/milestones/"+milestoneID+"/bind-carrier", map[string]string{
		serviceauth.HeaderName: strings.TrimSpace(gatewayToken),
	}, map[string]any{
		"carrierId":    carrierID,
		"capabilities": []string{"carrier", "token_metering", "artifact_delivery"},
	}, &response)
	if err != nil {
		return "", fmt.Errorf("bind order carrier: %w", err)
	}
	if response.Binding.ID == "" {
		return "", errors.New("bind order carrier: missing binding id")
	}
	return response.Binding.ID, nil
}

func (c *smokeClient) createCarrierJob(ctx context.Context, baseURL, gatewayToken, orderID, milestoneID, bindingID, input string) (string, error) {
	var response struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	err := c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/orders/"+orderID+"/milestones/"+milestoneID+"/jobs", map[string]string{
		serviceauth.HeaderName: strings.TrimSpace(gatewayToken),
	}, map[string]any{
		"bindingId": bindingID,
		"input":     input,
	}, &response)
	if err != nil {
		return "", fmt.Errorf("create carrier job: %w", err)
	}
	if response.Job.ID == "" {
		return "", errors.New("create carrier job: missing job id")
	}
	return response.Job.ID, nil
}

func buildUsageReportedEnvelope(bindingID, executionID, milestoneID, callbackSecret string, step usageReportedStep, eventAt time.Time) carrier.IntegrationCallbackEnvelope {
	kind := step.Kind
	if strings.TrimSpace(string(kind)) == "" {
		kind = core.UsageChargeKindStep
	}
	proofRef := firstNonEmptyString(step.ProofRef, "fiber:proof:"+firstNonEmptyString(step.EventID, executionID))
	proofTS := eventAt.UTC().Format(time.RFC3339)
	signature := usageproof.Sign(strings.TrimSpace(callbackSecret), usageproof.Proof{
		ExecutionID: executionID,
		MilestoneID: milestoneID,
		Kind:        string(kind),
		AmountCents: step.AmountCents,
		Timestamp:   proofTS,
	})
	return carrier.IntegrationCallbackEnvelope{
		EventID:            step.EventID,
		Sequence:           step.Sequence,
		EventType:          "usage.reported",
		BindingID:          bindingID,
		CarrierExecutionID: executionID,
		CreatedAt:          proofTS,
		Payload: map[string]any{
			"jobId":          executionID,
			"kind":           kind,
			"amountCents":    step.AmountCents,
			"proofRef":       proofRef,
			"proofSignature": signature,
			"proofTimestamp": proofTS,
		},
	}
}

func (c *smokeClient) sendCarrierIntegrationCallback(ctx context.Context, baseURL string, cfg USDIMarketplaceE2EConfig, envelope carrier.IntegrationCallbackEnvelope) error {
	if strings.TrimSpace(envelope.CreatedAt) == "" {
		envelope.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/carrier/callbacks/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Carrier-Key-Id", cfg.CarrierCallbackKeyID)
	req.Header.Set("X-Carrier-Event-Id", envelope.EventID)
	req.Header.Set("X-Carrier-Event-Sequence", strconv.FormatInt(envelope.Sequence, 10))
	req.Header.Set("X-Carrier-Signature", carrier.SignIntegrationCallbackBody(cfg.CarrierCallbackSecret, body))
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("carrier integration callback status %d", res.StatusCode)
	}
	return nil
}

func (c *smokeClient) waitFundingRecordState(ctx context.Context, baseURL, token string, filters map[string]string, timeout time.Duration, acceptedStates ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	accepted := make(map[string]struct{}, len(acceptedStates))
	for _, state := range acceptedStates {
		accepted[strings.ToUpper(strings.TrimSpace(state))] = struct{}{}
	}
	for {
		records, err := c.listFundingRecords(ctx, baseURL, token, filters)
		if err != nil {
			return "", err
		}
		for _, record := range records {
			state := strings.ToUpper(strings.TrimSpace(record.State))
			if _, ok := accepted[state]; ok {
				return state, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting funding record state %v", acceptedStates)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *smokeClient) listFundingRecords(ctx context.Context, baseURL, token string, filters map[string]string) ([]struct {
	ID    string `json:"id"`
	State string `json:"state"`
}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/funding-records", nil)
	if err != nil {
		return nil, err
	}
	for key, value := range authHeaders(token) {
		req.Header.Set(key, value)
	}
	query := req.URL.Query()
	for key, value := range filters {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	req.URL.RawQuery = query.Encode()
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("funding records status %d", res.StatusCode)
	}
	var payload struct {
		Records []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Records, nil
}

func (c *smokeClient) waitOrderStatus(ctx context.Context, baseURL, token, orderID string, timeout time.Duration, acceptedStatuses ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	accepted := make(map[string]struct{}, len(acceptedStatuses))
	for _, status := range acceptedStatuses {
		accepted[strings.ToLower(strings.TrimSpace(status))] = struct{}{}
	}
	for {
		status, err := c.getOrderStatus(ctx, baseURL, token, orderID)
		if err == nil {
			if _, ok := accepted[strings.ToLower(strings.TrimSpace(status))]; ok {
				return status, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting order status %v", acceptedStatuses)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *smokeClient) getOrderStatus(ctx context.Context, baseURL, token, orderID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/orders/"+orderID, nil)
	if err != nil {
		return "", err
	}
	for key, value := range authHeaders(token) {
		req.Header.Set(key, value)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("order detail status %d", res.StatusCode)
	}
	var payload struct {
		Order struct {
			Status string `json:"status"`
		} `json:"order"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Order.Status == "" {
		return "", errors.New("order detail missing status")
	}
	return payload.Order.Status, nil
}

func (c *smokeClient) verifyCarrierWithConfig(ctx context.Context, baseURL string, cfg USDIMarketplaceE2EConfig) (string, error) {
	healthReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/carrier/codeagent/health?hostId="+cfg.CarrierHostID+"&agentId="+cfg.CarrierAgentID+"&backend="+cfg.CarrierBackend+"&workspaceRoot="+cfg.CarrierWorkspaceRoot, nil)
	if err != nil {
		return "", err
	}
	healthRes, err := c.httpClient.Do(healthReq)
	if err != nil {
		return "", err
	}
	defer healthRes.Body.Close()
	if healthRes.StatusCode != http.StatusOK {
		return "", fmt.Errorf("codeagent health status %d", healthRes.StatusCode)
	}

	var runResponse struct {
		Run struct {
			Result struct {
				PolicyDecision string `json:"policy_decision"`
			} `json:"result"`
		} `json:"run"`
	}
	err = c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/v1/carrier/codeagent/run", map[string]any{
		"hostId":        cfg.CarrierHostID,
		"agentId":       cfg.CarrierAgentID,
		"backend":       cfg.CarrierBackend,
		"workspaceRoot": cfg.CarrierWorkspaceRoot,
		"capability":    "run_shell",
		"command":       "pwd",
	}, &runResponse)
	if err != nil {
		return "", fmt.Errorf("carrier run: %w", err)
	}
	if runResponse.Run.Result.PolicyDecision == "" {
		return "", errors.New("carrier run: missing policy decision")
	}
	return runResponse.Run.Result.PolicyDecision, nil
}

func amountFromCents(cents int64) string {
	value := float64(cents) / 100
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func splitCSV(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
