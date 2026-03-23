package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/demoenv"
	"github.com/chenyu/1-tok/internal/platform"
)

var ErrDemoNotReady = errors.New("demo environment is not ready")
var ensureDemoBuyerTopUpRailFunc = ensureDemoBuyerTopUpRailLiquidity

const (
	activeCarrierBindingAlreadyExistsMessage    = "active binding already exists"
	activeSettlementBindingAlreadyExistsMessage = "active settlement binding already exists"
)

type DemoRunConfig struct {
	Demo demoenv.Config
	USDI USDIMarketplaceE2EConfig
}

type DemoRunSummary struct {
	Status        demoenv.Status `json:"status"`
	Actions       []string       `json:"actions,omitempty"`
	BuyerOrgID    string         `json:"buyerOrgId,omitempty"`
	ProviderOrgID string         `json:"providerOrgId,omitempty"`
	OpsOrgID      string         `json:"opsOrgId,omitempty"`
	BuyerEmail    string         `json:"buyerEmail,omitempty"`
	ProviderEmail string         `json:"providerEmail,omitempty"`
	OpsEmail      string         `json:"opsEmail,omitempty"`
	TopUpInvoice  string         `json:"topUpInvoice,omitempty"`
	TopUpRecordID string         `json:"topUpRecordId,omitempty"`
}

func DemoRunConfigFromEnv() DemoRunConfig {
	return DemoRunConfig{
		Demo: demoenv.ConfigFromEnv(),
		USDI: USDIMarketplaceE2EConfigFromEnv(),
	}
}

func RunDemoPrepare(ctx context.Context, cfg DemoRunConfig) (DemoRunSummary, error) {
	return runDemoReady(ctx, cfg, true)
}

func RunDemoVerify(ctx context.Context, cfg DemoRunConfig) (DemoRunSummary, error) {
	return runDemoReady(ctx, cfg, false)
}

