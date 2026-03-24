package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/serviceauth"
)

const portalSessionCookieName = "one_tok_session"
const buyerPortalMarker = "Request board"
const providerPortalMarker = "Price the open board"
const opsPortalMarker = "Put human review at the top. Push everything else down a layer."

type PortalConfig struct {
	WebBaseURL          string
	APIBaseURL          string
	IAMBaseURL          string
	ExecutionBaseURL    string
	ExecutionEventToken string
}

type PortalSummary struct {
	RFQID             string `json:"rfqId"`
	BidID             string `json:"bidId"`
	OrderID           string `json:"orderId"`
	DisputeID         string `json:"disputeId"`
	ResolvedDisputeID string `json:"resolvedDisputeId"`
	CreditApproved    bool   `json:"creditApproved"`
}

type portalAccount struct {
	email    string
	password string
}

func PortalConfigFromEnv() PortalConfig {
	return PortalConfig{
		WebBaseURL:          envOrDefault("RELEASE_PORTAL_SMOKE_WEB_BASE_URL", "http://127.0.0.1:3000"),
		APIBaseURL:          envOrDefault("RELEASE_PORTAL_SMOKE_API_BASE_URL", "http://127.0.0.1:8080"),
		IAMBaseURL:          envOrDefault("RELEASE_PORTAL_SMOKE_IAM_BASE_URL", "http://127.0.0.1:8081"),
		ExecutionBaseURL:    envOrDefault("RELEASE_PORTAL_SMOKE_EXECUTION_BASE_URL", "http://127.0.0.1:8085"),
		ExecutionEventToken: envOrDefault("RELEASE_PORTAL_SMOKE_EXECUTION_EVENT_TOKEN", ""),
	}
}

