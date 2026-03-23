package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/demoenv"
	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/release"
)

const demoStatusHTTPTimeout = 5 * time.Second

type settlementFundingRecord struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	BuyerOrgID string `json:"buyerOrgId"`
	Amount     string `json:"amount"`
	State      string `json:"state"`
}

type demoResolvedActors struct {
	BuyerOrgID    string
	ProviderOrgID string
	OpsOrgID      string
	Actors        []demoenv.ActorStatus
	Blockers      []string
}

func (s *Server) handleDemoStatus(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	status := s.buildDemoStatus(r.Context(), r.Header.Get("Authorization"))
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (s *Server) handleDemoPrepare(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	summary, err := s.demoPrepare(r.Context())
	if err == nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"summary": summary})
		return
	}

	statusCode := http.StatusBadGateway
	if errors.Is(err, release.ErrDemoNotReady) {
		statusCode = http.StatusConflict
	}
	httputil.WriteJSON(w, statusCode, map[string]any{
		"error":   err.Error(),
		"summary": summary,
	})
}

func (s *Server) handleDemoWarmup(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	var payload struct {
		ProviderOrgID         string `json:"providerOrgId"`
		MinimumAvailableCents int64  `json:"minimumAvailableCents"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "invalid json")
			return
		}
	}
	if strings.TrimSpace(payload.ProviderOrgID) == "" {
		payload.ProviderOrgID = s.resolveDemoActors(r.Context()).ProviderOrgID
	}
	if payload.MinimumAvailableCents <= 0 {
		payload.MinimumAvailableCents = s.demoConfig.MinProviderLiquidityCents
	}
	if strings.TrimSpace(payload.ProviderOrgID) == "" {
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "providerOrgId is required")
		return
	}

	pool, err := s.app.WarmProviderSettlementPool(payload.ProviderOrgID, payload.MinimumAvailableCents)
	if err != nil {
		statusCode := http.StatusBadRequest
		switch {
		case errors.Is(err, platform.ErrProviderSettlementPoolUnavailable),
			errors.Is(err, platform.ErrProviderSettlementProvisionerMissing):
			statusCode = http.StatusConflict
		}
		httputil.WriteJSON(w, statusCode, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"pool": pool})
}

func (s *Server) buildDemoStatus(ctx context.Context, authHeader string) demoenv.Status {
	actors := s.resolveDemoActors(ctx)
	status := demoenv.Status{
		ResourcePrefix: s.demoConfig.ResourcePrefix,
		Services: []demoenv.ServiceStatus{
			{
				ID:      "api-gateway",
				Label:   "API Gateway",
				BaseURL: s.demoConfig.APIBaseURL,
				Healthy: true,
				Detail:  "current request reached api-gateway",
			},
		},
		Actors: actors.Actors,
		BuyerBalance: demoenv.BuyerBalanceStatus{
			BuyerOrgID:           actors.BuyerOrgID,
			MinimumRequiredCents: s.demoConfig.MinBuyerBalanceCents,
		},
		ProviderSettlement: demoenv.ProviderSettlementStatus{
			ProviderOrgID:        actors.ProviderOrgID,
			MinimumRequiredCents: s.demoConfig.MinProviderLiquidityCents,
		},
	}

	status.Services = append(status.Services, s.checkHealth(ctx, "settlement", "Settlement", s.demoConfig.SettlementBaseURL))
	status.Services = append(status.Services, s.checkHealth(ctx, "execution", "Execution", s.demoConfig.ExecutionBaseURL))
	status.Services = append(status.Services, s.checkHealth(ctx, "carrier", "Carrier", s.demoConfig.CarrierBaseURL))
	status.Services = append(status.Services, s.checkHealth(ctx, "fiber-adapter", "Fiber Adapter", s.demoConfig.FiberAdapterBaseURL))
	if strings.TrimSpace(s.demoConfig.IAMBaseURL) != "" {
		status.Services = append(status.Services, s.checkHealth(ctx, "iam", "IAM", s.demoConfig.IAMBaseURL))
	}
	status.BlockerReasons = append(status.BlockerReasons, actors.Blockers...)

	if strings.TrimSpace(actors.BuyerOrgID) == "" {
		status.BlockerReasons = append(status.BlockerReasons, "buyer demo actor could not be resolved")
	} else {
		balance, blockers := s.loadBuyerBalanceStatus(ctx, authHeader, actors.BuyerOrgID)
		status.BuyerBalance = balance
		status.BlockerReasons = append(status.BlockerReasons, blockers...)
	}

	if strings.TrimSpace(actors.ProviderOrgID) == "" {
		status.BlockerReasons = append(status.BlockerReasons, "provider demo actor could not be resolved")
	} else {
		providerStatus, blockers := s.loadProviderSettlementStatus(actors.ProviderOrgID)
		status.ProviderSettlement = providerStatus
		status.BlockerReasons = append(status.BlockerReasons, blockers...)
	}

	return demoenv.FinalizeStatus(status)
}

func (s *Server) checkHealth(ctx context.Context, id, label, baseURL string) demoenv.ServiceStatus {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return demoenv.ServiceStatus{
			ID:      id,
			Label:   label,
			BaseURL: baseURL,
			Healthy: false,
			Detail:  "base url is not configured",
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/healthz", nil)
	if err != nil {
		return demoenv.ServiceStatus{ID: id, Label: label, BaseURL: baseURL, Healthy: false, Detail: err.Error()}
	}
	client := &http.Client{Timeout: demoStatusHTTPTimeout}
	res, err := client.Do(req)
	if err != nil {
		return demoenv.ServiceStatus{ID: id, Label: label, BaseURL: baseURL, Healthy: false, Detail: err.Error()}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return demoenv.ServiceStatus{
			ID:      id,
			Label:   label,
			BaseURL: baseURL,
			Healthy: false,
			Detail:  fmt.Sprintf("unexpected status %d", res.StatusCode),
		}
	}
	return demoenv.ServiceStatus{ID: id, Label: label, BaseURL: baseURL, Healthy: true}
}

func (s *Server) loadBuyerBalanceStatus(ctx context.Context, authHeader, buyerOrgID string) (demoenv.BuyerBalanceStatus, []string) {
	status := demoenv.BuyerBalanceStatus{
		BuyerOrgID:           buyerOrgID,
		MinimumRequiredCents: s.demoConfig.MinBuyerBalanceCents,
	}
	if strings.TrimSpace(s.demoConfig.SettlementBaseURL) == "" {
		return status, []string{"DEMO_SETTLEMENT_BASE_URL is not configured"}
	}
	records, err := fetchSettlementFundingRecords(ctx, s.demoConfig.SettlementBaseURL, authHeader, map[string]string{
		"buyerOrgId": buyerOrgID,
		"kind":       "buyer_topup",
	})
	if err != nil {
		return status, []string{fmt.Sprintf("load buyer topups: %v", err)}
	}
	for _, record := range records {
		if record.Kind != "buyer_topup" {
			continue
		}
		if strings.EqualFold(record.State, "SETTLED") {
			status.SettledTopUpCount++
			status.SettledTopUpCents += parseAmountToCents(record.Amount)
			continue
		}
		status.PendingTopUpCount++
	}
	status.MeetsMinimumThreshold = status.SettledTopUpCents >= status.MinimumRequiredCents
	return status, nil
}

func (s *Server) loadProviderSettlementStatus(providerOrgID string) (demoenv.ProviderSettlementStatus, []string) {
	status := demoenv.ProviderSettlementStatus{
		ProviderOrgID:        providerOrgID,
		MinimumRequiredCents: s.demoConfig.MinProviderLiquidityCents,
	}
	blockers := make([]string, 0, 3)

	carrierBinding, err := s.app.GetProviderCarrierBinding(providerOrgID)
	if err != nil {
		blockers = append(blockers, "provider carrier binding is missing")
	} else {
		status.CarrierBindingStatus = carrierBinding.Status
	}

	settlementBinding, err := s.app.GetProviderSettlementBinding(providerOrgID)
	if err != nil {
		blockers = append(blockers, "provider settlement binding is missing")
	} else {
		status.SettlementBindingStatus = settlementBinding.Status
	}

	pool, err := s.app.GetProviderSettlementPool(providerOrgID)
	if err != nil {
		blockers = append(blockers, "provider liquidity pool is missing")
		return status, blockers
	}
	status.PoolStatus = string(pool.Status)
	status.ReadyChannelCount = pool.ReadyChannelCount
	status.AvailableToAllocateCents = pool.AvailableToAllocateCents
	status.ReservedOutstandingCents = pool.ReservedOutstandingCents
	status.MeetsMinimumThreshold =
		status.CarrierBindingStatus == "active" &&
			status.SettlementBindingStatus == "active" &&
			pool.Status == platform.ProviderLiquidityPoolStatusHealthy &&
			pool.AvailableToAllocateCents >= status.MinimumRequiredCents
	return status, blockers
}

func (s *Server) resolveDemoActors(ctx context.Context) demoResolvedActors {
	buyerOrgID, buyerStatus, buyerBlockers := s.resolveDemoActor(ctx, s.demoConfig.Buyer)
	providerOrgID, providerStatus, providerBlockers := s.resolveDemoActor(ctx, s.demoConfig.Provider)
	opsOrgID, opsStatus, opsBlockers := s.resolveDemoActor(ctx, s.demoConfig.Ops)
	return demoResolvedActors{
		BuyerOrgID:    buyerOrgID,
		ProviderOrgID: providerOrgID,
		OpsOrgID:      opsOrgID,
		Actors:        []demoenv.ActorStatus{buyerStatus, providerStatus, opsStatus},
		Blockers: append(
			append(buyerBlockers, providerBlockers...),
			opsBlockers...,
		),
	}
}

func (s *Server) resolveDemoActor(ctx context.Context, cfg demoenv.ActorConfig) (string, demoenv.ActorStatus, []string) {
	role := strings.TrimSpace(cfg.OrganizationKind)
	if role == "" {
		role = "unknown"
	}
	status := demoenv.ActorStatus{
		Role:  role,
		Email: cfg.Email,
		OrgID: strings.TrimSpace(cfg.OrganizationID),
	}
	if status.OrgID != "" {
		status.Ready = true
		status.Detail = "configured from demo environment"
		return status.OrgID, status, nil
	}
	if strings.TrimSpace(s.demoConfig.IAMBaseURL) == "" {
		status.Detail = "iam base url is not configured"
		return "", status, []string{"DEMO_IAM_BASE_URL is not configured"}
	}
	if strings.TrimSpace(cfg.Email) == "" || strings.TrimSpace(cfg.Password) == "" {
		status.Detail = "email or password is not configured"
		return "", status, []string{role + " demo credentials are not configured"}
	}

	orgID, err := fetchDemoActorOrgID(ctx, s.demoConfig.IAMBaseURL, cfg)
	if err != nil {
		status.Detail = err.Error()
		return "", status, []string{role + " actor lookup failed: " + err.Error()}
	}
	status.OrgID = orgID
	status.Ready = true
	status.Detail = "resolved via IAM login"
	return orgID, status, nil
}

func fetchDemoActorOrgID(ctx context.Context, baseURL string, cfg demoenv.ActorConfig) (string, error) {
	loginPayload, err := json.Marshal(map[string]string{
		"email":    cfg.Email,
		"password": cfg.Password,
	})
	if err != nil {
		return "", err
	}
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/sessions", bytes.NewReader(loginPayload))
	if err != nil {
		return "", err
	}
	loginReq.Header.Set("Accept", "application/json")
	loginReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: demoStatusHTTPTimeout}
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return "", err
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("iam session status %d", loginRes.StatusCode)
	}
	var loginBody struct {
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	}
	if err := json.NewDecoder(loginRes.Body).Decode(&loginBody); err != nil {
		return "", err
	}
	if strings.TrimSpace(loginBody.Session.Token) == "" {
		return "", errors.New("iam session token missing")
	}

	meReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/me", nil)
	if err != nil {
		return "", err
	}
	meReq.Header.Set("Accept", "application/json")
	meReq.Header.Set("Authorization", "Bearer "+loginBody.Session.Token)
	meRes, err := client.Do(meReq)
	if err != nil {
		return "", err
	}
	defer meRes.Body.Close()
	if meRes.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("iam me status %d", meRes.StatusCode)
	}
	var meBody struct {
		Memberships []struct {
			Organization struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			} `json:"organization"`
		} `json:"memberships"`
	}
	if err := json.NewDecoder(meRes.Body).Decode(&meBody); err != nil {
		return "", err
	}
	for _, membership := range meBody.Memberships {
		if membership.Organization.Kind == cfg.OrganizationKind && strings.TrimSpace(membership.Organization.ID) != "" {
			return membership.Organization.ID, nil
		}
	}
	return "", fmt.Errorf("no %s membership found", cfg.OrganizationKind)
}

func fetchSettlementFundingRecords(ctx context.Context, baseURL, authHeader string, filters map[string]string) ([]settlementFundingRecord, error) {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/v1/funding-records")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	for key, value := range filters {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(authHeader) != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{Timeout: demoStatusHTTPTimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}

	var payload struct {
		Records []settlementFundingRecord `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Records, nil
}

func parseAmountToCents(raw string) int64 {
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