func runDemoReady(ctx context.Context, cfg DemoRunConfig, mutate bool) (DemoRunSummary, error) {
	client := &smokeClient{httpClient: &http.Client{Timeout: usdiMarketplaceE2EHTTPTimeout}}
	summary := DemoRunSummary{
		BuyerOrgID:    cfg.Demo.Buyer.OrganizationID,
		ProviderOrgID: cfg.Demo.Provider.OrganizationID,
		OpsOrgID:      cfg.Demo.Ops.OrganizationID,
		BuyerEmail:    cfg.Demo.Buyer.Email,
		ProviderEmail: cfg.Demo.Provider.Email,
		OpsEmail:      cfg.Demo.Ops.Email,
	}

	if mutate {
		if err := client.bootstrapCarrierTarget(ctx, &cfg.USDI); err != nil {
			return summary, err
		}
	}

	buyer, changed, err := ensureDemoActor(ctx, client, cfg.Demo.IAMBaseURL, cfg.Demo.Buyer, mutate)
	if err != nil {
		summary.Status = blockedDemoStatus(cfg, "buyer actor setup failed: "+err.Error())
		return summary, ErrDemoNotReady
	}
	if changed {
		summary.Actions = append(summary.Actions, "buyer account created")
	}
	provider, changed, err := ensureDemoActor(ctx, client, cfg.Demo.IAMBaseURL, cfg.Demo.Provider, mutate)
	if err != nil {
		summary.Status = blockedDemoStatus(cfg, "provider actor setup failed: "+err.Error())
		return summary, ErrDemoNotReady
	}
	if changed {
		summary.Actions = append(summary.Actions, "provider account created")
	}
	ops, changed, err := ensureDemoActor(ctx, client, cfg.Demo.IAMBaseURL, cfg.Demo.Ops, mutate)
	if err != nil {
		summary.Status = blockedDemoStatus(cfg, "ops actor setup failed: "+err.Error())
		return summary, ErrDemoNotReady
	}
	if changed {
		summary.Actions = append(summary.Actions, "ops account created")
	}

	summary.BuyerOrgID = firstNonEmptyString(cfg.Demo.Buyer.OrganizationID, buyer.OrgID)
	summary.ProviderOrgID = firstNonEmptyString(cfg.Demo.Provider.OrganizationID, provider.OrgID)
	summary.OpsOrgID = firstNonEmptyString(cfg.Demo.Ops.OrganizationID, ops.OrgID)

	if mutate {
		if err := client.ensureProviderCarrierBinding(ctx, cfg.Demo.APIBaseURL, summary.ProviderOrgID, cfg.USDI); err != nil {
			return summary, err
		}
		if err := client.ensureProviderSettlementBinding(ctx, cfg.Demo.APIBaseURL, summary.ProviderOrgID, cfg.USDI); err != nil {
			return summary, err
		}
	}

	status, err := client.getDemoStatus(ctx, cfg.Demo.APIBaseURL, ops.Token)
	if err != nil {
		return summary, err
	}
	summary.Status = status

	if mutate && (!status.BuyerBalance.MeetsMinimumThreshold || !status.ProviderSettlement.MeetsMinimumThreshold) {
		if err := ensureDemoFNNBootstrapFunc(ctx, cfg); err != nil {
			return summary, fmt.Errorf("ensure demo fnn funding: %w", err)
		}
	}

	if mutate && !status.BuyerBalance.MeetsMinimumThreshold {
		topUpInvoice, topUpRecordID, err := client.createBuyerTopUp(ctx, cfg.Demo.SettlementBaseURL, buyer, cfg.Demo.SettlementServiceToken, cfg.Demo.BuyerTopUpAmount)
		if err != nil {
			return summary, err
		}
		summary.TopUpInvoice = topUpInvoice
		summary.TopUpRecordID = topUpRecordID
		topUpNeededCents := cfg.Demo.MinBuyerBalanceCents
		if topUpAmountCents := parseDemoAmountToCents(cfg.Demo.BuyerTopUpAmount); topUpAmountCents > topUpNeededCents {
			topUpNeededCents = topUpAmountCents
		}
		if err := ensureDemoBuyerTopUpRailFunc(ctx, cfg.USDI, topUpNeededCents); err != nil {
			return summary, fmt.Errorf("ensure buyer topup rail: %w", err)
		}
		if _, err := client.payInvoiceViaFiber(ctx, cfg.USDI, buyer.OrgID, topUpInvoice, cfg.Demo.BuyerTopUpAmount, "USDI"); err != nil {
			return summary, err
		}
		if _, err := client.waitFundingRecordState(ctx, cfg.Demo.SettlementBaseURL, buyer.Token, map[string]string{
			"buyerOrgId": summary.BuyerOrgID,
			"kind":       "buyer_topup",
		}, 90*time.Second, "SETTLED"); err != nil {
			return summary, err
		}
		summary.Actions = append(summary.Actions, "buyer top-up settled")
	}

	if mutate && !status.ProviderSettlement.MeetsMinimumThreshold {
		if _, err := client.warmDemoProviderLiquidity(ctx, cfg.Demo.APIBaseURL, ops.Token, summary.ProviderOrgID, cfg.Demo.MinProviderLiquidityCents); err != nil {
			return summary, err
		}
		summary.Actions = append(summary.Actions, "provider liquidity warmed")
	}

	status, err = client.getDemoStatus(ctx, cfg.Demo.APIBaseURL, ops.Token)
	if err != nil {
		return summary, err
	}
	summary.Status = status
	if summary.Status.Verdict != demoenv.VerdictReady {
		return summary, ErrDemoNotReady
	}
	return summary, nil
}