func RunPortalSmoke(ctx context.Context, cfg PortalConfig) (PortalSummary, error) {
	webBaseURL := strings.TrimRight(cfg.WebBaseURL, "/")
	apiBaseURL := strings.TrimRight(cfg.APIBaseURL, "/")
	iamBaseURL := strings.TrimRight(cfg.IAMBaseURL, "/")
	executionBaseURL := strings.TrimRight(cfg.ExecutionBaseURL, "/")
	if webBaseURL == "" || apiBaseURL == "" || iamBaseURL == "" || executionBaseURL == "" {
		return PortalSummary{}, errors.New("web, api, iam, and execution base urls are required")
	}

	client := &smokeClient{httpClient: &http.Client{Timeout: 10 * time.Second}}
	if err := client.expectStatus(ctx, webBaseURL+"/login", http.StatusOK); err != nil {
		return PortalSummary{}, fmt.Errorf("web login health: %w", err)
	}

	suffix := nanoSuffix()
	buyer, err := client.createPortalUser(ctx, iamBaseURL, "buyer", suffix)
	if err != nil {
		return PortalSummary{}, err
	}
	provider, err := client.createPortalUser(ctx, iamBaseURL, "provider", suffix)
	if err != nil {
		return PortalSummary{}, err
	}
	ops, err := client.createPortalUser(ctx, iamBaseURL, "ops", suffix)
	if err != nil {
		return PortalSummary{}, err
	}

	buyerClient, err := newPortalClient()
	if err != nil {
		return PortalSummary{}, err
	}
	providerClient, err := newPortalClient()
	if err != nil {
		return PortalSummary{}, err
	}
	opsClient, err := newPortalClient()
	if err != nil {
		return PortalSummary{}, err
	}

	if err := loginPortal(ctx, buyerClient, webBaseURL, buyer, "/buyer", buyerPortalMarker); err != nil {
		return PortalSummary{}, fmt.Errorf("buyer login: %w", err)
	}
	if err := loginPortal(ctx, providerClient, webBaseURL, provider, "/provider", providerPortalMarker); err != nil {
		return PortalSummary{}, fmt.Errorf("provider login: %w", err)
	}
	if err := loginPortal(ctx, opsClient, webBaseURL, ops, "/ops", opsPortalMarker); err != nil {
		return PortalSummary{}, fmt.Errorf("ops login: %w", err)
	}

	buyerToken, err := bearerFromPortalClient(buyerClient, webBaseURL)
	if err != nil {
		return PortalSummary{}, err
	}
	providerToken, err := bearerFromPortalClient(providerClient, webBaseURL)
	if err != nil {
		return PortalSummary{}, err
	}
	opsToken, err := bearerFromPortalClient(opsClient, webBaseURL)
	if err != nil {
		return PortalSummary{}, err
	}

	rfqTitle := "Need live carrier triage " + suffix
	if _, err := submitForm(ctx, buyerClient, webBaseURL+"/buyer/rfqs", url.Values{
		"title":              {rfqTitle},
		"category":           {"agent-ops"},
		"scope":              {"Investigate the failure, stabilize the runtime, and summarize next steps."},
		"budgetCents":        {"4200"},
		"responseDeadlineAt": {time.Now().Add(24 * time.Hour).Format(time.RFC3339Nano)},
	}, "/buyer", buyerPortalMarker); err != nil {
		return PortalSummary{}, fmt.Errorf("create rfq: %w", err)
	}

	rfqs, err := client.listRFQs(ctx, apiBaseURL, buyerToken)
	if err != nil {
		return PortalSummary{}, err
	}
	rfq, err := findRFQByTitle(rfqs, rfqTitle)
	if err != nil {
		return PortalSummary{}, err
	}

	if _, err := submitForm(ctx, providerClient, fmt.Sprintf("%s/provider/rfqs/%s/bids", webBaseURL, rfq.ID), url.Values{
		"message":              {"Carrier-ready response, availability, and outcome."},
		"quoteCents":           {"3900"},
		"milestoneTitle":       {"Execution"},
		"milestoneBudgetCents": {"4200"},
	}, "/provider", providerPortalMarker); err != nil {
		return PortalSummary{}, fmt.Errorf("create bid: %w", err)
	}

	bids, err := client.listRFQBids(ctx, apiBaseURL, providerToken, rfq.ID)
	if err != nil {
		return PortalSummary{}, err
	}
	if len(bids) == 0 || bids[0].ID == "" {
		return PortalSummary{}, errors.New("portal smoke: missing bid after provider submit")
	}

	if _, err := submitForm(ctx, buyerClient, fmt.Sprintf("%s/buyer/rfqs/%s/award", webBaseURL, rfq.ID), url.Values{
		"bidId":        {bids[0].ID},
		"fundingMode":  {"credit"},
		"creditLineId": {"credit_1"},
	}, "/buyer", buyerPortalMarker); err != nil {
		return PortalSummary{}, fmt.Errorf("award rfq: %w", err)
	}

	rfqs, err = client.listRFQs(ctx, apiBaseURL, buyerToken)
	if err != nil {
		return PortalSummary{}, err
	}
	rfq, err = findRFQByTitle(rfqs, rfqTitle)
	if err != nil {
		return PortalSummary{}, err
	}
	if rfq.OrderID == "" {
		return PortalSummary{}, errors.New("portal smoke: awarded rfq missing order id")
	}

	if err := client.settleOrderMilestone(ctx, executionBaseURL, cfg.ExecutionEventToken, rfq.OrderID); err != nil {
		return PortalSummary{}, err
	}

	if err := client.createDispute(ctx, apiBaseURL, buyerToken, rfq.OrderID); err != nil {
		return PortalSummary{}, err
	}

	creditURL, err := submitForm(ctx, opsClient, webBaseURL+"/ops/credits/decision", url.Values{
		"completedOrders":    {"12"},
		"successfulPayments": {"11"},
		"failedPayments":     {"1"},
		"disputedOrders":     {"1"},
		"lifetimeSpendCents": {"480000"},
	}, "/ops", opsPortalMarker)
	if err != nil {
		return PortalSummary{}, fmt.Errorf("credit decision: %w", err)
	}
	creditApproved := creditURL.Query().Get("creditApproved")

	disputes, err := client.listDisputes(ctx, apiBaseURL, opsToken)
	if err != nil {
		return PortalSummary{}, err
	}
	dispute, err := findDisputeByOrder(disputes, rfq.OrderID)
	if err != nil {
		return PortalSummary{}, err
	}

	resolveURL, err := submitForm(ctx, opsClient, fmt.Sprintf("%s/ops/disputes/%s/resolve", webBaseURL, dispute.ID), url.Values{
		"resolution": {"Approved reimbursement after ops evidence review."},
	}, "/ops", opsPortalMarker)
	if err != nil {
		return PortalSummary{}, fmt.Errorf("resolve dispute: %w", err)
	}
	resolvedDisputeID := resolveURL.Query().Get("resolvedDisputeId")

	return PortalSummary{
		RFQID:             rfq.ID,
		BidID:             bids[0].ID,
		OrderID:           rfq.OrderID,
		DisputeID:         dispute.ID,
		ResolvedDisputeID: resolvedDisputeID,
		CreditApproved:    creditApproved == "true",
	}, nil
}

func newPortalClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
	}, nil
}

func loginPortal(ctx context.Context, client *http.Client, webBaseURL string, user portalAccount, nextPath, expectedBody string) error {
	_, err := submitForm(ctx, client, webBaseURL+"/auth/login", url.Values{
		"email":    {user.email},
		"password": {user.password},
		"next":     {nextPath},
	}, nextPath, expectedBody)
	return err
}

func submitForm(ctx context.Context, client *http.Client, targetURL string, values url.Values, expectedPath, expectedBody string) (*url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", res.StatusCode, targetURL)
	}
	if res.Request == nil || res.Request.URL == nil || res.Request.URL.Path != expectedPath {
		return nil, fmt.Errorf("expected final path %s, got %v", expectedPath, res.Request.URL)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 10<<20)) // 10MB max
	if err != nil {
		return nil, err
	}
	if expectedBody != "" && !strings.Contains(string(body), expectedBody) {
		return nil, fmt.Errorf("expected body to contain %q", expectedBody)
	}
	return res.Request.URL, nil
}

