package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/serviceauth"
)

type Config struct {
	APIBaseURL          string
	SettlementBaseURL   string
	ExecutionBaseURL    string
	ExecutionEventToken string
	IncludeWithdrawal   bool
	IncludeCarrierProbe bool
}

type Summary struct {
	OrderID            string
	Invoice            string
	WithdrawalID       string
	FundingRecordCount int
	CodeAgentPolicy    string
}

type smokeClient struct {
	httpClient *http.Client
}

type statusError struct {
	StatusCode int
}

func (e statusError) Error() string {
	return fmt.Sprintf("unexpected status %d", e.StatusCode)
}

func RunSmoke(ctx context.Context, cfg Config) (Summary, error) {
	client := &smokeClient{httpClient: &http.Client{Timeout: 10 * time.Second}}

	if err := client.health(ctx, cfg.APIBaseURL); err != nil {
		return Summary{}, fmt.Errorf("api health: %w", err)
	}
	if err := client.health(ctx, cfg.SettlementBaseURL); err != nil {
		return Summary{}, fmt.Errorf("settlement health: %w", err)
	}
	if err := client.health(ctx, cfg.ExecutionBaseURL); err != nil {
		return Summary{}, fmt.Errorf("execution health: %w", err)
	}

	orderID, err := client.createMarketplaceOrder(ctx, cfg.APIBaseURL)
	if err != nil {
		if isStatusCode(err, http.StatusNotFound) {
			orderID, err = client.createOrder(ctx, cfg.APIBaseURL)
		}
		if err != nil {
			return Summary{}, err
		}
	}
	if err := client.settleViaExecution(ctx, cfg.ExecutionBaseURL, cfg.ExecutionEventToken, orderID); err != nil {
		return Summary{}, err
	}

	invoice, err := client.createInvoice(ctx, cfg.SettlementBaseURL, orderID)
	if err != nil {
		return Summary{}, err
	}
	if err := client.syncSettledFeed(ctx, cfg.SettlementBaseURL); err != nil {
		return Summary{}, err
	}

	fundingCount, err := client.countFundingRecords(ctx, cfg.SettlementBaseURL, map[string]string{
		"kind":    "invoice",
		"orderId": orderID,
	})
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		OrderID:            orderID,
		Invoice:            invoice,
		FundingRecordCount: fundingCount,
	}

	if cfg.IncludeWithdrawal {
		withdrawalID, err := client.requestWithdrawal(ctx, cfg.SettlementBaseURL)
		if err != nil {
			return Summary{}, err
		}
		if err := client.syncWithdrawals(ctx, cfg.SettlementBaseURL); err != nil {
			return Summary{}, err
		}
		fundingCount, err = client.countFundingRecords(ctx, cfg.SettlementBaseURL, nil)
		if err != nil {
			return Summary{}, err
		}
		summary.WithdrawalID = withdrawalID
		summary.FundingRecordCount = fundingCount
	}

	if cfg.IncludeCarrierProbe {
		policy, err := client.verifyCarrier(ctx, cfg.ExecutionBaseURL)
		if err != nil {
			return Summary{}, err
		}
		summary.CodeAgentPolicy = policy
	}

	return summary, nil
}

func ConfigFromEnv() Config {
	return Config{
		APIBaseURL:          envOrDefault("RELEASE_SMOKE_API_BASE_URL", "http://127.0.0.1:8080"),
		SettlementBaseURL:   envOrDefault("RELEASE_SMOKE_SETTLEMENT_BASE_URL", "http://127.0.0.1:8083"),
		ExecutionBaseURL:    envOrDefault("RELEASE_SMOKE_EXECUTION_BASE_URL", "http://127.0.0.1:8085"),
		ExecutionEventToken: envOrDefault("RELEASE_SMOKE_EXECUTION_EVENT_TOKEN", ""),
		IncludeWithdrawal:   envBool("RELEASE_SMOKE_INCLUDE_WITHDRAWAL"),
		IncludeCarrierProbe: envBool("RELEASE_SMOKE_INCLUDE_CARRIER_PROBE"),
	}
}

func (c *smokeClient) health(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/healthz", nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	return nil
}

func (c *smokeClient) createOrder(ctx context.Context, baseURL string) (string, error) {
	var response struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/orders", map[string]any{
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"title":         "release smoke order",
		"fundingMode":   "credit",
		"creditLineId":  "credit_1",
		"milestones": []map[string]any{
			{
				"id":             "ms_1",
				"title":          "smoke milestone",
				"basePriceCents": 1200,
				"budgetCents":    1800,
			},
		},
	}, &response)
	if err != nil {
		return "", fmt.Errorf("create order: %w", err)
	}
	if response.Order.ID == "" {
		return "", errors.New("create order: missing order id")
	}
	return response.Order.ID, nil
}

func (c *smokeClient) createMarketplaceOrder(ctx context.Context, baseURL string) (string, error) {
	rfqID, err := c.createRFQ(ctx, baseURL)
	if err != nil {
		return "", err
	}
	bidID, err := c.createBid(ctx, baseURL, rfqID)
	if err != nil {
		return "", err
	}
	return c.awardRFQ(ctx, baseURL, rfqID, bidID)
}