func ensureDemoActor(ctx context.Context, client *smokeClient, baseURL string, cfg demoenv.ActorConfig, allowSignup bool) (actorIdentity, bool, error) {
	if strings.TrimSpace(baseURL) == "" {
		return actorIdentity{OrgID: cfg.OrganizationID, Email: cfg.Email}, false, nil
	}
	actor, err := client.loginIAMUser(ctx, baseURL, cfg)
	if err == nil {
		return actor, false, validateDemoActorIdentity(cfg, actor)
	}
	if !allowSignup {
		return actorIdentity{}, false, err
	}
	actor, err = client.signupIAMUser(ctx, baseURL, cfg)
	if err != nil {
		return actorIdentity{}, false, err
	}
	return actor, true, validateDemoActorIdentity(cfg, actor)
}

func validateDemoActorIdentity(cfg demoenv.ActorConfig, actor actorIdentity) error {
	if expected := strings.TrimSpace(cfg.OrganizationID); expected != "" && expected != strings.TrimSpace(actor.OrgID) {
		return fmt.Errorf("org id mismatch: expected %s got %s", expected, actor.OrgID)
	}
	return nil
}

func blockedDemoStatus(cfg DemoRunConfig, blockers ...string) demoenv.Status {
	return demoenv.FinalizeStatus(demoenv.Status{
		ResourcePrefix: cfg.Demo.ResourcePrefix,
		BlockerReasons: blockers,
		Actors: []demoenv.ActorStatus{
			{Role: "buyer", Email: cfg.Demo.Buyer.Email, OrgID: cfg.Demo.Buyer.OrganizationID, Ready: false},
			{Role: "provider", Email: cfg.Demo.Provider.Email, OrgID: cfg.Demo.Provider.OrganizationID, Ready: false},
			{Role: "ops", Email: cfg.Demo.Ops.Email, OrgID: cfg.Demo.Ops.OrganizationID, Ready: false},
		},
		BuyerBalance: demoenv.BuyerBalanceStatus{
			BuyerOrgID:           cfg.Demo.Buyer.OrganizationID,
			MinimumRequiredCents: cfg.Demo.MinBuyerBalanceCents,
		},
		ProviderSettlement: demoenv.ProviderSettlementStatus{
			ProviderOrgID:        cfg.Demo.Provider.OrganizationID,
			MinimumRequiredCents: cfg.Demo.MinProviderLiquidityCents,
		},
	})
}

func (c *smokeClient) loginIAMUser(ctx context.Context, baseURL string, cfg demoenv.ActorConfig) (actorIdentity, error) {
	var response struct {
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/v1/sessions", map[string]any{
		"email":    cfg.Email,
		"password": cfg.Password,
	}, &response)
	if err != nil {
		return actorIdentity{}, fmt.Errorf("iam login: %w", err)
	}
	orgID, err := c.lookupIAMOrgID(ctx, baseURL, response.Session.Token, cfg.OrganizationKind)
	if err != nil {
		return actorIdentity{}, err
	}
	return actorIdentity{
		Token:    response.Session.Token,
		OrgID:    orgID,
		Email:    cfg.Email,
		Password: cfg.Password,
	}, nil
}

func (c *smokeClient) signupIAMUser(ctx context.Context, baseURL string, cfg demoenv.ActorConfig) (actorIdentity, error) {
	var response struct {
		Organization struct {
			ID string `json:"id"`
		} `json:"organization"`
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	}
	err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/v1/signup", map[string]any{
		"email":            cfg.Email,
		"password":         cfg.Password,
		"name":             cfg.Name,
		"organizationName": cfg.OrganizationName,
		"organizationKind": cfg.OrganizationKind,
	}, &response)
	if err != nil {
		return actorIdentity{}, fmt.Errorf("iam signup: %w", err)
	}
	return actorIdentity{
		Token:    response.Session.Token,
		OrgID:    response.Organization.ID,
		Email:    cfg.Email,
		Password: cfg.Password,
	}, nil
}