func bearerFromPortalClient(client *http.Client, webBaseURL string) (string, error) {
	baseURL, err := url.Parse(webBaseURL)
	if err != nil {
		return "", err
	}
	for _, cookie := range client.Jar.Cookies(baseURL) {
		if cookie.Name == portalSessionCookieName && strings.TrimSpace(cookie.Value) != "" {
			return cookie.Value, nil
		}
	}
	return "", errors.New("portal smoke: missing session cookie")
}

func (c *smokeClient) createPortalUser(ctx context.Context, iamBaseURL, kind, suffix string) (portalAccount, error) {
	user := portalAccount{
		email:    fmt.Sprintf("%s-smoke-%s@example.com", kind, suffix),
		password: "correct horse battery staple 123",
	}

	err := c.postJSON(ctx, iamBaseURL+"/v1/signup", map[string]any{
		"email":            user.email,
		"password":         user.password,
		"name":             strings.ToUpper(kind[:1]) + kind[1:] + " Smoke",
		"organizationName": strings.ToUpper(kind[:1]) + kind[1:] + " Smoke Org",
		"organizationKind": kind,
	}, nil)
	if err != nil {
		return portalAccount{}, fmt.Errorf("create %s user: %w", kind, err)
	}

	return user, nil
}

func (c *smokeClient) expectStatus(ctx context.Context, targetURL string, status int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != status {
		return fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	return nil
}

func (c *smokeClient) listRFQs(ctx context.Context, apiBaseURL, token string) ([]rfqRecord, error) {
	var response struct {
		RFQs []rfqRecord `json:"rfqs"`
	}
	if err := c.getJSON(ctx, apiBaseURL+"/api/v1/rfqs", token, &response); err != nil {
		return nil, fmt.Errorf("list rfqs: %w", err)
	}
	return response.RFQs, nil
}

func (c *smokeClient) listRFQBids(ctx context.Context, apiBaseURL, token, rfqID string) ([]bidRecord, error) {
	var response struct {
		Bids []bidRecord `json:"bids"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("%s/api/v1/rfqs/%s/bids", apiBaseURL, rfqID), token, &response); err != nil {
		return nil, fmt.Errorf("list bids: %w", err)
	}
	return response.Bids, nil
}

func (c *smokeClient) createDispute(ctx context.Context, apiBaseURL, token, orderID string) error {
	return c.postJSONWithToken(ctx, fmt.Sprintf("%s/api/v1/orders/%s/disputes", apiBaseURL, orderID), token, map[string]any{
		"milestoneId": "ms_1",
		"reason":      "Output incomplete",
		"refundCents": 900,
	}, nil)
}

func (c *smokeClient) settleOrderMilestone(ctx context.Context, executionBaseURL, eventToken, orderID string) error {
	return c.postJSONWithHeaders(ctx, fmt.Sprintf("%s/v1/carrier/events", executionBaseURL), map[string]string{
		serviceauth.HeaderName: strings.TrimSpace(eventToken),
	}, map[string]any{
		"orderId":     orderID,
		"milestoneId": "ms_1",
		"eventType":   "milestone_ready",
		"summary":     "Portal smoke milestone settled before dispute.",
		"source":      "portal_smoke",
	}, nil)
}

func (c *smokeClient) listDisputes(ctx context.Context, apiBaseURL, token string) ([]disputeRecord, error) {
	var response struct {
		Disputes []disputeRecord `json:"disputes"`
	}
	if err := c.getJSON(ctx, apiBaseURL+"/api/v1/disputes", token, &response); err != nil {
		return nil, fmt.Errorf("list disputes: %w", err)
	}
	return response.Disputes, nil
}

func (c *smokeClient) getJSON(ctx context.Context, targetURL, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return statusError{StatusCode: res.StatusCode}
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *smokeClient) postJSONWithToken(ctx context.Context, targetURL, token string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return statusError{StatusCode: res.StatusCode}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type rfqRecord struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	OrderID string `json:"orderId"`
}

type bidRecord struct {
	ID string `json:"id"`
}

type disputeRecord struct {
	ID      string `json:"id"`
	OrderID string `json:"orderId"`
}

func findRFQByTitle(rfqs []rfqRecord, title string) (rfqRecord, error) {
	for _, rfq := range rfqs {
		if rfq.Title == title {
			return rfq, nil
		}
	}
	return rfqRecord{}, errors.New("portal smoke: rfq not found")
}

func findDisputeByOrder(disputes []disputeRecord, orderID string) (disputeRecord, error) {
	for _, dispute := range disputes {
		if dispute.OrderID == orderID {
			return dispute, nil
		}
	}
	return disputeRecord{}, errors.New("portal smoke: dispute not found")
}