func (c *smokeClient) createRFQ(ctx context.Context, baseURL string) (string, error) {
	var response struct {
		RFQ struct {
			ID string `json:"id"`
		} `json:"rfq"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/rfqs", map[string]any{
		"buyerOrgId":         "buyer_1",
		"title":              "release smoke rfq",
		"category":           "agent-ops",
		"scope":              "Need a carrier-ready operator to run a one-step smoke order.",
		"budgetCents":        1800,
		"responseDeadlineAt": "2026-03-15T12:00:00Z",
	}, &response)
	if err != nil {
		return "", fmt.Errorf("create rfq: %w", err)
	}
	if response.RFQ.ID == "" {
		return "", errors.New("create rfq: missing rfq id")
	}
	return response.RFQ.ID, nil
}

func (c *smokeClient) createBid(ctx context.Context, baseURL, rfqID string) (string, error) {
	var response struct {
		Bid struct {
			ID string `json:"id"`
		} `json:"bid"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/rfqs/"+rfqID+"/bids", map[string]any{
		"providerOrgId": "provider_1",
		"message":       "release smoke provider bid",
		"quoteCents":    1200,
		"milestones": []map[string]any{
			{
				"id":             "ms_1",
				"title":          "smoke milestone",
				"basePriceCents": 1200,
				"budgetCents":    1800,
			},
		},
	}, &response)
	if err != nil {
		return "", fmt.Errorf("create bid: %w", err)
	}
	if response.Bid.ID == "" {
		return "", errors.New("create bid: missing bid id")
	}
	return response.Bid.ID, nil
}

func (c *smokeClient) awardRFQ(ctx context.Context, baseURL, rfqID, bidID string) (string, error) {
	var response struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/rfqs/"+rfqID+"/award", map[string]any{
		"bidId":        bidID,
		"fundingMode":  "credit",
		"creditLineId": "credit_1",
	}, &response)
	if err != nil {
		return "", fmt.Errorf("award rfq: %w", err)
	}
	if response.Order.ID == "" {
		return "", errors.New("award rfq: missing order id")
	}
	return response.Order.ID, nil
}

func (c *smokeClient) settleViaExecution(ctx context.Context, baseURL, token, orderID string) error {
	return c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/v1/carrier/events", map[string]string{
		serviceauth.HeaderName: strings.TrimSpace(token),
	}, map[string]any{
		"orderId":     orderID,
		"milestoneId": "ms_1",
		"eventType":   "milestone_ready",
		"summary":     "release smoke settlement",
	}, nil)
}

func (c *smokeClient) createInvoice(ctx context.Context, baseURL, orderID string) (string, error) {
	var response struct {
		Invoice string `json:"invoice"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/v1/invoices", map[string]any{
		"orderId":       orderID,
		"milestoneId":   "ms_1",
		"buyerOrgId":    "buyer_1",
		"providerOrgId": "provider_1",
		"asset":         "CKB",
		"amount":        "12.5",
	}, &response)
	if err != nil {
		return "", fmt.Errorf("create invoice: %w", err)
	}
	if response.Invoice == "" {
		return "", errors.New("create invoice: missing invoice")
	}
	return response.Invoice, nil
}

func (c *smokeClient) syncSettledFeed(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/settled-feed", nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("settled feed status %d", res.StatusCode)
	}
	return nil
}

func (c *smokeClient) requestWithdrawal(ctx context.Context, baseURL string) (string, error) {
	var response struct {
		ID string `json:"id"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/v1/withdrawals", map[string]any{
		"providerOrgId": "provider_1",
		"asset":         "USDI",
		"amount":        "10",
		"destination": map[string]any{
			"kind":           "PAYMENT_REQUEST",
			"paymentRequest": "fiber:invoice:example",
		},
	}, &response)
	if err != nil {
		return "", fmt.Errorf("request withdrawal: %w", err)
	}
	if response.ID == "" {
		return "", errors.New("request withdrawal: missing id")
	}
	return response.ID, nil
}

func (c *smokeClient) syncWithdrawals(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/withdrawals/status?providerOrgId=provider_1", nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("withdrawal status sync status %d", res.StatusCode)
	}
	return nil
}

func (c *smokeClient) verifyCarrier(ctx context.Context, baseURL string) (string, error) {
	healthReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/carrier/codeagent/health?hostId=host_1&agentId=agent_1&backend=codex&workspaceRoot=/workspace", nil)
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
		"hostId":        "host_1",
		"agentId":       "agent_1",
		"backend":       "codex",
		"workspaceRoot": "/workspace",
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

func (c *smokeClient) countFundingRecords(ctx context.Context, baseURL string, filters map[string]string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/funding-records", nil)
	if err != nil {
		return 0, err
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
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("funding records status %d", res.StatusCode)
	}

	var payload struct {
		Records []json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return len(payload.Records), nil
}

func (c *smokeClient) postJSON(ctx context.Context, url string, payload any, target any) error {
	return c.postJSONWithHeaders(ctx, url, nil, payload, target)
}

func (c *smokeClient) postJSONWithHeaders(ctx context.Context, url string, headers map[string]string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		if strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return statusError{StatusCode: res.StatusCode}
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func isStatusCode(err error, statusCode int) bool {
	var target statusError
	return errors.As(err, &target) && target.StatusCode == statusCode
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