func (c *smokeClient) lookupIAMOrgID(ctx context.Context, baseURL, token, organizationKind string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/me", nil)
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
	if res.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("iam me status %d", res.StatusCode)
	}
	var payload struct {
		Memberships []struct {
			Organization struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			} `json:"organization"`
		} `json:"memberships"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	for _, membership := range payload.Memberships {
		if strings.EqualFold(membership.Organization.Kind, organizationKind) {
			return membership.Organization.ID, nil
		}
	}
	return "", fmt.Errorf("iam me missing %s organization", organizationKind)
}

func (c *smokeClient) getDemoStatus(ctx context.Context, baseURL, opsToken string) (demoenv.Status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/ops/demo/status", nil)
	if err != nil {
		return demoenv.Status{}, err
	}
	for key, value := range authHeaders(opsToken) {
		req.Header.Set(key, value)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return demoenv.Status{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return demoenv.Status{}, fmt.Errorf("demo status status %d", res.StatusCode)
	}
	var payload struct {
		Status demoenv.Status `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return demoenv.Status{}, err
	}
	return payload.Status, nil
}

func (c *smokeClient) warmDemoProviderLiquidity(ctx context.Context, baseURL, opsToken, providerOrgID string, minimumAvailableCents int64) (platform.ProviderLiquidityPool, error) {
	var response struct {
		Pool platform.ProviderLiquidityPool `json:"pool"`
	}
	err := c.postJSONWithHeaders(ctx, strings.TrimRight(baseURL, "/")+"/api/v1/ops/demo/warmup", authHeaders(opsToken), map[string]any{
		"providerOrgId":         providerOrgID,
		"minimumAvailableCents": minimumAvailableCents,
	}, &response)
	if err != nil {
		return platform.ProviderLiquidityPool{}, fmt.Errorf("demo warmup: %w", err)
	}
	return response.Pool, nil
}

func (c *smokeClient) ensureProviderCarrierBinding(ctx context.Context, baseURL, providerOrgID string, cfg USDIMarketplaceE2EConfig) error {
	binding, err := c.getProviderCarrierBinding(ctx, baseURL, providerOrgID)
	if err == nil && strings.EqualFold(binding.Status, "active") {
		return nil
	}
	if err != nil && !isStatusCode(err, http.StatusNotFound) {
		return err
	}
	bindingID, err := c.registerProviderCarrierBinding(ctx, baseURL, providerOrgID, cfg)
	if err != nil {
		if !errorContainsFold(err, activeCarrierBindingAlreadyExistsMessage) {
			return err
		}
		binding, getErr := c.getProviderCarrierBinding(ctx, baseURL, providerOrgID)
		if getErr != nil {
			return getErr
		}
		bindingID = binding.ID
	}
	return c.verifyProviderCarrierBinding(ctx, baseURL, bindingID)
}

func (c *smokeClient) ensureProviderSettlementBinding(ctx context.Context, baseURL, providerOrgID string, cfg USDIMarketplaceE2EConfig) error {
	binding, err := c.getProviderSettlementBinding(ctx, baseURL, providerOrgID)
	if err == nil && strings.EqualFold(binding.Status, "active") {
		return nil
	}
	if err != nil && !isStatusCode(err, http.StatusNotFound) {
		return err
	}
	_, err = c.registerProviderSettlementBinding(ctx, baseURL, providerOrgID, cfg)
	if err != nil && !errorContainsFold(err, activeSettlementBindingAlreadyExistsMessage) {
		return err
	}
	return nil
}

func (c *smokeClient) getProviderCarrierBinding(ctx context.Context, baseURL, providerOrgID string) (platform.ProviderCarrierBinding, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/carrier-bindings/"+providerOrgID, nil)
	if err != nil {
		return platform.ProviderCarrierBinding{}, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return platform.ProviderCarrierBinding{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return platform.ProviderCarrierBinding{}, statusError{StatusCode: res.StatusCode}
	}
	var payload struct {
		Binding platform.ProviderCarrierBinding `json:"binding"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return platform.ProviderCarrierBinding{}, err
	}
	return payload.Binding, nil
}

func (c *smokeClient) getProviderSettlementBinding(ctx context.Context, baseURL, providerOrgID string) (platform.ProviderSettlementBinding, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/provider-settlement-bindings/"+providerOrgID, nil)
	if err != nil {
		return platform.ProviderSettlementBinding{}, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return platform.ProviderSettlementBinding{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return platform.ProviderSettlementBinding{}, statusError{StatusCode: res.StatusCode}
	}
	var payload struct {
		Binding platform.ProviderSettlementBinding `json:"binding"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return platform.ProviderSettlementBinding{}, err
	}
	return payload.Binding, nil
}

func postJSONBytes(body any) (*bytes.Reader, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(encoded), nil
}

func errorContainsFold(err error, fragment string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(fragment))
}

func ensureDemoBuyerTopUpRailLiquidity(ctx context.Context, cfg USDIMarketplaceE2EConfig, minimumAvailableCents int64) error {
	provisioner, err := platform.NewFNNProviderSettlementProvisionerFromEnv()
	if err != nil {
		return err
	}
	if provisioner == nil {
		return nil
	}

	if strings.TrimSpace(cfg.BuyerTopUpInvoiceRPCURL) == "" {
		return errors.New("buyer topup invoice rpc url is required")
	}
	if strings.TrimSpace(cfg.BuyerTopUpInvoiceP2PHost) == "" {
		return errors.New("buyer topup invoice p2p host is required")
	}
	if strings.TrimSpace(cfg.BuyerTopUpUDTTypeScriptJSON) == "" {
		return errors.New("buyer topup udt type script json is required")
	}
	if minimumAvailableCents <= 0 {
		minimumAvailableCents = 5_000
	}

	udtTypeScript, err := parseProviderSettlementUDTTypeScriptJSON(cfg.BuyerTopUpUDTTypeScriptJSON)
	if err != nil {
		return fmt.Errorf("parse buyer topup udt type script: %w", err)
	}

	invoiceNode := newReleaseRawFNNClient(cfg.BuyerTopUpInvoiceRPCURL)
	invoiceInfo, err := invoiceNode.NodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("buyer topup invoice node info: %w", err)
	}
	peerID, err := derivePeerIDFromNodeID(invoiceInfo.NodeID)
	if err != nil {
		return fmt.Errorf("derive buyer topup invoice peer id: %w", err)
	}

	_, err = provisioner.EnsureProviderLiquidity(platform.EnsureProviderLiquidityInput{
		ProviderOrgID: marketplaceTreasuryUserID,
		Binding: platform.ProviderSettlementBinding{
			ID:            "demo_topup_rail",
			ProviderOrgID: marketplaceTreasuryUserID,
			Asset:         "USDI",
			PeerID:        peerID,
			P2PAddress:    multiaddrForP2P(cfg.BuyerTopUpInvoiceP2PHost, cfg.BuyerTopUpInvoiceP2PPort, peerID),
			NodeRPCURL:    cfg.BuyerTopUpInvoiceRPCURL,
			UDTTypeScript: platform.UDTTypeScript{
				CodeHash: udtTypeScript.CodeHash,
				HashType: udtTypeScript.HashType,
				Args:     udtTypeScript.Args,
			},
			Status: "active",
		},
		NeededReserveCents: minimumAvailableCents,
		CurrentPool: platform.ProviderLiquidityPool{
			ProviderSettlementBindingID: "demo_topup_rail",
			ProviderOrgID:               marketplaceTreasuryUserID,
			Asset:                       "USDI",
			Status:                      platform.ProviderLiquidityPoolStatusHealthy,
			TotalSpendableCents:         minimumAvailableCents,
			AvailableToAllocateCents:    minimumAvailableCents,
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func parseDemoAmountToCents(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int64(parsed * 100)
}
